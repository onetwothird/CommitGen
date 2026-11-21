package main

// Commands that work on commits that already exist: pr, changelog, reword.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/onetwothird/CommitGen/internal/cache"
	"github.com/onetwothird/CommitGen/internal/changelog"
	"github.com/onetwothird/CommitGen/internal/git"
	"github.com/onetwothird/CommitGen/internal/heuristic"
	"github.com/onetwothird/CommitGen/internal/llm"
	"github.com/onetwothird/CommitGen/internal/prompt"
)

// text asks the provider chain for a free-form reply (not a commit message),
// reusing a cached reply for the same request.
func (s *session) text(kind string, req llm.Request) (string, error) {
	key := cache.Key(kind, describe(s.targets), req.System, req.User)
	if t, ok := s.cache.Get(key); ok && !s.f.noCache {
		warn("reused the text made earlier for these exact changes (no API call; --no-cache for a new one)")
		return t, nil
	}
	stop := spinner("generating with " + s.label())
	raw, err := s.p.Generate(s.ctx, req, nil)
	stop()
	if err != nil {
		return "", err
	}
	t := prompt.CleanText(raw)
	if t == "" {
		return "", errors.New("the model returned an empty reply")
	}
	if s.p.Used() != "offline" {
		s.cache.Put(key, t)
	} else if len(s.targets) > 1 && s.targets[0].name != "offline" {
		warn("used the offline generator (no AI) - review the text before using it")
	}
	return t, nil
}

func noArgs(cmd string, pos []string, usage string) error {
	if len(pos) > 0 {
		return fmt.Errorf("unexpected argument %q\n  usage: CommitGen %s %s", pos[0], cmd, usage)
	}
	return nil
}

// ---- pr ----

