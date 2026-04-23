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

Calls `config.Load(configFile)`, then iterates `config.AllFields()` and calls `config.ApplyFlag()` for every flag whose `.Changed` is true. Global-scope flags are looked up on `rootCmd.PersistentFlags()`; `"gen-env"`-scope flags on `genEnvCmd.Flags()`. Finally calls `config.Validate(cfg)` — if validation fails it prints the error to stderr and exits with code 1. Called automatically by `cobra.OnInitialize` before any command's `RunE`.

### Command files

| File | Command | Flags | Description |
|------|---------|-------|-------------|
| `config.go` | `config` | — | Parent command; groups config subcommands |
| `config_init.go` | `config init` | — | Writes `~/.config/envsecrets.toml` from `config.GenerateConfigTemplate()`, then prints a tip about the default `op_vault`. Errors if file already exists. |
| `config_show.go` | `config show` | `--verbose` / `-v` | Prints an emoji-grid table (compact default) or per-source blocks (`--verbose` / `-v`). OPTION column width is computed dynamically from the longest key name. Verbose mode: one block per setting, each source shows ✅ + value or 🚫 + `(not set)`, with `⬆️ superseded by …` on losing sources and `🏆 ← active` on the winner. Unexported helpers: `srcRow` (type), `verboseOutput()`, `supersededBy()`, `displayWidth()`, `padRight()`, `boolEmoji()`, `sourceCell()`. |
| `store.go` | `store <key> <value>` | — | Writes a secret via `Manager.Set()` to Keychain and the configured durable store. |
| `fetch.go` | `fetch <key>` | — | Reads a secret via `Manager.Get()`. Prints raw value to stdout (no newline). |
| `update.go` | `update <key> <value>` | — | Semantic alias for `store`; calls `Manager.Set()`. |
| `delete.go` | `delete <key>` | `--force` / `-f` | Deletes from both backends via `Manager.Delete()`. Prompts for confirmation (names the durable store generically) unless `--force`. |
| `sync.go` | `sync` | — | Pulls all items from the configured durable store into Keychain via `Manager.Sync()`. Reports count. |
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

Governs all resolved runtime configuration. The eight configurable fields carry struct tags that drive the entire config system. The `cmd` layer reads from this after `Load` returns and after `ApplyFlag` calls.

| Field | Type | Struct tags | Description |
|-------|------|-------------|-------------|
| `Vault` | `string` | `cfg:"vault" env:"ENVSECRETS_VAULT" flag:"vault" default:"envsecrets" scope:"global"` | Stem name of the dedicated local keychain file. Must match `[a-zA-Z0-9][a-zA-Z0-9_-]*`. File lives at `~/.local/share/envsecrets/<Vault>.keychain`. |
| `OpVault` | `string` | `cfg:"op_vault" env:"ENVSECRETS_OP_VAULT" flag:"op-vault" default:"Envsecrets" scope:"global"` | 1Password vault name where secrets are stored. |
| `DurableBackend` | `string` | `cfg:"durable_backend" env:"ENVSECRETS_DURABLE_BACKEND" flag:"durable-backend" default:"1password" scope:"global"` | Selects the durable backend: `"1password"` (default), `"keepassxc"`, or `"keeper"`. Unrecognised values fall back to `"1password"` with a warning. |
| `KpxcDB` | `string` | `cfg:"kpxc_db" env:"ENVSECRETS_KPXC_DB" flag:"kpxc-db" default:"envsecrets" scope:"global"` | Stem name of the KeePassXC database. Must match `[a-zA-Z0-9][a-zA-Z0-9_-]*`. Database lives at `~/.local/share/envsecrets/<KpxcDB>.kdbx`. |
| `KsmConfig` | `string` | `cfg:"ksm_config" env:"ENVSECRETS_KSM_CONFIG" flag:"ksm-config" default:"~/.config/envsecrets/ksm-config.json" scope:"global"` | Path to the Keeper Secrets Manager device-credentials JSON file. Created automatically on first use when a One-Time Access Token is provided. Used when `DurableBackend == "keeper"`. |
| `KsmFolder` | `string` | `cfg:"ksm_folder" env:"ENVSECRETS_KSM_FOLDER" flag:"ksm-folder" default:"" scope:"global"` | Keeper Secrets Manager shared folder UID. Required only when creating new secrets; reads and deletes work without it. Used when `DurableBackend == "keeper"`. |
| `Template` | `string` | `cfg:"template" env:"ENVSECRETS_TEMPLATE" flag:"template" default:".env.tpl" scope:"gen-env"` | Path to the `gen-env` template file. |
| `Output` | `string` | `cfg:"output" env:"ENVSECRETS_OUTPUT" flag:"output" default:".env" scope:"gen-env"` | Path to the `gen-env` output file. |
| `FilePath` | `string` | — | Resolved path to the config file (may not exist on disk). Not iterated by `AllFields`. |
| `FileFound` | `bool` | — | True when the config file was found and successfully read. Not iterated by `AllFields`. |
| `Sources` | `map[string]SourceState` | — | Per-field source attribution, keyed by the `cfg` tag value (e.g. `"vault"`). Not iterated by `AllFields`. |

