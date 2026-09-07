Output es-sync.gif

Require envsecrets
Require op

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 32
Set Width 1200
Set Height 600

# --- PRE-AUTHORIZATION ---
Hide
# Sign in and export the session token silently
Type "export OP_SESSION_my=$(op signin --raw)" Enter
# Give yourself enough time to do the biometric/password unlock
Sleep 5s
# Clear the screen so no auth text remains
Type "clear" Enter
Show
# -------------------------

# On a fresh machine — pull every secret from 1Password into local Keychain
# Run this once to bootstrap offline access; subsequent fetches skip the network
Type "envsecrets sync" Enter
Wait
Sleep 3s