func prCmd(args []string) error {
	f, pos := parseFlags(args)
	if err := noArgs("pr", pos, "[--base main] [--create] [-m hint]"); err != nil {
		return err
	}
	if _, err := git.RepoRoot(); err != nil {
		return err
	}
	s, cancel, err := newSession(f)
	if err != nil {
		return err
	}
	defer cancel()

	base := f.base
	if base == "" {
		if base = git.DefaultBranch(); base == "" {
			return errors.New("can't tell which branch the pull request goes into; pass --base <branch>")
		}
	}
	if !git.Verify(base) {
		return fmt.Errorf("base branch %q not found; pass --base <branch> (and run `git fetch` if it's on the remote)", base)
	}
	mb, err := git.MergeBase(base, "HEAD")
	if err != nil {
		return fmt.Errorf("%s and this branch have no common history: %w", base, err)
	}
	commits, err := git.Log(mb+"..HEAD", "--no-merges")
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		return fmt.Errorf("this branch has no commits that aren't in %s yet; commit your work first", base)
	}
	diff, err := git.Diff(mb, "HEAD", s.cfg.Exclude)
	if err != nil {
		return err
	}
	stat, _ := git.DiffStat(mb, "HEAD")
	diff = s.prepare(diff, "the commits on this branch")

	lines := make([]string, len(commits))
	for i, c := range commits {
		lines[i] = c.Short + " " + c.Subject
	}
	in := prompt.PRInput{
		Base: base, Branch: s.branch, Commits: lines, Stat: stat, Diff: diff,
		Style: s.cfg.Style, Language: s.cfg.Language, Hint: f.hint, MaxDiff: s.cfg.MaxDiff,
	}
	req := llm.Request{
		System: prompt.PRSystem(in), User: prompt.PRUser(in),
		Temperature: s.cfg.Temperature, MaxTokens: 1500, Diff: diff, Style: s.cfg.Style,
		Offline: func() string { return offlinePR(commits, diff, stat, s) },
	}
	if f.dryRun {
		fmt.Printf("providers (in order): %s\n\n=== system ===\n%s\n=== user ===\n%s", describe(s.targets), req.System, req.User)
		return nil
	}
	text, err := s.text("pr-v1", req)
	if err != nil {
		return err
	}
	title, body := prompt.SplitTitle(text)
	if title == "" {
		return errors.New("the reply had no title; try again or use another provider")
	}
	fmt.Printf("%s\n\n%s\n", title, body)

	if !f.create {
		if isTTY(os.Stderr) {
			fmt.Fprintln(os.Stderr, "\nCommitGen: tip: `CommitGen pr --create` opens this pull request with the GitHub CLI")
		}
		return nil
	}
	gh, err := exec.LookPath("gh")
	if err != nil {
		return errors.New("--create needs the GitHub CLI: install it from https://cli.github.com and run `gh auth login`")
	}
	if !f.yes {
		if !isTTY(os.Stdin) {
			return errors.New("not creating the pull request without confirmation; add -y")
		}
		if ask("\ncreate this pull request with gh? [y]es [n]o", "y") != "y" {
			return nil
		}
	}
	tmp, err := os.CreateTemp("", "CommitGen-pr-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(body + "\n")
	tmp.Close()
	cmd := exec.Command(gh, "pr", "create", "--title", title, "--body-file", tmp.Name(),
		"--base", strings.TrimPrefix(base, "origin/"))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// offlinePR writes a pull request description without AI, from the commit
// subjects and the diff.
func offlinePR(commits []git.Commit, diff, stat string, s *session) string {
	title := commits[0].Subject
	if len(commits) > 1 {
		title, _, _ = strings.Cut(heuristic.Generate(diff, heuristic.Options{
			Style: s.cfg.Style, Scopes: s.cfg.Scopes, NoBreaking: !s.cfg.Breaking,
		}), "\n")
	}
	if title == "" {
		title = s.branch
	}
	var b strings.Builder
	b.WriteString(title + "\n\n## Summary\n\n")
	summary := fmt.Sprintf("%d commit(s)", len(commits))
	if st := strings.Split(strings.TrimSpace(stat), "\n"); len(st) > 0 && st[len(st)-1] != "" {
		summary += ", " + strings.TrimSpace(st[len(st)-1])
	}
	b.WriteString(summary + ".\n\n## Changes\n\n")
	var notes []string
	for _, c := range commits {
		fmt.Fprintf(&b, "- %s (%s)\n", c.Subject, c.Short)
		if m := regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE: (.+)$`).FindStringSubmatch(c.Body); m != nil {
			notes = append(notes, m[1])
		} else if regexp.MustCompile(`^\w+(\([^)]*\))?!: `).MatchString(c.Subject) {
			notes = append(notes, c.Subject)
		}
	}
	b.WriteString("\n## Testing\n\n- [ ] Describe how you tested this\n")
	if len(notes) > 0 {
		b.WriteString("\n## Notes\n\nBreaking changes:\n\n")
		for _, n := range notes {
			b.WriteString("- " + n + "\n")
		}
	}
	return b.String()
}

// ---- changelog ----

func changelogCmd(args []string) error {
	f, pos := parseFlags(args)
	if err := noArgs("changelog", pos, "[--from v1.0.0] [--to HEAD] [--release v1.1.0] [--full]"); err != nil {
		return err
	}
	if _, err := git.RepoRoot(); err != nil {
		return err
	}
	to := f.to
	if to == "" {
		to = "HEAD"
	}
	if !git.Verify(to) {
		return fmt.Errorf("--to %q is not a commit, branch or tag", to)
	}
	heading := f.release
	toTag := git.ExactTag(to)
	from := f.from
	if from == "" {
		from = git.LatestTag(to)
		if from != "" && from == toTag {
			// HEAD is the release itself: show what changed since the tag before it.
			from = git.LatestTag(to + "^")
		}
	} else if !git.Verify(from) {
		return fmt.Errorf("--from %q is not a commit, branch or tag", from)
	}
	if heading == "" {
		heading = toTag
	}
	rangeSpec := to
	if from != "" {
		rangeSpec = from + ".." + to
	}
	commits, err := git.Log(rangeSpec, "--no-merges")
	if err != nil {
		return err
	}
	entries := make([]changelog.Commit, len(commits))
	for i, c := range commits {
		entries[i] = changelog.Commit{Short: c.Short, Subject: c.Subject, Body: c.Body}
	}
	date := ""
	if heading != "" {
		date = git.CommitDate(to)
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
	}
	out, hidden := changelog.Render(entries, changelog.Options{Heading: heading, Date: date, Full: f.full})
	fmt.Print(out)
	what := "the start of history"
	if from != "" {
		what = from
	}
	note := fmt.Sprintf("%d commit(s) since %s", len(commits), what)
	if hidden > 0 {
		note += fmt.Sprintf("; %d refactor/docs/test/chore commit(s) not shown (--full lists them)", hidden)
	}
	fmt.Fprintln(os.Stderr, "\nCommitGen: "+note)
	return nil
}

// ---- reword ----

// lazyRe matches messages that say nothing about the change.
var lazyRe = regexp.MustCompile(`(?i)^(wip|fix|fixes|fixed|fix stuff|update|updates|updated|changes|change|stuff|misc|test|tests|tmp|temp|asdf|minor|cleanup|commit|save|done|work|progress|\.+|-+)\.?$`)

func rewordCmd(args []string) error {
	f, pos := parseFlags(args)
	n := 0
	if len(pos) > 1 {
		return fmt.Errorf("unexpected argument %q\n  usage: CommitGen reword [N] [-y] [--full] [--force]", pos[1])
	}
	if len(pos) == 1 {
		var err error
		if n, err = strconv.Atoi(pos[0]); err != nil || n < 1 {
			return fmt.Errorf("%q is not a number of commits (usage: CommitGen reword 3)", pos[0])
		}
	}
	if _, err := git.RepoRoot(); err != nil {
		return err
	}
	if busy := git.Busy(); busy != "" {
		return fmt.Errorf("%s is in progress; finish or abort it first", busy)
	}
	head, err := git.RevParse("HEAD")
	if err != nil {
		return errors.New("this repository has no commits yet")
	}

	up := git.Upstream()
	var commits []git.Commit
	switch {
	case n > 0:
		commits, err = git.Log("HEAD", "--first-parent", "-n", strconv.Itoa(n))
	case up != "":
		commits, err = git.Log(up + "..HEAD")
		if err == nil && len(commits) == 0 {
			return fmt.Errorf("every commit on this branch is already in %s.\n  To reword pushed commits, give a number: `CommitGen reword 3 --force`", up)
		}
	default:
		return errors.New("this branch has no upstream, so CommitGen can't tell which commits are new.\n" +
			"  Give the number of commits to look at, e.g. `CommitGen reword 3`")
	}
	if err != nil {
		return err
	}
	pushed := 0
	for _, c := range commits {
		if len(c.Parents) > 1 {
			return fmt.Errorf("%s is a merge commit; CommitGen can only reword a straight line of commits (try a smaller number)", c.Short)
		}
		if up != "" && git.IsAncestor(c.Hash, up) || up == "" && git.OnRemote(c.Hash) {
			pushed++
		}
	}
	if pushed > 0 && !f.force {
		return fmt.Errorf("%d of these commits are already pushed. Rewording rewrites history, so you'd have to\n"+
			"  `git push --force-with-lease` afterwards, which affects anyone who pulled them.\n"+
			"  Run again with --force if that's OK, or reword fewer commits", pushed)
	}

	s, cancel, err := newSession(f)
	if err != nil {
		return err
	}
	defer cancel()

	msgs := make([]string, len(commits))
	changed := 0
	fmt.Println()
	for i, c := range commits {
		msgs[i] = c.Message
		if !f.full && s.valid(c.Message) && !lazyRe.MatchString(c.Subject) {
			fmt.Printf("  %s  %s  (kept)\n", c.Short, c.Subject)
			continue
		}
		diff, err := git.Show(c.Hash, s.cfg.Exclude)
		if err != nil {
			return err
		}
		if strings.TrimSpace(diff) == "" {
			fmt.Printf("  %s  %s  (kept: no reviewable changes)\n", c.Short, c.Subject)
			continue
		}
		stat, _ := git.ShowStat(c.Hash)
		diff = s.prepare(diff, "commit "+c.Short)
		out, err := s.generate(s.buildRequest(diff, stat, c.Message), stat, 1, false)
		if err != nil {
			return fmt.Errorf("stopped at %s, nothing was changed: %w", c.Short, err)
		}
		if strings.TrimSpace(out[0]) == strings.TrimSpace(c.Message) {
			fmt.Printf("  %s  %s  (kept)\n", c.Short, c.Subject)
			continue
		}
		msgs[i] = out[0]
		changed++
		fmt.Printf("  %s  %s\n     -> %s\n", c.Short, c.Subject, indentBy(out[0], "        "))
	}
	fmt.Println()
	if changed == 0 {
		fmt.Fprintln(os.Stderr, "CommitGen: nothing to reword; every message already looks fine (--full rewrites them anyway)")
		return nil
	}
	if f.dryRun || f.print {
		fmt.Fprintln(os.Stderr, "CommitGen: preview only, no commits were changed")
		return nil
	}
	if !f.yes {
		if !isTTY(os.Stdin) {
			return errors.New("not rewriting commits without confirmation; add -y")
		}
		if ask(fmt.Sprintf("rewrite %d commit message(s)? [y]es [n]o", changed), "n") != "y" {
			return nil
		}
	}
	for _, c := range commits {
		if git.Signed(c.Hash) {
			warn("some of these commits are signed; the rewritten copies won't be (sign again with `git rebase --exec 'git commit --amend --no-edit -S'`)")
			break
		}
	}

	parents := commits[0].Parents
	newHead := ""
	rewritten := false
	for i, c := range commits {
		if !rewritten && msgs[i] == c.Message {
			newHead = c.Hash // unchanged and its parent is unchanged: keep it as is
		} else {
			rewritten = true
			if newHead, err = git.Recommit(c.Hash, msgs[i], parents); err != nil {
				return fmt.Errorf("rewriting %s failed, nothing was changed: %w", c.Short, err)
			}
		}
		parents = []string{newHead}
	}
	if err := git.UpdateHead(newHead, head, "CommitGen reword"); err != nil {
		return fmt.Errorf("couldn't move the branch, nothing was changed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "CommitGen: reworded %d commit(s). Undo with: git reset --soft ORIG_HEAD\n", changed)
	if pushed > 0 {
		fmt.Fprintln(os.Stderr, "  These commits were pushed before: update the remote with `git push --force-with-lease`.")
	}
	return nil
}

// ---- cache ----

func cacheCmd(args []string) error {
	sub := "path"
	if len(args) > 0 {
		sub = args[0]
	}
	c := cache.Default()
	if c == nil {
		return errors.New("this system has no cache directory")
	}
	switch sub {
	case "path":
		fmt.Println(c.Dir)
	case "clear":
		n, err := c.Clear()
		if err != nil {
			return err
		}
		fmt.Printf("removed %d saved message(s) from %s\n", n, c.Dir)
	default:
		return fmt.Errorf("unknown cache command %q (use: CommitGen cache clear|path)", sub)
	}
	return nil
}

// indentBy indents every line after the first, leaving blank lines empty.
func indentBy(s, pad string) string {
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = pad + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}
