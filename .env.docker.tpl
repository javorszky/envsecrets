# .env.docker.tpl
# This file is SAFE TO COMMIT. Values prefixed with `secret:` are resolved
# at runtime by `envsecrets gen-env` — the actual secrets never touch this file.
#
# Usage:
#   envsecrets gen-env --template .env.docker.tpl --output .env.docker
#   docker compose up

# Application
APP_NAME=MyApp
APP_ENV=local
APP_DEBUG=false
APP_KEY=secret:myapp_APP_KEY
APP_URL=http://localhost

# Database
DB_CONNECTION=mysql
DB_HOST=mysql
DB_PORT=3306
DB_DATABASE=myapp
DB_USERNAME=myapp
DB_PASSWORD=secret:myapp_DB_PASSWORD

# Cache / Queue
CACHE_DRIVER=redis
QUEUE_CONNECTION=redis
SESSION_DRIVER=redis
REDIS_HOST=redis
REDIS_PORT=6379

# Mail
MAIL_MAILER=smtp
MAIL_HOST=mailpit
MAIL_PORT=1025

# Stripe
STRIPE_KEY=secret:myapp_STRIPE_KEY
STRIPE_SECRET=secret:myapp_STRIPE_SECRET
STRIPE_WEBHOOK_SECRET=secret:myapp_STRIPE_WEBHOOK_SECRET
