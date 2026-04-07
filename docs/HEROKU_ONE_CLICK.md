# Heroku One-Click Deployment

This document covers one-click Heroku deployment for the Mongo-backed signed streaming flow.

## Deploy button

Use this URL:

https://heroku.com/deploy?template=https://github.com/DinukaSandeepa/TG-FileStreamBot

## What one-click sets up

The app uses:

- app.json (required env vars and defaults)
- Procfile web process: fsb run
- Heroku Go buildpack

## Required Heroku config vars

These must be set in the deploy form:

- API_ID
- API_HASH
- BOT_TOKEN
- MONGO_URI
- MONGO_DB
- STREAM_SIGNING_SECRET
- LINK_SIGN_API_KEY

Optional:

- MONGO_COLLECTION (default movies)
- STREAM_TOKEN_TTL_SEC (default 1800)
- LOG_CHANNEL (default 0 for Mongo-only mode)
- HOST (recommended to set after app creation)

## Post-deploy required step

Set HOST to your Heroku app URL.

Example:

- HOST=https://your-app-name.herokuapp.com

Why:

- Signed response url uses HOST as base URL

If HOST is empty, app auto-detects and may not match your public domain.

## Verify deployment

1. Health:

curl https://your-app-name.herokuapp.com/

2. Sign endpoint:

curl -H "X-API-Key: YOUR_LINK_SIGN_API_KEY" \
  "https://your-app-name.herokuapp.com/sign/db/68118db98f8f82c0e790c1fd"

3. Stream endpoint:

- Open returned url in browser, or test with Range:

curl -H "Range: bytes=0-0" -I "<returned-url>"

Expected:

- HTTP 206 Partial Content
- Content-Range header

## Frontend integration on Heroku

Use the same backend signer pattern documented in docs/FRONTEND_STREAMING.md.

Do not expose LINK_SIGN_API_KEY in browser code.

## Operational notes

1. If one-click app boots but /stream/db fails with no channels found:
   - Bot is not admin/member in source channel
   - sourceChannel/messageId mismatch in Mongo document

2. If /sign/db returns unauthorized:
   - Wrong LINK_SIGN_API_KEY sent by your backend

3. If heroku logs show bind errors:
   - Ensure app reads Heroku PORT env (already supported)

## Useful Heroku CLI commands

heroku logs --tail -a your-app-name

heroku config -a your-app-name

heroku config:set HOST=https://your-app-name.herokuapp.com -a your-app-name

heroku restart -a your-app-name
