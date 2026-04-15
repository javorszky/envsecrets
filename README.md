# envsecrets

A lightweight macOS CLI for storing and retrieving secrets as environment variables. Think of it as a poor-man's Vault — secrets live in your **macOS Keychain** for fast offline access, with **1Password** as an optional durable backup and cross-machine sync layer.

## How it works

```
Write:  Keychain (required) → 1Password (best-effort, if available)
Read:   Keychain (fast path) → 1Password (fallback, then cached to Keychain)
Sync:   1Password → Keychain (run once on a new machine)
```

Keychain is always available. 1Password is optional — the tool works fully offline without it, and only uses it when the desktop app is running and signed in.

## Requirements

- macOS
- Go 1.21+ (to install from source)
- [1Password CLI (`op`)](https://developer.1password.com/docs/cli/) — optional, needed for sync and cross-machine sharing

## Install

```sh
go install github.com/javorszky/envsecrets@latest
```

Or build locally:

```sh
go build -o envsecrets .
mv envsecrets /usr/local/bin/
```

## Commands

### `store` — save a secret

```sh
envsecrets store DATABASE_URL "postgres://user:pass@localhost/mydb"
envsecrets store --vault Work SLACK_TOKEN "xoxb-..."
```

Writes to Keychain and, if 1Password is available, also to your vault. If 1Password is unreachable, the secret is still saved locally and a warning is printed.

### `fetch` — read a secret

```sh
envsecrets fetch DATABASE_URL
export DATABASE_URL=$(envsecrets fetch DATABASE_URL)
```

Reads from Keychain first. On a miss, falls back to 1Password and caches the result in Keychain for future offline use. Prints the raw value with no trailing newline — safe for shell substitution.

### `update` — update an existing secret

```sh
envsecrets update API_KEY "new-value"
```

Semantically equivalent to `store`. Both upsert — `update` signals intent that the key already exists.

### `delete` — remove a secret

```sh
envsecrets delete DATABASE_URL
envsecrets delete --force OLD_API_KEY
```

Removes from both Keychain and 1Password. Prompts for confirmation unless `--force` / `-f` is passed.

### `sync` — pull all secrets from 1Password to Keychain

```sh
envsecrets sync
envsecrets sync --vault Work
```

Fetches every item from the configured 1Password vault and writes it into the local Keychain. Run this once on a new machine to bootstrap your local secrets from 1Password.

Requires the 1Password desktop app to be running and unlocked.

### `gen-env` — resolve a template into a `.env` file

```sh
envsecrets gen-env
envsecrets gen-env --template .env.tpl --output .env
```

Reads a template file and resolves `secret:` references into real values, writing a ready-to-use `.env` file. Useful for projects that load config from a `.env` file at startup.

## Template format

Create a `.env.tpl` file — safe to commit, contains no real secrets:

```sh
# .env.tpl

# Plain values are copied verbatim
APP_ENV=production
LOG_LEVEL=info

# Lines with secret: are resolved from your secrets store
DATABASE_URL=secret:myproject_DATABASE_URL
API_KEY=secret:myproject_API_KEY
WEBHOOK_SECRET=secret:myproject_WEBHOOK_SECRET
```

Add `.env` to your `.gitignore` and keep `.env.tpl` tracked:

```gitignore
.env
.env.*
!.env.tpl
```

Run `envsecrets gen-env` to produce `.env` with real values injected. Lines without `secret:` are copied verbatim; blank lines and comments are preserved.

## Vault configuration

By default, envsecrets uses the `Private` 1Password vault. Override it two ways:

```sh
# Environment variable (set once in your shell profile)
export ENVSECRETS_VAULT=MyVault

# Per-command flag
envsecrets store --vault MyVault MY_SECRET "value"
```

## New machine setup

When you move to a new Mac and already have secrets in 1Password:

```sh
# 1. Install envsecrets
go install github.com/javorszky/envsecrets@latest

# 2. Sign in to 1Password and unlock the desktop app

# 3. Pull all secrets into the local Keychain
envsecrets sync

# 4. All secrets are now available locally, offline
envsecrets fetch DATABASE_URL
```

## Key naming convention

Keys are stored as-is. It's useful to namespace them to avoid collisions between projects:

```sh
# Store with a project prefix
envsecrets store myproject_DATABASE_URL "postgres://..."
envsecrets store myproject_API_KEY "sk-..."

# Reference the prefixed names in your .env.tpl
DATABASE_URL=secret:myproject_DATABASE_URL
API_KEY=secret:myproject_API_KEY
```

This way, secrets from different projects coexist in the same Keychain and 1Password vault without collision.

## License

MIT — see [LICENSE](LICENSE).
