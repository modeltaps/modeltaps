#!/usr/bin/env bash
# 所有脚本共用的变量与工具函数。默认端口全部避开 3000/8080/5432/6379/1025/8025，
# 任何一项都可以用同名环境变量覆盖。
set -euo pipefail

AG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$AG_DIR/../.." && pwd)"
VAR_DIR="$AG_DIR/var"
OUT_DIR="$AG_DIR/out"

# 可选的本地覆盖文件，格式为 shell 赋值语句。
if [[ -f "$AG_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  source "$AG_DIR/.env"
fi

: "${COMPOSE_PROJECT_NAME:=modeltaps-ag-it}"
export COMPOSE_PROJECT_NAME

: "${AUTHGEAR_IMAGE:=quay.io/theauthgear/authgear-server:2026-06-11.0}"
: "${POSTGRES_IMAGE:=postgres:16-trixie}"
: "${REDIS_IMAGE:=redis:6.2.21}"
: "${MAILPIT_IMAGE:=axllent/mailpit:latest}"

: "${AUTHGEAR_APP_ID:=modeltaps}"
: "${AUTHGEAR_PORT:=3110}"        # AuthGear 登录页 / OIDC issuer
: "${AUTHGEAR_ADMIN_PORT:=3112}"  # AuthGear Admin GraphQL API
: "${MAILPIT_HTTP_PORT:=8126}"    # Mailpit Web UI 与 API
: "${MAILPIT_SMTP_PORT:=1126}"    # Mailpit SMTP

: "${MODELTAPS_PORT:=3210}"          # 0 表示随机取一个空闲端口
# Modeltaps 的会话 Cookie 已改名为 modeltaps_session，不再与 AuthGear 的 session 撞名，
# 同挂 localhost 也不会互相覆盖。这里仍把两边分到不同主机名，只是为了让「哪一边的
# 请求」在抓包与日志里一眼可分；改成 localhost 也能跑通。
: "${MODELTAPS_HOST:=127.0.0.1}"
: "${ROOT_PASSWORD:=modeltaps-root-pw}"

: "${OIDC_SLUG:=authgear}"
: "${OIDC_CLIENT_ID:=modeltaps}"
: "${OIDC_CLIENT_SECRET:=modeltaps-authgear-it-client-secret}"
# 项目只有邮箱一种登录标识，不申请 phone scope。
: "${OIDC_SCOPES:=openid,email,profile}"
: "${LINK_BY_VERIFIED_EMAIL:=false}"

# DEV_MODE=true 时 AuthGear 不外发邮件，改为把整条正文写进日志
# （pkg/lib/messaging/sender.go 的 devModeSendEmail）。
: "${DEV_MODE:=true}"

export AUTHGEAR_IMAGE POSTGRES_IMAGE REDIS_IMAGE MAILPIT_IMAGE
export AUTHGEAR_APP_ID AUTHGEAR_PORT AUTHGEAR_ADMIN_PORT
export MAILPIT_HTTP_PORT MAILPIT_SMTP_PORT DEV_MODE

AUTHGEAR_ORIGIN="http://localhost:${AUTHGEAR_PORT}"
MAILPIT_API="http://localhost:${MAILPIT_HTTP_PORT}"

dc() {
  docker compose -f "$AG_DIR/docker-compose.yaml" "$@"
}

log() {
  printf '\033[36m==>\033[0m %s\n' "$*"
}

die() {
  printf '\033[31mERROR:\033[0m %s\n' "$*" >&2
  exit 1
}

# wait_http <url> <seconds> [期望的 curl 成功即可]
wait_http() {
  local url="$1" timeout="${2:-90}" i=0
  while (( i < timeout )); do
    if curl -fsS -o /dev/null --max-time 3 "$url" 2>/dev/null; then
      return 0
    fi
    sleep 1
    (( i++ ))
  done
  return 1
}

# free_port 返回一个当前空闲的 TCP 端口
free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

# stop_modeltaps 停掉 modeltaps-up.sh 起的宿主机进程：先按 pid 文件，再按二进制路径兜底，
# 保证 down.sh 之后 `pgrep -f ittest/authgear/out/modeltaps` 为空。
stop_modeltaps() {
  local pid pidfile="$OUT_DIR/modeltaps.pid"
  if [[ -f "$pidfile" ]]; then
    pid="$(cat "$pidfile" 2>/dev/null || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      log "停止 Modeltaps 进程 $pid"
      kill "$pid" 2>/dev/null || true
    fi
    rm -f "$pidfile"
  fi
  # 二进制路径唯一（out/modeltaps），不会误伤其它 Modeltaps 实例。
  pkill -f "^${OUT_DIR}/modeltaps\$" 2>/dev/null || true
  local i=0
  while (( i < 20 )) && pgrep -f "^${OUT_DIR}/modeltaps\$" >/dev/null 2>&1; do
    sleep 0.5
    (( i++ ))
  done
  pkill -9 -f "^${OUT_DIR}/modeltaps\$" 2>/dev/null || true
}

require_cmd() {
  for c in "$@"; do
    command -v "$c" >/dev/null 2>&1 || die "缺少命令：$c"
  done
}
