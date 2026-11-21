package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	Provider     string            // preset name, "auto" or "offline"
	BaseURL      string            // empty = the provider's default
	Model        string            // empty = the provider's default
	APIKey       string            // key for the main provider
	Fallback     []string          // providers to try, in order, when the main one fails
	Keys         map[string]string // <provider>_api_key entries
	Models       map[string]string // <provider>_model entries
	MaxDiff      int
	Temperature  float64
	Style        string
	Language     string
	Exclude      []string
	Scopes       []string
	Instructions string
	History      int
	Timeout      int
	NumCtx       int
	MaxWait      int // seconds to wait out a rate limit before using offline mode
}

func Defaults() Config {
	return Config{
		Provider:    "auto",
		Keys:        map[string]string{},
		Models:      map[string]string{},
		MaxDiff:     12000,
		Temperature: 0.2,
		Style:       "conventional",
		Language:    "English",
		History:     10,
		Timeout:     60,
		NumCtx:      8192,
		MaxWait:     30,
	}
}

func UserConfigPath() string {
	if p := os.Getenv("COMMITGEN_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return preferExisting(filepath.Join(dir, "CommitGen", "config.toml"), filepath.Join(dir, legacyName, "config.toml"))
}

// RepoConfigName is the per-repository config file.
const RepoConfigName = ".CommitGen.toml"

// legacyName is the lowercase name older versions used for config paths.
const legacyName = "commitgen"

// RepoConfigPath returns the repo config file, preferring .CommitGen.toml but
// still reading a .CommitGen.toml written by older versions.
func RepoConfigPath(repoRoot string) string {
	return preferExisting(filepath.Join(repoRoot, RepoConfigName), filepath.Join(repoRoot, "."+legacyName+".toml"))
}

// preferExisting returns legacy only when it exists and current doesn't.
// (On case-insensitive file systems both names are the same file.)
func preferExisting(current, legacy string) string {
	if _, err := os.Stat(current); err == nil {
		return current
	}
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return current
}

func Load(repoRoot string) (Config, error) {
	c := Defaults()
	if err := c.mergeFile(UserConfigPath()); err != nil {
		return c, err
	}
	if repoRoot != "" {
		if err := c.mergeFile(RepoConfigPath(repoRoot)); err != nil {
			return c, err
		}
	}
	c.mergeEnv()
	return c, nil
}

func (c *Config) mergeEnv() {
	set := func(k string, dst *string) {
		if v := os.Getenv(k); v != "" {
			*dst = v
		}
	}
	set("COMMITGEN_PROVIDER", &c.Provider)
	set("COMMITGEN_BASE_URL", &c.BaseURL)
	set("COMMITGEN_MODEL", &c.Model)
	set("COMMITGEN_API_KEY", &c.APIKey)
	set("COMMITGEN_STYLE", &c.Style)
	set("COMMITGEN_LANGUAGE", &c.Language)
	if v := os.Getenv("COMMITGEN_FALLBACK"); v != "" {
		c.Fallback = SplitList(v)
	}
}

// SplitList splits "a, b,c" into ["a" "b" "c"].
func SplitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c *Config) mergeFile(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	kv, err := parseTOML(f)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for k, v := range kv {
		if err := c.apply(k, v); err != nil {
			return fmt.Errorf("%s: %s: %w", path, k, err)
		}
	}
	return nil
}

func (c *Config) apply(key string, v value) error {
	var err error
	switch key {
	case "provider":
		c.Provider, err = v.str()
	case "base_url":
		c.BaseURL, err = v.str()
	case "model":
		c.Model, err = v.str()
	case "api_key":
		c.APIKey, err = v.str()
	case "style":
		c.Style, err = v.str()
	case "language":
		c.Language, err = v.str()
	case "instructions":
		c.Instructions, err = v.str()
	case "max_diff":
		c.MaxDiff, err = v.int()
	case "history":
		c.History, err = v.int()
	case "timeout":
		c.Timeout, err = v.int()
	case "max_wait":
		c.MaxWait, err = v.int()
	case "num_ctx":
		c.NumCtx, err = v.int()
	case "temperature":
		c.Temperature, err = v.float()
	case "exclude":
		c.Exclude, err = v.list()
	case "scopes":
		c.Scopes, err = v.list()
	case "fallback":
		if v.isArr {
			c.Fallback = v.arr
		} else {
			var s string
			s, err = v.str()
			c.Fallback = SplitList(s)
		}
	default:
		if name, ok := strings.CutSuffix(key, "_api_key"); ok && name != "" {
			c.Keys[name], err = v.str()
		} else if name, ok := strings.CutSuffix(key, "_model"); ok && name != "" {
			c.Models[name], err = v.str()
		} else {
			return fmt.Errorf("unknown key")
		}
	}
	return err
}

type value struct {
	raw   string
	arr   []string
	isArr bool
}

