# Multi-stage build: React frontend -> Go server (single static binary) -> runtime.
# Build context is the repo root (see deploy/docker-compose.yml).
# syntax=docker/dockerfile:1

# 1. Build the React app.
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2. Build the Go server (crafting knowledge base is embedded via go:embed).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/poefissure-server ./cmd/poefissure-server

# 3. Minimal runtime image.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/poefissure-server /app/poefissure-server
COPY --from=web /web/dist /app/web/dist
RUN mkdir -p /data && chown -R app /data
ENV WEB_DIR=/app/web/dist \
    TOKEN_FILE=/data/token.json \
    SNAP_DIR=/data/snapshots \
    STAT_IDS_FILE=/data/trade_stat_ids.json \
    PORT=8080
USER app
EXPOSE 8080
ENTRYPOINT ["/app/poefissure-server"]
CMD ["serve"]
