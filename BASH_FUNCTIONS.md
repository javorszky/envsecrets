# Bash Function Equivalents

All `envsecrets` commands can be replaced by a handful of bash functions that call the same underlying macOS tools — `security` for Keychain and `op` for 1Password. There is nothing the binary does that these functions cannot.

Two versions are provided depending on how much setup you want:

| | Simple | Full |
|---|---|---|
| Backend | macOS login keychain | Dedicated `.keychain` file + optional 1Password |
| Setup required | None | None — vault is created automatically on first `es_store` |
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

On first use, `es_store` and `es_create_vault` both:
- create the keychain file with a randomly generated password
- store that password in your login keychain for transparent future access
- write an access-details file to `~/Documents/` (the same file the binary writes) so you can always recover the keychain password

If 1Password is available and the target vault does not exist, it is created automatically and an access-details file is written to `~/Documents/` as well.

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
#   export ENVSECRETS_VAULT=envsecrets      # keychain file name (no .keychain extension)
#   export ENVSECRETS_OP_VAULT=Private      # 1Password vault name
#
# 1Password items are stored as "envsecrets.<KEY>" to avoid collisions with
# existing entries in shared vaults such as Private.

# ---------------------------------------------------------------------------
# Private helpers
# ---------------------------------------------------------------------------

# _es_write_kc_access_file — writes the keychain access-details file to
# ~/Documents. Matches the format written by the envsecrets binary so that
# `_es_unlock` can parse it as a fallback if the login-keychain entry is lost.
_es_write_kc_access_file() {
  local vault="$1" kc="$2" password="$3"
  local access_file="${HOME}/Documents/envsecrets-${vault}-keychain-access.txt"

  (umask 177; cat > "$access_file" <<ENVSECRETS_EOF
envsecrets Keychain Access Details
===================================
Created: $(date +%Y-%m-%d)

Vault name    : ${vault}
Keychain file : ${kc}

KEEP THIS FILE SAFE — it contains the password to your envsecrets keychain.
Anyone who can read this file can unlock the keychain and read your secrets.

To open the keychain in Keychain Access (GUI):
  1. Open Keychain Access  (Applications > Utilities > Keychain Access)
  2. File > Add Keychain...
  3. Select the keychain file shown above
  4. Enter the password when prompted (see below)

To unlock and inspect the keychain from the terminal:
  security unlock-keychain -p '<password>' '${kc}'

The envsecrets CLI stores this password in your login keychain for everyday
use. If that entry is ever lost, envsecrets will read it from this file and
restore the login-keychain entry automatically — no manual steps needed.

# --- do not edit below this line ---
vault: ${vault}
keychain-path: ${kc}
password: ${password}
ENVSECRETS_EOF
  )
  printf 'es: keychain access details written to %s\n' "$access_file" >&2
  printf '    Keep this file safe — it contains your keychain password.\n' >&2
}

# _es_write_op_access_file — writes the 1Password access-details file to
# ~/Documents when a new vault is created. Matches the format written by
# the envsecrets binary.
_es_write_op_access_file() {
  local op_vault="$1"
  local access_file="${HOME}/Documents/envsecrets-${op_vault}-1password-access.txt"

  (umask 177; cat > "$access_file" <<ENVSECRETS_EOF
envsecrets 1Password Vault Access Details
==========================================
Created: $(date +%Y-%m-%d)

Vault name: ${op_vault}

Your envsecrets secrets are stored in the "${op_vault}" vault in 1Password.

To access your secrets without the envsecrets CLI:
  1. Open 1Password
  2. Select the "${op_vault}" vault from the sidebar
  3. Secrets are stored as Login items; the value is in the password field
ENVSECRETS_EOF
  )
  printf 'es: 1Password vault access details written to %s\n' "$access_file" >&2
}

# _es_unlock — unlock the dedicated keychain file.
# Reads the password from the login keychain. Falls back to the access file
# written at vault-creation time if the login-keychain entry is missing, and
# restores the entry automatically.
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

