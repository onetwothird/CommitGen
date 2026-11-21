package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clean unsets every env var Load reads, so the developer's own settings
// can't leak into tests.
func clean(t *testing.T) {
	t.Helper()
	for _, k := range []string{"COMMITGEN_PROVIDER", "COMMITGEN_BASE_URL", "COMMITGEN_MODEL", "COMMITGEN_API_KEY",
		"COMMITGEN_STYLE", "COMMITGEN_LANGUAGE", "COMMITGEN_FALLBACK", "COMMITGEN_CONFIG"} {
		t.Setenv(k, "")
	}
}

func TestLoadMergesFilesAndEnv(t *testing.T) {
	clean(t)
	dir := t.TempDir()
	user := filepath.Join(dir, "user.toml")
	os.WriteFile(user, []byte(`
model = "llama3.2"   # comment
max_diff = 5000
temperature = 0.5
exclude = ["*.snap", 'docs/*']
`), 0o644)
	repo := filepath.Join(dir, "repo")
	os.Mkdir(repo, 0o755)
	os.WriteFile(filepath.Join(repo, ".commitgen.toml"), []byte(`scopes = ["api", "ui"]
style = "simple"`), 0o644)

	t.Setenv("COMMITGEN_CONFIG", user)
	t.Setenv("COMMITGEN_BASE_URL", "http://example:1234/v1")

	c, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "llama3.2" || c.MaxDiff != 5000 || c.Temperature != 0.5 {
		t.Errorf("user file not applied: %+v", c)
	}
	if len(c.Exclude) != 2 || c.Exclude[1] != "docs/*" {
		t.Errorf("exclude = %v", c.Exclude)
	}
	if c.Style != "simple" || len(c.Scopes) != 2 {
		t.Errorf("repo file not applied: %+v", c)
	}
	if c.BaseURL != "http://example:1234/v1" {
		t.Errorf("env not applied: %s", c.BaseURL)
	}
}

func TestDefaults(t *testing.T) {
	clean(t)
	t.Setenv("COMMITGEN_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider != "auto" || c.BaseURL != "" || c.Model != "" || c.Timeout != 60 {
		t.Errorf("defaults = %+v", c)
	}
}

func TestProviderKeysModelsAndFallback(t *testing.T) {
	clean(t)
	f := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(f, []byte(`provider = "groq"
fallback = ["gemini", "offline"]
groq_api_key = "gsk_x"
gemini_model = "gemini-2.5-flash"
`), 0o644)
	t.Setenv("COMMITGEN_CONFIG", f)
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider != "groq" || strings.Join(c.Fallback, ",") != "gemini,offline" ||
		c.Keys["groq"] != "gsk_x" || c.Models["gemini"] != "gemini-2.5-flash" {
		t.Errorf("got %+v", c)
	}
	t.Setenv("COMMITGEN_FALLBACK", "openrouter, offline")
	c, _ = Load("")
	if strings.Join(c.Fallback, ",") != "openrouter,offline" {
		t.Errorf("env fallback = %v", c.Fallback)
	}
}

func TestUnknownKey(t *testing.T) {
	clean(t)
	f := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(f, []byte(`modle = "x"`), 0o644)
	t.Setenv("COMMITGEN_CONFIG", f)
	if _, err := Load(""); err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestSetKeepsCommentsAndRoundTrips(t *testing.T) {
	clean(t)
	f := filepath.Join(t.TempDir(), "sub", "config.toml")
	if err := Set(f, map[string]any{"provider": "gemini", "gemini_api_key": "abc", "fallback": []string{"groq", "offline"}}); err != nil {
		t.Fatal(err)
	}
	// second write replaces values instead of duplicating them, and nil removes
	if err := Set(f, map[string]any{"provider": "groq", "gemini_api_key": nil, "max_diff": 4000}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(f)
	text := string(data)
	if strings.Count(text, "provider = ") != 1 || strings.Contains(text, `gemini_api_key = "abc"`) {
		t.Errorf("file:\n%s", text)
	}
	if !strings.Contains(text, "# commitgen config") {
		t.Error("comments from the sample were lost")
	}
	t.Setenv("COMMITGEN_CONFIG", f)
	c, err := Load("")
	if err != nil {
		t.Fatalf("written file does not load: %v\n%s", err, text)
	}
	if c.Provider != "groq" || c.MaxDiff != 4000 || strings.Join(c.Fallback, ",") != "groq,offline" {
		t.Errorf("round trip: %+v", c)
	}
}

func TestSampleLoads(t *testing.T) {
	clean(t)
	f := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(f, []byte(Sample), 0o644)
	t.Setenv("COMMITGEN_CONFIG", f)
	if _, err := Load(""); err != nil {
		t.Errorf("sample config does not parse: %v", err)
	}
}
