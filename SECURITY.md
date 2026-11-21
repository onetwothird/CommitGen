# Security policy

## Supported versions

Security fixes go into the latest release. Please update before reporting: `CommitGen version` shows what you have.

## Reporting a vulnerability

**Please don't open a public issue for security problems.**

Report it privately on GitHub: open the repository's **Security** tab and click **Report a vulnerability**. Include:

- what the problem is and what an attacker could do with it,
- steps to reproduce (a small repo or diff helps),
- your OS, `CommitGen version` and the provider you use.

You'll get a reply within a week. Once a fix is released, the advisory is published and you're credited unless you'd rather not be.

## What CommitGen does to protect you

- **Secrets are masked before anything leaves your machine.** API keys, tokens, private keys, passwords in URLs and `password = "..."`-style values in the diff are replaced with `[REDACTED]` before the diff goes to an AI provider. `CommitGen --dry-run` shows exactly what would be sent.
- **Commits with real-looking keys are stopped.** If the staged changes contain a known key format (GitHub, AWS, Google, OpenAI, Anthropic, Stripe live, Slack, Groq, GitLab, Hugging Face, SendGrid, private keys), automatic commits (`-c -y`, `--each`) refuse to run and the git hook adds a warning to the message editor. Masking only protects what the AI sees: git would still store the key. Use `--allow-secrets` for fake keys in tests.
- **Offline mode sends nothing anywhere.** `--offline` (or `provider = "offline"`) works without network access.
- **Local models stay local.** Ollama, LM Studio and llama.cpp run on your machine.
- **Keys are never printed.** `CommitGen config show` and `CommitGen providers` say whether a key is set, never what it is. The config file is created with `0600` permissions.
- **The cache stores messages, not code.** Messages for diffs you already generated are saved in your user cache folder (`CommitGen cache path`) for 7 days. `CommitGen cache clear` removes them; `cache = false` turns the cache off.
- **No third-party Go dependencies**, so there is no dependency supply chain to compromise. CI runs `govulncheck` and CodeQL on every push.

## If you committed a secret

Masking and the guard reduce the risk, but they can't catch everything. If a real key reached a commit:

1. **Revoke or rotate the key first.** Assume it's compromised once it's pushed.
2. Remove it from history (for example with `git filter-repo`) before pushing, or as soon as possible after.
3. Move the key to an environment variable or a secret manager.
