Output es-config.gif

Require envsecrets

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 32
Set Width 1200
Set Height 600

# --- SETUP: back up any existing config so the demo starts from a clean state ---
Hide
Type "mv ~/.config/envsecrets.toml ~/.config/envsecrets.toml.bak 2>/dev/null; true" Enter
Wait
Type "clear" Enter
Show
# -------------------------------------------------------------------------------

# Before: no config file — all values come from built-in defaults
Type "envsecrets config show" Enter
Wait
Sleep 1s

# Create a config file populated with defaults and inline documentation
Type "envsecrets config init" Enter
Wait
Sleep 500ms

# After: config file is found — values and their source are visible
Type "envsecrets config show" Enter
Wait
Sleep 3s

# Cleanup: remove the demo config and restore any backup (hidden)
Hide
Type "rm -f ~/.config/envsecrets.toml" Enter
Type "mv ~/.config/envsecrets.toml.bak ~/.config/envsecrets.toml 2>/dev/null; true" Enter
Wait
Show
