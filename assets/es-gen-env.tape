Output es-gen-env.gif

Require envsecrets
Require op

Set Shell "zsh"
Set Theme "Catppuccin Macchiato"
Set FontSize 32
Set Width 1200
Set Height 700

# --- PRE-AUTHORIZATION + SETUP ---
Hide
# Sign in and export the session token silently
Type "export OP_SESSION_my=$(op signin --raw)" Enter
# Give yourself enough time to do the biometric/password unlock
Sleep 5s
# Store the demo secrets
Type "envsecrets store DEMO_DB_URL 'postgres://admin:p4ssw0rd@localhost/myapp'" Enter
Wait
Type "envsecrets store DEMO_API_KEY 'sk_live_abc123xyz789'" Enter
Wait
Type "envsecrets store DEMO_WEBHOOK_SECRET 'whsec_abc123xyz789'" Enter
Wait
# Create the demo template file
Type "cat<<'EOF' > .env.demo.tpl" Enter
Type "# .env.demo.tpl — safe to commit, no real secrets inside" Enter
Type "APP_ENV=production" Enter
Type "LOG_LEVEL=info" Enter
Type "" Enter
Type "DATABASE_URL=secret:DEMO_DB_URL" Enter
Type "API_KEY=secret:DEMO_API_KEY" Enter
Type "WEBHOOK_SECRET=secret:DEMO_WEBHOOK_SECRET" Enter
Type "EOF" Enter
Wait
# Clear so the setup doesn't appear in the demo
Type "clear" Enter
Show
# ---------------------------------

# Before: the template — plain values mixed with secret: references
Type "cat .env.demo.tpl" Enter
Wait
Sleep 1s

# Resolve all secret: references into real values
Type "envsecrets gen-env --template .env.demo.tpl --output .env.demo" Enter
Wait
Sleep 500ms

# After: .env.demo contains the resolved secrets — never commit this file
Type "cat .env.demo" Enter
Wait
Sleep 3s

# Cleanup (hidden)
Hide
Type "rm -f .env.demo.tpl .env.demo" Enter
Type "envsecrets delete --force DEMO_DB_URL" Enter
Wait
Type "envsecrets delete --force DEMO_API_KEY" Enter
Wait
Type "envsecrets delete --force DEMO_WEBHOOK_SECRET" Enter
Wait
Show
