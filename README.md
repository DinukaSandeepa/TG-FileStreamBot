<h1 align="center">Telegram File Stream Bot</h1>
<p align="center">
  </a>
  <p align="center">
    <a herf="https://github.com/DinukaSandeepa/TG-FileStreamBot">
        <img src="https://filestream.bot/logo.svg" height="100" width="100" alt="File Stream Bot Logo">
    </a>
</p>
  <p align="center">
    A Telegram bot to <b>generate direct link</b> for your Telegram files.
    <br />
    << <a href="https://filestream.bot/getting-started">Visit docs</a> |  <a href="https://filestream.bot/support">Support</a> >>
    <br />
    <a href="https://heroku.com/deploy?template=https://github.com/DinukaSandeepa/TG-FileStreamBot">
      <img src="https://www.herokucdn.com/deploy/button.svg" alt="Deploy to Heroku">
    </a>
  </p>
</p>

## Documentation

- Frontend streaming integration: [docs/FRONTEND_STREAMING.md](docs/FRONTEND_STREAMING.md)
- Heroku one-click deployment: [docs/HEROKU_ONE_CLICK.md](docs/HEROKU_ONE_CLICK.md)

## Run Locally

### Prerequisites

- Go 1.25 or newer
- A Telegram bot token (`BOT_TOKEN`)
- Telegram API credentials (`API_ID`, `API_HASH`)
- Optional: Telegram log channel ID (`LOG_CHANNEL`) for legacy forwarding mode
- Optional: Telegram subtitle channel ID (`SUBTITLE_CHANNEL_ID`) as fallback for subtitle docs without `sourceChannel`

### 1) Prepare environment

Create a local `fsb.env` file in project root and fill values.

```bash
touch fsb.env
```

For local testing, set this explicitly in `fsb.env`:

- `HOST=http://localhost:8080`
- `PORT=8080`

For Mongo-backed signed links, also set:

- `MONGO_URI`
- `MONGO_DB`
- `MONGO_COLLECTION` (default `movies`)
- `MONGO_SUBTITLES_COLLECTION` (default `subtitles`)
- `STREAM_SIGNING_SECRET`
- `STREAM_TOKEN_TTL_SEC`
- `LINK_SIGN_API_KEY`

For smoother playback on unstable/VPN networks, tune:

- `STREAM_CONCURRENCY` (default `4`)
- `STREAM_BUFFER_COUNT` (default `8`)
- `STREAM_INITIAL_BUFFER_MB` (default `4`, set `0` to disable startup prebuffer)
- `STREAM_OPEN_ENDED_CHUNK_MB` (default `0`, caps `bytes=start-` requests; `0` disables)
- `STREAM_TIMEOUT_SEC` (default `30`)
- `STREAM_MAX_RETRIES` (default `3`)

Recommended starting values for laggy VPN/mobile networks:

- `STREAM_CONCURRENCY=6`
- `STREAM_BUFFER_COUNT=16`
- `STREAM_INITIAL_BUFFER_MB=8`
- `STREAM_OPEN_ENDED_CHUNK_MB=12`
- `STREAM_TIMEOUT_SEC=45`
- `STREAM_MAX_RETRIES=5`

If you only use Mongo signed links, you can keep:

- `LOG_CHANNEL=0`

If your subtitles collection stores `sourceChannel` per document, `SUBTITLE_CHANNEL_ID` can stay `0`.

### 2) Start the server (Go)

```bash
go mod tidy
go run ./cmd/fsb run
```

The API should be reachable at:

```text
http://localhost:8080/
```

### 3) Generate and use a signed DB stream link

Request signed URL for a Mongo ObjectId:

```bash
curl -sS -H "X-API-Key: <LINK_SIGN_API_KEY>" \
  "http://localhost:8080/sign/db/<mongo_object_id>"
```

Use returned `url` in your website player.

### 4) How to use startup buffering (YouTube-like behavior)

1. Set stream tuning variables in `fsb.env` (or pass equivalent CLI flags):

```env
STREAM_CONCURRENCY=6
STREAM_BUFFER_COUNT=16
STREAM_INITIAL_BUFFER_MB=8
STREAM_OPEN_ENDED_CHUNK_MB=12
STREAM_TIMEOUT_SEC=45
STREAM_MAX_RETRIES=5
```

2. Restart FSB so new values are loaded.

3. Keep browser player buffering enabled with `<video preload="auto" ...>`.

4. On `stalled` / `waiting` / `error`, request a fresh signed URL from your backend and retry from current playback position.

