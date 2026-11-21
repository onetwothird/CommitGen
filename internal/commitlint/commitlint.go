// Package commitlint reads the rules from a repo's commitlint config (JSON,
// YAML, JavaScript or package.json) so CommitGen writes messages that pass
// the repo's commit-msg check. Only the rules that shape a message are read:
// type-enum, scope-enum and header-max-length.
//
// JavaScript configs are not executed: the rules are read from the text, which
// covers the usual `rules: { 'type-enum': [2, 'always', [...]] }` form.
package commitlint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Files are checked in this order; the first one found is used.
var Files = []string{
	".commitlintrc", ".commitlintrc.json", ".commitlintrc.yaml", ".commitlintrc.yml",
	".commitlintrc.js", ".commitlintrc.cjs", ".commitlintrc.mjs", ".commitlintrc.ts", ".commitlintrc.cts",
	"commitlint.config.js", "commitlint.config.cjs", "commitlint.config.mjs", "commitlint.config.ts", "commitlint.config.cts",
}

type Rules struct {
	File      string   // config file the rules came from ("" = none found)
	Types     []string // allowed types (empty = any)
	Scopes    []string // allowed scopes (empty = any)
	HeaderMax int      // max first-line length (0 = no limit)
}

// Found reports whether a commitlint config was found.
func (r Rules) Found() bool { return r.File != "" }

// Load reads the commitlint config in root. A repo without one returns the
// zero Rules and no error.
func Load(root string) (Rules, error) {
	if root == "" {
		return Rules{}, nil
	}
	for _, name := range Files {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			r := Parse(string(b))
			r.File = name
			return r, nil
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return Rules{}, nil
	}
	var pkg map[string]json.RawMessage
	if json.Unmarshal(b, &pkg) != nil || pkg["commitlint"] == nil {
		return Rules{}, nil
	}
	r := Parse(string(pkg["commitlint"]))
	r.File = "package.json"
	return r, nil
}

var (
	quotedRe = regexp.MustCompile(`'([^']*)'|"([^"]*)"|` + "`([^`]*)`")
	wordRe   = regexp.MustCompile(`[A-Za-z][\w.-]*`)
	numRe    = regexp.MustCompile(`\b\d+\b`)
)

// Parse extracts the rules from the text of a config file.
func Parse(text string) Rules {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var r Rules
	if v, ok := ruleValue(text, "type-enum"); ok {
		r.Types = enumValues(v)
	}
	if v, ok := ruleValue(text, "scope-enum"); ok {
		r.Scopes = enumValues(v)
	}
	if v, ok := ruleValue(text, "header-max-length"); ok {
		if nums := numRe.FindAllString(v, -1); len(nums) >= 2 && nums[0] != "0" && !strings.Contains(v, "never") {
			r.HeaderMax, _ = strconv.Atoi(nums[len(nums)-1])
		}
	}
	return r
}

// ruleValue returns the raw value written after the rule key: a bracketed
// array (JSON, JS, YAML flow style) or an indented YAML block.
func ruleValue(text, key string) (string, bool) {
	re := regexp.MustCompile(`(?m)^([ \t]*)['"]?` + regexp.QuoteMeta(key) + `['"]?[ \t]*:`)
	loc := re.FindStringSubmatchIndex(text)
	if loc == nil {
		// JS/JSON written on one line: { "type-enum": [...] }
		re = regexp.MustCompile(`['"]` + regexp.QuoteMeta(key) + `['"]\s*:`)
		l := re.FindStringIndex(text)
		if l == nil {
			return "", false
		}
		loc = []int{l[0], l[1], l[0], l[0]}
	}
	indent := len(text[loc[2]:loc[3]])
	rest := text[loc[1]:]
	trimmed := strings.TrimLeft(rest, " \t")
	if strings.HasPrefix(trimmed, "[") {
		depth := 0
		for i, ch := range trimmed {
			switch ch {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					return trimmed[:i+1], true
				}
			}
		}
		return trimmed, true
	}
	line, after, _ := strings.Cut(rest, "\n")
	if strings.TrimSpace(line) != "" {
		return line, true
	}
	// YAML block: every following line indented deeper than the key.
	var b strings.Builder
	for _, l := range strings.Split(after, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if len(l)-len(strings.TrimLeft(l, " \t")) <= indent {
			break
		}
		b.WriteString(l + "\n")
	}
	return b.String(), b.Len() > 0
}

// enumValues pulls the allowed values out of [level, 'always', [values]].
// A disabled rule (level 0) or a "never" rule gives no restriction.
func enumValues(v string) []string {
	if nums := numRe.FindAllString(v, 1); len(nums) == 1 && nums[0] == "0" {
		return nil
	}
	var vals []string
	if qs := quotedRe.FindAllStringSubmatch(v, -1); len(qs) > 0 {
		for _, q := range qs {
			vals = append(vals, q[1]+q[2]+q[3])
		}
	} else {
		vals = wordRe.FindAllString(v, -1)
	}
	var out []string
	for _, s := range vals {
		switch s = strings.TrimSpace(s); s {
		case "never":
			return nil
		case "always", "":
		default:
			out = append(out, s)
		}
	}
	return out
}

var headerRe = regexp.MustCompile(`^(\w[\w-]*)(?:\(([^)]*)\))?!?: `)

// Problems lists the rules msg breaks, in words a model (or a person) can act on.
func (r Rules) Problems(msg string) []string {
	header, _, _ := strings.Cut(strings.TrimSpace(msg), "\n")
	var out []string
	if r.HeaderMax > 0 && len(header) > r.HeaderMax {
		out = append(out, fmt.Sprintf("the first line is %d characters; the limit is %d", len(header), r.HeaderMax))
	}
	m := headerRe.FindStringSubmatch(header)
	if m == nil {
		return out
	}
	if len(r.Types) > 0 && !contains(r.Types, m[1]) {
		out = append(out, fmt.Sprintf("type %q is not allowed; use one of: %s", m[1], strings.Join(r.Types, ", ")))
	}
	if len(r.Scopes) > 0 && m[2] != "" {
		for _, s := range strings.Split(m[2], ",") {
			if s = strings.TrimSpace(s); !contains(r.Scopes, s) {
				out = append(out, fmt.Sprintf("scope %q is not allowed; use one of: %s", s, strings.Join(r.Scopes, ", ")))
			}
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
