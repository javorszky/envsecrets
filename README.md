# envsecrets

A lightweight macOS CLI for storing and retrieving secrets as environment variables. Think of it as a poor-man's Vault — secrets live in your **macOS Keychain** for fast offline access, with **1Password** as an optional durable backup and cross-machine sync layer.

![short demo of storing a secret and using it in an env var](assets/es.webm)

_Developed collaboratively with [Claude](https://claude.ai) (Anthropic) — see [AI_USAGE.md](AI_USAGE.md)._

## Why

Most developers store secrets directly in shell profiles or project files:

```sh
# ~/.zshrc or ~/.bashrc
export AWS_ACCESS_KEY_ID="AKIA..."
export DATABASE_URL="postgres://user:s3cr3t@prod-host/db"

# .env
STRIPE_SECRET_KEY=sk_live_...
GITHUB_TOKEN=ghp_...
```

This is convenient but risky. A single accidental screen share, a `git add .` with the wrong `.gitignore`, or a curious pair-programming partner is all it takes to expose credentials that need immediate rotation.

envsecrets moves the secrets out of those files entirely. Store each secret once:

```sh
envsecrets store DATABASE_URL "postgres://user:s3cr3t@prod-host/db"
```

Then reference it by name wherever you'd normally paste the value:

```sh
# ~/.zshrc or ~/.bashrc
export DATABASE_URL=$(envsecrets fetch DATABASE_URL)

# .env.tpl  (committed to git — contains no real secrets)
DATABASE_URL=secret:DATABASE_URL
```

Now your shell profiles, `.env` files, and repositories contain only `envsecrets fetch` calls. Someone who sees your screen, clones your repo, or reads your dotfiles sees the command — not the secret. Your credentials stay locked inside macOS Keychain and, optionally, your 1Password vault.

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
envsecrets store --op-vault Work SLACK_TOKEN "xoxb-..."
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
envsecrets delete --force OLD_API_KEY   # skip confirmation prompt
```

Removes from both Keychain and 1Password. Prompts for confirmation unless `--force` / `-f` is passed.

### `sync` — pull all secrets from 1Password to Keychain

```sh
envsecrets sync
envsecrets sync --op-vault Work
```

Fetches every item from the configured 1Password vault and writes it into the local Keychain. Run this once on a new machine to bootstrap your local secrets from 1Password.

Requires the 1Password desktop app to be running and unlocked.

### `gen-env` — resolve a template into a `.env` file

```sh
envsecrets gen-env
envsecrets gen-env --template .env.tpl --output .env
```

Reads a template file and resolves `secret:` references into real values, writing a ready-to-use `.env` file. Useful for projects that load config from a `.env` file at startup.

> **Prefer not to install a binary?** Every command above has a pure-bash equivalent using the same `security` and `op` backends. See [BASH_FUNCTIONS.md](BASH_FUNCTIONS.md) for drop-in shell function alternatives — a simple login-keychain-only version and a full version that matches the binary's behaviour.

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

envsecrets uses two separate vault concepts:

| Setting | What it controls | Default |
|---------|-----------------|---------|
| `vault` | Name of the dedicated local keychain file (`~/.local/share/envsecrets/<name>.keychain`) | `envsecrets` |
| `op_vault` | 1Password vault where secrets are stored | `Envsecrets` |

> **Note:** The default `Envsecrets` vault is dedicated to envsecrets secrets, keeping them organised and separate from personal 1Password items. envsecrets will **create it automatically** on first write if it does not exist. You can rename it to anything you like.

### Setting the 1Password vault

```sh
# Environment variable (set once in your shell profile)
export ENVSECRETS_OP_VAULT=envsecrets

# Per-command flag
envsecrets store --op-vault envsecrets MY_SECRET "value"

# Config file (recommended — run once per machine)
envsecrets config init
# then edit ~/.config/envsecrets.toml and set:
#   op_vault = "envsecrets"
```

### Setting the keychain file name

```sh
# Environment variable
export ENVSECRETS_VAULT=myproject

# Per-command flag
envsecrets store --vault myproject MY_SECRET "value"
```

The keychain name is typically left at the default (`envsecrets`) unless you want per-project isolation.

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

## How envsecrets compares

Several tools solve adjacent problems. Here is where they overlap and where they differ.

| Tool | Local / offline | macOS Keychain | 1Password | Template file | Notes |
|------|:-:|:-:|:-:|:-:|-------|
| **envsecrets** | ✅ | ✅ dedicated file | ✅ optional | ✅ `gen-env` | This tool |
| `op run` / `op inject` | ❌ | ❌ | ✅ required | ✅ `op inject` | Built into the 1Password CLI. Same template idea, but always requires the 1Password app to be running. No Keychain cache. |
| op-fast | ✅ TTL cache | ✅ login KC | ✅ required | ❌ | Caches `op` calls in the OS keychain with a TTL. Drop-in proxy for the `op` CLI. Entries expire; envsecrets keeps them indefinitely. |
| envchain | ✅ | ✅ login KC | ❌ | ❌ | Injects secrets into a subprocess (`envchain ns -- cmd`). No `fetch`-to-stdout, no templates, no 1Password. Available via `brew install envchain`. |
| fnox | ✅ partial | ✅ login KC | ✅ svc account | ❌ | Multi-backend (Keychain, 1Password, AWS, Azure, Bitwarden). Auto-loads on `cd`. Requires a 1Password service account token (paid plan). Closest in spirit to envsecrets. |
| pass | ✅ | ❌ GPG | ❌ | ❌ | GPG-encrypted file tree. Works anywhere, syncs via git. Requires GPG key management. |
| Doppler / Infisical | ❌ | ❌ | ❌ | ❌ | SaaS platforms with team RBAC and audit logs. Require network access; no offline path. |
| dotenvx | ✅ | ❌ | ❌ | ❌ | Encrypts `.env` files so they are safe to commit to git. File-based, not vault-based. Cross-platform. |
| varlock | ✅ partial | ❌ | ✅ plugin | ❌ | Schema-based config tool. Replaces `.env.example` with a typed, validated `.env.schema` safe to commit and share with AI agents. No native secret storage — pulls values at runtime from 6 provider plugins (1Password, Infisical, AWS Secrets Manager, Azure Key Vault, Google Secret Manager, Bitwarden). Adds log redaction and type-safe leak prevention. Open source. |

### What makes envsecrets different

**Keychain as indefinite local cache for 1Password.** Other tools either use 1Password exclusively (always online) or use the Keychain exclusively (no durable backup). envsecrets does both: secrets fetched from 1Password are written into the local Keychain and served from there on every subsequent read, with no network call. 1Password is only contacted on a cold cache miss or a `sync`.

**Dedicated per-vault keychain file.** Every other tool that uses macOS Keychain stores secrets in your login keychain alongside passwords and certificates. envsecrets creates a separate `.keychain` file per vault with its own password, giving clear isolation. The file and its password are documented at creation time in `~/Documents/envsecrets-<vault>-keychain-access.txt` so you can always open it in Keychain Access directly.

**No account, no subscription, no network required.** The Keychain path works entirely offline. 1Password is opt-in.

### What envsecrets does not do

- **macOS only (for now)** — the `security` binary is macOS-only; there is no Linux or Windows keychain backend yet. Linux and Windows support is planned.
- **Team sharing with RBAC** — use Doppler, Infisical, or 1Password's built-in sharing for that
- **Directory-scoped auto-load** — fnox activates when you `cd` into a project; envsecrets does not
- **Push secrets back to 1Password** — `sync` is one direction only (1Password → Keychain)

### Name collision

There is an unrelated project at `github.com/envsecrets/envsecrets` — a now-defunct cloud SaaS secrets manager (disabled signups in 2024, no active development). It has no macOS Keychain or 1Password integration and is not usable for new users.

## License

MIT — see [LICENSE](LICENSE).
