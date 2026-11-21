package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/onetwothird/CommitGen/internal/config"
	"github.com/onetwothird/CommitGen/internal/git"
	"github.com/onetwothird/CommitGen/internal/llm"
)

// target is one resolved provider: preset + model + key + URL.
type target struct {
	name, model, base, key string
	preset                 llm.Preset
}

// keyFor finds the API key for a provider: <name>_api_key in config, then the
// preset's env vars, then (for the main provider only) api_key /
// COMMITGEN_API_KEY.
func keyFor(c config.Config, p llm.Preset, main bool) string {
	if k := c.Keys[p.Name]; k != "" {
		return k
	}
	if k, _ := p.EnvKey(); k != "" {
		return k
	}
	if main {
		return c.APIKey
	}
	return ""
}

func modelFor(c config.Config, p llm.Preset, main bool) string {
	if p.Kind == llm.KindOffline {
		return ""
	}
	if main && c.Model != "" {
		return c.Model
	}
	if m := c.Models[p.Name]; m != "" {
		return m
	}
	return p.Model
}

// mainProvider returns the effective main provider name. Old setups that only
// set base_url (e.g. COMMITGEN_BASE_URL for Groq) keep working.
func mainProvider(c config.Config) string {
	p := strings.ToLower(strings.TrimSpace(c.Provider))
	if p == "" || p == "auto" {
		if c.BaseURL != "" {
			if llm.IsOllama(c.BaseURL) {
				return "ollama"
			}
			return guessPreset(c.BaseURL)
		}
		return "auto"
	}
	return p
}

// guessPreset maps a base URL to a preset name ("openai" for anything unknown).
func guessPreset(base string) string {
	for _, p := range llm.Presets {
		if p.BaseURL != "" && !p.Local && strings.HasPrefix(strings.TrimRight(base, "/"), p.BaseURL) {
			return p.Name
		}
	}
	return "openai"
}

// autoOrder is the order `provider = "auto"` tries hosted providers in, when
// their keys are present.
var autoOrder = []string{"groq", "gemini", "cerebras", "openrouter", "mistral", "github", "openai", "anthropic"}

// resolve turns the config into an ordered list of providers to try, plus
// notes about entries that were skipped (e.g. missing key).
func resolve(c config.Config) (targets []target, skipped []string) {
	seen := map[string]bool{}
	add := func(name string, main bool) {
		p, ok := llm.Lookup(name)
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s: unknown provider (see `CommitGen providers`)", name))
			return
		}
		if seen[p.Name] && p.Name != "openai" {
			return
		}
		seen[p.Name] = true
		t := target{name: p.Name, preset: p, model: modelFor(c, p, main), key: keyFor(c, p, main), base: p.BaseURL}
		if main && c.BaseURL != "" {
			t.base = c.BaseURL
		}
		if p.Kind == llm.KindOllama && t.base == "" {
			t.base = "http://localhost:11434"
		}
		custom := main && c.BaseURL != "" && strings.TrimRight(c.BaseURL, "/") != p.BaseURL
		if p.NeedsKey() && t.key == "" && !custom {
			skipped = append(skipped, fmt.Sprintf("%s: no API key — set %s or run `CommitGen setup %s` (free key: %s)",
				p.Name, p.KeyEnv[0], p.Name, p.KeyURL))
			return
		}
		targets = append(targets, t)
	}

	main := mainProvider(c)
	if main == "auto" {
		for _, n := range autoOrder {
			p, _ := llm.Lookup(n)
			if keyFor(c, p, false) != "" {
				add(n, false)
			}
		}
		if m, ok := probeOllama(c); ok {
			p, _ := llm.Lookup("ollama")
			targets = append(targets, target{name: "ollama", preset: p, model: m, base: "http://localhost:11434"})
			seen["ollama"] = true
		}
	} else {
		add(main, true)
	}
	for _, f := range c.Fallback {
		add(f, false)
	}
	if !seen["offline"] && (main == "auto" || len(c.Fallback) == 0 || len(targets) == 0) {
		// Always end with the offline generator unless the user wrote an
		// explicit fallback list without it.
		add("offline", false)
	}
	return targets, skipped
}

// probeOllama is replaced in tests.
var probeOllama = ollamaModel

// ollamaModel checks (quickly) whether Ollama is running and picks an
// installed model: the configured one if present, else a known good small
// coder model, else the first one installed.
func ollamaModel(c config.Config) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:11434/api/tags", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	var tags struct {
		Models []struct{ Name string } `json:"models"`
	}
	if json.NewDecoder(resp.Body).Decode(&tags) != nil || len(tags.Models) == 0 {
		return "", false
	}
	have := map[string]bool{}
	for _, m := range tags.Models {
		have[m.Name] = true
	}
	prefs := []string{c.Models["ollama"], "qwen2.5-coder:7b", "qwen2.5-coder:3b", "qwen2.5-coder:1.5b",
		"llama3.2:3b", "qwen2.5:3b", "llama3.2:1b", "qwen2.5-coder:0.5b"}
	for _, m := range prefs {
		if m != "" && (have[m] || have[m+":latest"]) {
			return m, true
		}
	}
	return tags.Models[0].Name, true
}