5. Optional: disable startup prebuffer if startup delay is too high (`STREAM_INITIAL_BUFFER_MB=0`).

CLI equivalent:

```bash
go run ./cmd/fsb run \
  --stream-concurrency=6 \
  --stream-buffer-count=16 \
  --stream-initial-buffer-mb=8 \
  --stream-open-ended-chunk-mb=12 \
  --stream-timeout-sec=45 \
  --stream-max-retries=5
```

Quick verification:

- Sign URL: `GET /sign/db/:id`
- Request with `Range: bytes=0-` and confirm `206 Partial Content`
- Confirm response includes `Accept-Ranges: bytes`
- Simulate slow network and verify playback recovers after URL refresh

### Optional: Run with Docker Compose

```bash
docker compose up -d
```

This uses `fsb.env` mounted into the container.


## Mongo-Backed Signed Stream Links

This project now supports streaming files directly from existing MongoDB metadata records, without forwarding files again to a new log channel.

### Required environment variables

- `MONGO_URI` - MongoDB connection string
- `MONGO_DB` - Database name
- `MONGO_COLLECTION` - Collection name (default: `movies`)
- `MONGO_SUBTITLES_COLLECTION` - Subtitles collection name (default: `subtitles`)
- `STREAM_SIGNING_SECRET` - Secret used to sign stream tokens
- `STREAM_TOKEN_TTL_SEC` - Signed URL validity in seconds (default: `1800`)
- `LINK_SIGN_API_KEY` - API key required to request signed stream URLs
- `SUBTITLE_CHANNEL_ID` - Optional fallback subtitle channel ID when subtitle docs omit `sourceChannel`
- `STREAM_CONCURRENCY` - Parallel Telegram block downloads per stream request (default: `4`)
- `STREAM_BUFFER_COUNT` - Prefetch queue capacity in blocks (default: `8`)
- `STREAM_INITIAL_BUFFER_MB` - Initial server-side prebuffer before first byte is sent (default: `4`, `0` disables)
- `STREAM_OPEN_ENDED_CHUNK_MB` - Caps open-ended `Range: bytes=start-` playback requests to this many MB (default: `0`, disabled)
- `STREAM_TIMEOUT_SEC` - Per-block Telegram fetch timeout (default: `30`)
- `STREAM_MAX_RETRIES` - Retry attempts per failed block (default: `3`)

### Expected MongoDB document shape

Each file document in the target collection should have at least:

- `_id` (ObjectId)
- `messageId` (Telegram message ID in source channel)
- `sourceChannel` (Telegram channel ID where file currently exists)
- `fileName` (optional, used as fallback response filename)
- `fileSize` (optional metadata)
- `caption` (optional metadata)

### Endpoint flow for website integration

1. Request a signed URL:
  - `GET /sign/db/:id`
  - Header: `X-API-Key: <LINK_SIGN_API_KEY>`
  - Optional query: `d=true` to force download disposition

2. Use returned URL in your player/download button:
  - `GET /stream/db/:id?token=<signed-token>`

The stream URL expires automatically based on `STREAM_TOKEN_TTL_SEC`.

### Subtitle direct links (permanent)

Subtitle links are permanent and do not need token signing.

1. Direct subtitle by subtitle document ID:
  - `GET /subtitle/db/:id`

2. Convert SRT to VTT on the fly (direct link):
  - `GET /subtitle/db/:id?format=vtt`

3. Get ready-to-use subtitle links JSON:
  - `GET /subtitle/db/:id/links`

For complete browser and backend integration examples, see [docs/FRONTEND_STREAMING.md](docs/FRONTEND_STREAMING.md).

For Heroku deployment, see [docs/HEROKU_ONE_CLICK.md](docs/HEROKU_ONE_CLICK.md).




## Credits

- [@celestix](https://github.com/celestix) for [gotgproto](https://github.com/celestix/gotgproto)
- [@divyam234](https://github.com/divyam234/teldrive) for his [Teldrive](https://github.com/divyam234/teldrive) Project
- [@krau](https://github.com/krau) for adding image support

## Copyright

Copyright (C) 2026 [EverythingSuckz](https://github.com/EverythingSuckz) under [GNU Affero General Public License](https://www.gnu.org/licenses/agpl-3.0.en.html).

TG-FileStreamBot is Free Software: You can use, study share and improve it at your
will. Specifically you can redistribute and/or modify it under the terms of the
[GNU Affero General Public License](https://www.gnu.org/licenses/agpl-3.0.en.html) as
published by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version. Also keep in mind that all the forks of this repository MUST BE OPEN-SOURCE and MUST BE UNDER THE SAME LICENSE.