### Functions

| Signature | Description |
|-----------|-------------|
| `AllFields() []FieldMeta` | Returns `FieldMeta` for every struct field that has a `cfg` tag (currently 8: vault, op\_vault, durable\_backend, kpxc\_db, ksm\_config, ksm\_folder, template, output). Fields without a `cfg` tag are skipped. |
| `GlobalFields() []FieldMeta` | Subset of `AllFields()` where `Scope == "global"`. Delegates to `ScopedFields("global")`. Used by `root.go` for `PersistentFlags` registration. |
| `ScopedFields(scope string) []FieldMeta` | Subset of `AllFields()` where `Scope == scope`. Used by `gen_env.go` with `"gen-env"`. |
| `GetValue(cfg *Config, m FieldMeta) string` | Returns the current string value of the `Config` field described by `m`, using `m.FieldIndex` directly. Used by `config show` to retrieve each field's resolved value. |
| `ApplyFlag(cfg *Config, key, value string)` | Sets the `Config` field identified by `key` to `value`, sets `SourceFlag` bit on `cfg.Sources[key].Flags`, captures `FlagValue`, and sets `Active=ActiveFlag`. Called by `initConfig` for every changed CLI flag. |
| `ValidateStem(s string) bool` | Reports whether `s` is a valid stem name: matches `[a-zA-Z0-9][a-zA-Z0-9_-]*`. Used by `Validate`. |
| `Validate(cfg *Config) error` | Checks that `cfg.Vault` and `cfg.KpxcDB` are valid stem names. Returns a joined error for every invalid field. Called by `initConfig`; on failure prints to stderr and exits 1. |
| `Load(configFlagValue string) *Config` | Resolves the config file path (`--config` flag → `$ENVSECRETS_CONFIG` → `~/.config/envsecrets.toml`), binds env vars and defaults from struct tags, reads the TOML file, populates all fields via reflection, and returns a fully-populated `*Config`. Does **not** apply flag overrides — the `cmd` layer calls `ApplyFlag` for that. |
| `GenerateConfigTemplate() string` | Returns a documented TOML config file string, generated dynamically from struct tags. Each field gets a comment block with usage text, CLI flag name, and env var name. Used by `config init`. |
| `resolvePath(configFlagValue string) string` *(unexported)* | Returns the config file path to use, honouring `--config` flag → `$ENVSECRETS_CONFIG` → default (`~/.config/envsecrets.toml`). |

---

## `internal/keychain/` — macOS Keychain Backend

Each vault gets a dedicated keychain file at `~/.local/share/envsecrets/<vault>.keychain`. The file's password is auto-generated on first use and stored in the login keychain under service `envsecrets-keychain-<vault>`, and also written to a human-readable access-details file at `~/Documents/envsecrets-<vault>-keychain-access.txt`.

### Errors

| Name | Description |
|------|-------------|
| `ErrNotFound` | Returned when a keychain entry does not exist (wraps exit code 44 from `security`). Alias for `storeerr.ErrNotFound`. |

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
| `ErrNotFound` | Returned when a 1Password item does not exist. Alias for `storeerr.ErrNotFound`. |
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

## `internal/keepassxc/` — KeePassXC Backend

Wraps the `keepassxc-cli` tool to store and retrieve secrets from a KeePass database file. Implements `secrets.SecretStore`. The database password is auto-generated on first use and stored in the macOS login keychain under service `envsecrets-keepassxc-<stem>` (where `<stem>` is the `kpxc_db` config value), mirroring the keychain backend's approach. A human-readable access-details file is also written to `~/Documents/envsecrets-<stem>-keepassxc-access.txt`.

### Errors

