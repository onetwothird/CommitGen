package prompt

import (
	"fmt"
	"regexp"
	"strings"
)

// PRInput is what the model sees when writing a pull request description.
type PRInput struct {
	Base, Branch string
	Commits      []string // "abc1234 feat(x): subject"
	Stat, Diff   string
	Style        string
	Language     string
	Hint         string
	MaxDiff      int
}

func PRSystem(in PRInput) string {
	var b strings.Builder
	b.WriteString("You are an expert software engineer who writes clear pull request descriptions for code review.\n\nRules:\n")
	if in.Style == "simple" {
		b.WriteString("- Line 1 is the pull request title: a short imperative summary, max 72 characters, no trailing period.\n")
	} else {
		b.WriteString("- Line 1 is the pull request title in Conventional Commits form, `type(scope): summary`, max 72 characters, for example `feat(cache): reuse messages for unchanged diffs`.\n")
	}
	b.WriteString("- Then one blank line, then the description in GitHub Markdown with these sections:\n")
	b.WriteString("  `## Summary`: 1-3 sentences on what the change does and why.\n")
	b.WriteString("  `## Changes`: short bullet points, grouped by area. Focus on behaviour, not on every file.\n")
	b.WriteString("  `## Testing`: how the change was or can be verified, based on the tests and code in the diff. Do not claim tests were run.\n")
	b.WriteString("  `## Notes`: only if there are breaking changes, migrations or follow-ups; otherwise leave this section out.\n")
	b.WriteString("- Never invent changes that are not in the diff or the commits.\n")
	b.WriteString("- Output ONLY the title and description. No code fences around the whole reply, no preamble.\n")
	if in.Language != "" && !strings.EqualFold(in.Language, "english") {
		fmt.Fprintf(&b, "- Write in %s (keep the type keyword and section headings in English).\n", in.Language)
	}
	return b.String()
}

func PRUser(in PRInput) string {
	var b strings.Builder
	if in.Hint != "" {
		fmt.Fprintf(&b, "Context from the author: %s\n\n", in.Hint)
	}
	fmt.Fprintf(&b, "Branch %s, to be merged into %s.\n\n", in.Branch, in.Base)
	b.WriteString("Commits (oldest first):\n")
	for _, c := range in.Commits {
		fmt.Fprintf(&b, "  %s\n", c)
	}
	if in.Stat != "" {
		fmt.Fprintf(&b, "\nFiles changed:\n%s\n", in.Stat)
	}
	diff := in.Diff
	if in.MaxDiff > 0 && len(diff) > in.MaxDiff {
		diff = Truncate(diff, in.MaxDiff)
	}
	fmt.Fprintf(&b, "\nDiff:\n%s\n", diff)
	b.WriteString("\n---\nTask: write the pull request title (line 1) and description for the changes above.\n")
	return b.String()
}

var (
	wholeFenceRe = regexp.MustCompile("(?s)^```[a-zA-Z]*\\n(.*?)\\n?```$")
	textPrefixRe = regexp.MustCompile(`(?i)^(sure[!,.]?\s*)?(here('s| is) (the|a|your) (pull request|pr)[^\n]*:)\s*`)
	titleLabelRe = regexp.MustCompile(`(?i)^(#+\s*)?(\*\*)?(title|pr title)(\*\*)?:\s*`)
)

// CleanText strips what models wrap around a free-form reply: reasoning
// blocks, a preamble and a code fence around everything.
func CleanText(s string) string {
	s = thinkRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	s = strings.TrimSpace(textPrefixRe.ReplaceAllString(s, ""))
	if m := wholeFenceRe.FindStringSubmatch(s); m != nil {
		s = strings.TrimSpace(m[1])
	}
	return s
}

// SplitTitle separates the first line (the title) from the rest, dropping a
// "Title:" label or heading marks around the title.
func SplitTitle(s string) (title, body string) {
	title, body, _ = strings.Cut(CleanText(s), "\n")
	title = strings.TrimSpace(titleLabelRe.ReplaceAllString(strings.TrimSpace(title), ""))
	title = strings.TrimSpace(strings.TrimLeft(title, "# "))
	title = strings.Trim(title, "*`\"")
	return title, strings.TrimSpace(body)
}
