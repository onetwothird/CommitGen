<div align="center">

# CommitGen

**Good commit messages from your staged diff — on any PC, with or without AI.**

[![CI](https://github.com/onetwothird/CommitGen/actions/workflows/ci.yml/badge.svg)](https://github.com/onetwothird/CommitGen/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/onetwothird/CommitGen?style=flat-square)](go.mod)
[![Latest Tag](https://img.shields.io/github/v/tag/onetwothird/CommitGen?style=flat-square&label=version)](https://github.com/onetwothird/CommitGen/tags)
[![Go Report Card](https://goreportcard.com/badge/github.com/onetwothird/CommitGen?style=flat-square)](https://goreportcard.com/report/github.com/onetwothird/CommitGen)
[![License: MIT](https://img.shields.io/github/license/onetwothird/CommitGen?style=flat-square)](LICENSE)

[Quick start](#quick-start) •
[Providers](#choosing-a-provider) •
[Usage](#usage) •
[Configuration](#configuration) •
[Beyond commits](#beyond-commit-messages) •
[Troubleshooting](#troubleshooting) •
[Security](SECURITY.md) •
[Contributing](CONTRIBUTING.md)

</div>

---

```console
$ git add -A
$ CommitGen

  feat(auth): add refresh token rotation

  - rotate refresh tokens on every use to limit replay attacks
  - store token family id so reuse revokes the whole chain

[c]ommit  [e]dit  [r]egenerate  [q]uit (c):
```

## Why CommitGen?

- **Works for everyone.** No GPU, no API key and no internet? CommitGen still writes a Conventional Commit from your diff with its built-in **offline mode**. Add a **free** AI provider in about a minute when you want better messages.
- **Never stuck on one provider.** Free tiers run out. CommitGen moves to the next provider in your **fallback chain** when it hits a rate limit, a quota, a bad key or a network error, and it ends with offline mode, so you always get a message.
- **Light.** One small static binary with no runtime and no dependencies. It runs on old laptops, CI machines and WSL.
- **Safe.** It reads only staged changes and masks API keys, tokens and passwords before anything is sent. If you're about to commit a real-looking key, it warns you and automatic commits stop. `--dry-run` shows exactly what would be sent.
- **Saves your quota.** A message made for a diff is reused if you ask again (the hook after a preview, a commit you aborted), so the same change never costs two API calls.
- **More than commits.** Pull request descriptions (`CommitGen pr`), release notes (`CommitGen changelog`) and fixing bad messages before you push (`CommitGen reword`).

## Quick start

**1. Install** (pick one):

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/onetwothird/CommitGen/main/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/onetwothird/CommitGen/main/install.ps1 | iex
```

```bash
# Anywhere with Go 1.22+
go install github.com/onetwothird/CommitGen@latest
```

> `go install` names the binary `CommitGen`, the same name the installers use.

<details>
<summary><b>Package managers and Docker</b> (from the next release)</summary>

```powershell
# Windows: Scoop or winget
scoop bucket add onetwothird https://github.com/onetwothird/scoop-bucket
scoop install CommitGen
winget install onetwothird.CommitGen
```

```bash
# macOS / Linux: Homebrew (installs tab completion too).
# Homebrew and Docker only allow lowercase names; the command is still CommitGen.
brew install onetwothird/tap/commitgen

# Docker: nothing to install but Docker
docker build -t commitgen https://github.com/onetwothird/CommitGen.git
docker run --rm -v "$PWD:/repo" commitgen -p --offline
```
</details>

**2. Use it.** Inside any git repository:

```bash
git add -A
CommitGen
```

That's it. With nothing set up, CommitGen uses **offline mode**: rule-based, instant and private.

**3. Optional: turn on AI** (free options, about a minute):

```bash
CommitGen setup
```

The wizard lists the providers, shows where to get a free key, tests the connection and saves everything. Not sure which to pick? Choose **Gemini**: it's free, needs no credit card and runs on any PC.

## Choosing a provider

Run `CommitGen providers` to see this list with the keys you already have.

| Provider | Cost | Speed | Runs on | Needs | Best for |
|---|---|---|---|---|---|
| `offline` | Free | Instant | Anything | Nothing | No internet, no key, CI, last-resort fallback |
| `gemini` | Free tier, no card | Fast | Any PC | [Free key](https://aistudio.google.com/apikey) | **Recommended default**: generous daily quota |
| `groq` | Free tier, no card | Very fast | Any PC | [Free key](https://console.groq.com/keys) | Speed; smaller daily token limit |
| `openrouter` | Free models, no card | Varies | Any PC | [Free key](https://openrouter.ai/keys) | `openrouter/free` picks any free model that's up |
| `cerebras` | Free tier, no card | Very fast | Any PC | [Free key](https://cloud.cerebras.ai) | Large free token allowance |
| `github` | Free with GitHub | Fast | Any PC | [Token with `models:read`](https://github.com/settings/personal-access-tokens) | You already have a GitHub account |
| `mistral` | Free Experiment plan | Fast | Any PC | [Free key](https://console.mistral.ai/api-keys) | European provider |
| `ollama` | Free, private | Depends on PC | 4 GB+ RAM | [Ollama](https://ollama.com/download) | Private company code |
| `lmstudio` | Free, private | Depends on PC | 4 GB+ RAM | [LM Studio](https://lmstudio.ai) | Desktop app users |
| `llamacpp` | Free, private | Depends on PC | 2 GB+ RAM | [llama.cpp](https://github.com/ggml-org/llama.cpp) / llamafile | Lightest local setup |
| `openai` | Paid (cheap) | Fast | Any PC | [Key](https://platform.openai.com/api-keys) | Also any OpenAI-compatible URL |
| `anthropic` | Paid (cheap) | Fast | Any PC | [Key](https://console.anthropic.com/settings/keys) | Claude models |

Free tiers and model lineups change often. Check each provider's page for current limits.

### Low-spec PC?

You don't need a strong computer:

- **No GPU, 4 GB RAM or less:** use a hosted free provider (`gemini`, `groq`, `openrouter`, `cerebras`). Your PC only sends a small request.
- **No internet, or company code that must stay local:** use `offline`, or a tiny local model:

  | Model | Download | RAM needed | Quality |
  |---|---|---|---|
  | `qwen2.5-coder:0.5b` | ~400 MB | 2 GB | basic |
  | `qwen2.5-coder:1.5b` *(default for ollama)* | ~1 GB | 4 GB | good |
  | `qwen2.5-coder:3b` | ~2 GB | 8 GB | better |
  | `qwen2.5-coder:7b` | ~5 GB | 16 GB or a GPU | best |

  ```bash
  ollama pull qwen2.5-coder:1.5b
  CommitGen setup ollama
  ```

### Fallback chain

The main provider is tried first, then each fallback in order:

```toml
provider = "groq"
fallback = ["gemini", "openrouter", "offline"]
```

```console
$ CommitGen
CommitGen: groq failed (rate limit / quota reached), trying gemini
  fix(api): handle empty pagination cursor
```

How failures are handled:

- **Short rate limit** ("try again in 8s", the usual per-minute limit on free tiers): if only offline mode is left, CommitGen waits and retries, up to `max_wait` seconds (default 30). If another AI provider is available, it uses that instead.
- **Long limit** (daily quota used up): the provider is paused until its reset time, then used again. During a long `--each` run, later commits go back to it automatically.
- **Bad key, unknown model or no credit:** the provider is skipped for the rest of the run.
- With `provider = "auto"` (the default), CommitGen uses every provider whose key it finds (`GROQ_API_KEY`, `GEMINI_API_KEY`, …), then a running Ollama, then `offline`.
- `offline` is added at the end automatically unless you write your own `fallback` list without it.
- Providers without a key are skipped, with a note telling you where to get one.

## Usage

```text
CommitGen [flags]              generate a message for the staged changes
CommitGen setup [provider]     guided setup (key, model, test, fallback)
CommitGen providers            list providers, costs and which keys are set
CommitGen pr [--base main] [--create]     pull request title and description
CommitGen changelog [--from] [--to] [--release] [--full]
CommitGen reword [N] [-y] [--full] [--force]
CommitGen hook install|uninstall
CommitGen config init|path|show
CommitGen cache clear|path
CommitGen completion bash|zsh|fish|powershell
CommitGen version
```

| Flag | Description |
|---|---|
| `-c` | Generate, then open `git commit` in your editor to review |
| `-y` | With `-c`: commit immediately without opening the editor |
| `-p` | Print the message only (for scripts and aliases) |
| `-a` | Stage all changes first (like `git add -A`) |
| `--each` | Commit every staged file separately, each with its own message |
| `-n <num>` | Generate several candidates and pick one |
| `-m "<text>"` | Extra context, e.g. `-m "closes #42"`. In offline mode it becomes the summary |
| `--offline` | No AI for this run |
| `--provider <name>` | Use another provider for this run, e.g. `--provider gemini` |
| `--model <name>` | Override the model |
| `--url <url>` | Override the API base URL |
| `--style <name>` | `conventional` or `simple` |
| `--lang <language>` | Language of the message, e.g. `--lang Spanish` |
| `--max <chars>` | Max diff characters sent (fewer = fewer tokens used) |
| `--dry-run` | Show exactly what would be sent, then exit |
| `--no-redact` | Disable secret masking |
| `--no-cache` | Don't reuse a message made earlier for the same diff |
| `--allow-secrets` | Let `-c -y` and `--each` commit even if the diff has a real-looking key (for fake test keys) |
| `-v` | Verbose: show the provider chain and diff size |
| `--base <branch>` | `pr`: branch the pull request goes into (default: origin's default branch) |
| `--create` | `pr`: open the pull request with the [GitHub CLI](https://cli.github.com) |
| `--from`, `--to` | `changelog`: range of commits (default: latest tag to `HEAD`) |
| `--release <name>` | `changelog`: heading, e.g. `v1.2.0` (default: the tag at `--to`, else Unreleased) |
| `--full` | `changelog`: list every type; `reword`: also rewrite messages that already look fine |
| `--force` | `reword`: allow rewriting commits that are already pushed |

### Examples

```bash
CommitGen                          # draft, then commit / edit / regenerate
CommitGen -c -y                    # generate and commit in one step
CommitGen -a --each                # one commit per changed file
CommitGen -a --each -p             # preview those messages, commit nothing
CommitGen -n 3                     # pick from three candidates
CommitGen -m "fixes #42"           # give the model a hint
CommitGen --offline -m "fix login" # instant, no AI: "fix(auth): fix login"
CommitGen --provider ollama        # one-off local run
CommitGen --lang Japanese          # write the message in Japanese
CommitGen hook install             # plain `git commit` now drafts the message
git config --global alias.ai '!CommitGen -c'   # then: git ai
```

### Offline mode

Offline mode reads the diff and decides:

- **Type:** `docs` (Markdown, `docs/`), `test` (`*_test.go`, `*.spec.ts`, `tests/` …), `ci` (`.github/workflows`, GitLab CI …), `build` (`go.mod`, `package.json`, `Dockerfile` …), `chore` (config files), `style` (whitespace-only), `feat` (new files or functions), `fix` (small or fix-related edits), `refactor` (removals, renames, larger rewrites). A `-m` hint like "fix …" or "add …" overrides it.
- **Scope:** the most specific shared folder, skipping generic ones like `src/` and `internal/`, and limited to your `scopes` list if you set one.
- **Summary and body:** new functions, classes and types are named. It recognizes Go, Python, JavaScript/TypeScript, PHP, Java, C#, Kotlin, Swift, Rust, Ruby, Elixir, C/C++, Dart, Lua and shell. The body lists each file with its `+/-` line counts.
- **Breaking changes:** removing an exported Go function or type, or changing its signature, adds `!` and a `BREAKING CHANGE:` footer. The same goes for Rust `pub` items. Code in `internal/`, `cmd/`, `main` packages and tests is skipped. Turn this off with `breaking = false`.

It can't know *why* you made a change, so add `-m "why"` or switch on AI for richer messages.

### Languages

AI providers can write in any language: set `language = "Português"` in your config or pass `--lang`. Conventional Commit type keywords (`feat`, `fix` …) stay in English. Offline mode writes in English.

### Tab completion

Press Tab to complete commands, flags, provider names, styles and languages (`CommitGen --provider gem` → `gemini`). Set it up once for your shell, then open a new terminal:

```powershell
# PowerShell (Windows)
Add-Content $PROFILE 'CommitGen completion powershell | Out-String | Invoke-Expression'
```

```bash
# Bash
echo 'source <(CommitGen completion bash)' >> ~/.bashrc

# Zsh
echo 'source <(CommitGen completion zsh)' >> ~/.zshrc

# Fish
CommitGen completion fish > ~/.config/fish/completions/CommitGen.fish
```

The scripts are also in [`completions/`](completions) and in every release archive, if you'd rather install them as files.

## Beyond commit messages

### Pull request descriptions

```console
$ CommitGen pr
feat(client): add timeout parameter to Dial

## Summary
Dial now accepts a timeout, so callers can stop waiting on slow servers.

## Changes
- client/client.go: Dial(addr, timeout) ...

## Testing
...
```

It reads every commit and the full diff since the branch left `main`. `--create` opens the pull request with `gh pr create`, and `--base develop` picks another target branch.

### Release notes

```bash
CommitGen changelog                       # since the latest tag, grouped by type
CommitGen changelog --release v1.2.0      # heading "## [v1.2.0] - <date>"
CommitGen changelog --from v1.0.0 --full  # include refactors, docs, tests and chores
```

Breaking changes (`feat!:` or a `BREAKING CHANGE:` footer) get their own section at the top. No AI is needed, because Conventional Commits already say what each change is. Right after tagging, `CommitGen changelog` shows that release.

### Fix bad messages before you push

```console
$ CommitGen reword
  3f2a91c  feat(cache): reuse messages  (kept)
  8b1d0e4  wip
     -> fix(git): handle detached HEAD in Branch
  c71e5a2  update
     -> docs(readme): document the cache

rewrite 2 commit message(s)? [y]es [n]o (n):
```

With no number, it looks at the commits you haven't pushed yet; `CommitGen reword 5` looks at the last five. Good messages are kept unless you pass `--full`. Only the messages change: your files, author and dates stay the same, and `git reset --soft ORIG_HEAD` undoes it. Pushed commits are left alone unless you pass `--force`.

### In GitHub Actions

Write release notes when you push a tag:

```yaml
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0            # the changelog needs the tags
      - id: notes
        uses: onetwothird/CommitGen@main
        with:
          command: changelog        # or: pr
      - run: gh release create "$GITHUB_REF_NAME" --notes-file "${{ steps.notes.outputs.file }}"
        env:
          GH_TOKEN: ${{ github.token }}
```

The default `provider: offline` needs no key. For AI-written pull request text, set `provider: groq` (or another) and pass the key as an env var.

### Team rules: commitlint and issue links

- **commitlint:** if the repo has a commitlint config (`.commitlintrc*`, `commitlint.config.*` or a `commitlint` key in `package.json`), CommitGen follows its `type-enum`, `scope-enum` and `header-max-length` rules and asks the model to fix any message that breaks them. JavaScript configs are read, not run.
- **Issue links:** on a branch like `fix/123-login` or `feature/PROJ-42-search`, messages get a `Refs #123` / `Refs PROJ-42` footer, unless the message already mentions it. Use `issue_footer = "Closes"` to close the issue on merge, or `issue_footer = ""` to turn this off.

## Get the most out of CommitGen

1. **Use two or three free providers.** Each free tier has its own limit, and the fallback chain adds them up. Gemini, Groq and OpenRouter together go a long way, all free:

   ```bash
   CommitGen setup groq        # add a key
   CommitGen setup gemini      # the last one you set up becomes the main provider;
                               # providers that already have keys are offered as fallbacks
   ```

   Or put the keys in environment variables (`GEMINI_API_KEY`, `GROQ_API_KEY`, `OPENROUTER_API_KEY`). The default `provider = "auto"` uses them all.

2. **Commit small, focused changes.** Smaller diffs give better messages, use fewer tokens and make a cleaner history. `git add -p` stages part of a file.

3. **Say why with `-m`.** The diff shows *what* changed but not *why*: `CommitGen -m "users were logged out on refresh"`.

4. **Hook it into `git commit`.** `CommitGen hook install` drafts a message in your editor whenever you run plain `git commit`.

5. **Set project rules once.** A committed `.CommitGen.toml` with `scopes`, `instructions` and `exclude` makes every teammate's messages consistent.

6. **Batch commits without burning the quota.** `CommitGen -a --each` makes one request per file. For big batches:
   - preview first with `-p`
   - lower `max_diff` (e.g. `4000`) so each request is small
   - raise `max_wait` (e.g. `60`) to wait out per-minute limits instead of using offline messages
   - or use `--offline` for routine files (lockfiles, config, renames) and save AI for real code changes

7. **Review before you push.** `CommitGen -c` opens your editor with the draft. AI and offline messages are both good starting points, not final truth.

## Configuration

`CommitGen setup` writes the config for you. To edit it by hand:

```bash
CommitGen config path    # where the global config lives
CommitGen config init    # write a commented starter file
CommitGen config show    # effective settings and the provider chain
```

Add a `.CommitGen.toml` to a repository to set project rules (commit it so your team shares them):

```toml
style        = "conventional"
scopes       = ["api", "ui", "cli"]
exclude      = ["*.snap", "docs/generated/*"]
instructions = "Reference the Jira ticket from the branch name, e.g. PROJ-123."
```

**Precedence:** defaults → global config → repo `.CommitGen.toml` → environment variables → flags.

### All options

| Key | Default | Description |
|---|---|---|
| `provider` | `auto` | `auto`, `offline`, or a provider name from the table above |
| `fallback` | — | Providers to try next, e.g. `["gemini", "offline"]` |
| `model` | provider default | Model for the main provider |
| `base_url` | provider default | For custom OpenAI-compatible servers (vLLM, LocalAI, a proxy…) |
| `api_key` | — | Key for the main provider (prefer env vars or `<name>_api_key`) |
| `<name>_api_key` | — | Key for one provider, e.g. `gemini_api_key = "..."` |
| `<name>_model` | — | Model for one provider, e.g. `ollama_model = "qwen2.5-coder:3b"` |
| `style` | `conventional` | `conventional` or `simple` |
| `language` | `English` | Language of the message |
| `max_diff` | `12000` | Max diff characters sent. Lower it to stretch free-tier quotas |
| `history` | `10` | Recent commit subjects used to match your repo's style |
| `temperature` | `0.2` | Sampling temperature |
| `timeout` | `60` | Request timeout in seconds |
| `num_ctx` | `8192` | Ollama context window in tokens |
| `max_wait` | `30` | Seconds to wait out a short rate limit before falling back to offline mode (`0` = never wait) |
| `exclude` | — | Extra pathspecs left out of the diff (lockfiles, `dist/` and images are already skipped) |
| `scopes` | — | Allowed Conventional Commit scopes |
| `instructions` | — | Extra rules for the model |
| `cache` | `true` | Reuse the message for a diff you already generated (saved for 7 days in `CommitGen cache path`) |
| `issue_footer` | `Refs` | Footer for the branch's issue (`fix/123-x` → `Refs #123`); `Closes`, or `""` to turn off |
| `breaking` | `true` | Offline mode: mark removed or changed exported Go/Rust API as a breaking change |

### Environment variables

| Variable | Purpose |
|---|---|
| `GROQ_API_KEY`, `GEMINI_API_KEY` (or `GOOGLE_API_KEY`), `OPENROUTER_API_KEY`, `CEREBRAS_API_KEY`, `MISTRAL_API_KEY`, `GITHUB_MODELS_TOKEN` (or `GITHUB_TOKEN`), `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` | Provider keys |
| `COMMITGEN_PROVIDER`, `COMMITGEN_MODEL`, `COMMITGEN_BASE_URL`, `COMMITGEN_API_KEY` | Main provider settings |
| `COMMITGEN_FALLBACK` | Comma-separated fallback list, e.g. `gemini,offline` |
| `COMMITGEN_STYLE`, `COMMITGEN_LANGUAGE` | Message style and language |
| `COMMITGEN_CONFIG` | Use a different config file |

<details>
<summary><b>Setting environment variables permanently</b></summary>

```bash
# macOS / Linux: add to ~/.bashrc or ~/.zshrc
export GEMINI_API_KEY=your_key
```

```powershell
# Windows: saved for terminals opened afterwards
setx GEMINI_API_KEY "your_key"
```

On Windows, `setx` only affects terminals opened **after** it runs. In VS Code, quit and reopen the whole app. To use the value in the current window right away: `$env:GEMINI_API_KEY = "your_key"`.
</details>

> **Keep keys secret.** Don't paste them into issues, screenshots or commits. Keys saved by `CommitGen setup` go in your user config file (created with owner-only permissions), never in the repository.

## Troubleshooting

| Message | What to do |
|---|---|
| `nothing is staged` | Stage files with `git add <file>` or `git add -A`, or run `CommitGen -a`. |
| `this folder is not a git repository` | `cd` into your project, or run `git init`. |
| `git is not installed or not on your PATH` | Install from [git-scm.com](https://git-scm.com/downloads) and reopen the terminal. |
| `rate limit or free-tier quota reached` | Add a fallback (`CommitGen setup`), lower `max_diff`, wait until the time shown, or use `--offline`. |
| `the API key was rejected or is missing` | Re-run `CommitGen setup <provider>`. `CommitGen providers` shows where to get a key. |
| `skipping gemini: no API key` | The provider is in your chain but has no key. Set it, or remove it from `fallback`. |
| `could not connect — is the local model server running?` | Start Ollama (`ollama serve`) or LM Studio's server, or use a hosted provider. |
| `not found — check base_url and the model name` | Wrong model name. For Ollama: `ollama pull <model>`. For hosted providers, check their model list. |
| The model describes the diff instead of writing a message | CommitGen retries automatically. Also try staging less at once, adding `-m "..."`, or a larger model. |
| `not committing: the changes contain what looks like a real API key` | Remove the key from the staged files (use an environment variable instead). If it's a fake key in a test, pass `--allow-secrets`. |
| `reused the message made earlier for this exact diff` | That's the cache saving an API call. Press `r` to regenerate, or use `--no-cache`. |
| `the message breaks this repo's commitlint rules` | The model couldn't satisfy `type-enum` / `header-max-length`; edit the message, or use `-m` to steer it. |
| Settings don't seem to apply | Run `CommitGen config show` to see the effective provider chain. On Windows, open a new terminal after `setx`. |
| Old setup with `COMMITGEN_BASE_URL` | Still works: the URL is matched to its provider (Groq, OpenAI, …) or treated as a custom server. |

The git hook never blocks a commit: if generation fails, `git commit` continues as normal. It also skips merges, amends and `-m` commits.

## How it works

```text
git diff --cached → skip noise → scan + mask secrets → build prompt → cache? → provider chain → clean up → git commit
                                                                               │
                                                 groq → gemini → … → ollama → offline
```

1. Reads **staged** changes only. Lockfiles, minified assets, build output and images are skipped, and big diffs are trimmed while keeping every file header.
2. Warns about anything that looks like a secret (and stops automatic commits of real-looking keys), then masks it.
3. Adds the file summary, branch name and recent commit subjects so the message matches your repo's style.
4. Reuses the saved message if this exact request was answered before; otherwise tries each provider in the chain until one answers.
5. Cleans the reply (code fences, quotes, preambles, `<think>` blocks). If it isn't a valid commit message, or breaks the repo's commitlint rules, asks the model to fix it.
6. Adds the branch's issue footer and hands the message to `git commit`.

## FAQ

<details>
<summary><b>Does my code leave my machine?</b></summary>

Only if you use a hosted provider, and then only the filtered, secret-masked staged diff goes to that provider. `offline`, `ollama`, `lmstudio` and `llamacpp` keep everything local. Run `CommitGen --dry-run` to see exactly what would be sent. The local cache stores generated messages only, never your code. See [SECURITY.md](SECURITY.md) for details and how to report a vulnerability.
</details>

<details>
<summary><b>How do I make free tiers last longer?</b></summary>

- Commit smaller pieces. That also gives you a cleaner history.
- Lower `max_diff` (for example `6000`).
- Put two or three free providers in `fallback` so the quotas add up.
- Avoid `-n 3` on free tiers: it makes three requests.
- For `--each` on many files, raise `max_wait` so short per-minute limits are waited out instead of falling back to offline mode.
</details>

<details>
<summary><b>Can I use it in CI or scripts?</b></summary>

Yes. `CommitGen -p` prints the message only. With no key in the environment it uses offline mode, so it never needs network access.
</details>

## Development

```bash
go test ./...
go vet ./...
go build -o CommitGen .
```

```text
.
├── main.go            CLI entry point, commit flow, hook, config
├── history.go         `pr`, `changelog`, `reword` and `cache` commands
├── providers.go       provider resolution, fallback chain, friendly errors
├── setup.go           `CommitGen setup` wizard
├── completion.go      `CommitGen completion` (embeds the scripts below)
├── completions/       Bash, Zsh, Fish and PowerShell tab completion
├── action.yml         GitHub Action (changelog / pr in CI)
├── Dockerfile         container image
├── install.sh         installer for macOS / Linux
├── install.ps1        installer for Windows
└── internal/
    ├── cache/         saved messages for diffs already generated
    ├── changelog/     release notes from Conventional Commits
    ├── commitlint/    reads a repo's commitlint rules
    ├── config/        config files and env vars
    ├── git/           git commands (diff, log, history rewriting)
    ├── heuristic/     offline (no-AI) message generator, breaking-change detection
    ├── llm/           provider presets, clients, errors, fallback chain
    ├── prompt/        prompts for commits and pull requests, output cleanup
    ├── redact/        secret scanning and masking
    └── refs/          issue references from branch names
```

Adding a provider that speaks the OpenAI API is usually one entry in [`internal/llm/presets.go`](internal/llm/presets.go). See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
