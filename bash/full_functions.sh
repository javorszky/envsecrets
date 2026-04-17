# envsecrets — full bash equivalents (dedicated keychain + optional 1Password)
#
# Configuration (set in your shell profile before sourcing these functions):
#   export ENVSECRETS_VAULT=envsecrets      # keychain file name (no .keychain extension)
#   export ENVSECRETS_OP_VAULT=Envsecrets   # 1Password vault name

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
  local op_vault="${ENVSECRETS_OP_VAULT:-Envsecrets}"

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
  local op_vault="${ENVSECRETS_OP_VAULT:-Envsecrets}"

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
    op item edit --vault "$op_vault" "$key" "password=$value" &>/dev/null 2>&1 \
    || op item create --category Login --vault "$op_vault" \
         --title "$key" "password=$value" &>/dev/null 2>&1 \
    || printf 'es: warning: 1Password write failed for "%s"\n' "$key" >&2
  fi
}

es_fetch() {
  local key="$1"
  [[ -z "$key" ]] && { printf 'usage: es_fetch KEY\n' >&2; return 1; }

  local vault="${ENVSECRETS_VAULT:-envsecrets}"
  local kc="${HOME}/.local/share/envsecrets/${vault}.keychain"
  local op_vault="${ENVSECRETS_OP_VAULT:-Envsecrets}"

  _es_unlock || return 1

  local value
  value=$(security find-generic-password -a "$USER" -s "$key" -w "$kc" 2>/dev/null)
  if [[ -n "$value" ]]; then
    printf '%s' "$value"
    return
  fi

  # Cache miss — try 1Password and write the result back into the local keychain.
  if command -v op &>/dev/null && op account list &>/dev/null 2>&1; then
    value=$(op read "op://${op_vault}/${key}/password" 2>/dev/null) || {
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
  local op_vault="${ENVSECRETS_OP_VAULT:-Envsecrets}"

  _es_unlock || return 1
  security delete-generic-password -a "$USER" -s "$key" "$kc" 2>/dev/null \
    || { printf 'es: "%s" not found in keychain\n' "$key" >&2; return 1; }

  # 1Password is best-effort.
  if command -v op &>/dev/null && op account list &>/dev/null 2>&1; then
    op item delete --vault "$op_vault" "$key" &>/dev/null 2>&1 \
      || printf 'es: warning: 1Password delete failed for "%s"\n' "$key" >&2
  fi
}
