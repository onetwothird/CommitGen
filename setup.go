package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/onetwothird/CommitGen/internal/config"
	"github.com/onetwothird/CommitGen/internal/llm"
)

// setupChoices is the menu shown by `commitgen setup`, easiest first.
var setupChoices = []struct{ name, label string }{
	{"gemini", "Google Gemini     free tier, no card — good quality, generous daily limit"},
	{"groq", "Groq              free tier, no card — very fast, smaller daily limit"},
	{"openrouter", "OpenRouter        free models, no card — routes to any free model available"},
	{"cerebras", "Cerebras          free tier, no card — very fast"},
	{"github", "GitHub Models     free with a GitHub account (personal access token)"},
	{"mistral", "Mistral           free Experiment plan"},
	{"ollama", "Ollama            local & private; small models run on 4 GB RAM"},
	{"lmstudio", "LM Studio         local & private, desktop app"},
	{"llamacpp", "llama.cpp         local & private, lightest local server"},
	{"openai", "OpenAI / custom   paid, or any OpenAI-compatible URL"},
	{"anthropic", "Anthropic         paid (Claude)"},
	{"offline", "Offline           no AI at all: rule-based, works anywhere"},
}

type prompter struct{ in *bufio.Reader }

func (p prompter) ask(q, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", q, def)
	} else {
		fmt.Printf("%s: ", q)
	}
	line, err := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		if err != nil && def == "" {
			return ""
		}
		return def
	}
	return line
}

func (p prompter) yes(q string, def bool) bool {
	d := "Y/n"
	if !def {
		d = "y/N"
	}
	a := strings.ToLower(p.ask(q+" ("+d+")", ""))
	if a == "" {
		return def
	}
	return a[0] == 'y'
}

