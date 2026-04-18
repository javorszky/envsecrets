Output es-config-precedence.gif

Require envsecrets

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 24
Set Width 1200
Set Height 600

# --- SETUP ---
Hide
Type "mv ~/.config/envsecrets.toml ~/.config/envsecrets.toml.bak 2>/dev/null; true" Enter
Wait
# Create the config file with 3 of the 4 options set
Type "cat > ~/.config/envsecrets.toml << 'EOF'" Enter
Type 'vault = "myproject"' Enter
Type 'op_vault = "TeamVault"' Enter
Type 'template = ".env.staging.tpl"' Enter
Type "EOF" Enter
Wait
Type "clear" Enter
Show
# -------------

# Show what is in the config file
Type "cat ~/.config/envsecrets.toml" Enter
Wait
Sleep 1s

# Set env vars for 2 of the 3 options that are in the config file
Type "export ENVSECRETS_OP_VAULT=EnvVault" Enter
Sleep 300ms
Type "export ENVSECRETS_TEMPLATE=.env.prod.tpl" Enter
Sleep 500ms

# Use a flag for one of the two env-var options
# All four sources appear in one config show:
#   vault    → config file  (in file; no env var, no flag)
#   op_vault → flag         (in file + env var + flag — flag wins)
#   template → env var      (in file + env var — env var beats file)
#   output   → default      (not set anywhere)
Type "envsecrets config show --op-vault FlagVault" Enter
Wait
Sleep 4s

# Cleanup (hidden)
Hide
Type "unset ENVSECRETS_OP_VAULT ENVSECRETS_TEMPLATE" Enter
Type "rm -f ~/.config/envsecrets.toml" Enter
Type "mv ~/.config/envsecrets.toml.bak ~/.config/envsecrets.toml 2>/dev/null; true" Enter
Wait
Show
