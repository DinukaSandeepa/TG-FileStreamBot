# Frontend Integration Guide (Mongo Signed Streaming)

This guide explains how to integrate the DB-backed streaming endpoints into a website frontend safely.

## Overview

The streaming flow has 2 endpoints:

1. Sign endpoint (server-to-server only):
   - GET /sign/db/:id
   - Requires X-API-Key header
   - Returns a short-lived signed URL

2. Stream endpoint (player/download):
   - GET /stream/db/:id?token=...
   - Token is validated and expires based on STREAM_TOKEN_TTL_SEC

Subtitle flow has permanent endpoints (no token required):

- GET /subtitle/db/:id
- GET /subtitle/db/:id?format=vtt (SRT to VTT conversion)
- GET /subtitle/db/:id/links

## Required backend environment

Your stream service must have these configured:

- MONGO_URI
- MONGO_DB
- MONGO_COLLECTION (default: movies)
- MONGO_SUBTITLES_COLLECTION (default: subtitles)
- STREAM_SIGNING_SECRET
- STREAM_TOKEN_TTL_SEC
- LINK_SIGN_API_KEY
- SUBTITLE_CHANNEL_ID (optional fallback if subtitle docs do not store sourceChannel)

## Mongo document requirements

Each document must include:

- _id (ObjectId)
- sourceChannel (Telegram channel id)
- messageId (Telegram message id in that channel)

Recommended extras:

- fileName
- fileSize
- caption

## Security model

Do not expose LINK_SIGN_API_KEY in browser code.

Correct pattern:

- Browser calls your website backend endpoint (for example /api/stream-url/:id)
- Website backend calls FSB /sign/db/:id with X-API-Key
- Backend returns only the signed stream URL to browser

## Backend adapter example (Node/Express)

```javascript
import express from "express";

const app = express();
const FSB_BASE_URL = process.env.FSB_BASE_URL; // https://your-fsb-domain
const FSB_SIGN_API_KEY = process.env.FSB_SIGN_API_KEY;

app.get("/api/stream-url/:id", async (req, res) => {
  try {
    const id = req.params.id;
    const forceDownload = req.query.download === "1";
    const url = new URL(`${FSB_BASE_URL}/sign/db/${id}`);
    if (forceDownload) {
      url.searchParams.set("d", "true");
    }

    const r = await fetch(url, {
      headers: {
        "X-API-Key": FSB_SIGN_API_KEY,
      },
    });

    if (!r.ok) {
      const text = await r.text();
      return res.status(r.status).json({ ok: false, error: text || "sign failed" });
    }

    const payload = await r.json();
    return res.json({
      ok: true,
      streamUrl: payload.url,
      expiresAt: payload.expiresAt,
      expiresIn: payload.expiresIn,
    });
  } catch (err) {
    return res.status(500).json({ ok: false, error: err.message });
  }
});
```

## Frontend player example (React)

```jsx
import { useEffect, useState } from "react";

export default function VideoPlayer({ mongoId }) {
  const [streamUrl, setStreamUrl] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;

    async function loadUrl() {
      setError("");
      try {
        const r = await fetch(`/api/stream-url/${mongoId}`);
        const data = await r.json();
        if (!r.ok || !data.ok) {
          throw new Error(data.error || "Failed to get stream URL");
        }
        if (!cancelled) {
          setStreamUrl(data.streamUrl);
        }
      } catch (e) {
        if (!cancelled) {
          setError(e.message);
        }
      }
    }

    loadUrl();
    return () => {
      cancelled = true;
    };
  }, [mongoId]);

  if (error) return <p>Stream error: {error}</p>;
  if (!streamUrl) return <p>Preparing stream...</p>;

  return (
    <video
      controls
      autoPlay
      playsInline
      preload="metadata"
      src={streamUrl}
      style={{ width: "100%", maxWidth: 960 }}
    />
  );
}
```

## Download link example

Your backend can request a download-mode signed URL by adding d=true to sign endpoint:

- GET /sign/db/:id?d=true

Use returned URL as anchor href.

## Subtitle link usage example

For subtitle id `6824a95cbf08f4af6f2f9ca8`:

- Original subtitle: `/subtitle/db/6824a95cbf08f4af6f2f9ca8`
- VTT converted subtitle: `/subtitle/db/6824a95cbf08f4af6f2f9ca8?format=vtt`
- Auto links JSON: `/subtitle/db/6824a95cbf08f4af6f2f9ca8/links`

Player usage example:

```html
<video id="player" controls src="{{SIGNED_STREAM_URL}}"></video>
<script>
  const video = document.getElementById("player");
  const track = document.createElement("track");
  track.kind = "subtitles";
  track.label = "Tamil";
  track.srclang = "ta";
  track.src = "https://your-fsb-domain/subtitle/db/6824a95cbf08f4af6f2f9ca8?format=vtt";
  track.default = true;
  video.appendChild(track);
</script>
```

## Token expiry handling

If token expires while user opens old page:

- /stream/db returns token expired (401)
- Frontend should re-request a fresh signed URL from your backend

Recommended UX:

- On media error, call /api/stream-url/:id again once
- Retry with the new URL

## Validation checklist

1. GET / returns server health JSON.
2. GET /sign/db/:id with API key returns url.
3. Range request to returned URL returns HTTP 206.
4. Video element can seek (Accept-Ranges present).

## Common issues

1. no channels found:
   - Bot cannot access sourceChannel
   - Wrong sourceChannel/messageId in Mongo
2. invalid token:
   - Token expired or tampered
3. unauthorized from /sign/db:
   - Wrong or missing LINK_SIGN_API_KEY
4. browser mixed-content block:
   - Frontend uses https but stream URL is http
5. subtitle format is not convertible to vtt:
  - Conversion route only supports `.srt` (or already `.vtt`)