func buildChain(c config.Config, verbose bool) (*llm.Chain, []target, error) {
	targets, skipped := resolve(c)
	for _, s := range skipped {
		if verbose || len(targets) == 0 || !strings.HasPrefix(s, "offline") {
			warn("skipping " + s)
		}
	}
	if len(targets) == 0 {
		return nil, nil, errors.New("no provider is usable. Run `CommitGen setup`, or use `CommitGen --offline` for a no-AI message")
	}
	ch := &llm.Chain{Notify: func(m string) { warn(m) }, MaxWait: time.Duration(c.MaxWait) * time.Second}
	if c.MaxWait <= 0 {
		ch.MaxWait = -1 // never wait
	}
	for _, t := range targets {
		p, err := llm.New(llm.Options{
			Provider: t.name, BaseURL: t.base, APIKey: t.key,
			Timeout: time.Duration(c.Timeout) * time.Second, NumCtx: c.NumCtx,
		})
		if err != nil {
			warn(fmt.Sprintf("skipping %s: %v", t.name, err))
			continue
		}
		ch.Links = append(ch.Links, llm.Link{Name: t.name, Model: t.model, Provider: p})
	}
	if len(ch.Links) == 0 {
		return nil, nil, errors.New("no provider is usable. Run `CommitGen setup`")
	}
	return ch, targets, nil
}

func describe(targets []target) string {
	var parts []string
	for _, t := range targets {
		s := t.name
		if t.model != "" {
			s += " (" + t.model + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " → ")
}

func warn(msg string) { fmt.Fprintln(os.Stderr, "CommitGen:", msg) }

// friendly rewrites common errors with what to do next.
func friendly(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, git.ErrNotRepo):
		return errors.New("this folder is not a git repository.\n  cd into your project first, or run `git init` to start one")
	case errors.Is(err, git.ErrNoGit):
		return errors.New("git is not installed or not on your PATH.\n  Install it from https://git-scm.com/downloads and reopen your terminal")
	case errors.Is(err, context.Canceled):
		return errors.New("cancelled")
	}
	var ce *llm.ChainError
	if errors.As(err, &ce) {
		hint := "\n  Tips:"
		kinds := map[llm.ErrKind]bool{}
		for _, e := range ce.Errs {
			kinds[llm.KindOf(e)] = true
		}
		if kinds[llm.ErrRateLimit] || kinds[llm.ErrNoCredit] {
			hint += "\n  - free-tier limit hit: add another free provider as a fallback (`CommitGen setup`)," +
				"\n    lower max_diff to use fewer tokens, or run `CommitGen --offline` for now"
		}
		if kinds[llm.ErrAuth] {
			hint += "\n  - check the API key (`CommitGen providers` shows where to get one)"
		}
		if kinds[llm.ErrUnreachable] {
			hint += "\n  - check your internet connection, or start your local model server"
		}
		if kinds[llm.ErrNotFound] {
			hint += "\n  - check the model name (--model) and base_url"
		}
		hint += "\n  - add `\"offline\"` to fallback so you always get a message"
		return errors.New(err.Error() + hint)
	}
	return err
}

func providersCmd() error {
	cfg, _ := config.Load(repoRootOrEmpty())
	fmt.Println("Providers (use with `CommitGen setup <name>` or `provider = \"<name>\"`):")
	fmt.Println()
	fmt.Printf("  %-11s %-28s %-24s %s\n", "NAME", "COST", "DEFAULT MODEL", "STATUS")
	main := mainProvider(cfg)
	for _, p := range llm.Presets {
		status := "ready"
		if p.NeedsKey() {
			// The main provider may use api_key / COMMITGEN_API_KEY.
			if keyFor(cfg, p, p.Name == main) == "" {
				status = "needs key: " + p.KeyEnv[0]
			} else {
				status = "key found"
			}
		} else if p.Local && p.Kind != llm.KindOffline {
			status = "needs local server"
		}
		model := p.Model
		switch {
		case p.Kind == llm.KindOffline:
			model = "-"
		case model == "":
			model = "(loaded model)"
		}
		fmt.Printf("  %-11s %-28s %-24s %s\n", p.Name, p.Free, model, status)
	}
	fmt.Println()
	fmt.Println("Where to get free keys:")
	for _, p := range llm.Presets {
		if p.KeyURL != "" {
			fmt.Printf("  %-11s %s\n", p.Name, p.KeyURL)
		}
	}
	return nil
}

func repoRootOrEmpty() string {
	r, _ := git.RepoRoot()
	return r
}
