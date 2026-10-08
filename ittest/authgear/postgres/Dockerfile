# AuthGear 的 audit log 迁移需要 pg_partman 扩展，官方镜像不带。
# 官方 docker-compose 示例是从源码编译，这里直接用 PGDG 的 apt 包，构建更快。
ARG BASE_IMAGE=postgres:16-trixie
FROM ${BASE_IMAGE}

RUN set -eux; \
  apt-get update; \
  apt-get install -y --no-install-recommends postgresql-16-partman; \
  rm -rf /var/lib/apt/lists/*
