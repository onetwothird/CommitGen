package redact

import "regexp"

type rule struct {
	re   *regexp.Regexp
	repl string
}

var rules = []rule{
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), "[REDACTED PRIVATE KEY]"},
	{regexp.MustCompile(`\bsk-(ant-|proj-)?[A-Za-z0-9_\-]{20,}`), "[REDACTED]"},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{30,}`), "[REDACTED]"},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "[REDACTED]"},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`), "[REDACTED]"},
	{regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`), "[REDACTED]"},
	{regexp.MustCompile(`\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{16,}`), "[REDACTED]"},     // Stripe
	{regexp.MustCompile(`\bgsk_[A-Za-z0-9]{20,}`), "[REDACTED]"},                         // Groq
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{40,}`), "[REDACTED]"},                 // GitHub fine-grained
	{regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20,}`), "[REDACTED]"},                    // GitLab
	{regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}`), "[REDACTED]"},                          // Hugging Face
	{regexp.MustCompile(`\bSG\.[A-Za-z0-9_\-]{16,}\.[A-Za-z0-9_\-]{16,}`), "[REDACTED]"}, // SendGrid
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`), "[REDACTED JWT]"},
	{regexp.MustCompile(`(://[^/\s:@]+:)([^/\s@]{3,})@`), "${1}[REDACTED]@"}, // user:password@host in URLs
	{regexp.MustCompile(`(?i)((?:api[_-]?key|secret|token|password|passwd|pwd|access[_-]?key|client[_-]?secret)["']?\s*[:=]\s*["']?)([^\s"'\[]{6,})`), "${1}[REDACTED]"},
	// Names used in code: stripeKey = "...", dbPassword: '...', MAILGUN_SECRET = "..."
	{regexp.MustCompile("((?:[a-z0-9]+(?:Key|Secret|Token|Password|Passwd|Credential)s?|[A-Za-z0-9]+_(?:KEY|SECRET|TOKEN|PASSWORD))\\s*(?::=|=|:)\\s*[\"'`])([^\"'`\\s\\[]{8,})"), "${1}[REDACTED]"},
}

func Secrets(s string) (string, int) {
	n := 0
	for _, r := range rules {
		s = r.re.ReplaceAllStringFunc(s, func(m string) string {
			n++
			return r.re.ReplaceAllString(m, r.repl)
		})
	}
	return s, n
}
