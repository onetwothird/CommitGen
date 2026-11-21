// Package refs finds the issue a branch is about (fix/123-login → #123,
// feature/PROJ-42-search → PROJ-42) and adds it to a commit message footer.
package refs

import (
	"regexp"
	"strings"
)

var (
	// Jira/Linear style keys anywhere in the branch: PROJ-42, ENG-7.
	keyRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9])([A-Z][A-Z0-9]{1,9}-[1-9][0-9]{0,6})(?:[^0-9]|$)`)
	// A GitHub/GitLab issue number at the start of the last branch segment:
	// 123-login, gh-123, issue-123, issue_123, #123.
	numRe = regexp.MustCompile(`^(?i:(?:gh|issue|issues)[-_]?)?#?([1-9][0-9]{0,6})(?:[-_]|$)`)
	// Footer lines: "Refs: #1", "Closes #1", "BREAKING CHANGE: ...".
	trailerRe = regexp.MustCompile(`^([A-Za-z][A-Za-z-]*|BREAKING CHANGE)(: | #)`)
)

// FromBranch returns the issue reference in a branch name, or "".
func FromBranch(branch string) string {
	if branch == "" {
		return ""
	}
	if m := keyRe.FindStringSubmatch(branch); m != nil {
		return m[1]
	}
	last := branch[strings.LastIndex(branch, "/")+1:]
	if m := numRe.FindStringSubmatch(last); m != nil {
		return "#" + m[1]
	}
	return ""
}

// Mentions reports whether msg already refers to ref.
func Mentions(msg, ref string) bool {
	re := regexp.MustCompile(`(?i)(^|[^A-Za-z0-9])` + regexp.QuoteMeta(ref) + `([^0-9]|$)`)
	return re.MatchString(msg)
}

// AddFooter appends "<word> <ref>" (e.g. "Refs #123") unless the message
// already mentions ref. It joins an existing footer block instead of starting
// a new paragraph.
func AddFooter(msg, word, ref string) string {
	word = strings.TrimSpace(word)
	if ref == "" || word == "" || Mentions(msg, ref) {
		return msg
	}
	line := word + " " + ref
	msg = strings.TrimRight(msg, "\n ")
	paras := strings.Split(msg, "\n\n")
	if len(paras) > 1 && isFooter(paras[len(paras)-1]) {
		return msg + "\n" + line
	}
	return msg + "\n\n" + line
}

func isFooter(p string) bool {
	for _, l := range strings.Split(p, "\n") {
		if !trailerRe.MatchString(strings.TrimSpace(l)) {
			return false
		}
	}
	return true
}
