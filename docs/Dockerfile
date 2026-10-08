# 文档站镜像:VitePress 构建静态产物,nginx 托管。仅用于本地/自部署场景,
# 由 docker-compose.yml 中的 docs profile 选装,不参与网关主镜像的发布流程。
# The build context is the repository root (the pnpm lockfile lives there):
# docker build -f docs/Dockerfile .
# docs/Dockerfile.dockerignore replaces the root .dockerignore for this build.

# node:20-alpine
FROM node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS builder

WORKDIR /build

# config.mts 开启了 lastUpdated,VitePress 会 spawn git 取文件时间戳,缺少 git 二进制会直接构建失败;
# 构建上下文不含 .git,时间戳取不到值,页面上的"最后更新"会省略,不影响其余内容。
# Corepack ships with this Node image and activates the pnpm version pinned by
# packageManager in package.json.
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN apk add --no-cache git && corepack enable pnpm

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY docs/package.json docs/
RUN pnpm install --frozen-lockfile --filter docs

COPY docs docs/
RUN cd docs && pnpm docs:build

# nginx:alpine
FROM nginx:alpine@sha256:db35bfc6b2951e7f8a72db5db120288c127ffaeeb4a6d4b95a26fead017d5913

LABEL org.opencontainers.image.source="https://github.com/modeltaps/modeltaps" \
      org.opencontainers.image.description="Modeltaps documentation site (VitePress static build served by nginx)" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

COPY --from=builder /build/docs/.vitepress/dist /usr/share/nginx/html

EXPOSE 80
