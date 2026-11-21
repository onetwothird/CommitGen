package heuristic

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

type Status int

const (
	Modified Status = iota
	Added
	Deleted
	Renamed
)

type File struct {
	Path, OldPath  string
	Status         Status
	Added, Removed int
	Binary         bool
	NewSymbols     []string 
	addedText      []string
	removedText    []string
}

func Parse(diff string) []File {
	var files []File
	var cur *File
	for _, line := range strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			files = append(files, File{})
			cur = &files[len(files)-1]
			a, b := splitGitPaths(strings.TrimPrefix(line, "diff --git "))
			cur.OldPath, cur.Path = a, b
		case cur == nil:
			continue
		case strings.HasPrefix(line, "new file mode"):
			cur.Status = Added
		case strings.HasPrefix(line, "deleted file mode"):
			cur.Status = Deleted
		case strings.HasPrefix(line, "rename from "):
			cur.Status, cur.OldPath = Renamed, strings.TrimPrefix(line, "rename from ")
		case strings.HasPrefix(line, "rename to "):
			cur.Path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "Binary files "):
			cur.Binary = true
		case strings.HasPrefix(line, "+++ "), strings.HasPrefix(line, "--- "):
		case strings.HasPrefix(line, "+"):
			cur.Added++
			cur.addedText = append(cur.addedText, line[1:])
		case strings.HasPrefix(line, "-"):
			cur.Removed++
			cur.removedText = append(cur.removedText, line[1:])
		}
	}
	for i := range files {
		f := &files[i]
		if f.Status != Renamed && f.OldPath != f.Path && f.OldPath != "" {
			f.Status = Renamed
		}
		f.NewSymbols = newSymbols(f.addedText, f.removedText)
	}
	return files
}

func splitGitPaths(s string) (string, string) {
	if i := strings.Index(s, " b/"); i >= 0 && strings.HasPrefix(s, "a/") {
		return s[2:i], s[i+3:]
	}
	parts := strings.Fields(s)
	if len(parts) == 2 {
		return strings.TrimPrefix(parts[0], "a/"), strings.TrimPrefix(parts[1], "b/")
	}
	return s, s
}

var symbolRes = []*regexp.Regexp{
	regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*[\[(]`),                                                                                                                       // Go
	regexp.MustCompile(`^type\s+([A-Za-z_]\w*)\s+(?:struct|interface)`),                                                                                                                         // Go types
	regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`),                                                                                                                            // Python
	regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`),                                                                                                                                            // Python/JS/PHP/Java
	regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)\s*\(`),                                                                                // JS
	regexp.MustCompile(`^\s*(?:export\s+)?const\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\([^)]*\)\s*=>`),                                                                                        // JS arrow
	regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?function\s+([A-Za-z_]\w*)\s*\(`),                                                                                      // PHP
	regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?fn\s+([A-Za-z_]\w*)`),                                                                                                         // Rust
	regexp.MustCompile(`^\s*(?:pub\s+)?(?:struct|enum|trait)\s+([A-Z]\w*)`),                                                                                                                     // Rust types
	regexp.MustCompile(`^\s*(?:export\s+)?(?:interface|type|enum)\s+([A-Z]\w*)`),                                                                                                                // TypeScript
	regexp.MustCompile(`^\s*(?:public|private|protected|internal)?\s*(?:abstract\s+|sealed\s+|static\s+|partial\s+|final\s+)*(?:class|interface|enum|record|struct)\s+([A-Z]\w*)`),              // Java/C#/Kotlin
	regexp.MustCompile(`^\s*(?:public|private|protected|internal)\s+(?:static\s+|final\s+|async\s+|override\s+|virtual\s+|abstract\s+|synchronized\s+)*[\w<>\[\],.? ]+?\s+([A-Za-z_]\w*)\s*\(`), // Java/C# methods
	regexp.MustCompile(`^\s*(?:(?:private|public|internal|override|suspend|open)\s+)*fun\s+(?:<[^>]+>\s*)?(?:[\w.]+\.)?([A-Za-z_]\w*)\s*\(`),                                                    // Kotlin
	regexp.MustCompile(`^\s*(?:(?:public|private|internal|static|override|@\w+)\s+)*func\s+([A-Za-z_]\w*)\s*[<(]`),                                                                              // Swift
	regexp.MustCompile(`^\s*def\s+(?:self\.)?([A-Za-z_]\w*[?!]?)`),                                                                                                                              // Ruby
	regexp.MustCompile(`^\s*(?:module|class)\s+([A-Z]\w*)`),                                                                                                                                     // Ruby/Elixir
	regexp.MustCompile(`^\s*defp?\s+([a-z_]\w*[?!]?)`),                                                                                                                                          // Elixir
	regexp.MustCompile(`^(?:static\s+|inline\s+|extern\s+)*(?:const\s+)?[A-Za-z_][\w:<>*& ]*?[\s*&]([A-Za-z_]\w*)\s*\([^;]*$`),                                                                  // C/C++/Dart
	regexp.MustCompile(`^\s*(?:local\s+)?function\s+([A-Za-z_][\w.:]*)\s*\(`),                                                                                                                   // Lua
	regexp.MustCompile(`^\s*([A-Za-z_]\w*)\s*\(\)\s*\{`),                                                                                                                                        // shell functions
}

