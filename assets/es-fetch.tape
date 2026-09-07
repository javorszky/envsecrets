Output es-fetch.gif

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
# Store the demo key so it exists in the local Keychain
Type "envsecrets store DEMO_STRIPE_KEY 'sk_live_abc123xyz789'" Enter
Wait
# Clear so the setup doesn't appear in the demo
Type "clear" Enter
Show
# ---------------------------------

# Fetch a secret into an environment variable
# Reads from local Keychain — fast, no network required
Type "export STRIPE_KEY=\$(envsecrets fetch DEMO_STRIPE_KEY)" Enter
Wait
Sleep 500ms

# Use it
Type "echo \$STRIPE_KEY" Enter
Wait
Sleep 3s

# Cleanup (hidden)
Hide
Type "envsecrets delete --force DEMO_STRIPE_KEY" Enter
Wait
Show