# _es_ensure_op_vault — creates the 1Password vault if it does not already
# exist and writes the access-details file to ~/Documents. Best-effort:
# a failure prints a warning but does not abort the calling function.
_es_ensure_op_vault() {
  local op_vault="${ENVSECRETS_OP_VAULT:-Private}"

  # Case-insensitive check, matching the binary's strings.EqualFold behaviour.
  if op vault list --format json 2>/dev/null \
       | grep -qi "\"name\":\"${op_vault}\""; then
    return 0  # vault already exists
  fi

  op vault create "$op_vault" &>/dev/null || {
    printf 'es: warning: could not create 1Password vault "%s"\n' "$op_vault" >&2
    return 1
  }

  printf 'es: info: 1Password vault "%s" created\n' "$op_vault" >&2
  _es_write_op_access_file "$op_vault"
}

# ---------------------------------------------------------------------------
# Public functions
# ---------------------------------------------------------------------------

# es_create_vault — create the dedicated keychain file and write access docs.
# Called automatically by es_store on first use. Can also be run explicitly
# for initial setup before storing any secrets.
es_create_vault() {
  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"

  if [[ -f "$kc" ]]; then
    printf 'es: keychain vault "%s" already exists at %s\n' "$vault" "$kc" >&2
    return 0
  fi

  local password
  password=$(openssl rand -hex 32) || {
    printf 'es: failed to generate keychain password\n' >&2
    return 1
  }

  mkdir -p "$(dirname "$kc")" || {
    printf 'es: failed to create directory %s\n' "$(dirname "$kc")" >&2
    return 1
  }

  security create-keychain -p "$password" "$kc" 2>/dev/null || {
    printf 'es: failed to create keychain at %s\n' "$kc" >&2
    return 1
  }

  # Disable auto-lock so the keychain stays available between sessions.
  security set-keychain-settings "$kc" 2>/dev/null

  # Store the password in the login keychain for transparent future access.
  security add-generic-password -U \
    -a "$USER" -s "envsecrets-keychain-${vault}" -w "$password" 2>/dev/null \
    || printf 'es: warning: could not store keychain password in login keychain\n' >&2

  # Write the access-details file to ~/Documents (best-effort).
  _es_write_kc_access_file "$vault" "$kc" "$password" \
    || printf 'es: warning: could not write keychain access file\n' >&2

  printf 'es: keychain vault "%s" created at %s\n' "$vault" "$kc" >&2
}

es_store() {
  local key="$1" value="$2"
  [[ -z "$key" || -z "$value" ]] && { printf 'usage: es_store KEY VALUE\n' >&2; return 1; }

  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"
  local op_vault="${ENVSECRETS_OP_VAULT:-Private}"

  # Create the keychain vault on first use; unlock it if it already exists.
  if [[ ! -f "$kc" ]]; then
    es_create_vault || return 1
  else
    _es_unlock || return 1
  fi

  security add-generic-password -U -a "$USER" -s "$key" -w "$value" "$kc"

  # 1Password is best-effort: a failure prints a warning but does not abort.
  if command -v op &>/dev/null && op account list &>/dev/null 2>&1; then
    _es_ensure_op_vault  # create vault if needed; warns on failure, does not abort
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
# Optional: create the vault explicitly before first use
es_create_vault

# Store a secret (creates vault automatically if not already done)
es_store DATABASE_URL "postgres://user:pass@localhost/mydb"

export DATABASE_URL=$(es_fetch DATABASE_URL)

es_list

es_delete DATABASE_URL
```

Override configuration per-command with inline env var assignment:

```sh
ENVSECRETS_OP_VAULT=Work es_store SLACK_TOKEN "xoxb-..."
```

### What gets written on first use

When `es_create_vault` runs (explicitly or via `es_store`):

- `~/.local/share/envsecrets/<vault>.keychain` — the dedicated keychain file
- `~/Documents/envsecrets-<vault>-keychain-access.txt` — the keychain password and recovery instructions; **keep this file safe**

When `es_store` runs for the first time with 1Password available and the target vault does not exist:

- The vault is created in 1Password
- `~/Documents/envsecrets-<vault>-1password-access.txt` — vault location and access instructions

### Tip

Keep the functions in a dedicated file and source it from your profile:

```sh
# ~/.zshrc or ~/.bashrc
source ~/.config/envsecrets_functions.sh
```
