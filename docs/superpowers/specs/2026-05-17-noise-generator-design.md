# Noise Generator Design

**Date:** 2026-05-17  
**Status:** Approved

## Problem

HASS automation streams audio to a Google Home speaker during night hours (22:00–09:00). Google Home plays a "bong" sound at the start of each new Cast session. A single bong when the night starts is acceptable; bongs mid-night are not. A local noise generator eliminates dependency on external audio services and gives full control over the stream lifecycle.

Known constraint: WiFi restarts at 03:00 daily, dropping the Cast session for a few minutes. This is handled entirely by a HASS automation (mute volume before 03:00, restart stream + restore volume when `media_player` entity comes back online). Go has no role in the workaround.

## Goals

- Serve an infinite white/pink noise audio stream over HTTP.
- Expose a health sensor via MQTT for the HASS dashboard (consistent with how the service exposes other runtime entities).
- Add a general-purpose Gin HTTP server to the service (not a one-off audio server).
- No new CGO dependencies; no `ffmpeg` requirement.

## Architecture

```
app.go
├── ga.App (WebSocket HASS)     — existing
├── entities.Runtime (MQTT)     — existing
└── http.Server (Gin)           — new
    ├── GET /health              (infrastructure/Docker use)
    └── GET /noise/:type
```

The HTTP server starts in a separate goroutine from `app.go`, sharing the same `context` for graceful shutdown. It does not participate in the component framework (no event/entity listeners, schedules, or intervals).

A new `health` component is registered in the existing component framework. It publishes an MQTT sensor via `entities.Runtime` so HASS auto-discovers it and shows it on the dashboard.

## Components

### `internal/domain/noise/`

Pure domain package — no I/O.

```go
type Generator interface {
    Fill(buf []int16)
}

func NewGenerator(noiseType string) (Generator, error)
```

**White noise** — samples drawn from `math/rand` scaled to `[-32768, 32767]`. Stateless.

**Pink noise** — Voss-McCartney algorithm. Maintains 16 rows of running sums; updates one row per sample based on trailing zeros of an incrementing counter. Pure integer arithmetic, ~20 lines, no external deps.

### `internal/tech/http/`

- `server.go` — Gin engine setup, route registration, `Start(ctx)` / shutdown.
- `noise/handler.go` — streaming handler.
- `health/handler.go` — `/health` endpoint for Docker `HEALTHCHECK` and infrastructure monitoring.

### Streaming: `GET /noise/:type`

Accepts `white` or `pink`; returns 404 for unknown types.

Response headers:
```
Content-Type: audio/wav
Cache-Control: no-cache
Transfer-Encoding: chunked
```

On connection:
1. Write a 44-byte WAV header — RIFF + fmt chunk + data chunk preamble — with both size fields set to `0x7FFFFFFF`. At 44100 Hz mono 16-bit, `0x7FFFFFFF` bytes ≈ 13.5 hours, safely above the 11-hour night window.
2. Loop: `gen.Fill(buf)` → encode as little-endian `int16` bytes → write → flush.
3. Exit when the request context is cancelled or the write fails (client disconnected).

Buffer: 4096 samples (~93 ms at 44100 Hz). Each connection owns its own generator goroutine; no shared mutable state.

Audio spec: 44100 Hz, mono, 16-bit signed PCM.

### Health HTTP: `GET /health`

```json
{"status": "ok", "uptime": "3h42m"}
```

Returns `200 OK`. Intended for Docker `HEALTHCHECK` and external infrastructure monitoring, not for the HASS dashboard.

### Health MQTT sensor: `internal/tech/homeassistant/devices/health/`

A component registered in the existing framework. Uses `entities.Runtime` to publish a discoverable MQTT sensor — consistent with how other runtime entities (e.g. reminders) work.

- Entity name: `home-go_health` (follows the existing `AppPrefix` convention)
- Published value: uptime string (e.g. `"3h42m"`)
- Update interval: 60 seconds via `Intervals()`
- On service death: MQTT LWT automatically marks the entity `unavailable` in HASS

HASS auto-discovers the sensor and it appears on the dashboard like any other entity. No manual HASS configuration required.

## Configuration

Two new env vars added to `config.go`:

| Variable    | Default     | Description           |
|-------------|-------------|-----------------------|
| `HTTP_PORT` | `8080`      | Gin server port       |
| `HTTP_HOST` | `0.0.0.0`   | Gin server bind host  |

`docker-compose.yaml` exposes `HTTP_PORT`.

## HASS Integration

HASS automation calls:
```yaml
service: media_player.play_media
data:
  media_content_id: "http://<go-host>:8080/noise/pink"
  media_content_type: music
```

The health sensor is auto-discovered via MQTT — no manual HASS configuration needed.

## WiFi Restart Workaround (HASS-side)

1. At 02:58 — HASS sets Google Home volume to 0.
2. WiFi restarts at 03:00; Cast session drops.
3. HASS watches `media_player` state; when it becomes `idle`/`playing` after 03:05, re-issues `play_media` pointing to Go stream URL.
4. HASS restores volume (bong plays silently).

Go service reconnects to HASS WebSocket automatically via the existing `gome-assistant` library reconnect behaviour.

## What Does Not Change

- Existing component framework untouched.
- No changes to any existing domain package.
- `app.go` gains ~15 lines to start the HTTP server goroutine and register the health component.
- `config.go` gains two fields.

## Upgrade Path

If WAV streaming proves unreliable on a specific Cast firmware, replace the streaming handler with an ffmpeg-piped MP3 stream. The noise generator domain package, health MQTT component, and Gin server structure remain unchanged.
