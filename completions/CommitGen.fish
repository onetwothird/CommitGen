# Fish completion for CommitGen.
#
# Load it now:        CommitGen completion fish | source
# Load it every time: CommitGen completion fish > ~/.config/fish/completions/CommitGen.fish

for cmd in CommitGen commitgen
    complete -c $cmd -f

    # sub-commands
    set -l subs setup providers pr changelog reword hook config cache completion version help
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a setup -d 'Pick an AI provider (guided)'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a providers -d 'List providers, costs and keys'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a pr -d 'Write a pull request title and description'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a changelog -d 'Release notes since the last tag'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a reword -d 'Rewrite bad messages of recent commits'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a hook -d 'Install or remove the git hook'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a config -d 'Show or create the config file'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a cache -d 'Clear or locate saved messages'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a completion -d 'Print a shell completion script'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a version -d 'Print the version'
    complete -c $cmd -n "not __fish_seen_subcommand_from $subs" -a help -d 'Show help'

    set -l providers offline gemini groq openrouter cerebras github mistral ollama lmstudio llamacpp openai anthropic
    complete -c $cmd -n "__fish_seen_subcommand_from setup" -a "$providers"
    complete -c $cmd -n "__fish_seen_subcommand_from hook" -a "install uninstall --force"
    complete -c $cmd -n "__fish_seen_subcommand_from config" -a "init path show"
    complete -c $cmd -n "__fish_seen_subcommand_from cache" -a "clear path"
    complete -c $cmd -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell"

    # flags (Go accepts -flag and --flag; long ones are offered with --)
    complete -c $cmd -o c -d 'Commit with the message (opens your editor)'
    complete -c $cmd -o y -d 'With -c: commit without the editor'
    complete -c $cmd -o p -d 'Print the message only'
    complete -c $cmd -o a -d 'Stage all changes first'
    complete -c $cmd -o v -d 'Verbose output'
    complete -c $cmd -o n -x -a '2 3 4 5' -d 'Number of candidates'
    complete -c $cmd -o m -x -d 'Extra context for the message'
    complete -c $cmd -l each -d 'One commit per staged file'
    complete -c $cmd -l offline -d 'No AI: rule-based message'
    complete -c $cmd -l dry-run -d 'Show what would be sent, then exit'
    complete -c $cmd -l no-redact -d 'Do not mask secrets'
    complete -c $cmd -l provider -x -a "auto $providers" -d 'Provider for this run'
    complete -c $cmd -l model -x -d 'Model name'
    complete -c $cmd -l url -x -d 'OpenAI-compatible base URL'
    complete -c $cmd -l style -x -a 'conventional simple' -d 'Message style'
    complete -c $cmd -l lang -x -a 'English Spanish Portuguese French German Italian Dutch Japanese Korean Chinese Filipino Indonesian Vietnamese Russian Turkish Arabic Hindi' -d 'Language of the message'
    complete -c $cmd -l max -x -d 'Max diff characters sent'
    complete -c $cmd -l no-cache -d 'Do not reuse a saved message'
    complete -c $cmd -l allow-secrets -d 'Commit automatically even with a key in the diff'
    complete -c $cmd -l base -x -a '(__fish_git_branches)' -d 'pr: branch to merge into'
    complete -c $cmd -l create -d 'pr: open it with the GitHub CLI'
    complete -c $cmd -l from -x -a '(__fish_git_refs)' -d 'changelog: start of range'
    complete -c $cmd -l to -x -a '(__fish_git_refs)' -d 'changelog: end of range'
    complete -c $cmd -l release -x -d 'changelog: heading, e.g. v1.2.0'
    complete -c $cmd -l full -d 'changelog: every type; reword: every commit'
    complete -c $cmd -l force -d 'reword: allow pushed commits'
end
