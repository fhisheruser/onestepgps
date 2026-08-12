# Single image: the Go binary serves both the API and the built frontend.
#
# ponytail: no separate Nginx container. The backend already has a STATIC_DIR
# mode, so one process on one port removes a reverse proxy, a second image and
# a CORS configuration. Split them again only if the frontend needs a CDN.

# ---- 1. Build the frontend -------------------------------------------------
FROM node:20-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 2. Build the API ------------------------------------------------------
FROM golang:1.23-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# CGO off because the SQLite driver is pure Go; that is what allows the
# distroless static base below.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fleetview ./cmd/server

# ---- 3. Runtime ------------------------------------------------------------
# ponytail: alpine rather than distroless. Distroless would be ~8 MB smaller,
# but it cannot chown the /data volume for a non-root user and has no binary to
# health-check with. One RUN line buys both.
FROM alpine:3.20

# ca-certificates is not optional: without it the OneStepGPS HTTPS call fails.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 fleetview \
 && mkdir -p /data && chown fleetview:fleetview /data

COPY --from=api /out/fleetview /app/fleetview
COPY --from=web /web/dist /app/web

ENV STATIC_DIR=/app/web \
    DB_PATH=/data/fleetview.db \
    PORT=8080 \
    APP_ENV=production

EXPOSE 8080
USER fleetview
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -qO- "http://127.0.0.1:${PORT}/healthz" || exit 1

ENTRYPOINT ["/app/fleetview"]
