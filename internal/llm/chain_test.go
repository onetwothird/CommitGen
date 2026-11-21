package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fake struct {
	out   string
	err   error
	calls atomic.Int32
	model string
}

func (f *fake) Generate(_ context.Context, r Request, _ OnToken) (string, error) {
	f.calls.Add(1)
	f.model = r.Model
	return f.out, f.err
}

func TestChainFallsBackAndSticks(t *testing.T) {
	a := &fake{err: &Error{Kind: ErrRateLimit, Base: "a"}}
	b := &fake{out: "feat: b"}
	var notes []string
	c := &Chain{Links: []Link{{Name: "a", Provider: a}, {Name: "b", Model: "mb", Provider: b}},
		Notify: func(m string) { notes = append(notes, m) }}

	got, err := c.Generate(context.Background(), Request{Model: "ignored"}, nil)
	if err != nil || got != "feat: b" {
		t.Fatalf("got %q, %v", got, err)
	}
	if b.model != "mb" {
		t.Errorf("link model not applied: %q", b.model)
	}
	if c.Used() != "b" || len(notes) != 1 || !strings.Contains(notes[0], "rate limit") {
		t.Errorf("used=%s notes=%v", c.Used(), notes)
	}
	// second call goes straight to b
	c.Generate(context.Background(), Request{}, nil)
	if a.calls.Load() != 1 || b.calls.Load() != 2 {
		t.Errorf("calls a=%d b=%d", a.calls.Load(), b.calls.Load())
	}
}

func TestChainAllFail(t *testing.T) {
	c := &Chain{Links: []Link{
		{Name: "a", Provider: &fake{err: &Error{Kind: ErrAuth, Base: "a"}}},
		{Name: "b", Provider: &fake{err: &Error{Kind: ErrUnreachable, Base: "b"}}},
	}}
	_, err := c.Generate(context.Background(), Request{}, nil)
	var ce *ChainError
	if !errors.As(err, &ce) || len(ce.Errs) != 2 {
		t.Fatalf("err = %v", err)
	}
	if KindOf(ce.Errs[0]) != ErrAuth || !strings.Contains(err.Error(), "every provider failed") {
		t.Errorf("err = %v", err)
	}
}

func TestChainStopsOnCancel(t *testing.T) {
	b := &fake{out: "x"}
	c := &Chain{Links: []Link{{Name: "a", Provider: &fake{err: context.Canceled}}, {Name: "b", Provider: b}}}
	if _, err := c.Generate(context.Background(), Request{}, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
	if b.calls.Load() != 0 {
		t.Error("should not fall back after cancel")
	}
}

func TestChainToOffline(t *testing.T) {
	diff := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-old\n+new\n"
	c := &Chain{Links: []Link{{Name: "groq", Provider: &fake{err: &Error{Kind: ErrRateLimit}}}, {Name: "offline", Provider: Offline{}}}}
	got, err := c.Generate(context.Background(), Request{Diff: diff}, nil)
	if err != nil || !strings.HasPrefix(got, "docs") {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestHTTPErrorKinds(t *testing.T) {
	cases := []struct {
		status int
		body   string
		hdr    string
		want   ErrKind
	}{
		{429, `{"error":"Rate limit reached"}`, "12", ErrRateLimit},
		{401, `{"error":"invalid key"}`, "", ErrAuth},
		{403, `{"error":"quota exceeded"}`, "", ErrRateLimit},
		{400, `{"error":{"message":"API key not valid"}}`, "", ErrAuth},
		{402, `insufficient credits`, "", ErrNoCredit},
		{404, `model not found`, "", ErrNotFound},
		{503, `overloaded`, "", ErrServer},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c.hdr != "" {
				w.Header().Set("Retry-After", c.hdr)
			}
			w.WriteHeader(c.status)
			fmt.Fprint(w, c.body)
		}))
		p, _ := New(Options{BaseURL: srv.URL, Timeout: 5 * time.Second})
		_, err := p.Generate(context.Background(), Request{}, nil)
		srv.Close()
		if KindOf(err) != c.want {
			t.Errorf("%d: kind = %v (%v)", c.status, KindOf(err), err)
		}
		var e *Error
		if c.hdr != "" && (!errors.As(err, &e) || e.RetryAfter != 12*time.Second || !strings.Contains(err.Error(), "12s")) {
			t.Errorf("retry-after not parsed: %v", err)
		}
	}
}

func TestUnreachable(t *testing.T) {
	p, _ := New(Options{Provider: "ollama", BaseURL: "http://127.0.0.1:1", Timeout: 2 * time.Second})
	_, err := p.Generate(context.Background(), Request{}, nil)
	if KindOf(err) != ErrUnreachable || !strings.Contains(err.Error(), "local model server") {
		t.Errorf("err = %v", err)
	}
}

func TestPresetsAndNew(t *testing.T) {
	for _, alias := range []string{"google", "GEMINI", "claude", "none", "llamafile"} {
		if _, ok := Lookup(alias); !ok {
			t.Errorf("alias %q not found", alias)
		}
	}
	if _, err := New(Options{Provider: "nope"}); err == nil || !strings.Contains(err.Error(), "gemini") {
		t.Errorf("unknown provider error should list choices: %v", err)
	}
	if _, err := New(Options{Provider: "groq"}); KindOf(err) != ErrAuth {
		t.Errorf("missing key should be ErrAuth: %v", err)
	}
	p, err := New(Options{Provider: "gemini", APIKey: "k"})
	if o, ok := p.(*openAI); err != nil || !ok || !strings.Contains(o.base, "generativelanguage") {
		t.Errorf("gemini preset: %T %v", p, err)
	}
	if p, _ := New(Options{Provider: "offline"}); p == nil {
		t.Error("offline provider missing")
	}
	// local OpenAI-compatible servers need no key
	if _, err := New(Options{Provider: "lmstudio"}); err != nil {
		t.Errorf("lmstudio: %v", err)
	}
	for _, p := range Presets {
		if p.NeedsKey() && p.KeyURL == "" {
			t.Errorf("%s needs a key but has no KeyURL", p.Name)
		}
	}
}

