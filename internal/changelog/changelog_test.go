package changelog

import (
	"strings"
	"testing"
)

var commits = []Commit{
	{"a1", "feat(cache): reuse messages for unchanged diffs", ""},
	{"b2", "fix: handle empty branch names", ""},
	{"c3", "feat(cli)!: rename --max to --max-diff", ""},
	{"d4", "refactor(git): split history helpers", ""},
	{"e5", "docs: explain the cache", ""},
	{"f6", "perf(prompt): build prompts once", "BREAKING CHANGE: prompt.Input.Max was removed"},
	{"g7", "wip", ""},
	{"h8", "Merge branch 'main' into feature", ""},
}

func TestRender(t *testing.T) {
	out, hidden := Render(commits, Options{Heading: "v1.2.0", Date: "2026-11-22"})
	want := `## [v1.2.0] - 2026-11-22

### Breaking changes

- **cli:** rename --max to --max-diff (c3)
- **prompt:** prompt.Input.Max was removed (f6)

### Features

- **cache:** reuse messages for unchanged diffs (a1)
- **cli:** rename --max to --max-diff (c3)

### Bug fixes

- handle empty branch names (b2)

### Performance

- **prompt:** build prompts once (f6)
`
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	if hidden != 3 {
		t.Errorf("expected 3 hidden commits (refactor, docs, wip), got %d", hidden)
	}
}

func TestRenderFull(t *testing.T) {
	out, hidden := Render(commits, Options{Full: true})
	if hidden != 0 {
		t.Errorf("full should hide nothing, hid %d", hidden)
	}
	for _, s := range []string{"## [Unreleased]", "### Refactoring", "### Documentation", "### Other changes\n\n- wip (g7)"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q in:\n%s", s, out)
		}
	}
	if strings.Contains(out, "Merge branch") {
		t.Error("merge commits must be skipped")
	}
}

func TestRenderEmpty(t *testing.T) {
	out, _ := Render([]Commit{{"x", "chore: bump deps", ""}}, Options{})
	if !strings.Contains(out, "No notable changes.") {
		t.Errorf("got:\n%s", out)
	}
}
