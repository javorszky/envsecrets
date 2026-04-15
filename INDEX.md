# envsecrets - File Index

Codebase table of contents for quick navigation. Consult this before scanning the repo.

## Project Root

| File | Purpose |
|------|---------|
| `main.go` | Entry point. Calls `cmd.Execute()`. |
| `go.mod` | Module `github.com/javorszky/envsecrets`, Go 1.26.2. Depends on `spf13/cobra`. |
| `go.sum` | Dependency checksums. |
| `LICENSE` | MIT license. |
| `README.md` | User-facing documentation: installation, usage, architecture. |
| `.env.docker.tpl` | Example template file. Lines with `secret:` prefix are resolved at runtime. |
| `.gitignore` | Ignores binary, IDE files, `.env` files (except `.env.docker.tpl`), coverage output. |
| `.golangci.yml` | golangci-lint v2 config. Standard linters + `gofmt`. |
| `Makefile` | Local dev commands: `build`, `lint`, `test`, `vet`, `fmt`, `govulncheck`, `check` (runs all). |
| `renovate.json` | Renovate config. Groups Go minor/patch deps and GitHub Actions updates. |
| `lefthook.yml` | Lefthook pre-commit hooks: format check (`gofmt -l`), lint (`golangci-lint`), test (`go test -race`). |

## `.github/workflows/` - CI

| File | Triggers | Jobs | Description |
|------|----------|------|-------------|
| `ci.yml` | `pull_request`, `merge_group` | **lint** (golangci-lint incl. staticcheck, govet, gofmt), **test** (`go test -race`), **govulncheck** | CI pipeline. All three jobs run in parallel. |

## `cmd/` - CLI Commands (Cobra)

All commands import `internal/secrets` and use `secrets.New(vaultFlag)` to create a `Manager`.

| File | Command | Args / Flags | Description |
|------|---------|-------------|-------------|
| `root.go` | (root) | `--vault` (persistent) | Root command setup. Exports `Execute()`. Default vault from `$ENVSECRETS_VAULT` or `"Private"`. |
| `store.go` | `store <key> <value>` | 2 positional | Writes a secret to both backends via `Manager.Set()`. |
| `fetch.go` | `fetch <key>` | 1 positional | Reads a secret via `Manager.Get()`. Prints raw value to stdout (no newline). |
| `update.go` | `update <key> <value>` | 2 positional | Semantic alias for store. Calls `Manager.Update()`. |
| `delete.go` | `delete <key>` | 1 positional, `--force`/`-f` | Deletes from both backends. Prompts for confirmation unless `--force`. |
| `sync.go` | `sync` | none | Pulls all items from 1Password vault into Keychain. Reports count. |
| `gen_env.go` | `gen-env` | `--template` (default `.env.docker.tpl`), `--output` (default `.env.docker`) | Resolves a template file: `secret:` values are fetched via `Manager.Get()`, other lines copied verbatim. |

## `internal/secrets/` - Orchestration Layer

| File | Key Exports | Description |
|------|-------------|-------------|
| `secrets.go` | `Manager` struct, `New(vault)`, `Get(key)`, `Set(key, value)`, `Update(key, value)`, `Delete(key)`, `Sync()`, `WithWarningWriter(w)` | Coordinates Keychain and 1Password. Read path: Keychain first, fallback to 1Password (caches result back to Keychain). Write path: Keychain must succeed, 1Password is best-effort. Delete: attempts both, combines errors via `errors.Join()`. |

## `internal/keychain/` - macOS Keychain Backend

| File | Key Exports | Description |
|------|-------------|-------------|
| `keychain.go` | `Get(service)`, `Set(service, value)`, `Delete(service)`, `ErrNotFound` | Wraps macOS `security` CLI. Stores generic passwords keyed by `$USER` (account) and service name. `Set` does delete-then-add to avoid duplicates. |

## `internal/onepassword/` - 1Password Backend

| File | Key Exports | Description |
|------|-------------|-------------|
| `onepassword.go` | `Client` struct, `New(vault)`, `Available()`, `Get(key)`, `Set(key, value)`, `Delete(key)`, `List()`, `ErrNotFound`, `ErrUnavailable` | Wraps 1Password `op` CLI. Stores secrets as Login items (title = key, password field = value). `Set` tries edit-first, falls back to create. `List` returns all item titles in the vault. Error classification maps `op` CLI output to sentinel errors. |

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