// keywords that the loose C-style pattern can mistake for a function name.
var notSymbols = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true, "catch": true,
	"else": true, "sizeof": true, "elif": true, "using": true, "new": true, "delete": true,
}

func newSymbols(added, removed []string) []string {
	old := map[string]bool{}
	for _, l := range removed {
		if s := symbol(l); s != "" {
			old[s] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, l := range added {
		if s := symbol(l); s != "" && !old[s] && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func symbol(line string) string {
	if t := strings.TrimSpace(line); t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") ||
		strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") || strings.HasPrefix(t, "return ") {
		return ""
	}
	for _, re := range symbolRes {
		if m := re.FindStringSubmatch(line); m != nil && !notSymbols[m[1]] {
			return m[1]
		}
	}
	return ""
}

type category int

const (
	catSource category = iota
	catDocs
	catTest
	catCI
	catBuild
	catChore
	catStyleSheet
)

func classify(p string) category {
	low := strings.ToLower(p)
	base := path.Base(low)
	ext := path.Ext(low)
	switch {
	case strings.HasPrefix(low, ".github/workflows/"), strings.HasPrefix(low, ".circleci/"),
		base == ".gitlab-ci.yml", base == "azure-pipelines.yml", base == "jenkinsfile",
		base == ".travis.yml", strings.HasPrefix(low, ".buildkite/"):
		return catCI
	case strings.HasSuffix(low, "_test.go"), strings.Contains(low, ".test."), strings.Contains(low, ".spec."),
		strings.HasPrefix(base, "test_") && ext == ".py", strings.HasSuffix(base, "_test.py"),
		strings.HasPrefix(low, "test/"), strings.HasPrefix(low, "tests/"), strings.Contains(low, "/tests/"),
		strings.Contains(low, "/test/"), strings.Contains(low, "__tests__/"), strings.HasSuffix(base, "test.php"),
		strings.Contains(low, "testdata/"):
		return catTest
	case ext == ".md", ext == ".rst", ext == ".adoc", ext == ".txt" && !strings.Contains(base, "requirements"),
		strings.HasPrefix(low, "docs/"), strings.HasPrefix(base, "readme"), strings.HasPrefix(base, "changelog"),
		base == "license", base == "contributing":
		return catDocs
	case base == "go.mod", base == "go.sum", base == "package.json", base == "package-lock.json",
		base == "yarn.lock", base == "pnpm-lock.yaml", base == "makefile", base == "dockerfile",
		strings.HasPrefix(base, "docker-compose"), base == "cargo.toml", base == "cargo.lock",
		base == "pyproject.toml", strings.HasPrefix(base, "requirements"), base == "setup.py",
		base == "composer.json", base == "composer.lock", base == "pom.xml", strings.HasSuffix(base, ".gradle"),
		strings.HasSuffix(base, ".gradle.kts"), base == ".goreleaser.yaml", base == ".goreleaser.yml",
		base == "cmakelists.txt", strings.HasPrefix(base, "webpack.config"), strings.HasPrefix(base, "vite.config"),
		strings.HasPrefix(base, "tsconfig"):
		return catBuild
	case strings.HasPrefix(base, "."), ext == ".ini", ext == ".yml", ext == ".yaml", ext == ".toml",
		ext == ".json", ext == ".xml", ext == ".env", ext == ".cfg", ext == ".conf", ext == ".properties":
		return catChore
	case ext == ".css", ext == ".scss", ext == ".sass", ext == ".less":
		return catStyleSheet
	}
	return catSource
}

type Options struct {
	Style  string   // "conventional" (default) or "simple"
	Scopes []string // allowed scopes; empty = any
	Hint   string   // user's -m text; used as the summary when given
}


func Generate(diff string, o Options) string {
	files := Parse(diff)
	if len(files) == 0 {
		return ""
	}
	if h := strings.TrimSpace(o.Hint); o.Style != "simple" && conventionalRe.MatchString(h) {
		// The user already wrote a conventional subject; keep it and add the file list.
		if body := bodyFor(files, o.Style); body != "" {
			return limit(h, 72) + "\n\n" + body
		}
		return limit(h, 72)
	}
	typ := commitType(files, o.Hint)
	// Name and scope the commit after the code it changes; docs, tests and
	// build files that ride along are still listed in the body.
	focus := files
	if src := sourceFiles(files); len(src) > 0 && len(src) < len(files) {
		focus = src
	}
	scope := scopeFor(focus, o.Scopes)
	if typ == "ci" {
		scope = "" // "ci(workflows)" adds nothing
	}
	if strings.EqualFold(scope, typ) {
		scope = "" // avoid "docs(docs)"
	}
	summary := summarize(focus, o.Hint)

	var subj string
	if o.Style == "simple" {
		subj = capitalize(summary)
	} else {
		subj = typ
		if scope != "" {
			subj += "(" + scope + ")"
		}
		subj += ": " + summary
	}
	subj = limit(subj, 72)

	body := bodyFor(files, o.Style)
	if body == "" {
		return subj
	}
	return subj + "\n\n" + body
}

var hintTypes = []struct {
	re  *regexp.Regexp
	typ string
}{
	{regexp.MustCompile(`(?i)\b(fix|fixes|fixed|bug|crash|broken|resolve[sd]?|patch|hotfix)\b`), "fix"},
	{regexp.MustCompile(`(?i)\b(doc|docs|readme|documentation)\b`), "docs"},
	{regexp.MustCompile(`(?i)\b(test|tests|testing|coverage)\b`), "test"},
	{regexp.MustCompile(`(?i)\b(refactor|cleanup|clean up|restructure|rename|simplify)\b`), "refactor"},
	{regexp.MustCompile(`(?i)\b(perf|performance|faster|speed up|optimi[sz]e)\b`), "perf"},
	{regexp.MustCompile(`(?i)\b(add|adds|added|new|feature|implement|support|introduce)\b`), "feat"},
}

var fixWords = regexp.MustCompile(`(?i)\b(fix|bug|workaround|hotfix|regression)\b`)

func commitType(files []File, hint string) string {
	for _, h := range hintTypes {
		if hint != "" && h.re.MatchString(hint) {
			return h.typ
		}
	}
	cats := map[category]int{}
	for _, f := range files {
		cats[classify(f.Path)]++
	}
	only := func(c category) bool { return cats[c] == len(files) }
	switch {
	case only(catDocs):
		return "docs"
	case only(catTest):
		return "test"
	case only(catCI):
		return "ci"
	case only(catBuild):
		return "build"
	case only(catChore):
		return "chore"
	}
	// Mixed: ignore docs/tests/build when source changed.
	var src []File
	for _, f := range files {
		if c := classify(f.Path); c == catSource || c == catStyleSheet {
			src = append(src, f)
		}
	}
	if len(src) == 0 {
		return "chore"
	}
	if allWhitespace(src) {
		return "style"
	}
	added, removed, newFiles, deleted, renamed, symbols := 0, 0, 0, 0, 0, 0
	fixy := false
	for _, f := range src {
		added += f.Added
		removed += f.Removed
		symbols += len(f.NewSymbols)
		switch f.Status {
		case Added:
			newFiles++
		case Deleted:
			deleted++
		case Renamed:
			renamed++
		}
		for _, l := range f.addedText {
			if fixWords.MatchString(l) {
				fixy = true
			}
		}
	}
	switch {
	case deleted == len(src), renamed == len(src):
		return "refactor"
	case newFiles > 0 || symbols > 0:
		return "feat"
	case fixy:
		return "fix"
	case removed > added*2:
		return "refactor"
	case added+removed <= 6:
		return "fix"
	}
	return "refactor"
}

func sourceFiles(files []File) []File {
	var src []File
	for _, f := range files {
		if c := classify(f.Path); c == catSource || c == catStyleSheet {
			src = append(src, f)
		}
	}
	return src
}

func allWhitespace(files []File) bool {
	for _, f := range files {
		if f.Status != Modified || f.Added == 0 {
			return false
		}
		norm := func(ls []string) string {
			var b strings.Builder
			for _, l := range ls {
				b.WriteString(strings.Join(strings.Fields(l), ""))
			}
			return b.String()
		}
		if norm(f.addedText) != norm(f.removedText) {
			return false
		}
	}
	return true
}

var genericDirs = map[string]bool{
	"src": true, "lib": true, "internal": true, "pkg": true, "app": true, "cmd": true,
	"source": true, "include": true, "packages": true, "modules": true, "components": true,
}

// scopeFor picks the most specific directory shared by the files, skipping
// generic ones like src/ or internal/.
func scopeFor(files []File, allowed []string) string {
	counts := map[string]int{}
	for _, f := range files {
		counts[scopeOf(f.Path)]++
	}
	best, n := "", 0
	for s, c := range counts {
		if c > n || c == n && s < best {
			best, n = s, c
		}
	}
	if n*2 <= len(files) && len(files) > 1 { // no clear majority
		best = ""
	}
	if len(allowed) > 0 {
		for _, a := range allowed {
			if strings.EqualFold(a, best) {
				return a
			}
		}
		return ""
	}
	return best
}

func scopeOf(p string) string {
	parts := strings.Split(path.Dir(p), "/")
	for _, d := range parts {
		d = strings.ToLower(d)
		if d == "." || d == "" || genericDirs[d] || strings.HasPrefix(d, ".") {
			continue
		}
		return d
	}
	if len(parts) == 1 && parts[0] == "." {
		// root-level file: use its name without extension, e.g. main.go -> main
		b := path.Base(p)
		if c := classify(p); c == catDocs || c == catBuild || c == catChore {
			return ""
		}
		return strings.TrimSuffix(b, path.Ext(b))
	}
	return ""
}

func summarize(files []File, hint string) string {
	if h := strings.TrimSpace(hint); h != "" {
		h = strings.TrimRight(h, ".")
		first, rest, _ := strings.Cut(h, " ")
		if v, ok := baseVerbs[strings.ToLower(first)]; ok {
			h = strings.TrimSpace(v + " " + rest)
		}
		return lowerFirst(h)
	}
	if len(files) == 1 {
		f := files[0]
		name := path.Base(f.Path)
		switch f.Status {
		case Added:
			return "add " + name
		case Deleted:
			return "remove " + name
		case Renamed:
			return fmt.Sprintf("rename %s to %s", path.Base(f.OldPath), name)
		}
		if len(f.NewSymbols) > 0 {
			return "add " + joinSymbols(f.NewSymbols) + " in " + name
		}
		return "update " + name
	}

	var syms []string
	added, deleted := 0, 0
	for _, f := range files {
		syms = append(syms, f.NewSymbols...)
		switch f.Status {
		case Added:
			added++
		case Deleted:
			deleted++
		}
	}
	where := groupName(files)
	switch {
	case deleted == len(files):
		return "remove " + where
	case added == len(files):
		return "add " + where
	case len(syms) > 0:
		return "add " + joinSymbols(syms)
	}
	return "update " + where
}

// baseVerbs turns "fixes"/"added" into the imperative "fix"/"add".
var baseVerbs = map[string]string{
	"fixes": "fix", "fixed": "fix", "adds": "add", "added": "add", "updates": "update",
	"updated": "update", "removes": "remove", "removed": "remove", "improves": "improve",
	"improved": "improve", "changes": "change", "changed": "change", "implements": "implement",
	"implemented": "implement", "refactors": "refactor", "refactored": "refactor",
}

var conventionalRe = regexp.MustCompile(`^(feat|fix|refactor|perf|docs|test|build|ci|chore|style|revert)(\([^)\n]+\))?!?: \S`)

func groupName(files []File) string {
	dirs := map[string]bool{}
	for _, f := range files {
		dirs[path.Dir(f.Path)] = true
	}
	if len(dirs) == 1 {
		for d := range dirs {
			if d != "." {
				return fmt.Sprintf("%d files in %s", len(files), d)
			}
		}
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, path.Base(f.Path))
	}
	if len(names) <= 3 {
		return humanJoin(names)
	}
	return fmt.Sprintf("%d files", len(files))
}

func joinSymbols(s []string) string {
	if len(s) > 2 {
		return humanJoin(s[:2]) + " and more"
	}
	return humanJoin(s)
}

func humanJoin(s []string) string {
	switch len(s) {
	case 0:
		return ""
	case 1:
		return s[0]
	case 2:
		return s[0] + " and " + s[1]
	}
	return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
}

func bodyFor(files []File, style string) string {
	if len(files) == 1 && len(files[0].NewSymbols) <= 2 {
		return ""
	}
	sorted := append([]File(nil), files...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Added+sorted[i].Removed > sorted[j].Added+sorted[j].Removed
	})
	var b strings.Builder
	const maxLines = 8
	for i, f := range sorted {
		if i == maxLines {
			fmt.Fprintf(&b, "- and %d more file(s)\n", len(sorted)-maxLines)
			break
		}
		verb := map[Status]string{Added: "add", Deleted: "remove", Renamed: "rename", Modified: "update"}[f.Status]
		if style == "simple" {
			verb = capitalize(verb)
		}
		line := "- " + verb + " " + f.Path
		if f.Status == Renamed {
			line = "- " + verb + " " + f.OldPath + " to " + f.Path
		}
		if f.Binary {
			line += " (binary)"
		} else if f.Added+f.Removed > 0 {
			line += fmt.Sprintf(" (+%d -%d)", f.Added, f.Removed)
		}
		if len(f.NewSymbols) > 0 {
			line += ": " + joinSymbols(f.NewSymbols)
		}
		b.WriteString(limit(line, 72) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func limit(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := strings.LastIndex(s[:n-1], " ")
	if cut < n/2 {
		cut = n - 1
	}
	return strings.TrimRight(s[:cut], " ,") + "..."
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func lowerFirst(s string) string {
	if len(s) > 1 && strings.ToUpper(s[:2]) == s[:2] { // keep acronyms like "API"
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
