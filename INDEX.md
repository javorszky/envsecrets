# envsecrets - File Index

Codebase table of contents for quick navigation. Consult this before scanning the repo.

## Project Root

| File | Purpose |
|------|---------|
| `main.go` | Entry point. Calls `cmd.Execute()`. |
| `go.mod` | Module `github.com/javorszky/envsecrets`, Go 1.26.2. Depends on `spf13/cobra`, `spf13/viper`, `stretchr/testify` (test). |
| `go.sum` | Dependency checksums. |
| `LICENSE` | MIT license. |
| `README.md` | User-facing documentation: installation, usage, architecture. |
| `.gitignore` | Ignores binary, IDE files, `.env` files (except `.env.tpl`), coverage output. |
| `.golangci.yml` | golangci-lint v2 config. Standard linters + `gofmt`. |
| `Makefile` | Local dev commands: `build`, `lint`, `test`, `vet`, `fmt`, `govulncheck`, `check` (runs all). |
| `renovate.json` | Renovate config. Groups Go minor/patch deps and GitHub Actions updates. |
| `lefthook.yml` | Lefthook pre-commit hooks: format check (`gofmt -l`), lint (`golangci-lint`), test (`go test -race`). |

## `.github/workflows/` - CI

| File | Triggers | Jobs | Description |
|------|----------|------|-------------|
| `ci.yml` | `pull_request`, `merge_group` | **lint** (golangci-lint incl. staticcheck, govet, gofmt), **test** (`go test -race`), **govulncheck** | CI pipeline. All three jobs run in parallel. |

## `cmd/` - CLI Commands (Cobra)

All commands share a package-level `cfg *config.Config` populated by `initConfig()` (registered via `cobra.OnInitialize` in `root.go`). No command imports viper directly.

| File | Command | Args / Flags | Description |
|------|---------|-------------|-------------|
| `root.go` | (root) | `--vault` (persistent), `--config` (persistent) | Root command setup. Exports `Execute()`. Calls `config.Load(configFile)` in `initConfig()`, then applies CLI flag overrides to the returned `*config.Config`. Stores result in package-level `cfg`. |
| `config.go` | `config` | none | Parent command for configuration subcommands. |
| `config_init.go` | `config init` | none | Writes `~/.config/envsecrets.toml` with defaults and inline documentation. Uses `cfg.FilePath`. Errors if file already exists. |
| `config_show.go` | `config show` | none | Prints all config options, current values, and source (`flag` / `env` / `config file` / `default`) from `cfg.Sources`. |
| `store.go` | `store <key> <value>` | 2 positional | Writes a secret to both backends via `Manager.Set()`. Uses `cfg.Vault`. |
| `fetch.go` | `fetch <key>` | 1 positional | Reads a secret via `Manager.Get()`. Prints raw value to stdout (no newline). Uses `cfg.Vault`. |
| `update.go` | `update <key> <value>` | 2 positional | Semantic alias for store. Calls `Manager.Update()`. Uses `cfg.Vault`. |
| `delete.go` | `delete <key>` | 1 positional, `--force`/`-f` | Deletes from both backends. Prompts for confirmation unless `--force`. Uses `cfg.Vault`. |
| `sync.go` | `sync` | none | Pulls all items from 1Password vault into Keychain. Reports count. Uses `cfg.Vault`. |
| `gen_env.go` | `gen-env` | `--template` (default `.env.tpl` → `$ENVSECRETS_TEMPLATE` → config file), `--output` (default `.env` → `$ENVSECRETS_OUTPUT` → config file) | Resolves a template file: `secret:` values are fetched via `Manager.Get()`, other lines copied verbatim. Uses `cfg.Vault`, `cfg.Template`, `cfg.Output`. |

## `internal/config/` - Configuration

| File | Key Exports | Description |
|------|-------------|-------------|
| `config.go` | `Config` struct, `Sources` struct, `Load(configFlagValue string) *Config` | Single home for all viper logic. `Load` resolves config file path (`--config` flag → `$ENVSECRETS_CONFIG` → `~/.config/envsecrets.toml`), binds env vars, sets defaults, reads the TOML file, and returns a fully-populated `*Config`. Source attribution (`"default"` / `"config file"` / `"env ($VAR)"`) is set on `Config.Sources`; flag overrides (`"flag (--name)"`) are applied by the cmd layer after Load returns. Viper is imported **only** in this package. |
| `config_test.go` | — | `package config_test`. Table-driven tests for `Load()`: default values, config-file values, env-var overrides, env-beats-file precedence, `FileFound`/`FilePath` state, and `ENVSECRETS_CONFIG` env-var path resolution. Uses `t.Setenv` and `t.TempDir` to stay hermetic. |

