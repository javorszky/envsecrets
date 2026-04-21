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
| `.golangci.yml` | golangci-lint v2 config. Standard linters + `gofmt`. `errcheck` configured to exclude `fmt.Fprintf/Fprintln/Fprint`. |
| `Makefile` | Local dev commands: `build`, `lint`, `test`, `vet`, `fmt`, `govulncheck`, `check` (runs all). |
| `renovate.json` | Renovate config. Groups Go minor/patch deps and GitHub Actions updates. |
| `lefthook.yml` | Lefthook pre-commit hooks: format check (`gofmt -l`), lint (`golangci-lint`), test (`go test -race`). |

---

## `.github/workflows/` — CI

All action refs are pinned to full commit SHAs (with tag comment) to prevent supply-chain attacks (`unpinned-uses`). All `actions/checkout` steps set `persist-credentials: false` (`artipacked`).

| File | Triggers | Jobs |
|------|----------|------|
| `ci.yml` | `pull_request`, `merge_group`, `workflow_call` | **lint** (golangci-lint incl. staticcheck, govet, gofmt), **test** (`go test -race`), **govulncheck**, **trivy** (SARIF to Security tab) — all run in parallel |
| `codeql.yml` | `pull_request` (branches: main), `merge_group` | **analyze** — CodeQL semantic analysis for Go using the `security-and-quality` query suite; uploads SARIF results to GitHub Security tab |
| `release.yml` | `push` (semver tags), `workflow_dispatch` | **ci** (calls `ci.yml`), **goreleaser-check**, **release** (GoReleaser + SBOM). Permissions are job-scoped: only the `release` job has `contents: write` and `id-token: write`. |
| `zizmor.yml` | `push` (main), `pull_request`, `merge_group` | **zizmor** — static analysis of all workflow files via `uvx zizmor --format=sarif`; uploads SARIF to GitHub Security tab under category `zizmor` |

---

## `cmd/` — CLI Commands (Cobra)

All commands share a package-level `cfg *config.Config` populated by `initConfig()` (registered via `cobra.OnInitialize` in `root.go`). No command imports viper directly.

### Package variables (`root.go`)

| Variable | Type | Description |
|----------|------|-------------|
| `configFile` | `string` | Value of `--config` flag |
| `cfg` | `*config.Config` | Fully resolved config; set by `initConfig()` before every command |

### `Execute() ` — `root.go`

Exported entry point called by `main`. Runs the root Cobra command; exits with code 1 on error.

### `initConfig()` — `root.go` (unexported)

Calls `config.Load(configFile)`, then iterates `config.AllFields()` and calls `config.ApplyFlag()` for every flag whose `.Changed` is true. Global-scope flags are looked up on `rootCmd.PersistentFlags()`; `"gen-env"`-scope flags on `genEnvCmd.Flags()`. Called automatically by `cobra.OnInitialize` before any command's `RunE`.

### Command files

| File | Command | Flags | Description |
|------|---------|-------|-------------|
| `config.go` | `config` | — | Parent command; groups config subcommands |
| `config_init.go` | `config init` | — | Writes `~/.config/envsecrets.toml` from `config.GenerateConfigTemplate()`. Errors if file already exists. Prints tip about `op_vault`. |
| `config_show.go` | `config show` | `--verbose` / `-v` | Prints an emoji-grid table (compact default) or per-source blocks (`--verbose` / `-v`). Verbose mode: one block per setting, each source shows ✅ + value or 🚫 + `(not set)`, with `⬆️ superseded by …` on losing sources and `🏆 ← active` on the winner. Unexported helpers: `srcRow` (type), `verboseOutput()`, `supersededBy()`, `displayWidth()`, `padRight()`, `boolEmoji()`, `sourceCell()`. |
| `store.go` | `store <key> <value>` | — | Writes a secret via `Manager.Set()`. Uses `cfg.Vault` + `cfg.OpVault`. |
| `fetch.go` | `fetch <key>` | — | Reads a secret via `Manager.Get()`. Prints raw value to stdout (no newline). |
| `update.go` | `update <key> <value>` | — | Semantic alias for `store`; calls `Manager.Set()`. |
| `delete.go` | `delete <key>` | `--force` / `-f` | Deletes from both backends via `Manager.Delete()`. Prompts for confirmation unless `--force`. |
| `sync.go` | `sync` | — | Pulls all items from the 1Password vault into Keychain via `Manager.Sync()`. Reports count. |
| `gen_env.go` | `gen-env` | `--template` (default `.env.tpl`), `--output` (default `.env`) | Resolves `secret:` prefixed values in a template file via `Manager.Get()`; copies other lines verbatim. Flags registered dynamically from `config.ScopedFields("gen-env")`. |

