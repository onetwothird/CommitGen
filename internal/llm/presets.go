package llm

import (
	"os"
	"sort"
	"strings"
)

// Kind is the wire protocol a provider speaks.
type Kind string

const (
	KindOpenAI    Kind = "openai"    // any OpenAI-compatible /chat/completions API
	KindAnthropic Kind = "anthropic" // Anthropic Messages API
	KindOllama    Kind = "ollama"    // Ollama native /api/chat
	KindOffline   Kind = "offline"   // no AI: rule-based message from the diff
)

// Preset describes a ready-to-use provider so users only have to pick a name
// (and maybe paste a key) instead of looking up URLs and model ids.
type Preset struct {
	Name    string
	Kind    Kind
	BaseURL string
	Model   string   // default model
	KeyEnv  []string // env vars checked for the API key, in order
	KeyURL  string   // where to get a key
	Local   bool     // runs on this machine
	Free    string   // short note about cost
	Notes   string   // one-line description for `CommitGen providers`
}

// NeedsKey reports whether the preset requires an API key.
func (p Preset) NeedsKey() bool { return len(p.KeyEnv) > 0 }

// EnvKey returns the first API key found in the preset's env vars.
func (p Preset) EnvKey() (string, string) {
	for _, e := range p.KeyEnv {
		if v := strings.TrimSpace(os.Getenv(e)); v != "" {
			return v, e
		}
	}
	return "", ""
}

// Presets is the built-in provider catalog. Hosted entries are listed roughly
// from most generous free tier to paid.
var Presets = []Preset{
	{
		Name: "offline", Kind: KindOffline, Local: true,
		Free:  "free, no key, no internet",
		Notes: "rule-based message from file paths and +/- lines; works anywhere",
	},
	{
		Name: "groq", Kind: KindOpenAI, BaseURL: "https://api.groq.com/openai/v1",
		Model: "openai/gpt-oss-20b", KeyEnv: []string{"GROQ_API_KEY"},
		KeyURL: "https://console.groq.com/keys",
		Free:   "free tier, no card", Notes: "very fast; daily token limits on the free tier",
	},
	{
		Name: "gemini", Kind: KindOpenAI, BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
		Model: "gemini-flash-latest", KeyEnv: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
		KeyURL: "https://aistudio.google.com/apikey",
		Free:   "free tier, no card", Notes: "Google AI Studio key; generous daily free quota",
	},
	{
		Name: "openrouter", Kind: KindOpenAI, BaseURL: "https://openrouter.ai/api/v1",
		Model: "openrouter/free", KeyEnv: []string{"OPENROUTER_API_KEY"},
		KeyURL: "https://openrouter.ai/keys",
		Free:   "free models, no card", Notes: "`openrouter/free` routes to whichever free model is available",
	},
	{
		Name: "cerebras", Kind: KindOpenAI, BaseURL: "https://api.cerebras.ai/v1",
		Model: "llama3.1-8b", KeyEnv: []string{"CEREBRAS_API_KEY"},
		KeyURL: "https://cloud.cerebras.ai",
		Free:   "free tier, no card", Notes: "very fast, large daily free token allowance",
	},
	{
		Name: "mistral", Kind: KindOpenAI, BaseURL: "https://api.mistral.ai/v1",
		Model: "mistral-small-latest", KeyEnv: []string{"MISTRAL_API_KEY"},
		KeyURL: "https://console.mistral.ai/api-keys",
		Free:   "free Experiment plan", Notes: "needs phone verification for the free plan",
	},
	{
		Name: "github", Kind: KindOpenAI, BaseURL: "https://models.github.ai/inference",
		Model: "openai/gpt-4o-mini", KeyEnv: []string{"GITHUB_MODELS_TOKEN", "GITHUB_TOKEN"},
		KeyURL: "https://github.com/settings/personal-access-tokens (needs models:read)",
		Free:   "free with a GitHub account", Notes: "GitHub Models; rate-limited free usage",
	},
	{
		Name: "ollama", Kind: KindOllama, BaseURL: "http://localhost:11434",
		Model: "qwen2.5-coder:1.5b", Local: true,
		Free: "free, private", Notes: "local; 1.5b runs on ~4 GB RAM, 7b needs ~8 GB+",
	},
	{
		Name: "lmstudio", Kind: KindOpenAI, BaseURL: "http://localhost:1234/v1", Local: true,
		Free: "free, private", Notes: "local; uses whatever model LM Studio has loaded",
	},
	{
		Name: "llamacpp", Kind: KindOpenAI, BaseURL: "http://localhost:8080/v1", Local: true,
		Free: "free, private", Notes: "local llama.cpp / llamafile server; lightest local option",
	},
	{
		Name: "openai", Kind: KindOpenAI, BaseURL: "https://api.openai.com/v1",
		Model: "gpt-4o-mini", KeyEnv: []string{"OPENAI_API_KEY"},
		KeyURL: "https://platform.openai.com/api-keys",
		Free:   "paid (cheap)", Notes: "also used for any custom OpenAI-compatible base_url",
	},
	{
		Name: "anthropic", Kind: KindAnthropic, BaseURL: "https://api.anthropic.com",
		Model: "claude-haiku-4-5", KeyEnv: []string{"ANTHROPIC_API_KEY"},
		KeyURL: "https://console.anthropic.com/settings/keys",
		Free:   "paid (cheap)", Notes: "Claude models",
	},
}

var aliases = map[string]string{
	"none": "offline", "heuristic": "offline", "local": "offline",
	"google": "gemini", "claude": "anthropic", "lm-studio": "lmstudio",
	"llama.cpp": "llamacpp", "llamafile": "llamacpp", "github-models": "github",
	"openai-compatible": "openai", "custom": "openai",
}

// Lookup finds a preset by name or alias (case-insensitive).
func Lookup(name string) (Preset, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if a, ok := aliases[n]; ok {
		n = a
	}
	for _, p := range Presets {
		if p.Name == n {
			return p, true
		}
	}
	return Preset{}, false
}

// Names lists preset names, sorted.
func Names() []string {
	var out []string
	for _, p := range Presets {
		out = append(out, p.Name)
	}
	sort.Strings(out)
	return out
}
