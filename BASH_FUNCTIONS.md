# Bash Function Equivalents

All `envsecrets` commands can be replaced by a handful of bash functions that call the same underlying macOS tools — `security` for Keychain and `op` for 1Password. There is nothing the binary does that these functions cannot.

Two versions are provided depending on how much setup you want:

| | Simple | Full |
|---|---|---|
| Backend | macOS login keychain | Dedicated `.keychain` file + optional 1Password |
| Setup required | None | Keychain file must already exist (created by `envsecrets store`) |
| Configuration | None | `$ENVSECRETS_VAULT`, `$ENVSECRETS_OP_VAULT` |
| 1Password | ❌ | ✅ best-effort |

---

## Simple version — login keychain, no setup

Secrets are stored directly in your login keychain under an `envsecrets.` prefix, keeping them separate from other login-keychain entries. No separate keychain file, no configuration, no 1Password.

Paste into `~/.zshrc` or `~/.bashrc`:

```bash
# envsecrets — simple bash equivalents (login keychain only)
# Secrets are stored as "envsecrets.<KEY>" in the default keychain.

es_store() {
  local key="$1" value="$2"
  [[ -z "$key" || -z "$value" ]] && { printf 'usage: es_store KEY VALUE\n' >&2; return 1; }
  security add-generic-password -U -a "$USER" -s "envsecrets.${key}" -w "$value"
}

es_fetch() {
  local key="$1"
  [[ -z "$key" ]] && { printf 'usage: es_fetch KEY\n' >&2; return 1; }
  security find-generic-password -a "$USER" -s "envsecrets.${key}" -w 2>/dev/null \
    || { printf 'es: "%s" not found\n' "$key" >&2; return 1; }
}

es_list() {
  security dump-keychain 2>/dev/null \
    | grep '"svce"<blob>="envsecrets\.' \
    | sed 's/.*"svce"<blob>="envsecrets\.\(.*\)"/\1/'
}

es_delete() {
  local key="$1"
  [[ -z "$key" ]] && { printf 'usage: es_delete KEY\n' >&2; return 1; }
  security delete-generic-password -a "$USER" -s "envsecrets.${key}" 2>/dev/null \
    || { printf 'es: "%s" not found\n' "$key" >&2; return 1; }
}
```

### Usage

```sh
es_store DATABASE_URL "postgres://user:pass@localhost/mydb"

export DATABASE_URL=$(es_fetch DATABASE_URL)

es_list

es_delete DATABASE_URL
```

---

## Full version — dedicated keychain file + optional 1Password

Mirrors the CLI behaviour exactly: secrets live in a dedicated `.keychain` file at `~/.local/share/envsecrets/<vault>.keychain`, with 1Password as an optional best-effort sync layer.

**Prerequisite:** the keychain file must already exist. Run `envsecrets store` once with the binary to create it, then you can use these functions in place of the binary going forward.

### Configuration

| Variable | Default | Purpose |
|---|---|---|
| `ENVSECRETS_VAULT` | `envsecrets` | Keychain file name (without `.keychain`) |
| `ENVSECRETS_OP_VAULT` | `Private` | 1Password vault name |

1Password items are always stored under the title `envsecrets.<KEY>` — regardless of which vault is used — to prevent collisions with existing items in a shared vault such as `Private`.

### Functions

Paste into `~/.zshrc` or `~/.bashrc`:

