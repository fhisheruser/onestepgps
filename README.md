# FleetView

Live GPS fleet tracking dashboard on the [OneStepGPS](https://onestepgps.com) public device API.
Go backend, Vue 3 frontend, one Docker image.

- **Live positions** — the server polls the provider every 10s and pushes updates to browsers over WebSocket.
- **Personalisation that sticks** — rename vehicles, recolour markers, upload a custom marker image, pin, hide, reorder, add notes. Stored per user in SQLite.
- **Honest freshness** — if the provider goes down the UI says *"showing cached positions"* rather than quietly displaying stale data as if it were live.

---

## Quick start

### Docker (one command, no keys needed)

```bash
docker compose up --build
```

Open <http://localhost:8080>. With no API key set it runs a built-in simulator: 10 vehicles driving around San Diego.

To use real data and the map, create a `.env` next to `docker-compose.yml`:

```bash
ONESTEPGPS_API_KEY=your-onestepgps-key
GOOGLE_MAPS_API_KEY=your-maps-browser-key
```

### Local development

```bash
cp backend/.env.example backend/.env
```

```bash
cd backend && go run ./cmd/server
```

```bash
cd frontend && npm install && npm run dev
```

The Vite dev server on `:5173` proxies `/api` to the backend on `:8080`, so the browser sees one origin and WebSocket upgrades work unchanged.

---

## Architecture

```
Browser ──REST + WebSocket──> Go API ──polling every 10s──> OneStepGPS
   │                            │
   └── Google Maps JS           └── SQLite (preferences, icons, history)
```

**The provider is polled once for everyone, not once per browser.** A single background poller writes the latest snapshot into an in-memory cache; every HTTP request and WebSocket push reads from that cache and merges the caller's saved preferences on top. Adding the hundredth browser tab adds zero upstream API calls.

```
backend/
  cmd/server/            entrypoint, wiring, graceful shutdown
  internal/domain/       entities + validation, no framework imports
  internal/provider/     OneStepGPS client, demo simulator (same interface)
  internal/service/      poller, snapshot cache, device + preference services
  internal/repository/   GORM/SQLite: preferences, icons, history
  internal/transport/    Gin handlers, DTOs, WebSocket hub
frontend/
  src/stores/            Pinia: fleet, preferences, ui
  src/services/          axios client, WebSocket client with backoff
  src/components/        cards, map, panels, CSS-3D vehicles
```

### Decisions worth knowing

**The OneStepGPS key never reaches the browser.** All provider calls happen server-side; the client only ever talks to this API. The Google Maps key *is* served to the browser (that is what a browser key is for) via `GET /api/v1/config` at runtime rather than baked into the bundle — so one built image works in every environment. Restrict it by HTTP referrer in Cloud Console.

**Stale data is served, labelled.** When the provider fails, the cache keeps serving the last good snapshot with `meta.stale: true` and the reason. A fleet dispatcher would rather see 90-second-old positions marked as such than an error page.

**WebSocket is an optimisation, never a requirement.** The client always keeps a REST polling fallback and reconnects with exponential backoff plus jitter. The badge in the header tells you which transport is actually live.

**Uploaded icons are validated on their bytes, not their filename**, served with `Content-Security-Policy: default-src 'none'` and `X-Content-Type-Options: nosniff`. SVG is deliberately rejected — an attacker-supplied SVG served from your own origin can execute script.

**The 3D vehicles are CSS, not WebGL.** `transform-style: preserve-3d` boxes with per-face shading. A WebGL library would add hundreds of kilobytes and a GPU context per card to draw what are, visually, shaded rectangles.

---

## API

All endpoints are under `/api/v1`. The user scope comes from the `X-User-Id` header (or `?userId=` for downloads and WebSocket, which cannot carry headers). It is a namespace, not authentication.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/config` | Runtime config for the browser (Maps key, intervals) |
| `GET` | `/devices` | Merged feed: devices + summary + freshness. Filters: `search`, `status`, `sort`, `dir`, `includeHidden`, `pinned` |
| `GET` | `/devices/:id` | One device |
| `GET` | `/devices/:id/history` | Breadcrumb trail (`minutes`, `limit`) |
| `GET` | `/fleet/summary` | KPIs only |
| `GET` | `/preferences` | Settings + all device preferences |
| `PUT` | `/preferences/settings` | Update global settings |
| `PUT` | `/preferences/devices/:id` | Rename, recolour, pin, hide, notes |
| `DELETE` | `/preferences/devices/:id` | Reset one vehicle |
| `POST` | `/preferences/devices/:id/icon` | Upload custom marker (multipart `icon`) |
| `DELETE` | `/preferences/devices/:id/icon` | Remove custom marker |
| `POST` | `/preferences/reorder` | Persist a custom order |
| `POST` | `/preferences/reset` | Reset everything for this user |
| `GET` | `/export/devices.csv` | CSV of the current view |
| `GET` | `/icons/:id` | Serve an uploaded marker |
| `GET` | `/ws` | WebSocket feed |
| `GET` | `/healthz`, `/readyz` | Liveness / readiness |

Errors use one envelope:

```json
{ "error": { "code": "validation_failed", "message": "markerColor must be a hex colour", "field": "markerColor" } }
```

---

## Configuration

Full list with defaults in [`backend/.env.example`](backend/.env.example). The ones that matter:

| Variable | Default | Notes |
|---|---|---|
| `ONESTEPGPS_API_KEY` | *(empty)* | Empty ⇒ demo simulator |
| `GOOGLE_MAPS_API_KEY` | *(empty)* | Empty ⇒ list works, map shows a setup message |
| `PORT` | `8080` | Honoured by Cloud Run automatically |
| `DB_PATH` | `data/fleetview.db` | |
| `POLL_INTERVAL` | `10s` | One upstream call per interval, for all users |
| `STATIC_DIR` | *(empty)* | Set by the Docker image to serve the built frontend |
| `CORS_ALLOWED_ORIGINS` | localhost dev ports | Only needed if the frontend is on another origin |

---

## Tests

```bash
cd backend && go test ./...
```

```bash
cd frontend && npm test
```

Backend covers the provider client (retries, malformed payloads, redaction), preference persistence, filtering/sorting/merging, and every HTTP handler including validation rejections, per-user scoping and CORS. Frontend covers unit conversion and formatting, the fleet store's optimistic updates and rollback, and card rendering.

---

## Deploying to Cloud Run

```bash
gcloud run deploy fleetview --source . --region us-central1 --allow-unauthenticated --max-instances 1 --set-env-vars ONESTEPGPS_API_KEY=...,GOOGLE_MAPS_API_KEY=...
```

**`--max-instances 1` is not optional with the default SQLite storage.** Cloud Run gives each instance its own ephemeral filesystem, so two instances would keep two divergent preference databases and a restart would wipe both. For real persistence, mount a GCS volume and point `DB_PATH` at it, or move the three repositories in `internal/repository/` to Cloud SQL — they are behind interfaces for exactly that reason.

Put the keys in Secret Manager rather than `--set-env-vars` for anything beyond a demo.
