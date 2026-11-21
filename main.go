package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onetwothird/CommitGen/internal/config"
	"github.com/onetwothird/CommitGen/internal/git"
	"github.com/onetwothird/CommitGen/internal/llm"
	"github.com/onetwothird/CommitGen/internal/prompt"
	"github.com/onetwothird/CommitGen/internal/redact"
)

var version = "dev"

const usage = `CommitGen — write commit messages from your staged diff

Usage:
  CommitGen [flags]            generate a message for the staged changes
  CommitGen setup [provider]   pick an AI provider (free options) - start here
  CommitGen providers          list providers, costs and which keys are set
  CommitGen --offline          no AI, no key, no internet: rule-based message
  CommitGen -a --each          commit every changed file separately, automatically
  CommitGen hook install       draft messages automatically on ` + "`git commit`" + `
  CommitGen hook uninstall
  CommitGen config init        write a starter config file
  CommitGen config path|show
  CommitGen completion <shell> tab completion for bash, zsh, fish or powershell
  CommitGen version

Examples:
  git add -A && CommitGen      draft, then [c]ommit / [e]dit / [r]egenerate
  CommitGen -c -y              generate and commit in one go
  CommitGen -m "fixes #42"     give the model (or offline mode) a hint
  CommitGen --provider gemini  use another provider just this once

Flags:
`

type flags struct {
	commit, yes, print, dryRun, noRedact, verbose bool
	offline                                       bool
	each, all                                     bool
	n                                             int
	hint, model, url, provider, style, lang, hook string
	maxDiff                                       int
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "hook":
			exit(hookCmd(args[1:]))
		case "config":
			exit(configCmd(args[1:]))
		case "setup", "init":
			exit(setupCmd(args[1:]))
		case "providers", "provider":
			exit(providersCmd())
		case "completion", "completions":
			exit(completionCmd(args[1:]))
		case "version":
			fmt.Println("CommitGen", version)
			return
		case "help":
			args = []string{"-h"}
		default:
			exit(fmt.Errorf("unknown command %q (see CommitGen -h)", args[0]))
		}
	}

	var f flags
	newFlagSet(&f).Parse(args)
	exit(generateCmd(f))
}

// newFlagSet defines every flag. The completion scripts are tested against
// this list, so new flags must be added there too.
func newFlagSet(f *flags) *flag.FlagSet {
	fs := flag.NewFlagSet("CommitGen", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage); fs.PrintDefaults() }
	fs.BoolVar(&f.commit, "c", false, "commit with the generated message (opens your editor to review)")
	fs.BoolVar(&f.yes, "y", false, "with -c: commit immediately, skip the editor")
	fs.BoolVar(&f.print, "p", false, "print the message only (no interactive menu); good for scripts")
	fs.BoolVar(&f.dryRun, "dry-run", false, "show exactly what would be sent to the model, then exit")
	fs.BoolVar(&f.noRedact, "no-redact", false, "do not mask secrets in the diff")
	fs.BoolVar(&f.verbose, "v", false, "verbose output")
	fs.IntVar(&f.n, "n", 1, "number of candidate messages to generate and pick from")
	fs.StringVar(&f.hint, "m", "", "extra context for the model, e.g. -m \"fixes login bug #42\"")
	fs.StringVar(&f.model, "model", "", "model name (overrides config)")
	fs.StringVar(&f.url, "url", "", "OpenAI-compatible base URL (overrides config)")
	fs.StringVar(&f.provider, "provider", "", "provider for this run: groq, gemini, openrouter, ollama, offline, ... (see `CommitGen providers`)")
	fs.BoolVar(&f.offline, "offline", false, "no AI: build the message from the diff with rules (no key, no internet)")
	fs.StringVar(&f.style, "style", "", "conventional | simple (overrides config)")
	fs.StringVar(&f.lang, "lang", "", "language of the message (overrides config)")
	fs.IntVar(&f.maxDiff, "max", 0, "max diff characters sent to the model (overrides config)")
	fs.BoolVar(&f.each, "each", false, "commit every staged file separately, each with its own message (automatic)")
	fs.BoolVar(&f.all, "a", false, "stage all changes first (like git add -A)")
	fs.StringVar(&f.hook, "hook", "", "internal: write message into this file (used by the git hook)")
	return fs
}