| Name | Description |
|------|-------------|
| `ErrNotFound` | Returned when a KeePassXC entry does not exist (`keepassxc-cli` stderr contains "could not find entry"). Alias for `storeerr.ErrNotFound`. |
| `ErrUnavailable` | Returned when `keepassxc-cli` is not on `$PATH`. |
| `ErrInvalidKey` | Returned by `ValidateKey` when a key is empty, contains `"/"` (KeePassXC path separator), contains `"\n"` or `"\r"` (break the stdin protocol), or has leading/trailing Unicode whitespace (causes `List` round-trip mismatches). |

### `Client` struct

| Field | Type | Description |
|-------|------|-------------|
| `stem` *(unexported)* | `string` | Database stem name (= `kpxc_db`); drives all three related artifacts: the `.kdbx` path, the login-keychain service name (`envsecrets-keepassxc-<stem>`), and the access-details filename. |
| `dbPath` *(unexported)* | `string` | Path to the `.kdbx` database file (see `DefaultDBPath`; typically absolute but may be relative if the home directory cannot be resolved). |

### Exported functions and methods

| Signature | Description |
|-----------|-------------|
| `DefaultDBPath(stem string) string` | Returns `~/.local/share/envsecrets/<stem>.kdbx`. Used by `New` and exposed so callers can display the resolved path. |
| `New(stem string) *Client` | Constructs a `Client`. `stem` is a bare name (e.g. `"envsecrets"`) that drives the database path, keychain service name, and access-details filename. |
| `(c *Client) Available(_ context.Context) bool` | Returns `true` if `keepassxc-cli` is on `$PATH`. |
| `(c *Client) EnsureVault(ctx context.Context) (bool, error)` | Creates the database if absent (`true, nil`); runs a `keepassxc-cli ls --quiet` probe to verify the stored password actually unlocks the existing DB (`false, nil`). Returns `ErrUnavailable` if `keepassxc-cli` is missing. |
| `(c *Client) Get(ctx context.Context, key string) (string, error)` | Reads the Password field via `keepassxc-cli show --attributes Password`. Returns `ErrNotFound` on miss, `ErrInvalidKey` if key fails `ValidateKey`. |
| `(c *Client) Set(ctx context.Context, key, value string) error` | edit-first (`keepassxc-cli edit --password-prompt`), then add on `ErrNotFound`. Returns `ErrInvalidKey` if key fails `ValidateKey`. |
| `(c *Client) Delete(ctx context.Context, key string) error` | Removes the entry via `keepassxc-cli rm`. Returns `ErrNotFound` if absent, `ErrInvalidKey` if key fails `ValidateKey`. |
| `(c *Client) List(ctx context.Context) ([]string, error)` | Returns root-level entry names via `keepassxc-cli ls` + `ParseListOutput`. Titles are preserved exactly as stored. |
| `ValidateKey(key string) error` | Single enforcement point for key constraints. Rejects: empty, contains `"/"`, contains `"\n"` or `"\r"`, leading/trailing Unicode whitespace. Returns `ErrInvalidKey` on failure, `nil` on success. Called by `Get`/`Set`/`Delete`; exported for caller pre-validation. |
| `ParseListOutput(output string) []string` | Extracts root-level entry names from `keepassxc-cli ls` output. Strips only a trailing `\r` per line (CRLF safety); skips blank lines, group names (suffix `/`), and lines with a leading Unicode whitespace rune (indented sub-entries). Titles are appended verbatim — no further normalisation. |

### Key unexported methods

