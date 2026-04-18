Output es-store.gif

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

# Before: key doesn't exist yet — fetch returns an error
Type "envsecrets fetch DEMO_STRIPE_KEY" Enter
Wait
Sleep 1s

# Store the secret — written to Keychain and 1Password
Type "envsecrets store DEMO_STRIPE_KEY 'sk_live_abc123xyz789'" Enter
Wait
Sleep 1s

# After: key is now available instantly from local Keychain
Type "envsecrets fetch DEMO_STRIPE_KEY" Enter
Wait
Sleep 3s

# Cleanup (hidden)
Hide
Type "envsecrets delete --force DEMO_STRIPE_KEY" Enter
Wait
Show
