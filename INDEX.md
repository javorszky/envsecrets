# envsecrets - File Index

Codebase table of contents and API reference. Consult this before scanning the repo.
After any code change, update the relevant section(s) below to keep signatures and descriptions current.

---

## Project Root

| File | Purpose |
|------|---------|
| `main.go` | Entry point. Calls `cmd.Execute()`. |
| `go.mod` | Module `github.com/javorszky/envsecrets`, Go 1.26.2. Depends on `spf13/cobra`, `spf13/viper`, `stretchr/testify` (test). |
| `go.sum` | Dependency checksums. |
| `LICENSE` | MIT license. |
| `README.md` | User-facing documentation: why, installation, usage, architecture. |
| `.gitignore` | Ignores binary, IDE files, `.env` files (except `.env.tpl`), coverage output. |
| `.golangci.yml` | golangci-lint v2 config. Standard linters + `gofmt`. |
| `Makefile` | Local dev commands: `build`, `lint`, `test`, `vet`, `fmt`, `govulncheck`, `check` (runs all). |
| `renovate.json` | Renovate config. Groups Go minor/patch deps and GitHub Actions updates. |
| `lefthook.yml` | Lefthook pre-commit hooks: format check (`gofmt -l`), lint (`golangci-lint`), test (`go test -race`). |

---

## `.github/workflows/` — CI

| File | Triggers | Jobs |
|------|----------|------|
| `ci.yml` | `pull_request`, `merge_group` | **lint** (golangci-lint incl. staticcheck, govet, gofmt), **test** (`go test -race`), **govulncheck** — all run in parallel |

---

## `cmd/` — CLI Commands (Cobra)

All commands share a package-level `cfg *config.Config` populated by `initConfig()` (registered via `cobra.OnInitialize` in `root.go`). No command imports viper directly.

### Package variables (`root.go`)

| Variable | Type | Description |
|----------|------|-------------|
| `configFile` | `string` | Value of `--config` flag |
| `vaultFlag` | `string` | Value of `--vault` flag |
| `opVaultFlag` | `string` | Value of `--op-vault` flag |
| `cfg` | `*config.Config` | Fully resolved config; set by `initConfig()` before every command |

### `Execute() ` — `root.go`

Exported entry point called by `main`. Runs the root Cobra command; exits with code 1 on error.

### `initConfig()` — `root.go` (unexported)

Calls `config.Load(configFile)`, then applies any `--vault` / `--op-vault` / `--template` / `--output` flag overrides to `cfg` and `cfg.Sources`. Called automatically by `cobra.OnInitialize` before any command's `RunE`.

### Command files

| File | Command | Flags | Description |
|------|---------|-------|-------------|
| `config.go` | `config` | — | Parent command; groups config subcommands |
| `config_init.go` | `config init` | — | Writes `~/.config/envsecrets.toml` from `defaultConfigContent`. Errors if file already exists. Prints tip about `op_vault`. |
| `config_show.go` | `config show` | — | Prints all config options, their current values, and source (`flag` / `env` / `config file` / `default`) from `cfg.Sources`. |
| `store.go` | `store <key> <value>` | — | Writes a secret via `Manager.Set()`. Uses `cfg.Vault` + `cfg.OpVault`. |
| `fetch.go` | `fetch <key>` | — | Reads a secret via `Manager.Get()`. Prints raw value to stdout (no newline). |
| `update.go` | `update <key> <value>` | — | Semantic alias for `store`; calls `Manager.Update()`. |
| `delete.go` | `delete <key>` | `--force` / `-f` | Deletes from both backends via `Manager.Delete()`. Prompts for confirmation unless `--force`. |
| `sync.go` | `sync` | — | Pulls all items from the 1Password vault into Keychain via `Manager.Sync()`. Reports count. |
| `gen_env.go` | `gen-env` | `--template` (default `.env.tpl`), `--output` (default `.env`) | Resolves `secret:` prefixed values in a template file via `Manager.Get()`; copies other lines verbatim. |

---

## `internal/config/` — Configuration

Single home for all viper logic. Viper is imported **only** in this package.

### `Config` struct

Governs all resolved runtime configuration. The `cmd` layer reads from this after `Load` returns and after applying flag overrides.