func (v value) str() (string, error) {
	if v.isArr {
		return "", fmt.Errorf("expected string, got array")
	}
	return unquote(v.raw)
}
func (v value) int() (int, error)       { return strconv.Atoi(v.raw) }
func (v value) float() (float64, error) { return strconv.ParseFloat(v.raw, 64) }
func (v value) list() ([]string, error) {
	if !v.isArr {
		return nil, fmt.Errorf("expected array")
	}
	return v.arr, nil
}

func unquote(s string) (string, error) {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"') {
		return strconv.Unquote(s)
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1], nil
	}
	return "", fmt.Errorf("expected quoted string, got %s", s)
}

func stripComment(line string) string {
	inStr := byte(0)
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case inStr != 0 && ch == '\\' && inStr == '"':
			i++
		case inStr != 0 && ch == inStr:
			inStr = 0
		case inStr == 0 && (ch == '"' || ch == '\''):
			inStr = ch
		case inStr == 0 && ch == '#':
			return line[:i]
		}
	}
	return line
}

func parseTOML(f *os.File) (map[string]value, error) {
	out := map[string]value{}
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(stripComment(sc.Text()))
		if line == "" || strings.HasPrefix(line, "[") && !strings.Contains(line, "=") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", n)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if strings.HasPrefix(v, "[") {
			if !strings.HasSuffix(v, "]") {
				return nil, fmt.Errorf("line %d: arrays must be on one line", n)
			}
			var items []string
			for _, p := range splitArray(v[1 : len(v)-1]) {
				s, err := unquote(strings.TrimSpace(p))
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", n, err)
				}
				items = append(items, s)
			}
			out[k] = value{arr: items, isArr: true}
			continue
		}
		out[k] = value{raw: v}
	}
	return out, sc.Err()
}

func splitArray(s string) []string {
	var parts []string
	var cur strings.Builder
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inStr == 0 && ch == ',' {
			if t := strings.TrimSpace(cur.String()); t != "" {
				parts = append(parts, t)
			}
			cur.Reset()
			continue
		}
		if inStr == 0 && (ch == '"' || ch == '\'') {
			inStr = ch
		} else if inStr == ch {
			inStr = 0
		}
		cur.WriteByte(ch)
	}
	if t := strings.TrimSpace(cur.String()); t != "" {
		parts = append(parts, t)
	}
	return parts
}

// Set updates (or adds) top-level keys in a config file, keeping comments and
// every other line. Values are written as TOML strings or arrays. A nil value
// removes the key. The file is created with 0600 permissions since it may
// hold API keys.
func Set(path string, kv map[string]any) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		data = []byte(Sample)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	done := map[string]bool{}
	var out []string
	for _, line := range lines {
		t := strings.TrimSpace(stripComment(line))
		k, _, ok := strings.Cut(t, "=")
		k = strings.TrimSpace(k)
		if v, managed := kv[k]; ok && managed {
			if !done[k] && v != nil {
				out = append(out, k+" = "+tomlValue(v))
			}
			done[k] = true
			continue
		}
		out = append(out, line)
	}
	var keys []string
	for k := range kv {
		if !done[k] && kv[k] != nil {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+" = "+tomlValue(kv[k]))
	}
	text := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o600)
}

func tomlValue(v any) string {
	switch x := v.(type) {
	case []string:
		q := make([]string, len(x))
		for i, s := range x {
			q[i] = strconv.Quote(s)
		}
		return "[" + strings.Join(q, ", ") + "]"
	case int:
		return strconv.Itoa(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return strconv.Quote(fmt.Sprint(x))
	}
}

const Sample = `# CommitGen config
# Location: ` + "`CommitGen config path`" + ` prints it. Per-repo overrides go in .CommitGen.toml.
# Easiest way to fill this in: run ` + "`CommitGen setup`" + `.

# Which provider writes the message. Run ` + "`CommitGen providers`" + ` for the full list.
#   auto     - use whatever is available (API keys in env, a running Ollama), else offline
#   offline  - no AI, no key, no internet: rule-based message from the diff
#   groq | gemini | openrouter | cerebras | mistral | github   (free tiers)
#   ollama | lmstudio | llamacpp                                (local)
#   openai | anthropic                                          (paid)
provider = "auto"

# model    = ""        # empty = the provider's default
# base_url = ""        # only for custom OpenAI-compatible servers

# When the provider fails (rate limit, quota, no internet), try these next.
# fallback = ["gemini", "offline"]

# API keys: environment variables are safest (GROQ_API_KEY, GEMINI_API_KEY,
# OPENROUTER_API_KEY, ...). You can also store them here, per provider:
# groq_api_key   = "gsk_..."
# gemini_api_key = "..."
# gemini_model   = "gemini-flash-latest"

style       = "conventional"   # or "simple"
language    = "English"
max_diff    = 12000            # characters of diff sent to the model (smaller = fewer tokens used)
history     = 10               # recent commit subjects used to match repo style
temperature = 0.2
timeout     = 60
num_ctx     = 8192             # Ollama context window
max_wait    = 30               # seconds to wait out a short rate limit before falling back to offline

# exclude = ["docs/generated/*", "*.snap"]
# scopes  = ["api", "ui", "cli"]
# instructions = "Mention the ticket ID from the branch name if present."
`