func exit(err error) {
	if err = friendly(err); err != nil {
		fmt.Fprintln(os.Stderr, "CommitGen:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func loadConfig(f flags) (config.Config, string, error) {
	root, err := git.RepoRoot()
	if err != nil {
		return config.Config{}, "", err
	}
	c, err := config.Load(root)
	if err != nil {
		return c, root, err
	}
	if f.offline {
		f.provider = "offline"
	}
	if f.provider != "" && !strings.EqualFold(f.provider, c.Provider) {
		// A one-off provider replaces the configured one, with its own defaults.
		c.BaseURL, c.Model, c.APIKey = "", "", ""
		if strings.EqualFold(f.provider, "offline") {
			c.Fallback = nil
		}
	}
	if f.model != "" {
		c.Model = f.model
	}
	if f.url != "" {
		c.BaseURL = f.url
	}
	if f.provider != "" {
		c.Provider = f.provider
	}
	if c.Timeout <= 0 {
		c.Timeout = 60
	}
	if f.style != "" {
		c.Style = f.style
	}
	if f.lang != "" {
		c.Language = f.lang
	}
	if f.maxDiff > 0 {
		c.MaxDiff = f.maxDiff
	}
	return c, root, nil
}

type session struct {
	cfg     config.Config
	f       flags
	p       *llm.Chain
	targets []target
	ctx     context.Context
}

func (s *session) request() (llm.Request, string, error) {
	diff, err := git.StagedDiff(s.cfg.Exclude)
	if err != nil {
		return llm.Request{}, "", err
	}
	stat, _ := git.StagedStat()
	if !s.f.noRedact {
		var n int
		diff, n = redact.Secrets(diff)
		if n > 0 {
			fmt.Fprintf(os.Stderr, "CommitGen: masked %d possible secret(s) before sending\n", n)
		}
	}
	in := prompt.Input{
		Diff: diff, Stat: stat, Branch: git.Branch(), History: git.RecentSubjects(s.cfg.History),
		Style: s.cfg.Style, Language: s.cfg.Language, Scopes: s.cfg.Scopes,
		Instructions: s.cfg.Instructions, Hint: s.f.hint, MaxDiff: s.cfg.MaxDiff,
	}
	if s.f.verbose {
		fmt.Fprintf(os.Stderr, "CommitGen: providers: %s (%d diff chars)\n", describe(s.targets), len(diff))
	}
	return llm.Request{
		System: prompt.System(in), User: prompt.User(in),
		Temperature: s.cfg.Temperature, MaxTokens: 400,
		Diff: diff, Hint: s.f.hint, Style: s.cfg.Style, Scopes: s.cfg.Scopes,
	}, stat, nil
}

func (s *session) generate(req llm.Request, stat string, n int) ([]string, error) {
	msgs, err := generateN(s.ctx, s.p, req, n, s.label())
	if err != nil {
		return nil, err
	}
	for i, m := range msgs {
		if prompt.Valid(m, s.cfg.Style) {
			continue
		}
		if s.f.verbose {
			fmt.Fprintf(os.Stderr, "CommitGen: model reply was not a commit message, asking it to rewrite:\n%s\n", m)
		}
		fix := req
		fix.User = prompt.RepairUser(m, stat, s.cfg.Style)
		if fixed, err := generateN(s.ctx, s.p, fix, 1, s.label()); err == nil && prompt.Valid(fixed[0], s.cfg.Style) {
			msgs[i] = fixed[0]
		} else {
			fmt.Fprintln(os.Stderr, "CommitGen: warning: the model's reply doesn't look like a commit message.\n"+
				"  Tips: stage fewer files at a time, add context with -m \"...\", or try a larger model (--model).")
		}
	}
	if s.p.Used() == "offline" && len(s.targets) > 1 && s.targets[0].name != "offline" {
		warn("used the offline generator (no AI) - review the message before committing")
	}
	return msgs, nil
}

// label is what the spinner shows: the provider currently in use.
func (s *session) label() string {
	used := s.p.Used()
	for _, t := range s.targets {
		if used == "" || t.name == used {
			if t.model != "" {
				return t.name + " (" + t.model + ")"
			}
			return t.name
		}
	}
	return "AI"
}

func newSession(f flags) (*session, context.CancelFunc, error) {
	cfg, _, err := loadConfig(f)
	if err != nil {
		return nil, nil, err
	}
	chain, targets, err := buildChain(cfg, f.verbose)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	return &session{cfg: cfg, f: f, p: chain, targets: targets, ctx: ctx}, cancel, nil
}

// firstRunTip tells new users with nothing configured that they're getting
// the offline generator and how to switch on AI.
func (s *session) firstRunTip() {
	if s.f.hook == "" && !s.f.dryRun && len(s.targets) == 1 && s.targets[0].name == "offline" &&
		mainProvider(s.cfg) == "auto" && !fileExists(config.UserConfigPath()) {
		warn("no AI provider set up yet, so using offline mode (rule-based).\n" +
			"  For AI-written messages run `CommitGen setup` - free options are available.")
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func generateCmd(f flags) error {
	if _, err := git.RepoRoot(); err != nil {
		return err
	}
	if f.all {
		if err := git.StageAll(); err != nil {
			return err
		}
	}
	s, cancel, err := newSession(f)
	if err != nil {
		return err
	}
	defer cancel()
	if f.each {
		return eachCmd(s)
	}

	req, stat, err := s.request()
	if err != nil {
		if f.hook != "" && errors.Is(err, git.ErrNothingStaged) {
			return nil
		}
		return err
	}
	s.firstRunTip()

	if f.dryRun {
		fmt.Printf("providers (in order): %s\n\n=== system ===\n%s\n=== user ===\n%s",
			describe(s.targets), req.System, req.User)
		return nil
	}

	gen := func(n int) ([]string, error) { return s.generate(req, stat, n) }

	if f.hook != "" {
		msgs, err := gen(1)
		if err != nil {
			fmt.Fprintln(os.Stderr, "CommitGen: skipped —", err)
			return nil
		}
		existing, _ := os.ReadFile(f.hook)
		return os.WriteFile(f.hook, []byte(msgs[0]+"\n"+string(existing)), 0o644)
	}

	interactive := isTTY(os.Stdin) && isTTY(os.Stdout) && !f.print && !f.commit
	for {
		msgs, err := gen(max(1, f.n))
		if err != nil {
			return err
		}
		msg := msgs[0]
		if len(msgs) > 1 {
			if msg, err = pick(msgs); err != nil {
				return err
			}
		}

		if f.commit {
			return commit(msg, !f.yes)
		}
		if !interactive {
			fmt.Println(msg)
			return nil
		}
		if len(msgs) == 1 {
			fmt.Printf("\n%s\n\n", indent(msg))
		}
		switch ask("[c]ommit  [e]dit  [r]egenerate  [q]uit", "c") {
		case "c":
			return commit(msg, false)
		case "e":
			return commit(msg, true)
		case "r":
			continue
		default:
			return nil
		}
	}
}

func generateN(ctx context.Context, p llm.Provider, req llm.Request, n int, label string) ([]string, error) {
	if n > 1 && req.Temperature < 0.7 {
		req.Temperature = 0.7
	}
	stop := spinner(fmt.Sprintf("generating with %s", label))
	defer stop()

	out := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			raw, err := p.Generate(ctx, req, nil)
			out[i], errs[i] = prompt.Clean(raw), err
		}(i)
	}
	wg.Wait()

	var good []string
	seen := map[string]bool{}
	for i := range out {
		if errs[i] == nil && out[i] != "" && !seen[out[i]] {
			seen[out[i]] = true
			good = append(good, out[i])
		}
	}
	if len(good) == 0 {
		if errs[0] != nil {
			return nil, errs[0]
		}
		return nil, errors.New("the model returned an empty message")
	}
	return good, nil
}

func commit(msg string, edit bool) error {
	tmp, err := os.CreateTemp("", "CommitGen-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(msg + "\n")
	tmp.Close()

	args := []string{"commit", "-F", tmp.Name()}
	if edit {
		args = append(args, "-e")
	}
	cmd := exec.Command("git", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "COMMITGEN_SKIP_HOOK=1")
	return cmd.Run()
}

func pick(msgs []string) (string, error) {
	fmt.Println()
	for i, m := range msgs {
		fmt.Printf("  %d) %s\n\n", i+1, strings.ReplaceAll(m, "\n", "\n     "))
	}
	if !isTTY(os.Stdin) {
		return msgs[0], nil
	}
	for {
		a := ask(fmt.Sprintf("pick 1-%d", len(msgs)), "1")
		if i, err := strconv.Atoi(a); err == nil && i >= 1 && i <= len(msgs) {
			return msgs[i-1], nil
		}
	}
}

func ask(q, def string) string {
	fmt.Printf("%s (%s): ", q, def)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def
	}
	return line[:1]
}

func indent(s string) string { return "  " + strings.ReplaceAll(s, "\n", "\n  ") }

func isTTY(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func spinner(label string) func() {
	if !isTTY(os.Stderr) {
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		start := time.Now()
		for i := 0; ; i++ {
			select {
			case <-done:
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-time.After(90 * time.Millisecond):
				fmt.Fprintf(os.Stderr, "\r%s %s… %.1fs", frames[i%len(frames)], label, time.Since(start).Seconds())
			}
		}
	}()
	return func() { close(done); wg.Wait() }
}

const hookMarker = "# installed by CommitGen"

const hookScript = `#!/bin/sh
` + hookMarker + `
# Drafts a commit message with CommitGen when you run plain ` + "`git commit`" + `.
# Skipped for merges, amends, -m messages, templates, and CommitGen's own commits.
[ -n "$COMMITGEN_SKIP_HOOK" ] && exit 0
case "$2" in merge|squash|commit|message|template) exit 0 ;; esac
cg=$(command -v CommitGen || command -v commitgen) || exit 0
"$cg" --hook "$1" </dev/null || true
`

func hookCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: CommitGen hook install|uninstall [--force]")
	}
	gitDir, err := git.GitDir()
	if err != nil {
		return errors.New("not inside a git repository")
	}
	path := filepath.Join(gitDir, "hooks", "prepare-commit-msg")
	force := len(args) > 1 && args[1] == "--force"
	existing, readErr := os.ReadFile(path)
	// Hooks from older versions say "installed by commitgen".
	ours := readErr == nil && strings.Contains(strings.ToLower(string(existing)), strings.ToLower(hookMarker))

	switch args[0] {
	case "install":
		if readErr == nil && !ours && !force {
			return fmt.Errorf("%s already exists and was not created by CommitGen (use --force to overwrite)", path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(hookScript), 0o755); err != nil {
			return err
		}
		fmt.Println("installed", path)
		fmt.Println("now just run `git commit` — CommitGen drafts the message in your editor.")
		return nil
	case "uninstall":
		if readErr != nil {
			fmt.Println("no hook installed")
			return nil
		}
		if !ours && !force {
			return fmt.Errorf("%s was not created by CommitGen (use --force to remove)", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Println("removed", path)
		return nil
	}
	return fmt.Errorf("unknown hook command %q", args[0])
}

func configCmd(args []string) error {
	sub := "show"
	if len(args) > 0 {
		sub = args[0]
	}
	path := config.UserConfigPath()
	switch sub {
	case "path":
		fmt.Println(path)
	case "init":
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(config.Sample), 0o600); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	case "show":
		root, _ := git.RepoRoot()
		c, err := config.Load(root)
		if err != nil {
			return err
		}
		note := ""
		if !fileExists(path) {
			note = " (not created yet - run `CommitGen setup`)"
		}
		targets, skipped := resolve(c)
		fmt.Printf("config file: %s%s\nprovider:    %s\nwill try:    %s\nstyle:       %s\nlanguage:    %s\nmax_diff:    %d\nhistory:     %d\ntimeout:     %ds\nmax_wait:    %ds\n",
			path, note, c.Provider, describe(targets), c.Style, c.Language, c.MaxDiff, c.History, c.Timeout, c.MaxWait)
		for _, sk := range skipped {
			fmt.Println("skipped:    ", sk)
		}
	default:
		return fmt.Errorf("unknown config command %q", sub)
	}
	return nil
}

func eachCmd(s *session) error {
	if root, err := git.RepoRoot(); err == nil {
		os.Chdir(root)
	}
	files, err := git.StagedFiles()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return git.ErrNothingStaged
	}
	snapshot, err := git.WriteTree()
	if err != nil {
		return err
	}
	restore := func() {
		git.RestoreIndex(snapshot)
		git.RefreshIndex()
	}
	preview := s.f.dryRun || s.f.print
	if preview {
		defer restore()
	}

	s.firstRunTip()
	fmt.Fprintf(os.Stderr, "CommitGen: %d file(s) staged, committing one by one\n", len(files))
	done := 0
	for i, file := range files {
		if err := git.ResetIndex(); err != nil {
			restore()
			return err
		}
		if err := git.StageFromTree(snapshot, file); err != nil {
			restore()
			return err
		}
		req, stat, err := s.request()
		if err == nil {
			var msgs []string
			msgs, err = s.generate(req, stat, 1)
			if err == nil {
				fmt.Printf("\n[%d/%d] %s\n%s\n", i+1, len(files), file, indent(msgs[0]))
				if !preview {
					err = commitQuiet(msgs[0])
				}
			}
		}
		if err != nil {
			restore()
			return fmt.Errorf("stopped at %s after %d commit(s): %w\n  the remaining files are still staged", file, done, err)
		}
		done++
	}
	git.RefreshIndex()
	if preview {
		fmt.Fprintf(os.Stderr, "\ncommitgen: preview only, nothing was committed\n")
	} else {
		fmt.Fprintf(os.Stderr, "\ncommitgen: created %d commit(s)\n", done)
	}
	return nil
}

func commitQuiet(msg string) error {
	tmp, err := os.CreateTemp("", "CommitGen-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(msg + "\n")
	tmp.Close()
	cmd := exec.Command("git", "commit", "-q", "-F", tmp.Name())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "COMMITGEN_SKIP_HOOK=1")
	return cmd.Run()
}