| Field | Type | Description |
|-------|------|-------------|
| `Vault` | `string` | Name of the dedicated local keychain file. File lives at `~/.local/share/envsecrets/<Vault>.keychain`. Default: `"envsecrets"`. |
| `OpVault` | `string` | 1Password vault name where secrets are stored. Default: `"Envsecrets"`. |
| `Template` | `string` | Path to the `gen-env` template file. Default: `".env.tpl"`. |
| `Output` | `string` | Path to the `gen-env` output file. Default: `".env"`. |
| `FilePath` | `string` | Resolved path to the config file (may not exist on disk). |
| `FileFound` | `bool` | True when the config file was found and successfully read. |
| `Sources` | `Sources` | Per-field attribution of where each value came from. |

### `Sources` struct

Records the origin of each `Config` field. Used by `config show`.

| Field | Type | Possible values |
|-------|------|-----------------|
| `Vault` | `string` | `"default"`, `"config file"`, `"env ($ENVSECRETS_VAULT)"`, `"flag (--vault)"` |
| `OpVault` | `string` | `"default"`, `"config file"`, `"env ($ENVSECRETS_OP_VAULT)"`, `"flag (--op-vault)"` |
| `Template` | `string` | `"default"`, `"config file"`, `"env ($ENVSECRETS_TEMPLATE)"`, `"flag (--template)"` |
| `Output` | `string` | `"default"`, `"config file"`, `"env ($ENVSECRETS_OUTPUT)"`, `"flag (--output)"` |

### Functions

| Signature | Description |
|-----------|-------------|
| `Load(configFlagValue string) *Config` | Resolves the config file path (`--config` flag → `$ENVSECRETS_CONFIG` → `~/.config/envsecrets.toml`), binds env vars, sets defaults, reads the TOML file, attributes sources, and returns a fully-populated `*Config`. Does **not** apply flag overrides — the `cmd` layer does that. |
| `resolvePath(configFlagValue string) string` *(unexported)* | Returns the config file path to use, honouring `--config` flag → `$ENVSECRETS_CONFIG` → default. |
| `sourceOf(key, envVar string, fileKeys map[string]bool) string` *(unexported)* | Returns `"default"`, `"config file"`, or `"env ($VAR)"` for a config key based on whether the env var is set or the key appears in the file. |

---

## `internal/keychain/` — macOS Keychain Backend

Each vault gets a dedicated keychain file at `~/.local/share/envsecrets/<vault>.keychain`. The file's password is auto-generated on first use and stored in the login keychain under service `envsecrets-keychain-<vault>`, and also written to a human-readable access-details file at `~/Documents/envsecrets-<vault>-keychain-access.txt`.

### Errors

| Name | Description |
|------|-------------|
| `ErrNotFound` | Returned when a keychain entry does not exist (wraps exit code 44 from `security`). |

### `Client` struct

Governs all operations against a single named keychain file.

| Field | Type | Description |
|-------|------|-------------|
| `vault` *(unexported)* | `string` | Vault name; determines the keychain file path and login-keychain service name. |
| `keychainPath` *(unexported)* | `string` | Absolute path to the `.keychain` file (`~/.local/share/envsecrets/<vault>.keychain`). |

### Exported functions and methods

| Signature | Description |
|-----------|-------------|
| `New(vault string) *Client` | Constructs a `Client`; sets `keychainPath` to `~/.local/share/envsecrets/<vault>.keychain`. |
| `(c *Client) Available(_ context.Context) bool` | Returns `true` if the `security` binary is on `$PATH`. |
| `(c *Client) EnsureVault(ctx context.Context) (bool, error)` | Creates the keychain file if absent (`true, nil`), or unlocks it if present (`false, nil`). Falls back to the access file if the login-keychain entry is missing. |
| `(c *Client) Get(ctx context.Context, service string) (string, error)` | Reads the generic-password entry for `service`; returns `ErrNotFound` on miss. Calls `ensure` first. |
| `(c *Client) Set(ctx context.Context, service, value string) error` | Upserts a generic-password entry (delete-then-add to avoid duplicates). Calls `ensure` first. |
| `(c *Client) Delete(ctx context.Context, service string) error` | Deletes the generic-password entry; returns `ErrNotFound` if absent. Calls `ensure` first. |
| `(c *Client) List(ctx context.Context) ([]string, error)` | Returns all service names in the keychain via `security dump-keychain` + `ParseDumpServices`. Calls `ensure` first. |
| `ParseDumpServices(output string) []string` | Extracts unique service names from `security dump-keychain` output. Exported for testability. |