## `internal/secrets/` - Orchestration Layer

| File | Key Exports | Description |
|------|-------------|-------------|
| `secrets.go` | `SecretStore` interface, `Manager` struct, `New(vault)`, `NewWithBackends(kc, op SecretStore)`, `Get(ctx, key)`, `Set(ctx, key, value)`, `Update(ctx, key, value)`, `Delete(ctx, key)`, `Sync(ctx)`, `WithWarningWriter(w)` | Coordinates two `SecretStore` backends (keychain and 1Password) through a single unified interface. All `Manager` methods accept a `context.Context` that is threaded down into every backend call. `New` wires up `keychain.New(vault)` and `onepassword.New(vault)`. `NewWithBackends` accepts any `SecretStore` implementations — used by tests. Read path: Keychain first, fallback to 1Password (caches result). Write path: Keychain must succeed, 1Password is best-effort. Delete: attempts both, combines errors via `errors.Join()`. |
| `secrets_test.go` | — | `package secrets_test`. Table-driven tests for all `Manager` methods using a unified in-memory `stubStore` implementing `SecretStore`. Each stub gets a `notFoundErr` sentinel (`keychain.ErrNotFound` or `onepassword.ErrNotFound`). Covers every branch: cache hits/misses, backend unavailability, partial failures, and warning emission. |

## `internal/keychain/` - macOS Keychain Backend

Uses a **dedicated keychain file per vault** at `~/.local/share/envsecrets/<vault>.keychain` instead of the login keychain. The file's password is auto-generated on first use and stored in the login keychain under service `envsecrets-keychain-<vault>`.

| File | Key Exports | Description |
|------|-------------|-------------|
| `keychain.go` | `Client` struct (`vault`, `keychainPath` fields), `New(vault)`, `(*Client).Available(ctx)`, `(*Client).Get(ctx, service)`, `(*Client).Set(ctx, service, value)`, `(*Client).Delete(ctx, service)`, `(*Client).List(ctx)`, `ParseDumpServices(output)`, `ErrNotFound` | Wraps macOS `security` CLI with a dedicated keychain file. `New(vault)` returns a `*Client`. All methods call `ensure(ctx)` first to create or unlock the keychain file. `Available` checks for the `security` binary. `List` parses `security dump-keychain` output via `ParseDumpServices`. `Set` does delete-then-add via the private `remove` helper. Satisfies `secrets.SecretStore`. |
| `keychain_test.go` | — | `package keychain_test`. Table-driven tests for `ParseDumpServices`: empty input, single item, multiple items, duplicates, special characters, non-matching lines. |

## `internal/onepassword/` - 1Password Backend

| File | Key Exports | Description |
|------|-------------|-------------|
| `onepassword.go` | `Client` struct, `New(vault)`, `(*Client).Available(ctx)`, `(*Client).Get(ctx, key)`, `(*Client).Set(ctx, key, value)`, `(*Client).Delete(ctx, key)`, `(*Client).List(ctx)`, `ParseTitles(json)`, `ErrNotFound`, `ErrUnavailable` | Wraps 1Password `op` CLI. All methods accept a `context.Context` and use `exec.CommandContext`. No package-level `Available()` — use the method directly. `ParseTitles` is exported for `_test` package access. `Set` tries edit-first, falls back to create. |
| `onepassword_test.go` | — | `package onepassword_test`. Table-driven tests for `ParseTitles`: empty input, single item, multiple items, special characters, no title fields. |

## Architecture

```
main.go -> cmd.Execute()
             |
    cmd/{store,fetch,update,delete,sync,gen_env}.go
             |
    internal/secrets/secrets.go  (Manager)
           /              \
  internal/keychain/    internal/onepassword/
  keychain.go           onepassword.go
      |                       |
  macOS `security`        1Password `op`
```

**Read path:** Keychain -> miss -> 1Password -> cache to Keychain -> return
**Write path:** Keychain (must succeed) -> 1Password (best-effort)
**Sync path:** 1Password `List()` -> fetch each -> write to Keychain
**Template path:** scan `.tpl` line-by-line -> resolve `secret:` prefixed values -> write output
