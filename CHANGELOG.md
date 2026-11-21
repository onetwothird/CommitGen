# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **`CommitGen pr`**: writes a pull request title and description (Summary, Changes, Testing, Notes) from the branch's commits and diff. `--create` opens it with the GitHub CLI; `--base` picks the target branch. Works offline too.
- **`CommitGen changelog`**: release notes from Conventional Commits since the latest tag, grouped into Breaking changes, Features, Bug fixes, Performance and Reverts (`--full` adds the rest). No AI needed.
- **`CommitGen reword [N]`**: rewrites lazy or invalid messages ("wip", "update") of unpushed commits, keeping files, authors and dates. Asks before changing anything, refuses pushed commits without `--force`, and can be undone with `git reset --soft ORIG_HEAD`.
- **Message cache**: asking again for the same diff reuses the earlier message instead of calling the API (`cache = true`, `--no-cache`, `CommitGen cache clear|path`).
- **Issue links**: branches like `fix/123-login` or `feature/PROJ-42` add a `Refs #123` / `Refs PROJ-42` footer (`issue_footer`).
- **Breaking-change detection** in offline mode: removing or changing the signature of exported Go functions and types, or Rust `pub` items, adds `!` and a `BREAKING CHANGE:` footer (`breaking`). AI providers are told to do the same.
- **commitlint support**: `type-enum`, `scope-enum` and `header-max-length` from `.commitlintrc*`, `commitlint.config.*` or `package.json` shape the prompt and are checked on every message.
- **Secret guard**: likely secrets in the staged changes are listed by file and line. Automatic commits (`-c -y`, `--each`) stop on known key formats unless `--allow-secrets` is passed, and the git hook adds a warning to the editor.
- **GitHub Action** (`uses: onetwothird/CommitGen@main`) for release notes and pull request text in CI, and a **Dockerfile**.
- **Release packaging** for Scoop, Homebrew and winget, and a release workflow that publishes on tag push.
- `SECURITY.md`, `CODE_OF_CONDUCT.md`, CodeQL code scanning, Dependabot and `govulncheck` in CI.
- **Offline mode** (`--offline`, `provider = "offline"`): a rule-based Conventional Commit generator that needs no AI, no API key and no internet. It detects type, scope and new functions/classes across Go, Python, JS/TS, PHP, Java, C#, Kotlin, Swift, Rust, Ruby, Elixir, C/C++, Dart, Lua and shell.
- **Provider presets**: `gemini`, `groq`, `openrouter`, `cerebras`, `mistral`, `github`, `ollama`, `lmstudio`, `llamacpp`, `openai`, `anthropic`. Each has a default model and its own API key env var.
- **Fallback chain** (`fallback = [...]`, `COMMITGEN_FALLBACK`): when a provider hits a rate limit, a quota, a bad key or a network error, the next one is tried, ending with offline mode.
- **`provider = "auto"`** (new default): uses every provider whose key is present, then a running Ollama with an installed model, then offline mode.
- **`CommitGen setup`**: guided setup that picks a provider, saves the key, tests the connection and sets fallbacks.
- **`CommitGen providers`**: lists providers, cost and which keys are set.
- Per-provider `<name>_api_key` and `<name>_model` config keys.
- Smarter rate-limit handling: wait out short limits (`max_wait`, default 30 s) when only offline mode is left, pause a limited provider until it resets instead of dropping it for the whole run, and read the exact wait from messages like "Please try again in 7.5s".
- Install scripts for macOS/Linux (`install.sh`) and Windows (`install.ps1`).
- Tab completion for Bash, Zsh, Fish and PowerShell: `CommitGen completion <shell>` (commands, flags, providers, styles and languages). CI checks each script, and tests keep them in sync with the real flags.
- `CONTRIBUTING.md`, issue and pull request templates.

### Changed

- The tool is now called **CommitGen** everywhere: the binary, help text, messages, release archives and config paths (`CommitGen/config.toml`, `.CommitGen.toml`). Existing lowercase config files and git hooks keep working.
- Clearer errors for rate limits (including the retry time), rejected keys, unknown models, unreachable servers, "nothing staged", "not a git repository" and missing git.
- `CommitGen config show` now prints the provider chain that will be used.
- Default timeout lowered from 120 s to 60 s.
- Default Ollama model is `qwen2.5-coder:1.5b`, which runs on about 4 GB of RAM.
- Gemini models get the same reasoning budget handling as `gpt-oss`.

### Fixed

- Offline subjects could be 2 characters over the 72-character limit when shortened.

- The git hook now also finds the binary when it is named `CommitGen` (the default name from `go install` on macOS and Linux).
- Config tests no longer pick up the developer's own `COMMITGEN_*` environment variables.
- Removed leftover merge-conflict markers from the README.

### Compatibility

- Existing setups keep working. A config or environment that only sets `base_url` (for example `COMMITGEN_BASE_URL=https://api.groq.com/openai/v1`) is matched to the right provider, or treated as a custom OpenAI-compatible server.
- With no config at all, CommitGen now uses offline mode (or a running Ollama) instead of failing when Ollama isn't running.

## [0.1.2] and earlier

See the [git history](https://github.com/onetwothird/CommitGen/commits/main).
