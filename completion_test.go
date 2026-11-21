package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/onetwothird/CommitGen/internal/llm"
)

var shells = map[string]string{
	"bash": bashCompletion, "zsh": zshCompletion, "fish": fishCompletion, "powershell": powershellCompletion,
}

// hasFlag reports whether a script offers the flag in any of the forms the
// shells use: -x / --long, or fish's "-o x" / "-l long".
func hasFlag(script, name string) bool {
	forms := []string{"-" + name, "--" + name}
	if len(name) == 1 {
		forms = append(forms, "-o "+name)
	} else {
		forms = append(forms, "-l "+name)
	}
	for _, f := range forms {
		for _, end := range []string{" ", "'", "[", "\"", "\n", "|", ")", "$"} {
			if strings.Contains(script, f+end) {
				return true
			}
		}
	}
	return false
}

func TestCompletionsListEveryFlag(t *testing.T) {
	var f flags
	fs := newFlagSet(&f)
	for shell, script := range shells {
		fs.VisitAll(func(fl *flag.Flag) {
			if fl.Name == "hook" { // internal, used by the git hook only
				return
			}
			if !hasFlag(script, fl.Name) {
				t.Errorf("%s completion is missing flag -%s", shell, fl.Name)
			}
		})
	}
}

func TestCompletionsListEveryProviderAndCommand(t *testing.T) {
	commands := []string{"setup", "providers", "hook", "config", "completion", "version", "help",
		"install", "uninstall", "init", "path", "show", "conventional", "simple"}
	for shell, script := range shells {
		for _, p := range llm.Presets {
			if !strings.Contains(script, p.Name) {
				t.Errorf("%s completion is missing provider %s", shell, p.Name)
			}
		}
		for _, c := range commands {
			if !strings.Contains(script, c) {
				t.Errorf("%s completion is missing %q", shell, c)
			}
		}
		if !strings.Contains(script, "CommitGen") {
			t.Errorf("%s completion does not register CommitGen", shell)
		}
	}
}

func TestCompletionCmd(t *testing.T) {
	if err := completionCmd([]string{"nushell"}); err == nil || !strings.Contains(err.Error(), "powershell") {
		t.Errorf("unknown shell should list the supported ones: %v", err)
	}
	for _, s := range []string{"bash", "zsh", "fish", "powershell", "pwsh", "BASH"} {
		if _, ok := completions[strings.ToLower(s)]; !ok {
			t.Errorf("%s not supported", s)
		}
	}
}