### Key unexported methods

| Signature | Description |
|-----------|-------------|
| `(c *Client) ensure(ctx context.Context) error` | Creates or unlocks the keychain file. Called at the top of every public method that accesses the file. |
| `(c *Client) createKeychainFile(ctx context.Context) error` | Generates a random password, creates the keychain file (`security create-keychain`), disables auto-lock, stores the password in the login keychain, and writes the access-details file. |
| `(c *Client) unlockKeychainFile(ctx context.Context) error` | Reads the password via `readKeychainPassword` and calls `security unlock-keychain`. |
| `(c *Client) readKeychainPassword(ctx context.Context) (string, error)` | Primary: reads from the login keychain. Fallback: reads from the access file and restores the login-keychain entry automatically. |
| `(c *Client) storeKeychainPassword(ctx context.Context, password string) error` | Upserts (`-U`) the password into the login keychain under service `envsecrets-keychain-<vault>`. |
| `(c *Client) accessFilePath() string` | Returns `~/Documents/envsecrets-<vault>-keychain-access.txt`. |
| `(c *Client) writeAccessFile(password string) error` | Writes a human-readable file (mode 0600) containing the keychain path, password, and Keychain Access / terminal recovery instructions. Prints the path to stderr. |
| `(c *Client) readAccessFile() (string, error)` | Parses the `password: <hex>` line from the machine-readable section of the access file. |
| `(c *Client) remove(ctx context.Context, service string) error` | Low-level delete of a single generic-password entry; used by `Delete` and `Set`. |
| `generatePassword() (string, error)` | Returns a 64-character hex string from 32 random bytes. |
| `currentUser() string` | Returns `$USER`, falling back to `$LOGNAME`. |

---

## `internal/onepassword/` — 1Password Backend

Wraps the `op` CLI. All secrets are stored as Login items; the item title is the key and the value is in the password field.

### Errors

| Name | Description |
|------|-------------|
| `ErrNotFound` | Returned when a 1Password item does not exist. |
| `ErrUnavailable` | Returned when the `op` binary is absent or the local 1Password app is not running / signed in. |

### `Client` struct

Governs all operations against a single named 1Password vault.

| Field | Type | Description |
|-------|------|-------------|
| `Vault` | `string` | 1Password vault name or ID (e.g. `"Envsecrets"`, `"Work"`). |

### Exported functions and methods

| Signature | Description |
|-----------|-------------|
| `New(vault string) *Client` | Constructs a `Client` for the given vault. |
| `(c *Client) Available(ctx context.Context) bool` | Returns `true` if `op` is on `$PATH` and `op account list` exits 0 (app running and signed in). |
| `(c *Client) EnsureVault(ctx context.Context) (bool, error)` | Lists vaults via `op vault list --format json`. Creates the vault if absent (`true, nil`). Also writes an access-details file to `~/Documents`. |
| `(c *Client) Get(ctx context.Context, key string) (string, error)` | Reads `op://<Vault>/<key>/password`. Returns `ErrNotFound` or `ErrUnavailable` as appropriate. |
| `(c *Client) Set(ctx context.Context, key, value string) error` | Edit-first, then create if `ErrNotFound`. |
| `(c *Client) Delete(ctx context.Context, key string) error` | Removes the Login item whose title is `key`. |
| `(c *Client) List(ctx context.Context) ([]string, error)` | Returns all item titles via `op item list --vault <Vault> --format json` + `ParseTitles`. |
| `ParseVaultNames(jsonStr string) []string` | Extracts `"name"` fields from `op vault list` JSON output. Exported for testability. |
| `ParseTitles(jsonStr string) ([]string, error)` | Extracts `"title"` fields from `op item list` JSON output. Exported for testability. |

### Key unexported methods