| Signature | Description |
|-----------|-------------|
| `(c *Client) add(ctx, key, value string) error` | Creates a new entry via `keepassxc-cli add --password-prompt`; stdin: `dbpw\nvalue\nvalue\n`. |
| `(c *Client) edit(ctx, key, value string) error` | Updates an existing entry via `keepassxc-cli edit --password-prompt`; stdin: `dbpw\nvalue\nvalue\n`. Returns `ErrNotFound` if entry absent. |
| `(c *Client) createDB(ctx) error` | Generates a random password, creates the `.kdbx` file via `keepassxc-cli db-create --set-password`, stores the password in login keychain, writes access file. |
| `(c *Client) readPassword(ctx) (string, error)` | Reads db password from login keychain (service `envsecrets-keepassxc-<stem>`). Falls back to access file on exit code 44 (item not found) and restores the keychain entry automatically. Other errors are returned immediately. |
| `(c *Client) storePassword(ctx, password string) error` | Upserts (`-U`) the password into the login keychain under service `envsecrets-keepassxc-<stem>`. |
| `firstRune(s string) rune` *(free function)* | Returns the first Unicode rune of a non-empty string; used by `ValidateKey`. |
| `lastRune(s string) rune` *(free function)* | Returns the last Unicode rune of a non-empty string; used by `ValidateKey`. |
| `classifyError(key string, out []byte) error` *(free function)* | Maps `keepassxc-cli` stderr to `ErrNotFound` or a generic error. |
| `(c *Client) accessFilePath() string` | Returns `~/Documents/envsecrets-<stem>-keepassxc-access.txt`. |
| `(c *Client) writeAccessFile(password string) error` | Writes a human-readable file (mode 0600) with db path, password, and GUI/CLI recovery instructions. |
| `(c *Client) readAccessFile() (string, error)` | Parses the `password: <hex>` line from the access file's machine-readable section. |
| `generatePassword() (string, error)` | Returns a 64-character hex string from 32 random bytes. |
| `currentUser() string` | Returns `$USER`, falling back to `$LOGNAME`. |

---

## `internal/keeper/` — Keeper Secrets Manager Backend

Wraps the Keeper Secrets Manager Go SDK (`github.com/keeper-security/secrets-manager-go/core`) to store and retrieve secrets from a cloud-hosted KSM application. Implements `secrets.SecretStore`. Authentication uses a One-Time Access Token (OAT) for initial device registration; subsequent calls use stored cryptographic credentials in a local JSON config file. The SDK authenticates at device level — SSO does not participate after the initial OAT setup.

Secrets are stored exclusively as Keeper `login` records (title = key, password field = value). Records with non-login types or duplicate titles cause explicit errors rather than silent data loss.

### Sentinel errors

| Name | When returned |
|------|--------------|
| `ErrNotFound` | Returned by `Get`, `Delete` when no record with that title exists. Alias for `storeerr.ErrNotFound`. |
| `ErrUnavailable` | Returned by `loadManager` when the config file cannot be loaded (absent or corrupt). Wraps the error from `loadManager`. |
| `ErrDuplicateTitles` | Returned by `lookupOne` when `GetSecretsByTitle` returns more than one result. Keeper does not enforce title uniqueness; the user must resolve duplicates in the web console. NOT caught by `isDurableNotFound`. |
| `ErrWrongType` | Returned by `lookupOne` when the matching record's type is not `"login"`. NOT caught by `isDurableNotFound`. |

### `Client` struct

| Field | Type | Description |
|-------|------|-------------|
| `configPath` *(unexported)* | `string` | Path to `ksm-config.json`; stores device credentials (private key, device ID, client ID) after first OAT initialisation. `~/` prefix is expanded at construction time. |
| `folderUID` *(unexported)* | `string` | Keeper shared folder UID used when creating new records. May be empty for read-only use; `Set` returns a clear error if empty when creation is needed. |
| `mu` *(unexported)* | `sync.Mutex` | Guards lazy initialisation of `sm`. |
| `sm` *(unexported)* | `*ksm.SecretsManager` | Cached SecretsManager; nil until first `loadManager` call. Reused across calls to avoid repeated config-file reads. NOT set from the OAT-initialised manager created during first-time `EnsureVault`. |

### Exported functions and methods

