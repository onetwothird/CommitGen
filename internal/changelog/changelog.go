// Package changelog turns Conventional Commits into release notes in the
// Keep a Changelog style. It needs no AI: the commit messages already say
// what each change is.
package changelog

import (
	"fmt"
	"regexp"
	"strings"
)

// Commit is the part of a commit the changelog needs.
type Commit struct {
	Short, Subject, Body string
}

type section struct {
	title string
	types []string
	full  bool // only listed with Options.Full
}

var sections = []section{
	{"Features", []string{"feat"}, false},
	{"Bug fixes", []string{"fix"}, false},
	{"Performance", []string{"perf"}, false},
	{"Reverts", []string{"revert"}, false},
	{"Refactoring", []string{"refactor"}, true},
	{"Documentation", []string{"docs"}, true},
	{"Tests", []string{"test"}, true},
	{"Build and CI", []string{"build", "ci"}, true},
	{"Other changes", []string{"chore", "style", ""}, true}, // "" = not a Conventional Commit
}

type Options struct {
	Heading string // e.g. "v1.2.0"; empty = "Unreleased"
	Date    string // YYYY-MM-DD, shown after the heading (not for Unreleased)
	Full    bool   // list refactors, docs, tests, chores and non-conventional commits too
}

var (
	subjectRe  = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?(!)?: (.+)$`)
	breakingRe = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE: (.+)$`)
)

type entry struct {
	typ, scope, summary, short, breaking string
}

func parse(c Commit) entry {
	e := entry{short: c.Short, summary: c.Subject}
	if m := subjectRe.FindStringSubmatch(c.Subject); m != nil {
		e.typ, e.scope, e.summary = strings.ToLower(m[1]), m[2], m[4]
		if m[3] != "" {
			e.breaking = m[4]
		}
	}
	if m := breakingRe.FindStringSubmatch(c.Body); m != nil {
		e.breaking = strings.TrimSpace(m[1])
	}
	return e
}

func (e entry) line(text string) string {
	if e.scope != "" {
		text = "**" + e.scope + ":** " + text
	}
	if e.short != "" {
		text += " (" + e.short + ")"
	}
	return "- " + text + "\n"
}

// Render returns the release notes and how many commits were left out
// because their type is only shown with Full.
func Render(commits []Commit, o Options) (string, int) {
	var b strings.Builder
	if o.Heading == "" || strings.EqualFold(o.Heading, "unreleased") {
		b.WriteString("## [Unreleased]\n")
	} else if o.Date != "" {
		fmt.Fprintf(&b, "## [%s] - %s\n", o.Heading, o.Date)
	} else {
		fmt.Fprintf(&b, "## [%s]\n", o.Heading)
	}

	entries := make([]entry, 0, len(commits))
	for _, c := range commits {
		if strings.HasPrefix(c.Subject, "Merge ") {
			continue
		}
		entries = append(entries, parse(c))
	}

	var breaking strings.Builder
	for _, e := range entries {
		if e.breaking != "" {
			breaking.WriteString(e.line(e.breaking))
		}
	}
	wrote := false
	if breaking.Len() > 0 {
		b.WriteString("\n### Breaking changes\n\n" + breaking.String())
		wrote = true
	}

	hidden := 0
	for _, s := range sections {
		var items strings.Builder
		for _, e := range entries {
			if !inSection(s, e.typ) {
				continue
			}
			if s.full && !o.Full {
				hidden++
				continue
			}
			items.WriteString(e.line(e.summary))
		}
		if items.Len() > 0 {
			b.WriteString("\n### " + s.title + "\n\n" + items.String())
			wrote = true
		}
	}
	if !wrote {
		b.WriteString("\nNo notable changes.\n")
	}
	return b.String(), hidden
}

func inSection(s section, typ string) bool {
	known := false
	for _, sec := range sections {
		for _, t := range sec.types {
			if t == typ && t != "" {
				known = true
			}
		}
	}
	for _, t := range s.types {
		if t == typ || t == "" && !known {
			return true
		}
	}
	return false
}
