Output es-update.gif

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
# Store the initial value so there's something to update
Type "envsecrets store DEMO_DB_URL 'postgres://admin:oldpass@localhost/myapp'" Enter
Wait
# Clear so the setup doesn't appear in the demo
Type "clear" Enter
Show
# ---------------------------------

# Before: show the current connection string
Type "envsecrets fetch DEMO_DB_URL" Enter
Wait
Sleep 1s

# Rotate to the new value — updates both Keychain and 1Password
Type "envsecrets update DEMO_DB_URL 'postgres://admin:newpass@localhost/myapp'" Enter
Wait
Sleep 1s

# After: new value is live
Type "envsecrets fetch DEMO_DB_URL" Enter
Wait
Sleep 3s

# Cleanup (hidden)
Hide
Type "envsecrets delete --force DEMO_DB_URL" Enter
Wait
Show
