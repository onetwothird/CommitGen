# Contributing to CommitGen

Thanks for helping. CommitGen aims to be useful to every developer, including people on old laptops, slow connections or no budget. Keep that in mind when you propose changes: offline mode and free providers come first.

## Getting started

```bash
git clone https://github.com/onetwothird/CommitGen.git
cd CommitGen
go test ./...
go build -o CommitGen .
./CommitGen --offline -p     # try it on your own staged changes
```

You need Go 1.22 or newer and git. The project has **no third-party dependencies**; please keep it that way.

## Ways to help

- **Add a provider.** Most hosted APIs are OpenAI-compatible, so a new provider is one entry in [`internal/llm/presets.go`](internal/llm/presets.go): name, base URL, default model, key env vars and the page where users get a key. Add it to `setupChoices` in [`setup.go`](setup.go), to the provider table in the README, and to the four scripts in [`completions/`](completions) (a test fails until you do).
- **Improve offline mode.** [`internal/heuristic`](internal/heuristic) decides the commit type, scope and summary. Support for more programming languages (function/class detection in `symbolRes`) and better file classification are very welcome. Add a test case for every rule you change.
- **Docs and translations.** Clearer setup steps, troubleshooting entries and examples help the most people.
- **Bug reports.** Use the issue templates. Include `CommitGen version`, your OS, `CommitGen config show` (it never prints keys) and the exact error.

## Pull requests

1. Create a branch: `git checkout -b feat/my-change`
2. Make the change and add tests next to the code (`*_test.go`).
3. Run the checks:

   ```bash
   gofmt -l .        # should print nothing
   go vet ./...
   go test ./...
   ```

   New flag? Add it to every script in `completions/` too; `TestCompletionsListEveryFlag` checks this.

4. Commit with [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:` …). CommitGen can write them for you.
5. Open a pull request describing what changed and why.
6. [CodeRabbit](https://coderabbit.ai), an AI reviewer, comments on your pull request within a few minutes with a summary and suggestions. Reply to it in the thread, or write `@coderabbitai review` after pushing more commits. A maintainer still does the final review; you don't have to accept every suggestion.

Tests must not depend on your machine: no real API keys, network calls or locally running models. Use `httptest` servers and the helpers already in the test files (for example `clean(t)` in the config tests and `isolate(t, …)` in `providers_test.go`).

## Code style

- Standard `gofmt` formatting and small, focused functions.
- Error messages should say what went wrong **and what to do next**.
- Never log or print API keys.

## Releasing

1. Move the `[Unreleased]` notes in `CHANGELOG.md` under the new version. `CommitGen changelog --release v0.2.0` drafts them.
2. Tag and push: `git tag v0.2.0 && git push origin v0.2.0`. The `release` workflow builds every platform and publishes the GitHub release.
3. Package managers are updated only when their token secret exists (Settings → Secrets and variables → Actions):

   | Secret | Needs |
   |---|---|
   | `SCOOP_BUCKET_TOKEN` | A public repo `onetwothird/scoop-bucket`, and a token that can push to it |
   | `HOMEBREW_TAP_TOKEN` | A public repo `onetwothird/homebrew-tap`, and a token that can push to it |
   | `WINGET_TOKEN` | A fork `onetwothird/winget-pkgs` of `microsoft/winget-pkgs`, and a token that can push to it and open pull requests |

   Use fine-grained tokens limited to those repositories.

## Security

Please report vulnerabilities privately, not in issues. See [SECURITY.md](SECURITY.md).

## Code of conduct

Be kind and assume good intent. Everyone taking part in this project is expected to follow the [code of conduct](CODE_OF_CONDUCT.md).
