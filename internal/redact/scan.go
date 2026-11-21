package redact

import (
	"regexp"
	"strconv"
	"strings"
)

// Finding is a possible secret on an added line of a diff.
type Finding struct {
	File   string
	Line   int    // line number in the new file (0 if unknown)
	Kind   string // e.g. "GitHub token"
	Strong bool   // a known key format, not just a suspicious variable name
}

type detector struct {
	kind   string
	re     *regexp.Regexp
	strong bool
}

// Known key formats are "strong": they are almost never placeholders.
// Name-based rules ("password = ...") are weak because they also match
// examples and ${ENV} references, so they only warn.
var detectors = []detector{
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`), true},
	{"OpenAI/Anthropic key", regexp.MustCompile(`\bsk-(ant-|proj-)?[A-Za-z0-9_\-]{20,}`), true},
	{"GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{30,}|\bgithub_pat_[A-Za-z0-9_]{40,}`), true},
	{"AWS access key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), true},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`), true},
	{"Slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`), true},
	{"Stripe key", regexp.MustCompile(`\b(?:sk|rk)_live_[A-Za-z0-9]{16,}`), true},
	{"Stripe test key", regexp.MustCompile(`\b(?:sk|rk)_test_[A-Za-z0-9]{16,}`), false},
	{"Groq key", regexp.MustCompile(`\bgsk_[A-Za-z0-9]{20,}`), true},
	{"GitLab token", regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20,}`), true},
	{"Hugging Face token", regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}`), true},
	{"SendGrid key", regexp.MustCompile(`\bSG\.[A-Za-z0-9_\-]{16,}\.[A-Za-z0-9_\-]{16,}`), true},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`), false},
	{"password in URL", regexp.MustCompile(`://[^/\s:@]+:[^/\s@$<{]{3,}@`), false},
	{"secret value", regexp.MustCompile(`(?i)(?:api[_-]?key|secret|token|password|passwd|access[_-]?key|client[_-]?secret)["']?\s*[:=]\s*["'][^\s"'$<{\[]{8,}["']`), false},
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)

// Scan reports possible secrets on lines the diff adds. Removed lines and
// context are ignored: only what would be newly committed matters.
func Scan(diff string) []Finding {
	var out []Finding
	file, line := "", 0
	for _, l := range strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(l, "+++ "), "b/")
		case strings.HasPrefix(l, "diff --git "), strings.HasPrefix(l, "--- "):
		case strings.HasPrefix(l, "@@"):
			if m := hunkRe.FindStringSubmatch(l); m != nil {
				line, _ = strconv.Atoi(m[1])
			}
		case strings.HasPrefix(l, "+"):
			for _, d := range detectors {
				if d.re.MatchString(l) {
					out = append(out, Finding{File: file, Line: line, Kind: d.kind, Strong: d.strong})
					break
				}
			}
			line++
		case strings.HasPrefix(l, "-"):
		default:
			line++
		}
	}
	return out
}

// AnyStrong reports whether a finding is a known key format.
func AnyStrong(fs []Finding) bool {
	for _, f := range fs {
		if f.Strong {
			return true
		}
	}
	return false
}
