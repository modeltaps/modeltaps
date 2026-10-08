#!/usr/bin/env bash
# 拉起基础设施（PostgreSQL / Redis / Mailpit）并跑完 AuthGear 的数据库迁移。
# 幂等：重复执行只会补齐缺失的部分。项目配置已经存在时顺带把 authgear 也拉起来。
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd docker curl python3

mkdir -p "$VAR_DIR/app" "$OUT_DIR"

log "写入 var/env"
cat >"$VAR_DIR/env" <<EOF
DEV_MODE=${DEV_MODE}
LOG_LEVEL=info
TRUST_PROXY=true
CONFIG_SOURCE_TYPE=local_fs
CONFIG_SOURCE_DIRECTORY=/app
CONFIG_SOURCE_WATCH=true
CUSTOM_RESOURCE_DIRECTORY=/app
DATABASE_URL=postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable
DATABASE_SCHEMA=public
AUDIT_DATABASE_URL=postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable
AUDIT_DATABASE_SCHEMA=public
SEARCH_DATABASE_URL=postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable
SEARCH_DATABASE_SCHEMA=public
REDIS_URL=redis://redis:6379/0
ANALYTIC_REDIS_URL=redis://redis:6379/1
AUDIT_LOG_ENABLED=true
SEARCH_ENABLED=true
EOF

log "启动 postgres / redis / mailpit（首次会构建带 pg_partman 的 postgres 镜像）"
dc up -d --wait --build postgres redis mailpit

log "执行 AuthGear 数据库迁移"
dc run --rm init-migrate >"$OUT_DIR/migrate.log" 2>&1 || {
  tail -30 "$OUT_DIR/migrate.log" >&2
  die "数据库迁移失败，完整输出见 out/migrate.log"
}

if [[ -f "$VAR_DIR/app/authgear.yaml" ]]; then
  log "检测到已有项目配置，拉起 authgear"
  dc up -d authgear
  wait_http "${AUTHGEAR_ORIGIN}/.well-known/openid-configuration" 90 \
    || die "AuthGear 未就绪，看 docker compose logs authgear"
fi

log "Mailpit: ${MAILPIT_API}"
log "完成。下一步：./init-project.sh"