---

## `internal/config/` — Configuration

Single home for all viper logic. Viper is imported **only** in this package. The `Config` struct is the single source of truth — struct tags drive flag registration, env var binding, default values, and config-file template generation. No setting needs to be specified in more than one place.

### `FieldMeta` struct

Parsed metadata for one configurable field. Returned by `AllFields()`, `GlobalFields()`, and `ScopedFields()`.

| Field | Type | Description |
|-------|------|-------------|
| `FieldIndex` | `int` | Position of the field in the `Config` struct (used for reflection-based get/set). |
| `FieldName` | `string` | Go field name, e.g. `"Vault"`. |
| `Key` | `string` | TOML key and viper key, e.g. `"op_vault"`. |
| `EnvVar` | `string` | Environment variable name, e.g. `"ENVSECRETS_OP_VAULT"`. |
| `Flag` | `string` | Cobra flag name, e.g. `"op-vault"`. |
| `Default` | `string` | Built-in default value. |
| `Usage` | `string` | Flag help text and config file comment. |
| `Scope` | `string` | `"global"` (registered on `rootCmd.PersistentFlags`) or `"gen-env"` (registered on `genEnvCmd.Flags`). |

### `ActiveSource` type

A `uint8` identifying the highest-priority source currently active for a config field.

| Symbol | Value | Meaning |
|--------|-------|---------|
| `ActiveDefault` | `0` | No file/env/flag present; using built-in default. |
| `ActiveFile` | `1` | Config file value is active. |
| `ActiveEnv` | `2` | Environment variable is active. |
| `ActiveFlag` | `3` | CLI flag is active. |

### `SourceFlags` type

A `byte` bitmask recording which sources contributed a value for one config field.

| Symbol | Value | Meaning |
|--------|-------|---------|
| `SourceFile` | `1 << 0` | Key was explicitly present in the config file. |
| `SourceEnv` | `1 << 1` | Env var was non-empty at load time. |
| `SourceFlag` | `1 << 2` | CLI flag was explicitly provided on the command line. |

| Method | Description |
|--------|-------------|
| `(f SourceFlags) Has(flag SourceFlags) bool` | Reports whether `flag` bits are set in `f`. |
| `(f SourceFlags) With(flag SourceFlags) SourceFlags` | Returns a copy of `f` with `flag` bits set. Does not mutate `f`. |
| `(f SourceFlags) String() string` | Returns `"none"`, `"file"`, `"env|flag"`, etc. for debugging and test output. |

### `SourceState` struct

Records which sources contributed a value for one config field and which is winning.

| Field | Type | Description |
|-------|------|-------------|
| `Flags` | `SourceFlags` | Bitmask of set sources. Use `Flags.Has(SourceFile)` etc. |
| `Active` | `ActiveSource` | Winning source: one of `ActiveDefault`, `ActiveFile`, `ActiveEnv`, `ActiveFlag`. |
| `FileValue` | `string` | Value as read from the config file; empty when `SourceFile` is not set. Populated by `Load()`. |
| `EnvValue` | `string` | Value of the env var at load time; empty when `SourceEnv` is not set. Populated by `Load()`. |
| `FlagValue` | `string` | Value passed via CLI flag; empty when `SourceFlag` is not set. Populated by `ApplyFlag()`. |

### `Config` struct

