package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/onetwothird/CommitGen/internal/config"
	"github.com/onetwothird/CommitGen/internal/git"
	"github.com/onetwothird/CommitGen/internal/llm"
)

var keyVars = []string{"GROQ_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "OPENROUTER_API_KEY", "CEREBRAS_API_KEY",
	"MISTRAL_API_KEY", "GITHUB_MODELS_TOKEN", "GITHUB_TOKEN", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"}

func isolate(t *testing.T, ollama string) {
	t.Helper()
	for _, k := range keyVars {
		t.Setenv(k, "")
	}
	old := probeOllama
	probeOllama = func(config.Config) (string, bool) { return ollama, ollama != "" }
	t.Cleanup(func() { probeOllama = old })
}

func names(ts []target) string {
	var s []string
	for _, t := range ts {
		s = append(s, t.name)
	}
	return strings.Join(s, ",")
}

func TestAutoWithNothingIsOffline(t *testing.T) {
	isolate(t, "")
	ts, _ := resolve(config.Defaults())
	if names(ts) != "offline" {
		t.Errorf("got %s", names(ts))
	}
}

func TestAutoUsesKeysAndOllama(t *testing.T) {
	isolate(t, "qwen2.5-coder:1.5b")
	t.Setenv("GEMINI_API_KEY", "g")
	t.Setenv("GROQ_API_KEY", "q")
	ts, _ := resolve(config.Defaults())
	if names(ts) != "groq,gemini,ollama,offline" {
		t.Errorf("got %s", names(ts))
	}
	if ts[2].model != "qwen2.5-coder:1.5b" || ts[1].key != "g" {
		t.Errorf("targets = %+v", ts)
	}
}

func TestExplicitProviderWithFallback(t *testing.T) {
	isolate(t, "")
	c := config.Defaults()
	c.Provider = "groq"
	c.Keys["groq"] = "from-config"
	c.Model = "llama-3.1-8b-instant"
	c.Fallback = []string{"gemini", "openrouter", "offline"}
	t.Setenv("OPENROUTER_API_KEY", "o")
	ts, skipped := resolve(c)
	// gemini has no key, so it is skipped with a helpful note
	if names(ts) != "groq,openrouter,offline" {
		t.Errorf("got %s", names(ts))
	}
	if ts[0].key != "from-config" || ts[0].model != "llama-3.1-8b-instant" || ts[1].model != "openrouter/free" {
		t.Errorf("targets = %+v", ts)
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0], "GEMINI_API_KEY") || !strings.Contains(skipped[0], "aistudio") {
		t.Errorf("skipped = %v", skipped)
	}
}

func TestExplicitFallbackWithoutOffline(t *testing.T) {
	isolate(t, "")
	c := config.Defaults()
	c.Provider = "ollama"
	c.Fallback = []string{"lmstudio"}
	ts, _ := resolve(c)
	if names(ts) != "ollama,lmstudio" {
		t.Errorf("an explicit fallback list should be respected: %s", names(ts))
	}
}

func TestLegacyBaseURLSetup(t *testing.T) {
	isolate(t, "")
	// Older README told users to set only COMMITGEN_BASE_URL/MODEL/API_KEY.
	c := config.Defaults()
	c.BaseURL = "https://api.groq.com/openai/v1"
	c.Model = "openai/gpt-oss-20b"
	c.APIKey = "gsk"
	ts, _ := resolve(c)
	if names(ts) != "groq,offline" || ts[0].key != "gsk" || ts[0].base != c.BaseURL {
		t.Errorf("got %+v", ts)
	}
	c.BaseURL = "http://my-server:9000/v1"
	c.APIKey = ""
	ts, _ = resolve(c)
	if names(ts) != "openai,offline" || ts[0].base != "http://my-server:9000/v1" {
		t.Errorf("custom URL without key should be allowed: %+v", ts)
	}
	c.BaseURL = "http://localhost:11434/v1"
	if ts, _ = resolve(c); ts[0].name != "ollama" {
		t.Errorf("ollama URL: %+v", ts)
	}
}

func TestFriendlyErrors(t *testing.T) {
	if e := friendly(git.ErrNotRepo); !strings.Contains(e.Error(), "git init") {
		t.Errorf("not repo: %v", e)
	}
	ce := &llm.ChainError{Names: []string{"groq"}, Errs: []error{&llm.Error{Kind: llm.ErrRateLimit, Base: "groq"}}}
	if e := friendly(ce); !strings.Contains(e.Error(), "fallback") || !strings.Contains(e.Error(), "--offline") {
		t.Errorf("rate limit: %v", e)
	}
	if friendly(nil) != nil {
		t.Error("nil should stay nil")
	}
	other := errors.New("boom")
	if friendly(other) != other {
		t.Error("unknown errors pass through")
	}
}
