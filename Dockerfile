
FROM node:20-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM golang:1.23-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fleetview ./cmd/server

FROM alpine:3.20

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
