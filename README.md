# envsecrets

A local CLI for managing env secrets across **macOS Keychain** (primary, always local) and **1Password** (durable sync layer).

## Read path

```
Keychain (fast, offline) → miss? → 1Password → cache back into Keychain
```

## Write path

```
Keychain (always) + 1Password (if available; failure is a warning, not an error)
```

This means:
- You can work completely offline as long as Keychain is populated.
- 1Password going down (app, service, or network) never blocks you.
- On a new machine, `sync` bootstraps your Keychain from 1Password once.

---

## Install

```bash
go install github.com/javorszky/envsecrets@latest
```

Or build locally:

```bash
go build -o envsecrets .
mv envsecrets /usr/local/bin/
```

---

## Commands

### `store <key> <value>`

Store a new secret in both backends.

```bash
envsecrets store STRIPE_SECRET sk_live_abc123
envsecrets store --vault Work DB_PASSWORD hunter2
```

### `fetch <key>`

Fetch a secret. Prints the raw value to stdout (no trailing newline).

```bash
envsecrets fetch STRIPE_SECRET

# Shell substitution friendly:
export DB_PASSWORD=$(envsecrets fetch DB_PASSWORD)
```

### `update <key> <value>`

Update an existing secret. Semantically equivalent to `store` (both upsert), but signals intent.

```bash
envsecrets update STRIPE_SECRET sk_live_newvalue
```

### `delete <key>`

Delete a secret from both backends. Prompts for confirmation.

```bash
envsecrets delete OLD_KEY
envsecrets delete --force OLD_KEY   # skip confirmation
```

### `sync`

Pull every item from the 1Password vault into Keychain. Run once on a new machine.

```bash
envsecrets sync
envsecrets sync --vault Work
```

Requires the 1Password desktop app to be running and unlocked. After this, all `fetch` calls work offline.

### `gen-env`

Resolve a committed template into a gitignored `.env` file.

```bash
envsecrets gen-env
envsecrets gen-env --template .env.docker.tpl --output .env.docker
```

---

## Template format (`.env.docker.tpl`)

Commit this file. Values prefixed with `secret:` are resolved from Keychain/1Password at generation time. Everything else is copied verbatim.

```dotenv
# Application
APP_KEY=secret:myapp_APP_KEY
APP_ENV=local
APP_DEBUG=false

# Database
DB_CONNECTION=mysql
DB_HOST=mysql
DB_PORT=3306
DB_DATABASE=myapp
DB_PASSWORD=secret:myapp_DB_PASSWORD

# Stripe
STRIPE_SECRET=secret:myapp_STRIPE_SECRET
```

Add the output file to `.gitignore`:

```
.env.docker
.env.staging
```

Run before `docker compose up`:

```bash
envsecrets gen-env && docker compose up
```

---

## Vault configuration

The 1Password vault defaults to `"Private"`. Override with:

- `--vault <name>` flag on any command
- `ENVSECRETS_VAULT=Work` environment variable

---

## New machine bootstrap

```bash
# 1. Install the tool
go install github.com/javorszky/envsecrets@latest

# 2. Make sure 1Password desktop app is open and unlocked

# 3. Pull everything into local Keychain
envsecrets sync

# 4. Generate your env file
envsecrets gen-env

# 5. Done — 1Password can go offline now
docker compose up
```
