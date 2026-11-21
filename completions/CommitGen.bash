# Bash completion for CommitGen.
#
# Load it now:        source <(CommitGen completion bash)
# Load it every time: echo 'source <(CommitGen completion bash)' >> ~/.bashrc

_CommitGen() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    local commands="setup providers pr changelog reword hook config cache completion version help"
    local providers="offline gemini groq openrouter cerebras github mistral ollama lmstudio llamacpp openai anthropic"
    local flags="-c -y -p -a -n -m -v --each --offline --dry-run --no-redact --no-cache --allow-secrets --provider --model --url --style --lang --max --base --create --from --to --release --full --force"

    # values for flags that take an argument
    case "$prev" in
        --provider|-provider)
            COMPREPLY=($(compgen -W "auto $providers" -- "$cur")); return ;;
        --style|-style)
            COMPREPLY=($(compgen -W "conventional simple" -- "$cur")); return ;;
        --lang|-lang)
            COMPREPLY=($(compgen -W "English Spanish Portuguese French German Italian Dutch Japanese Korean Chinese Filipino Indonesian Vietnamese Russian Turkish Arabic Hindi" -- "$cur")); return ;;
        -n)
            COMPREPLY=($(compgen -W "2 3 4 5" -- "$cur")); return ;;
        --base|-base|--from|-from|--to|-to)
            COMPREPLY=($(compgen -W "$(git for-each-ref --format='%(refname:short)' refs/heads refs/remotes refs/tags 2>/dev/null)" -- "$cur")); return ;;
        --model|-model|--url|-url|--max|-max|-m|--release|-release)
            return ;;  # free text
    esac

    # sub-commands
    if [ "$COMP_CWORD" -ge 2 ]; then
        case "${COMP_WORDS[1]}" in
            setup)
                [ "$COMP_CWORD" -eq 2 ] && COMPREPLY=($(compgen -W "$providers" -- "$cur")); return ;;
            hook)
                [ "$COMP_CWORD" -eq 2 ] && COMPREPLY=($(compgen -W "install uninstall" -- "$cur"))
                [ "$COMP_CWORD" -eq 3 ] && COMPREPLY=($(compgen -W "--force" -- "$cur"))
                return ;;
            config)
                [ "$COMP_CWORD" -eq 2 ] && COMPREPLY=($(compgen -W "init path show" -- "$cur")); return ;;
            cache)
                [ "$COMP_CWORD" -eq 2 ] && COMPREPLY=($(compgen -W "clear path" -- "$cur")); return ;;
            completion)
                [ "$COMP_CWORD" -eq 2 ] && COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "$cur")); return ;;
            providers|version|help)
                return ;;
        esac
    fi

    if [ "$COMP_CWORD" -eq 1 ] && [[ "$cur" != -* ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    else
        COMPREPLY=($(compgen -W "$flags" -- "$cur"))
    fi
}

# Also complete the lowercase name used by releases before the rename.
complete -F _CommitGen CommitGen commitgen
