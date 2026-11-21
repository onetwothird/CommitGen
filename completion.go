package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

// The scripts live in completions/ so they can be read, linted and packaged
// on their own; they're embedded so `CommitGen completion <shell>` works from
// a single binary.

//go:embed completions/CommitGen.bash
var bashCompletion string

//go:embed completions/_CommitGen
var zshCompletion string

//go:embed completions/CommitGen.fish
var fishCompletion string

//go:embed completions/CommitGen.ps1
var powershellCompletion string

var completions = map[string]*string{
	"bash":       &bashCompletion,
	"zsh":        &zshCompletion,
	"fish":       &fishCompletion,
	"powershell": &powershellCompletion,
	"pwsh":       &powershellCompletion,
}

const completionHelp = `usage: CommitGen completion bash|zsh|fish|powershell

Turn on tab completion for commands, flags and provider names:

  Bash        echo 'source <(CommitGen completion bash)' >> ~/.bashrc
  Zsh         echo 'source <(CommitGen completion zsh)' >> ~/.zshrc
  Fish        CommitGen completion fish > ~/.config/fish/completions/CommitGen.fish
  PowerShell  Add-Content $PROFILE 'CommitGen completion powershell | Out-String | Invoke-Expression'

Then open a new terminal and press Tab after "CommitGen ".`

func completionCmd(args []string) error {
	if len(args) == 0 {
		fmt.Println(completionHelp)
		return nil
	}
	script, ok := completions[strings.ToLower(args[0])]
	if !ok {
		return fmt.Errorf("unknown shell %q\n\n%s", args[0], completionHelp)
	}
	_, err := os.Stdout.WriteString(*script)
	return err
}