| Signature | Description |
|-----------|-------------|
| `New(configPath, folderUID string) *Client` | Returns a Client. Expands `~/` in `configPath`. Does not make any network calls. |
| `(c *Client) Available(_ context.Context) bool` | Returns `true` unconditionally. First-time setup is handled by `EnsureVault`; returning `true` here ensures `Manager.Set` does not skip the `EnsureVault` call that prompts for the OAT on first use. No network call. |
| `(c *Client) EnsureVault(_ context.Context) (bool, error)` | First-time: validates and prompts for OAT (no echo on TTY, pipe-safe), pre-creates the config file at `0600` to prevent the SDK's `os.Create` at `0666`, registers device with KSM via `initManager`, runs a probe; on probe failure removes the partial config so the next run re-prompts. If the config file appears concurrently (race between `Stat` and `OpenFile`), returns an error without consuming the OAT. Subsequent: best-effort `chmod` on the existing file only (`0600`) to tighten permissions from older or manually-created configs — the parent directory is not re-chmoded to avoid surprising users who point `--ksm-config` at a shared directory. Returns `(true, nil)` on first creation, `(false, nil)` if already valid. All SDK writes go through `secureStorage` which keeps the file at `0600`. |
| `(c *Client) Get(_ context.Context, key string) (string, error)` | Validates key (via `validateKey`), calls `lookupOne`, returns `record.Password()`. Returns `fmt.Errorf("keeper: get %q: %w", key, ErrNotFound)` when absent, or `ErrDuplicateTitles`/`ErrWrongType` from `lookupOne`. |
| `(c *Client) Set(_ context.Context, key, value string) error` | Validates key; if record exists calls `SetPassword`+`Save`; if absent creates a new `"login"` record with username `"envsecrets"` via `NewRecordCreate`+`NewLogin`+`NewPassword`+`CreateSecretWithRecordData`. Errors on `ErrDuplicateTitles`, `ErrWrongType`, or empty `folderUID` on create. |
| `(c *Client) Delete(_ context.Context, key string) error` | Validates key, calls `lookupOne`; returns `fmt.Errorf("keeper: delete %q: %w", key, ErrNotFound)` when absent, calls `DeleteSecrets([]string{record.Uid})` otherwise. Errors on `ErrDuplicateTitles` or `ErrWrongType`. |
| `(c *Client) List(_ context.Context) ([]string, error)` | Calls `sm.GetSecrets([]string{})`, skips non-login records, counts titles, warns on stderr and excludes duplicate titles. Returns sorted slice. One full-vault network fetch per call. |

### Key unexported functions

| Signature | Description |
|-----------|-------------|
| `(c *Client) lookupOne(sm *ksm.SecretsManager, key string) (*ksm.Record, error)` | Calls `sm.GetSecretsByTitle(key)`. Returns `(nil, nil)` when absent, `(record, nil)` for exactly one login record, `ErrDuplicateTitles` for multiple matches, `ErrWrongType` when the single match is not a `"login"` record. Never swallows SDK errors. |
| `(c *Client) loadManager() (*ksm.SecretsManager, error)` | Returns the cached `*ksm.SecretsManager`, creating it from `newSecureStorage(c.configPath)` on the first call. Guards the lazy-init with `c.mu`. |
| `(c *Client) initManager(oat string) (*ksm.SecretsManager, error)` | Creates a `*ksm.SecretsManager` with `opts.Token = oat` and `newSecureStorage` for one-time device registration. Result is NOT cached. |
| `readOAT() (string, error)` *(free function)* | Prompts on stderr. If stdin is a TTY, uses `term.ReadPassword` (no echo, from `golang.org/x/term`). If stdin is a pipe, reads one line with `bufio.Scanner`. Trims whitespace; validates via `validateOAT`; returns error on empty or malformed result. |
| `validateOAT(oat string) error` *(free function)* | Rejects tokens that would trigger the SDK's `klog.Warning` with the raw token: allows bare legacy tokens (no colon) and exactly `HOST:BASE64KEY`; rejects empty parts or >2 colon-separated parts. |
| `validateKey(key string) error` *(free function)* | Rejects empty keys, keys with leading/trailing whitespace, and keys containing embedded control characters (`\n`, `\r`, `\t`, `\x00`). |
| `newSecureStorage(configPath string) *secureStorage` *(free function)* | Wraps `ksm.NewFileKeyValueStorage` in a `secureStorage` that chmods the config file to `0600` after every write (`Set`, `Delete`, `DeleteAll`, `SaveStorage`). Eliminates the window where the SDK would leave the private key world-readable. The parent directory is secured once in `EnsureVault`, not re-chmodded on every write, so pointing `ksm-config` at a pre-existing directory does not alter that directory's permissions. |

---

## `internal/storeerr/` — Shared Error Sentinels

Leaf package that defines error values shared across all secret-store backends. Backends import this package to avoid an import cycle: `internal/secrets/` (the orchestration layer) also imports it.

| Symbol | Description |
|--------|-------------|
| `ErrNotFound` | Returned by any backend when a requested key does not exist. All `errors.Is(err, storeerr.ErrNotFound)` checks in the orchestration layer match this single sentinel regardless of which backend returned the error. |

Each backend (`keychain`, `onepassword`, `keepassxc`, `keeper`) declares `var ErrNotFound = storeerr.ErrNotFound` — callers that import a backend directly can still write `errors.Is(err, keychain.ErrNotFound)` and it resolves to the same value.

---

## `internal/secrets/` — Orchestration Layer

Coordinates reads and writes across the two backends through a single unified interface. Neither backend is imported directly by `cmd/`; all access goes through `Manager`.

