# Single version source: can be injected at build time via --build-arg VERSION=<tag> (usually from the workflow metadata-action);
# when not passed, falls back to the VERSION file in the repo, then to "dev". The frontend VITE_APP_VERSION and Go ldflags share this value.
ARG VERSION=

# node:24.21
FROM node:24.21@sha256:64af3819f9275802414d7cdc38c27e9d82bd564dec4d4da87d008255d36c63b4 AS builder

WORKDIR /build

# Corepack ships with this Node image and activates the pnpm version pinned by
# packageManager in package.json.
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable pnpm

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/

RUN pnpm install --frozen-lockfile --filter modeltaps-web

COPY ./web web/
COPY ./VERSION .
ARG VERSION
RUN VER="${VERSION:-$(cat VERSION 2>/dev/null || echo dev)}" && \
    cd web && VITE_APP_VERSION="$VER" pnpm build

# golang:1.27.1
FROM golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 AS builder2

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOPROXY=https://proxy.golang.org,direct

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=builder /build/web/build ./web/build
ARG VERSION
RUN VER="${VERSION:-$(cat VERSION 2>/dev/null || echo dev)}" && \
    go build -ldflags "-s -w -X 'github.com/modeltaps/modeltaps/common/config.Version=${VER}' -extldflags '-static'" -o modeltaps

# alpine:3.24
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

# Static OCI metadata. The dynamic revision/version (org.opencontainers.image.revision/version)
# are injected at build time by the workflow's docker/metadata-action and are not hard-coded here.
LABEL org.opencontainers.image.source="https://github.com/modeltaps/modeltaps" \
      org.opencontainers.image.description="Modeltaps — multi-provider AI gateway with organization/multi-tenant layer" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

RUN apk update && \
    apk upgrade && \
    apk add --no-cache ca-certificates tzdata && \
    update-ca-certificates 2>/dev/null || true

# Dedicated non-root user with fixed UID/GID=10001, so ownership of host-mounted volumes lines up.
RUN addgroup -g 10001 modeltaps && \
    adduser -D -H -u 10001 -G modeltaps modeltaps

COPY --from=builder2 /build/modeltaps /modeltaps

# /data is the runtime working directory (SQLite, logs, etc.), owned by the non-root user.
# Migration note: if an existing deployment created the host ./data volume owned by root, after switching to this image
# modeltaps (10001) inside the container will have no write permission. Pick one:
#   1) run `chown -R 10001:10001 ./data` on the host; or
#   2) override back to root in compose with `user: "0:0"` (not recommended, loses the non-root benefit).
RUN mkdir -p /data && chown -R modeltaps:modeltaps /data

EXPOSE 3000
WORKDIR /data
USER modeltaps

# busybox wget probes /api/status. Default port is 3000; if the listen port is overridden via the PORT env var,
# ${PORT:-3000} here follows it. Limitation: only the env form of PORT is recognized; if the port is set via the config file
# (viper's port key), the healthcheck still hits 3000 and needs adjusting, or use a compose healthcheck.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider "http://localhost:${PORT:-3000}/api/status" || exit 1

ENTRYPOINT ["/modeltaps"]
