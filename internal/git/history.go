package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Commit is one commit read from the log.
type Commit struct {
	Hash, Short string
	Parents     []string
	Subject     string
	Body        string // full message minus the subject line
	Message     string // full message
}

// runWith runs git with extra environment variables and stdin.
func runWith(env []string, stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrNoGit
		}
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return out.String(), nil
}

// Verify reports whether ref names a commit.
func Verify(ref string) bool {
	_, err := run("rev-parse", "--verify", "-q", ref+"^{commit}")
	return err == nil
}

// RevParse resolves ref to a full commit id.
func RevParse(ref string) (string, error) {
	s, err := run("rev-parse", "--verify", ref+"^{commit}")
	return strings.TrimSpace(s), err
}

// DefaultBranch guesses the branch pull requests go into: origin's HEAD,
// then main or master (remote or local).
func DefaultBranch() string {
	if s, err := run("symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	for _, b := range []string{"origin/main", "origin/master", "main", "master"} {
		if Verify(b) {
			return b
		}
	}
	return ""
}

// Upstream returns the current branch's upstream (e.g. origin/feature), or "".
func Upstream() string {
	s, err := run("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// MergeBase returns the best common ancestor of a and b.
func MergeBase(a, b string) (string, error) {
	s, err := run("merge-base", a, b)
	return strings.TrimSpace(s), err
}

// IsAncestor reports whether commit a is reachable from b.
func IsAncestor(a, b string) bool {
	_, err := run("merge-base", "--is-ancestor", a, b)
	return err == nil
}

// OnRemote reports whether any remote-tracking branch contains commit.
func OnRemote(commit string) bool {
	s, err := run("branch", "-r", "--contains", commit)
	return err == nil && strings.TrimSpace(s) != ""
}

// LatestTag returns the newest tag reachable from ref, or "".
func LatestTag(ref string) string {
	s, err := run("describe", "--tags", "--abbrev=0", ref)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// Log returns the commits in rangeSpec (e.g. "v1.0..HEAD"), oldest first.
// Extra arguments (like -n 3) are passed to git log.
func Log(rangeSpec string, extra ...string) ([]Commit, error) {
	args := append([]string{"log", "--format=%H%x1f%h%x1f%P%x1f%B%x1e"}, extra...)
	if rangeSpec != "" {
		args = append(args, rangeSpec)
	}
	out, err := run(args...)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, "\x1f", 4)
		if len(f) != 4 {
			continue
		}
		msg := strings.TrimSpace(f[3])
		subj, body, _ := strings.Cut(msg, "\n")
		commits = append(commits, Commit{
			Hash: f[0], Short: f[1], Parents: strings.Fields(f[2]),
			Subject: strings.TrimSpace(subj), Body: strings.TrimSpace(body), Message: msg,
		})
	}
	// git log lists newest first.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

// Diff returns the diff between two commits with noisy files excluded.
func Diff(from, to string, extraExcludes []string) (string, error) {
	args := append([]string{"diff", "--no-color", "--no-ext-diff", "-U3", from, to}, excludeSpecs(extraExcludes)...)
	return run(args...)
}

// DiffStat returns `git diff --stat` between two commits.
func DiffStat(from, to string) (string, error) {
	return run("diff", "--no-color", "--stat=120", from, to)
}

// Show returns the diff a single commit introduces (works for root commits).
func Show(commit string, extraExcludes []string) (string, error) {
	args := append([]string{"show", "--no-color", "--no-ext-diff", "-U3", "--format=", commit}, excludeSpecs(extraExcludes)...)
	return run(args...)
}

// ShowStat returns the --stat of a single commit.
func ShowStat(commit string) (string, error) {
	return run("show", "--no-color", "--stat=120", "--format=", commit)
}

// Busy names an operation in progress (merge, rebase, ...) or returns "".
func Busy() string {
	dir, err := GitDir()
	if err != nil {
		return ""
	}
	for name, what := range map[string]string{
		"MERGE_HEAD": "a merge", "rebase-merge": "a rebase", "rebase-apply": "a rebase or am",
		"CHERRY_PICK_HEAD": "a cherry-pick", "REVERT_HEAD": "a revert", "BISECT_LOG": "a bisect",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return what
		}
	}
	return ""
}

// Recommit creates a copy of commit with a new message and parents, keeping
// its tree, author and author date. It returns the new commit id.
func Recommit(commit, message string, parents []string) (string, error) {
	who, err := run("log", "-1", "--date=raw", "--format=%an%x00%ae%x00%ad", commit)
	if err != nil {
		return "", err
	}
	f := strings.SplitN(strings.TrimRight(who, "\n"), "\x00", 3)
	if len(f) != 3 {
		return "", fmt.Errorf("can't read the author of %s", commit)
	}
	env := []string{"GIT_AUTHOR_NAME=" + f[0], "GIT_AUTHOR_EMAIL=" + f[1], "GIT_AUTHOR_DATE=" + f[2]}
	args := []string{"commit-tree", commit + "^{tree}"}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	args = append(args, "-F", "-")
	s, err := runWith(env, message+"\n", args...)
	return strings.TrimSpace(s), err
}

// UpdateHead moves HEAD (and the branch it points to) from old to new,
// failing if HEAD moved in the meantime. ORIG_HEAD is set to old for undo.
func UpdateHead(newHead, oldHead, reason string) error {
	if _, err := run("update-ref", "-m", reason, "HEAD", newHead, oldHead); err != nil {
		return err
	}
	_, err := run("update-ref", "ORIG_HEAD", oldHead)
	return err
}

// Signed reports whether commit carries a signature.
func Signed(commit string) bool {
	s, err := run("cat-file", "commit", commit)
	return err == nil && strings.Contains(s, "\ngpgsig")
}

// ExactTag returns the tag pointing exactly at ref, or "".
func ExactTag(ref string) string {
	s, err := run("describe", "--tags", "--exact-match", ref)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// CommitDate returns the committer date of ref as YYYY-MM-DD.
func CommitDate(ref string) string {
	s, err := run("log", "-1", "--format=%cs", ref)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}