// setupCmd walks the user through choosing a provider, entering a key,
// testing it and picking a fallback, then saves the config.
//
//	commitgen setup                 interactive menu
//	commitgen setup gemini          skip the menu
func setupCmd(args []string) error {
	p := prompter{bufio.NewReader(os.Stdin)}
	path := config.UserConfigPath()
	cfg, _ := config.Load("")

	fmt.Println("commitgen setup — choose who writes your commit messages.")
	fmt.Println("Your answers are saved to", path)
	fmt.Println()

	var name string
	if len(args) > 0 {
		pre, ok := llm.Lookup(args[0])
		if !ok {
			return fmt.Errorf("unknown provider %q (choose one of: %s)", args[0], strings.Join(llm.Names(), ", "))
		}
		name = pre.Name
	} else {
		def := 1
		for i, c := range setupChoices {
			mark := "  "
			if pre, _ := llm.Lookup(c.name); pre.NeedsKey() && keyFor(cfg, pre, false) != "" {
				mark = "✓ " // key already available
				if def == 1 {
					def = i + 1
				}
			}
			fmt.Printf("  %2d) %s%s\n", i+1, mark, c.label)
		}
		fmt.Println()
		fmt.Println("  Not sure? Pick 1 (Gemini) — free, no credit card, works on any PC.")
		fmt.Println()
		for name == "" {
			a := p.ask("Choose a number", strconv.Itoa(def))
			if i, err := strconv.Atoi(a); err == nil && i >= 1 && i <= len(setupChoices) {
				name = setupChoices[i-1].name
			} else if pre, ok := llm.Lookup(a); ok {
				name = pre.Name
			} else if a == "" {
				return errors.New("setup cancelled")
			}
		}
	}
	pre, _ := llm.Lookup(name)
	set := map[string]any{"provider": pre.Name, "model": nil, "base_url": nil, "api_key": nil}

	// API key
	if pre.NeedsKey() {
		existing := keyFor(cfg, pre, false)
		envKey, envName := pre.EnvKey()
		fmt.Println()
		switch {
		case envKey != "":
			fmt.Printf("Found your key in $%s — commitgen will use it.\n", envName)
		case existing != "":
			fmt.Println("A key is already saved in your config.")
		default:
			fmt.Printf("%s needs a free API key. Get one here (takes about a minute):\n  %s\n\n", pre.Name, pre.KeyURL)
		}
		if envKey == "" {
			k := p.ask("Paste your API key (Enter to keep current)", "")
			if k == "" && existing == "" {
				fmt.Printf("\nNo key entered. You can set it later with the %s environment variable.\n", pre.KeyEnv[0])
			} else if k != "" {
				set[pre.Name+"_api_key"] = k
				existing = k
			}
		} else {
			existing = envKey
		}
		cfg.Keys[pre.Name] = existing
	}

	// Base URL (custom OpenAI-compatible / local servers)
	base := ""
	if pre.Name == "openai" || pre.Local && pre.Kind != llm.KindOffline {
		fmt.Println()
		def := pre.BaseURL
		if pre.Name == "openai" {
			fmt.Println("Using OpenAI itself? Keep the default. For any other OpenAI-compatible server, paste its /v1 URL.")
		}
		base = p.ask("Base URL", def)
		if base != def {
			set["base_url"] = base
		}
	}

	// Model
	model := modelFor(cfg, pre, false)
	if pre.Kind != llm.KindOffline {
		fmt.Println()
		if pre.Name == "ollama" {
			fmt.Println("Small models for weak PCs: qwen2.5-coder:1.5b (~1 GB, 4 GB RAM), qwen2.5-coder:3b (~2 GB, 8 GB RAM).")
			fmt.Println("Better quality if you have 16 GB RAM or a GPU: qwen2.5-coder:7b. Download with `ollama pull <model>`.")
		}
		m := p.ask("Model", model)
		if m != pre.Model {
			set[pre.Name+"_model"] = m
		}
		model = m
	}

	// Test it
	if pre.Kind != llm.KindOffline && (!pre.NeedsKey() || cfg.Keys[pre.Name] != "") {
		fmt.Println()
		if p.yes("Send a tiny test request now?", true) {
			if base == "" {
				base = pre.BaseURL
			}
			if err := testProvider(pre, base, cfg.Keys[pre.Name], model, cfg.NumCtx); err != nil {
				fmt.Println("  ✗ test failed:", friendly(err))
				fmt.Println("  Settings are still saved; fix the issue above and run `commitgen setup` again.")
			} else {
				fmt.Println("  ✓ it works")
			}
		}
	}

	// Fallback
	if pre.Kind != llm.KindOffline {
		fmt.Println()
		fmt.Println("When", pre.Name, "is unavailable (rate limit, quota, no internet), commitgen can try others.")
		fmt.Println("Offline mode at the end means you always get a message.")
		def := "offline"
		var others []string
		for _, n := range autoOrder {
			if n == pre.Name {
				continue
			}
			if op, _ := llm.Lookup(n); keyFor(cfg, op, false) != "" {
				others = append(others, n)
			}
		}
		if len(others) > 0 {
			def = strings.Join(append(others, "offline"), ", ")
		}
		fb := p.ask("Fallback order (comma separated, or \"none\")", def)
		if strings.EqualFold(fb, "none") {
			set["fallback"] = []string{}
		} else {
			set["fallback"] = config.SplitList(fb)
		}
	} else {
		set["fallback"] = nil
	}

	if err := config.Set(path, set); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Saved", path)
	fmt.Println("All set. Stage some changes and run `commitgen`:")
	fmt.Println("  git add -A")
	fmt.Println("  commitgen")
	return nil
}

func testProvider(pre llm.Preset, base, key, model string, numCtx int) error {
	prov, err := llm.New(llm.Options{Provider: pre.Name, BaseURL: base, APIKey: key, Timeout: 30 * time.Second, NumCtx: numCtx})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stop := spinner("testing " + pre.Name)
	out, err := prov.Generate(ctx, llm.Request{
		Model: model, MaxTokens: 20,
		System: "Reply with exactly: ok", User: "Say ok.",
	}, nil)
	stop()
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return errors.New("the model returned an empty reply")
	}
	return nil
}