Governs all resolved runtime configuration. The four configurable fields carry struct tags that drive the entire config system. The `cmd` layer reads from this after `Load` returns and after `ApplyFlag` calls.

| Field | Type | Struct tags | Description |
|-------|------|-------------|-------------|
| `Vault` | `string` | `cfg:"vault" env:"ENVSECRETS_VAULT" flag:"vault" default:"envsecrets" scope:"global"` | Name of the dedicated local keychain file. File lives at `~/.local/share/envsecrets/<Vault>.keychain`. |
| `OpVault` | `string` | `cfg:"op_vault" env:"ENVSECRETS_OP_VAULT" flag:"op-vault" default:"Envsecrets" scope:"global"` | 1Password vault name where secrets are stored. |
| `Template` | `string` | `cfg:"template" env:"ENVSECRETS_TEMPLATE" flag:"template" default:".env.tpl" scope:"gen-env"` | Path to the `gen-env` template file. |
| `Output` | `string` | `cfg:"output" env:"ENVSECRETS_OUTPUT" flag:"output" default:".env" scope:"gen-env"` | Path to the `gen-env` output file. |
| `FilePath` | `string` | — | Resolved path to the config file (may not exist on disk). Not iterated by `AllFields`. |
| `FileFound` | `bool` | — | True when the config file was found and successfully read. Not iterated by `AllFields`. |
| `Sources` | `map[string]SourceState` | — | Per-field source attribution, keyed by the `cfg` tag value (e.g. `"vault"`). Not iterated by `AllFields`. |

### Functions

| Signature | Description |
|-----------|-------------|
| `AllFields() []FieldMeta` | Returns `FieldMeta` for every struct field that has a `cfg` tag (currently 4: vault, op\_vault, template, output). Fields without a `cfg` tag are skipped. |
| `GlobalFields() []FieldMeta` | Subset of `AllFields()` where `Scope == "global"`. Delegates to `ScopedFields("global")`. Used by `root.go` for `PersistentFlags` registration. |
| `ScopedFields(scope string) []FieldMeta` | Subset of `AllFields()` where `Scope == scope`. Used by `gen_env.go` with `"gen-env"`. |
| `GetValue(cfg *Config, m FieldMeta) string` | Returns the current string value of the `Config` field described by `m`, using `m.FieldIndex` directly. Used by `config show` to retrieve each field's resolved value. |
| `ApplyFlag(cfg *Config, key, value string)` | Sets the `Config` field identified by `key` to `value`, sets `SourceFlag` bit on `cfg.Sources[key].Flags`, captures `FlagValue`, and sets `Active=ActiveFlag`. Called by `initConfig` for every changed CLI flag. |
| `Load(configFlagValue string) *Config` | Resolves the config file path (`--config` flag → `$ENVSECRETS_CONFIG` → `~/.config/envsecrets.toml`), binds env vars and defaults from struct tags, reads the TOML file, populates all fields via reflection, and returns a fully-populated `*Config`. Does **not** apply flag overrides — the `cmd` layer calls `ApplyFlag` for that. |
| `GenerateConfigTemplate() string` | Returns a documented TOML config file string, generated dynamically from struct tags. Each field gets a comment block with usage text, CLI flag name, and env var name. Used by `config init`. |
| `resolvePath(configFlagValue string) string` *(unexported)* | Returns the config file path to use, honouring `--config` flag → `$ENVSECRETS_CONFIG` → default (`~/.config/envsecrets.toml`). |

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
| `(c *Client) EnsureVault(ctx context.Context) (bool, error)` | Checks vault existence with `op vault get <name>` (exit-code only — no JSON parsing). Creates the vault if absent (`true, nil`); returns `(false, nil)` if it already exists. Also writes an access-details file to `~/Documents` on creation. |
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
| `classifyError(key string, err error) error` *(free function)* | Delegates to `classifyErrorWithOutput` with nil output. |
| `classifyErrorWithOutput(key string, err error, out []byte) error` *(free function)* | Maps op CLI stderr output to `ErrNotFound`, `ErrUnavailable`, or a generic error. |
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