func TestReasoningModelsGetRoom(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		got = string(b[:n])
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"fix: x"}}]}`)
	}))
	defer srv.Close()
	p, _ := New(Options{BaseURL: srv.URL, Timeout: 5 * time.Second})
	p.Generate(context.Background(), Request{Model: "gemini-flash-latest", MaxTokens: 400}, nil)
	if !strings.Contains(got, `"reasoning_effort":"low"`) || !strings.Contains(got, `"max_tokens":2048`) {
		t.Errorf("payload = %s", got)
	}
}

// seq returns errors from errs in order, then out forever.
type seq struct {
	errs  []error
	out   string
	calls int
}

func (s *seq) Generate(context.Context, Request, OnToken) (string, error) {
	s.calls++
	if s.calls <= len(s.errs) {
		return "", s.errs[s.calls-1]
	}
	return s.out, nil
}

func rl(d time.Duration) error { return &Error{Kind: ErrRateLimit, Base: "groq", RetryAfter: d} }

func TestChainWaitsOutShortLimitWhenOnlyOfflineIsLeft(t *testing.T) {
	g := &seq{errs: []error{rl(7500 * time.Millisecond)}, out: "feat: ai"}
	var slept time.Duration
	c := &Chain{Links: []Link{{Name: "groq", Provider: g}, {Name: "offline", Provider: Offline{}}},
		Sleep: func(_ context.Context, d time.Duration) error { slept += d; return nil }}
	got, err := c.Generate(context.Background(), Request{}, nil)
	if err != nil || got != "feat: ai" || g.calls != 2 {
		t.Fatalf("got %q, %v, calls=%d", got, err, g.calls)
	}
	if slept < 7500*time.Millisecond || slept > 9*time.Second {
		t.Errorf("slept %s", slept)
	}
}

func TestChainDoesNotWaitWhenAnotherAIIsAvailable(t *testing.T) {
	g := &seq{errs: []error{rl(5 * time.Second)}, out: "feat: groq"}
	gem := &fake{out: "feat: gemini"}
	c := &Chain{Links: []Link{{Name: "groq", Provider: g}, {Name: "gemini", Provider: gem}, {Name: "offline", Provider: Offline{}}},
		Sleep: func(context.Context, time.Duration) error { t.Fatal("should not sleep"); return nil }}
	if got, _ := c.Generate(context.Background(), Request{}, nil); got != "feat: gemini" {
		t.Errorf("got %q", got)
	}
}

func TestChainLongLimitFallsBackThenRecovers(t *testing.T) {
	now := time.Unix(0, 0)
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\n"
	g := &seq{errs: []error{rl(12 * time.Minute)}, out: "feat: ai"}
	c := &Chain{Links: []Link{{Name: "groq", Provider: g}, {Name: "offline", Provider: Offline{}}},
		Now:   func() time.Time { return now },
		Sleep: func(context.Context, time.Duration) error { t.Fatal("should not sleep 12m"); return nil }}

	got, _ := c.Generate(context.Background(), Request{Diff: diff}, nil)
	if c.Used() != "offline" || g.calls != 1 {
		t.Fatalf("first: %q used=%s calls=%d", got, c.Used(), g.calls)
	}
	// still cooling down: groq is not called again
	c.Generate(context.Background(), Request{Diff: diff}, nil)
	if g.calls != 1 {
		t.Errorf("groq called during cooldown")
	}
	// after the reset time groq is used again (this is what --each needs)
	now = now.Add(13 * time.Minute)
	if got, _ := c.Generate(context.Background(), Request{Diff: diff}, nil); got != "feat: ai" || c.Used() != "groq" {
		t.Errorf("after cooldown: %q used=%s", got, c.Used())
	}
}

func TestChainDisablesBadKey(t *testing.T) {
	bad := &fake{err: &Error{Kind: ErrAuth}}
	ok := &fake{out: "fix: x"}
	c := &Chain{Links: []Link{{Name: "groq", Provider: bad}, {Name: "gemini", Provider: ok}}, Now: func() time.Time { return time.Unix(1e9, 0) }}
	for i := 0; i < 3; i++ {
		c.Generate(context.Background(), Request{}, nil)
	}
	if bad.calls.Load() != 1 || ok.calls.Load() != 3 {
		t.Errorf("bad=%d ok=%d", bad.calls.Load(), ok.calls.Load())
	}
}

func TestRetryAfterFromBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "8")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":{"message":"Rate limit reached on tokens per minute (TPM): Limit 8000, Used 7900. Please try again in 1m7.5s.","code":"rate_limit_exceeded"}}`)
	}))
	defer srv.Close()
	p, _ := New(Options{BaseURL: srv.URL, Timeout: 5 * time.Second})
	_, err := p.Generate(context.Background(), Request{}, nil)
	var e *Error
	if !errors.As(err, &e) || e.RetryAfter != 67500*time.Millisecond {
		t.Errorf("RetryAfter = %v (%v)", e.RetryAfter, err)
	}
}
