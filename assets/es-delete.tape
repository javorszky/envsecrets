Output es-delete.gif

Require envsecrets
Require op

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 32
Set Width 1200
Set Height 600

# --- PRE-AUTHORIZATION + SETUP ---
Hide
# Sign in and export the session token silently
Type "export OP_SESSION_my=$(op signin --raw)" Enter
# Give yourself enough time to do the biometric/password unlock
Sleep 5s
# Store a key to demonstrate deletion
Type "envsecrets store DEMO_OLD_KEY 'sk_test_abc123'" Enter
Wait
# Clear so the setup doesn't appear in the demo
Type "clear" Enter
Show
# ---------------------------------

# Before: key exists
Type "envsecrets fetch DEMO_OLD_KEY" Enter
Wait
Sleep 1s

# Delete — prompts for confirmation; removes from Keychain and 1Password
Type "envsecrets delete DEMO_OLD_KEY" Enter
# Wait for the "Delete? [y/N]" prompt to appear
Sleep 500ms
Type "y" Enter
Wait
Sleep 1s

# After: key is gone from both backends
Type "envsecrets fetch DEMO_OLD_KEY" Enter
Wait
Sleep 3s