### `SecretStore` interface

Implemented by `*keychain.Client`, `*onepassword.Client`, `*keepassxc.Client`, and `*keeper.Client`.

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
| `kc` *(unexported)* | `SecretStore` | Keychain backend (fast local cache). |
| `durable` *(unexported)* | `SecretStore` | Durable backend — `*onepassword.Client`, `*keepassxc.Client`, or `*keeper.Client`, chosen at construction time. |
| `durableName` *(unexported)* | `string` | Display name for the durable backend (`"1Password"`, `"KeePassXC"`, or `"Keeper"`). Used in warning and error messages. |
| `warn` *(unexported)* | `io.Writer` | Destination for non-fatal warnings (default: `os.Stderr`). |

### Exported functions and methods

| Signature | Description |
|-----------|-------------|
| `New(keychainVault, opVault, durableBackend, kpxcDB, ksmConfig, ksmFolder string) *Manager` | Constructs a Manager. `durableBackend` selects `"keepassxc"` → `keepassxc.New(kpxcDB)`, `"keeper"` → `keeper.New(ksmConfig, ksmFolder)`, or `""` / `"1password"` → `onepassword.New(opVault)`; unrecognised values fall back to 1Password with a stderr warning (always stderr; emitted before `WithWarningWriter` can take effect). |
| `NewWithBackends(kc, durable SecretStore, durableName string) *Manager` | Accepts arbitrary `SecretStore` implementations and an explicit display name for the durable backend; used in tests with `stubStore`. |
| `(m *Manager) WithWarningWriter(w io.Writer) *Manager` | Overrides the warning writer; returns `m` for chaining. |
| `(m *Manager) Get(ctx context.Context, key string) (string, error)` | Keychain first. On miss, tries the durable store (if available) and caches the result back into Keychain. |
| `(m *Manager) Set(ctx context.Context, key, value string) error` | Calls `kc.EnsureVault` (fatal on error), writes to Keychain (fatal on error), then calls `durable.EnsureVault` + `durable.Set` (both non-fatal; warnings only). |
| `(m *Manager) Delete(ctx context.Context, key string) error` | Attempts both backends; collects errors via `errors.Join`; `ErrNotFound` on either side is silently ignored. |
| `(m *Manager) Sync(ctx context.Context) (synced int, err error)` | Lists all keys in the durable store, fetches each, writes to Keychain. Returns count of successfully written keys. For KeePassXC only, calls `durable.EnsureVault` first so the `.kdbx` file can be created on a fresh machine; for remote-backed backends (1Password) Sync does NOT auto-create — a mistyped vault name must surface as a listing error rather than silently create an empty vault. |

---

## Architecture

```
main.go → cmd.Execute()
              │
   cmd/{store,fetch,update,delete,sync,gen_env}.go
              │
   internal/secrets/secrets.go  (Manager)
          /              \
internal/keychain/    internal/onepassword/   internal/keepassxc/   internal/keeper/
keychain.go           onepassword.go          keepassxc.go          keeper.go
     │                       │                      │                     │
macOS `security`         1Password `op`       keepassxc-cli         KSM cloud API
```

The durable backend slot (right side) is selected at construction time by `cfg.DurableBackend`:
- `"1password"` (default) → `*onepassword.Client`
- `"keepassxc"` → `*keepassxc.Client`
- `"keeper"` → `*keeper.Client`

**Read path:** Keychain → miss → durable store → cache to Keychain → return  
**Write path:** `kc.EnsureVault` → Keychain (must succeed) → `durable.EnsureVault` → durable store (best-effort)  
**Delete path:** Keychain + durable store attempted independently; errors joined  
**Sync path:** durable store `List()` → fetch each → write to Keychain  
**Template path:** scan `.tpl` line-by-line → resolve `secret:` prefixed values → write output  

**Access-details files** (written once at vault/database creation):  
- Keychain: `~/Documents/envsecrets-<vault>-keychain-access.txt` — contains the keychain password  
- 1Password: `~/Documents/envsecrets-<vault>-1password-access.txt` — contains the vault name and access instructions  
- KeePassXC: `~/Documents/envsecrets-<stem>-keepassxc-access.txt` — contains the database path and password (where `<stem>` is the `kpxc_db` config value)
- Keeper: `~/.config/envsecrets/ksm-config.json` (default path) — device-credentials JSON written by the SDK on first OAT registration; no separate human-readable access file is created  
