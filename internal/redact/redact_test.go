package redact

import (
	"strings"
	"testing"
)

func TestSecrets(t *testing.T) {
	in := `+OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz123456
+aws = "AKIAIOSFODNN7EXAMPLE"
+"password": "hunter2hunter2"
+token: ghp_abcdefghijklmnopqrstuvwxyz0123456789
+name = "commitgen"`
	out, n := Secrets(in)
	for _, leak := range []string{"sk-proj-abc", "AKIAIOSFODNN7EXAMPLE", "hunter2", "ghp_abc"} {
		if strings.Contains(out, leak) {
			t.Errorf("leaked %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, `name = "commitgen"`) {
		t.Error("redacted a harmless line")
	}
	if n < 4 {
		t.Errorf("expected >=4 redactions, got %d", n)
	}
}

// fake joins pieces at run time so the source never contains a complete
// key-shaped string (GitHub push protection rejects those, even fake ones).
func fake(parts ...string) string { return strings.Join(parts, "") }

func TestMoreSecretFormats(t *testing.T) {
	stripe := fake("sk_", "live_", "51Hx", "Demo", "Demo", "Demo", "Demo", "1234")
	groq := fake("gsk_", "abcdefghij", "klmnopqrst", "uvwxyz0123")
	ghpat := fake("github_", "pat_", "11ABCDEFG0123456789_", "abcdefghijklmnopqrstuvwxyz0123")
	jwt := fake("eyJhbGciOiJIUzI1NiJ9", ".", "eyJzdWIiOiIxMjM0NTY3ODkwIn0", ".", "dozjgNryP4J3jVmNHl0w5N")

	in := strings.Join([]string{
		`+const paymentsClient = "` + stripe + `"`, // caught by the Stripe pattern, not the name
		`+const stripeKey = "` + fake("PLACEHOLDER", "_VALUE_1234") + `"`,
		`+GROQ = ` + groq,
		`+t := "` + ghpat + `"`,
		`+DATABASE_URL=postgres://admin:s3cretpass@db.example.com:5432/app`,
		`+auth: ` + jwt,
		`+dbPassword: 'correct-horse-battery'`,
		`+const MAILGUN_SECRET = "key-0123456789abcdef"`,
	}, "\n")
	out, n := Secrets(in)
	for _, leak := range []string{stripe, "PLACEHOLDER", groq, ghpat, "s3cretpass", jwt, "correct-horse", "key-0123456789"} {
		if strings.Contains(out, leak) {
			t.Errorf("leaked %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "db.example.com") || !strings.Contains(out, "stripeKey") {
		t.Errorf("redacted too much:\n%s", out)
	}
	if n != 8 {
		t.Errorf("each secret should be counted once: got %d, want 8\n%s", n, out)
	}
	// ordinary code must survive
	safe := `+monkey = "bananas and more"` + "\n" + `+const keyboardLayout = "qwertyuiop"` + "\n" + `+url := "https://example.com/a:b@c"`
	if got, n := Secrets(safe); n != 0 {
		t.Errorf("false positive (%d):\n%s", n, got)
	}
}
