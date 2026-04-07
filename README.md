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
- `STREAM_SIGNING_SECRET`
- `STREAM_TOKEN_TTL_SEC`
- `LINK_SIGN_API_KEY`

If you only use Mongo signed links, you can keep:

- `LOG_CHANNEL=0`

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
- `STREAM_SIGNING_SECRET` - Secret used to sign stream tokens
- `STREAM_TOKEN_TTL_SEC` - Signed URL validity in seconds (default: `1800`)
- `LINK_SIGN_API_KEY` - API key required to request signed stream URLs

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