| Signature | Description |
|-----------|-------------|
| `(c *Client) create(ctx context.Context, key, value string) error` | Creates a new Login item via `op item create`. |
| `(c *Client) edit(ctx context.Context, key, value string) error` | Updates an existing Login item via `op item edit`. |
| `(c *Client) classifyError(key string, err error) error` | Delegates to `classifyErrorWithOutput` with nil output. |
| `(c *Client) classifyErrorWithOutput(key string, err error, out []byte) error` | Maps op CLI stderr output to `ErrNotFound`, `ErrUnavailable`, or a generic error. |
| `(c *Client) accessFilePath() string` | Returns `~/Documents/envsecrets-<Vault>-1password-access.txt`. |
| `(c *Client) writeAccessFile() error` | Writes a human-readable file (mode 0600) with vault name and instructions for accessing secrets directly in 1Password. Prints the path to stderr. |

---

## `internal/secrets/` — Orchestration Layer

Coordinates reads and writes across the two backends through a single unified interface. Neither backend is imported directly by `cmd/`; all access goes through `Manager`.

### `SecretStore` interface

Both `*keychain.Client` and `*onepassword.Client` implement this interface.

| Method signature | Description |
|-----------------|-------------|
| `Available(ctx context.Context) bool` | Reports whether the backend is reachable. |
| `Get(ctx context.Context, key string) (string, error)` | Retrieves a secret by key. |
| `Set(ctx context.Context, key, value string) error` | Stores or updates a secret. |
| `Delete(ctx context.Context, key string) error` | Removes a secret. |
| `List(ctx context.Context) ([]string, error)` | Returns all keys held by the backend. |
| `EnsureVault(ctx context.Context) (bool, error)` | Creates the backend vault if absent, or unlocks/verifies it if present. Returns `(true, nil)` when newly created. |

### `Manager` struct

Governs the combined read/write/delete/sync logic across the two `SecretStore` backends.

| Field | Type | Description |
|-------|------|-------------|
| `kc` *(unexported)* | `SecretStore` | Keychain backend. |
| `op` *(unexported)* | `SecretStore` | 1Password backend. |
| `warn` *(unexported)* | `io.Writer` | Destination for non-fatal warnings (default: `os.Stderr`). |

### Exported functions and methods

| Signature | Description |
|-----------|-------------|
| `New(keychainVault, opVault string) *Manager` | Wires up `keychain.New(keychainVault)` and `onepassword.New(opVault)` as backends. |
| `NewWithBackends(kc, op SecretStore) *Manager` | Accepts arbitrary `SecretStore` implementations; used in tests with `stubStore`. |
| `(m *Manager) WithWarningWriter(w io.Writer) *Manager` | Overrides the warning writer; returns `m` for chaining. |
| `(m *Manager) Get(ctx context.Context, key string) (string, error)` | Keychain first. On miss, tries 1Password (if available) and caches the result back into Keychain. |
| `(m *Manager) Set(ctx context.Context, key, value string) error` | Calls `kc.EnsureVault` (fatal on error), writes to Keychain (fatal on error), then calls `op.EnsureVault` + `op.Set` (both non-fatal; warnings only). |
| `(m *Manager) Update(ctx context.Context, key, value string) error` | Semantic alias for `Set`; the distinction is at the CLI layer only. |
| `(m *Manager) Delete(ctx context.Context, key string) error` | Attempts both backends; collects errors via `errors.Join`; `ErrNotFound` on either side is silently ignored. |
| `(m *Manager) Sync(ctx context.Context) (synced int, err error)` | Lists all keys from 1Password, fetches each, writes to Keychain. Returns count of successfully written keys. |

---

## Architecture

```
main.go → cmd.Execute()
              │
   cmd/{store,fetch,update,delete,sync,gen_env}.go
              │
   internal/secrets/secrets.go  (Manager)
          /              \
internal/keychain/    internal/onepassword/
keychain.go           onepassword.go
     │                       │
macOS `security`         1Password `op`
```

**Read path:** Keychain → miss → 1Password → cache to Keychain → return  
**Write path:** `kc.EnsureVault` → Keychain (must succeed) → `op.EnsureVault` → 1Password (best-effort)  
**Delete path:** Keychain + 1Password attempted independently; errors joined  
**Sync path:** 1Password `List()` → fetch each → write to Keychain  
**Template path:** scan `.tpl` line-by-line → resolve `secret:` prefixed values → write output  

**Access-details files** (written once at vault creation):  
- Keychain: `~/Documents/envsecrets-<vault>-keychain-access.txt` — contains the keychain password  
- 1Password: `~/Documents/envsecrets-<vault>-1password-access.txt` — contains the vault name and access instructions  
