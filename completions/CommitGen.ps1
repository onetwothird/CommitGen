# PowerShell completion for CommitGen.
#
# Load it now:        CommitGen completion powershell | Out-String | Invoke-Expression
# Load it every time: Add-Content $PROFILE 'CommitGen completion powershell | Out-String | Invoke-Expression'

$commitGenCompleter = {
    param($wordToComplete, $commandAst, $cursorPosition)

    $providers = @(
        @('offline', 'No AI, no key, no internet'),
        @('gemini', 'Google Gemini (free tier)'),
        @('groq', 'Groq (free tier, very fast)'),
        @('openrouter', 'OpenRouter free models'),
        @('cerebras', 'Cerebras (free tier)'),
        @('github', 'GitHub Models (free with GitHub)'),
        @('mistral', 'Mistral (free Experiment plan)'),
        @('ollama', 'Local Ollama'),
        @('lmstudio', 'Local LM Studio'),
        @('llamacpp', 'Local llama.cpp / llamafile'),
        @('openai', 'OpenAI or any OpenAI-compatible URL'),
        @('anthropic', 'Anthropic Claude')
    )
    $commands = @(
        @('setup', 'Pick an AI provider (guided)'),
        @('providers', 'List providers, costs and keys'),
        @('hook', 'Install or remove the git hook'),
        @('config', 'Show or create the config file'),
        @('completion', 'Print a shell completion script'),
        @('version', 'Print the version'),
        @('help', 'Show help')
    )
    $flags = @(
        @('-c', 'Commit with the message (opens your editor)'),
        @('-y', 'With -c: commit without the editor'),
        @('-p', 'Print the message only'),
        @('-a', 'Stage all changes first'),
        @('-v', 'Verbose output'),
        @('-n', 'Number of candidates'),
        @('-m', 'Extra context for the message'),
        @('--each', 'One commit per staged file'),
        @('--offline', 'No AI: rule-based message'),
        @('--dry-run', 'Show what would be sent, then exit'),
        @('--no-redact', 'Do not mask secrets'),
        @('--provider', 'Provider for this run'),
        @('--model', 'Model name'),
        @('--url', 'OpenAI-compatible base URL'),
        @('--style', 'Message style'),
        @('--lang', 'Language of the message'),
        @('--max', 'Max diff characters sent')
    )
    $languages = 'English', 'Spanish', 'Portuguese', 'French', 'German', 'Italian', 'Dutch', 'Japanese', 'Korean',
                 'Chinese', 'Filipino', 'Indonesian', 'Vietnamese', 'Russian', 'Turkish', 'Arabic', 'Hindi'

    # Words typed so far, without the command name and the word being completed.
    $words = @($commandAst.CommandElements | Select-Object -Skip 1 | ForEach-Object { $_.ToString() })
    if ($wordToComplete -ne '' -and $words.Count -gt 0 -and $words[-1] -eq $wordToComplete) {
        $words = @($words | Select-Object -SkipLast 1)
    }
    $prev = if ($words.Count -gt 0) { $words[-1] } else { '' }
    $first = if ($words.Count -gt 0) { $words[0] } else { '' }

    $result = switch -Regex ($prev) {
        '^--?provider$' { @(,@('auto', 'Use every provider that has a key')) + $providers; break }
        '^--?style$' { @(@('conventional', 'type(scope): summary'), @('simple', 'Plain summary line')); break }
        '^--?lang$' { $languages | ForEach-Object { ,@($_, "Write the message in $_") }; break }
        '^-n$' { '2', '3', '4', '5' | ForEach-Object { ,@($_, "$_ candidates") }; break }
        '^(--?model|--?url|--?max|-m)$' { @(); break }
        default {
            switch ($first) {
                'setup' { if ($words.Count -eq 1) { $providers } else { @() } }
                'hook' {
                    if ($words.Count -eq 1) { @(@('install', 'Draft messages on git commit'), @('uninstall', 'Remove the hook')) }
                    elseif ($words.Count -eq 2) { ,@(,@('--force', 'Overwrite a hook not made by CommitGen')) }
                    else { @() }
                }
                'config' { if ($words.Count -eq 1) { @(@('init', 'Write a starter config'), @('path', 'Print the config path'), @('show', 'Show effective settings')) } else { @() } }
                'completion' { if ($words.Count -eq 1) { 'bash', 'zsh', 'fish', 'powershell' | ForEach-Object { ,@($_, "$_ completion script") } } else { @() } }
                { $_ -in 'providers', 'version', 'help' } { @() }
                default {
                    if ($words.Count -eq 0 -and -not $wordToComplete.StartsWith('-')) { $commands } else { $flags }
                }
            }
        }
    }

    foreach ($item in $result) {
        if ($item[0] -like "$wordToComplete*") {
            [System.Management.Automation.CompletionResult]::new($item[0], $item[0], 'ParameterValue', $item[1])
        }
    }
}

# Also complete the lowercase name used by releases before the rename.
foreach ($name in 'CommitGen', 'CommitGen.exe', 'commitgen', 'commitgen.exe') {
    Register-ArgumentCompleter -Native -CommandName $name -ScriptBlock $commitGenCompleter
}
