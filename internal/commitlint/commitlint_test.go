package commitlint

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var want = Rules{Types: []string{"feat", "fix", "docs"}, Scopes: []string{"api", "ui"}, HeaderMax: 72}

func check(t *testing.T, name string, got Rules) {
	t.Helper()
	got.File = ""
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %+v, want %+v", name, got, want)
	}
}

func TestParseFormats(t *testing.T) {
	formats := map[string]string{
		"js": `module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [2, 'always', ['feat', 'fix', 'docs']],
    'scope-enum': [2, 'always', ['api', 'ui']],
    'header-max-length': [2, 'always', 72],
  },
};`,
		"json": `{"extends": ["@commitlint/config-conventional"], "rules": {"type-enum": [2, "always", ["feat", "fix", "docs"]], "scope-enum": [2, "always", ["api", "ui"]], "header-max-length": [2, "always", 72]}}`,
		"json pretty": `{
  "rules": {
    "type-enum": [
      2,
      "always",
      ["feat", "fix", "docs"]
    ],
    "scope-enum": [2, "always", ["api", "ui"]],
    "header-max-length": [2, "always", 72]
  }
}`,
		"yaml flow": `extends:
  - "@commitlint/config-conventional"
rules:
  type-enum: [2, always, [feat, fix, docs]]
  scope-enum: [2, always, [api, ui]]
  header-max-length: [2, always, 72]
`,
		"yaml block": `rules:
  type-enum:
    - 2
    - always
    - - feat
      - fix
      - docs
  scope-enum:
    - 2
    - always
    - [api, ui]
  header-max-length:
    - 2
    - always
    - 72
`,
	}
	for name, text := range formats {
		check(t, name, Parse(text))
	}
}

func TestDisabledRules(t *testing.T) {
	r := Parse(`rules: { 'type-enum': [0, 'always', ['feat']], 'scope-enum': [2, 'never', ['x']], 'header-max-length': [0, 'always', 50] }`)
	if r.Types != nil || r.Scopes != nil || r.HeaderMax != 0 {
		t.Errorf("disabled rules should not restrict: %+v", r)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	if r, _ := Load(dir); r.Found() {
		t.Fatal("no config should be found")
	}
	pkg := `{"name": "x", "commitlint": {"rules": {"type-enum": [2, "always", ["feat", "fix", "docs"]], "scope-enum": [2, "always", ["api", "ui"]], "header-max-length": [2, "always", 72]}}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644)
	r, _ := Load(dir)
	if r.File != "package.json" {
		t.Fatalf("expected package.json, got %q", r.File)
	}
	check(t, "package.json", r)

	os.WriteFile(filepath.Join(dir, ".commitlintrc.yml"), []byte("rules:\n  type-enum: [2, always, [feat]]\n"), 0o644)
	if r, _ := Load(dir); r.File != ".commitlintrc.yml" || len(r.Types) != 1 {
		t.Errorf("a commitlint file should win over package.json: %+v", r)
	}
}

func TestProblems(t *testing.T) {
	if p := want.Problems("feat(api): add search"); len(p) != 0 {
		t.Errorf("valid message reported: %v", p)
	}
	if p := want.Problems("chore(db): bump"); len(p) != 2 {
		t.Errorf("expected bad type and scope, got %v", p)
	}
	long := "feat: " + string(make([]byte, 80))
	if p := want.Problems(long); len(p) != 1 {
		t.Errorf("expected header too long, got %v", p)
	}
	if p := (Rules{}).Problems("anything goes"); len(p) != 0 {
		t.Errorf("no rules, no problems: %v", p)
	}
}