```bash
# envsecrets — full bash equivalents (dedicated keychain + optional 1Password)
#
# Configuration (set in your shell profile before sourcing these functions):
#   export ENVSECRETS_VAULT=envsecrets        # keychain file name
#   export ENVSECRETS_OP_VAULT=Private        # 1Password vault name
#
# 1Password items are stored as "envsecrets.<KEY>" to avoid collisions.

# _es_unlock — unlock the dedicated keychain file.
# Reads the password from the login keychain.  Falls back to the access file
# written by envsecrets at vault-creation time if the login-keychain entry
# is missing, and restores the entry automatically.
_es_unlock() {
  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"
  local password

  password=$(security find-generic-password \
    -a "$USER" -s "envsecrets-keychain-${vault}" -w 2>/dev/null)

  if [[ -z "$password" ]]; then
    local access_file="${HOME}/Documents/envsecrets-${vault}-keychain-access.txt"
    password=$(grep '^password: ' "$access_file" 2>/dev/null | awk '{print $2}')
    if [[ -z "$password" ]]; then
      printf 'es: cannot unlock keychain "%s"\n' "$vault" >&2
      printf '    login-keychain entry missing; access file not found at:\n' >&2
      printf '    %s\n' "$access_file" >&2
      return 1
    fi
    # Restore the login-keychain entry so future calls succeed without the file.
    security add-generic-password -U \
      -a "$USER" -s "envsecrets-keychain-${vault}" -w "$password" 2>/dev/null
  fi

  security unlock-keychain -p "$password" "$kc" 2>/dev/null
}

es_store() {
  local key="$1" value="$2"
  [[ -z "$key" || -z "$value" ]] && { printf 'usage: es_store KEY VALUE\n' >&2; return 1; }

  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"
  local op_vault="${ENVSECRETS_OP_VAULT:-Private}"

  _es_unlock || return 1
  security add-generic-password -U -a "$USER" -s "$key" -w "$value" "$kc"

  # 1Password is best-effort: a failure prints a warning but does not abort.
  if command -v op &>/dev/null && op account list &>/dev/null 2>&1; then
    op item edit --vault "$op_vault" "envsecrets.${key}" "password=$value" &>/dev/null 2>&1 \
    || op item create --category Login --vault "$op_vault" \
         --title "envsecrets.${key}" "password=$value" &>/dev/null 2>&1 \
    || printf 'es: warning: 1Password write failed for "%s"\n' "$key" >&2
  fi
}

es_fetch() {
  local key="$1"
  [[ -z "$key" ]] && { printf 'usage: es_fetch KEY\n' >&2; return 1; }

  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"
  local op_vault="${ENVSECRETS_OP_VAULT:-Private}"

  _es_unlock || return 1

  local value
  value=$(security find-generic-password -a "$USER" -s "$key" -w "$kc" 2>/dev/null)
  if [[ -n "$value" ]]; then
    printf '%s' "$value"
    return
  fi

  # Cache miss — try 1Password and write the result back into the local keychain.
  if command -v op &>/dev/null && op account list &>/dev/null 2>&1; then
    value=$(op read "op://${op_vault}/envsecrets.${key}/password" 2>/dev/null) || {
      printf 'es: "%s" not found in keychain or 1Password\n' "$key" >&2
      return 1
    }
    security add-generic-password -U -a "$USER" -s "$key" -w "$value" "$kc"
    printf '%s' "$value"
  else
    printf 'es: "%s" not found in keychain (1Password unavailable)\n' "$key" >&2
    return 1
  fi
}

es_list() {
  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"

  _es_unlock || return 1
  security dump-keychain "$kc" 2>/dev/null \
    | grep '"svce"<blob>=' \
    | sed 's/.*"svce"<blob>="\(.*\)"/\1/'
}

es_delete() {
  local key="$1"
  [[ -z "$key" ]] && { printf 'usage: es_delete KEY\n' >&2; return 1; }

  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"
  local op_vault="${ENVSECRETS_OP_VAULT:-Private}"

  _es_unlock || return 1
  security delete-generic-password -a "$USER" -s "$key" "$kc" 2>/dev/null \
    || { printf 'es: "%s" not found in keychain\n' "$key" >&2; return 1; }

  # 1Password is best-effort.
  if command -v op &>/dev/null && op account list &>/dev/null 2>&1; then
    op item delete --vault "$op_vault" "envsecrets.${key}" &>/dev/null 2>&1 \
      || printf 'es: warning: 1Password delete failed for "%s"\n' "$key" >&2
  fi
}
```

### Usage

```sh
es_store DATABASE_URL "postgres://user:pass@localhost/mydb"

export DATABASE_URL=$(es_fetch DATABASE_URL)

es_list

es_delete DATABASE_URL
```

Override configuration per-command with inline env var assignment:

```sh
ENVSECRETS_OP_VAULT=Work es_store SLACK_TOKEN "xoxb-..."
```

### Tip

Keep the functions in a dedicated file and source it from your profile:

```sh
# ~/.zshrc or ~/.bashrc
source ~/.config/envsecrets_functions.sh
```
