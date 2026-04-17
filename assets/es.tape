Output es.mp4

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
# Clear the screen so no 'eval' or 'op' text remains
Type "clear" Enter
Show
# -------------------------

# Your actual demo
Type "envsecrets store FOO 'bar'" Enter
Wait
Sleep 500ms
Type "export API_KEY=$(envsecrets fetch FOO)" Enter
Wait
Sleep 500ms
Type "echo $API_KEY" Enter
Wait

# Optional: Keep the final result on screen for a moment
Sleep 3s