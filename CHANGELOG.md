# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- **Offline mode** (`--offline`, `provider = "offline"`): a rule-based Conventional Commit generator that needs no AI, no API key and no internet. It detects type, scope and new functions/classes across Go, Python, JS/TS, PHP, Java, C#, Kotlin, Swift, Rust, Ruby, Elixir, C/C++, Dart, Lua and shell.
- **Provider presets**: `gemini`, `groq`, `openrouter`, `cerebras`, `mistral`, `github`, `ollama`, `lmstudio`, `llamacpp`, `openai`, `anthropic`. Each has a default model and its own API key env var.
- **Fallback chain** (`fallback = [...]`, `COMMITGEN_FALLBACK`): when a provider hits a rate limit, a quota, a bad key or a network error, the next one is tried, ending with offline mode.
- **`provider = "auto"`** (new default): uses every provider whose key is present, then a running Ollama with an installed model, then offline mode.
- **`commitgen setup`**: guided setup that picks a provider, saves the key, tests the connection and sets fallbacks.
- **`commitgen providers`**: lists providers, cost and which keys are set.
- Per-provider `<name>_api_key` and `<name>_model` config keys.
- Install scripts for macOS/Linux (`install.sh`) and Windows (`install.ps1`).
- `CONTRIBUTING.md`, issue and pull request templates.

### Changed

- Clearer errors for rate limits (including the retry time), rejected keys, unknown models, unreachable servers, "nothing staged", "not a git repository" and missing git.
- `commitgen config show` now prints the provider chain that will be used.
- Default timeout lowered from 120 s to 60 s.
- Default Ollama model is `qwen2.5-coder:1.5b`, which runs on about 4 GB of RAM.
- Gemini models get the same reasoning budget handling as `gpt-oss`.

### Fixed

- The git hook now also finds the binary when it is named `CommitGen` (the default name from `go install` on macOS and Linux).
- Config tests no longer pick up the developer's own `COMMITGEN_*` environment variables.
- Removed leftover merge-conflict markers from the README.

### Compatibility

- Existing setups keep working. A config or environment that only sets `base_url` (for example `COMMITGEN_BASE_URL=https://api.groq.com/openai/v1`) is matched to the right provider, or treated as a custom OpenAI-compatible server.
- With no config at all, CommitGen now uses offline mode (or a running Ollama) instead of failing when Ollama isn't running.

## [0.1.2] and earlier

See the [git history](https://github.com/onetwothird/CommitGen/commits/main).
