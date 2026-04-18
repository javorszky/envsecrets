Output es-config-precedence.gif

Require envsecrets

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 24
Set Width 1200
Set Height 600

# --- SETUP: ensure a clean state with no config file and no env vars ---
Hide
Type "mv ~/.config/envsecrets.toml ~/.config/envsecrets.toml.bak 2>/dev/null; true" Enter
Wait
Type "clear" Enter
Show
# -----------------------------------------------------------------------

# Level 1 — BUILT-IN DEFAULT
# No config file, no env vars, no flags.
# SOURCE column shows "default" for every option.
Type "# Level 1 — default (no config file, no env vars, no flags)" Enter
Sleep 300ms
Type "envsecrets config show" Enter
Wait
Sleep 2s
Type "clear" Enter

# Level 2 — CONFIG FILE
# Create a minimal config file that sets op_vault.
# SOURCE for op_vault changes from "default" → "config file".
# All other options are not in the file, so they stay "default".
Type "# Level 2 — config file beats default" Enter
Sleep 300ms
Type "cat > ~/.config/envsecrets.toml << 'EOF'" Enter
Type 'op_vault = "TeamVault"' Enter
Type "EOF" Enter
Wait
Type "envsecrets config show" Enter
Wait
Sleep 2s
Type "clear" Enter

# Level 3 — ENVIRONMENT VARIABLE
# Env var ENVSECRETS_OP_VAULT is set for this one command only (inline assignment).
# SOURCE for op_vault changes from "config file" → "env ($ENVSECRETS_OP_VAULT)".
# The config file is still present but the env var wins.
Type "# Level 3 — env var beats config file" Enter
Sleep 300ms
Type "ENVSECRETS_OP_VAULT=EnvVault envsecrets config show" Enter
Wait
Sleep 2s
Type "clear" Enter

# Level 4 — CLI FLAG
# --op-vault flag is the highest priority; beats env var, config file, and default.
# SOURCE for op_vault changes to "flag (--op-vault)".
# Env var is still set inline here to prove the flag overrides it too.
Type "# Level 4 — CLI flag beats everything" Enter
Sleep 300ms
Type "ENVSECRETS_OP_VAULT=EnvVault envsecrets config show --op-vault FlagVault" Enter
Wait
Sleep 3s

# Cleanup (hidden)
Hide
Type "rm -f ~/.config/envsecrets.toml" Enter
Type "mv ~/.config/envsecrets.toml.bak ~/.config/envsecrets.toml 2>/dev/null; true" Enter
Wait
Show
