package heuristic

import (
	"strings"
	"testing"

	"github.com/onetwothird/CommitGen/internal/prompt"
)

func mod(path string, removed, added []string) string {
	var b strings.Builder
	b.WriteString("diff --git a/" + path + " b/" + path + "\nindex 111..222 100644\n--- a/" + path + "\n+++ b/" + path + "\n@@ -1,3 +1,3 @@\n")
	for _, l := range removed {
		b.WriteString("-" + l + "\n")
	}
	for _, l := range added {
		b.WriteString("+" + l + "\n")
	}
	return b.String()
}

func newFile(path string, lines ...string) string {
	var b strings.Builder
	b.WriteString("diff --git a/" + path + " b/" + path + "\nnew file mode 100644\nindex 000..222\n--- /dev/null\n+++ b/" + path + "\n@@ -0,0 +1 @@\n")
	for _, l := range lines {
		b.WriteString("+" + l + "\n")
	}
	return b.String()
}

func delFile(path string, lines ...string) string {
	var b strings.Builder
	b.WriteString("diff --git a/" + path + " b/" + path + "\ndeleted file mode 100644\n--- a/" + path + "\n+++ /dev/null\n@@ -1 +0,0 @@\n")
	for _, l := range lines {
		b.WriteString("-" + l + "\n")
	}
	return b.String()
}

func subject(msg string) string { s, _, _ := strings.Cut(msg, "\n"); return s }

func TestTypes(t *testing.T) {
	cases := []struct {
		name, diff, want string
	}{
		{"docs", mod("README.md", []string{"old"}, []string{"new"}), "docs: update README.md"},
		{"test", mod("internal/git/git_test.go", []string{"a"}, []string{"b"}), "test(git): update git_test.go"},
		{"ci", mod(".github/workflows/ci.yml", []string{"a"}, []string{"b"}), "ci: update ci.yml"},
		{"build", mod("go.mod", []string{"go 1.21"}, []string{"go 1.22"}), "build: update go.mod"},
		{"new file", newFile("internal/cache/cache.go", "package cache", "func Get(k string) string {"), "feat(cache): add cache.go"},
		{"new func", mod("internal/api/server.go", nil, []string{"func (s *Server) Health(w http.ResponseWriter) {", "}"}), "feat(api): add Health in server.go"},
		{"delete", delFile("old/legacy.py", "x = 1"), "refactor(old): remove legacy.py"},
		{"small change", mod("src/app.js", []string{"if (a > b) {"}, []string{"if (a >= b) {"}), "fix: update app.js"},
		{"fix words", mod("lib/parse.rb", []string{"x"}, []string{"# fix crash on empty input", "return if s.empty?", "y", "z", "w", "v"}), "fix: update parse.rb"},
		{"whitespace", mod("pkg/util/util.go", []string{"a := 1"}, []string{"a  :=  1"}), "style(util): update util.go"},
		{"docs dir", newFile("docs/guide.md", "# Guide"), "docs: add guide.md"},
		{"config file", newFile("ci.yml", "name: ci"), "chore: add ci.yml"},
		{"root file", mod("main.go", []string{"a", "b"}, []string{"c", "d", "e", "f", "g", "h"}), "refactor(main): update main.go"},
	}
	for _, c := range cases {
		got := Generate(c.diff, Options{})
		if subject(got) != c.want {
			t.Errorf("%s: got %q, want %q", c.name, subject(got), c.want)
		}
		if !prompt.Valid(got, "conventional") {
			t.Errorf("%s: not a valid conventional message: %q", c.name, got)
		}
	}
}

func TestRename(t *testing.T) {
	d := "diff --git a/src/old.ts b/src/new.ts\nsimilarity index 100%\nrename from src/old.ts\nrename to src/new.ts\n"
	if got := subject(Generate(d, Options{})); got != "refactor: rename old.ts to new.ts" {
		t.Errorf("got %q", got)
	}
}

func TestMultiFileBodyAndScope(t *testing.T) {
	d := newFile("internal/llm/chain.go", "package llm", "func NewChain() *Chain {", "type Chain struct {") +
		mod("internal/llm/llm.go", []string{"a"}, []string{"b"}) +
		mod("README.md", []string{"a"}, []string{"b", "c"})
	got := Generate(d, Options{})
	if subject(got) != "feat(llm): add NewChain and Chain" {
		t.Errorf("subject = %q", subject(got))
	}
	if !strings.Contains(got, "\n\n- add internal/llm/chain.go (+3 -0): NewChain and Chain") ||
		!strings.Contains(got, "- update README.md (+2 -1)") {
		t.Errorf("body = %q", got)
	}
}

func TestHintAndOptions(t *testing.T) {
	d := mod("internal/auth/login.go", []string{"a"}, []string{"b"})
	if got := subject(Generate(d, Options{Hint: "Fixes login loop on expired tokens."})); got != "fix(auth): fix login loop on expired tokens" {
		t.Errorf("hint: %q", got)
	}
	if got := subject(Generate(d, Options{Hint: "feat(ui): new button"})); got != "feat(ui): new button" {
		t.Errorf("conventional hint kept: %q", got)
	}
	if got := subject(Generate(d, Options{Style: "simple"})); got != "Update login.go" {
		t.Errorf("simple: %q", got)
	}
	if got := subject(Generate(d, Options{Scopes: []string{"api"}})); got != "fix: update login.go" {
		t.Errorf("disallowed scope should be dropped: %q", got)
	}
	if got := subject(Generate(d, Options{Scopes: []string{"Auth"}})); got != "fix(Auth): update login.go" {
		t.Errorf("allowed scope: %q", got)
	}
	if Generate("", Options{}) != "" {
		t.Error("empty diff should give empty message")
	}
}

func TestSymbolsAcrossLanguages(t *testing.T) {
	cases := map[string]string{
		"def parse_args(argv):":                         "parse_args",
		"export async function loadUser(id) {":          "loadUser",
		"export const useCart = (items) => {":           "useCart",
		"public function store(Request $request)":       "store",
		"pub fn spawn_worker(cfg: &Config) {":           "spawn_worker",
		"public static void main(String[] args) {":      "main",
		"public sealed class OrderService":              "OrderService",
		"    private async Task<int> CountAsync(int x)": "CountAsync",
		"suspend fun fetchData(url: String): String {":  "fetchData",
		"func viewDidLoad() {":                          "viewDidLoad",
		"  def valid?":                                  "valid?",
		"export interface Props {":                      "Props",
		"static int parse_header(const char *buf) {":    "parse_header",
		"local function setup(opts)":                    "setup",
		"deploy() {":                                    "deploy",
	}
	for line, want := range cases {
		if got := symbol(line); got != want {
			t.Errorf("symbol(%q) = %q, want %q", line, got, want)
		}
	}
	for _, line := range []string{"if (x) {", "return foo(bar);", "// func old() {", "    for (i = 0; i < n; i++) {"} {
		if got := symbol(line); got != "" {
			t.Errorf("symbol(%q) = %q, want none", line, got)
		}
	}
}

func TestLongSubjectIsTrimmed(t *testing.T) {
	d := mod("a.go", nil, nil)
	got := Generate(d, Options{Hint: strings.Repeat("word ", 30)})
	if len(subject(got)) > 72 {
		t.Errorf("subject too long: %d", len(subject(got)))
	}
}
