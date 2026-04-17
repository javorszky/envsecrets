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
