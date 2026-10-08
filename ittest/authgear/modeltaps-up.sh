#!/usr/bin/env bash
# 编译当前分支并以 SQLite + 独立工作目录启动 Modeltaps，然后通过后台 HTTP 接口
# 登记 slug=authgear 的 OIDC 提供方。幂等：重复执行会先停掉上一次的进程。
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd docker curl python3 go

[[ -f "$VAR_DIR/app/authgear.yaml" ]] || die "请先执行 ./up.sh 与 ./init-project.sh"

DATA_DIR="$OUT_DIR/modeltaps-data"
BIN="$OUT_DIR/modeltaps"
COOKIES="$OUT_DIR/modeltaps-cookies.txt"

stop_modeltaps

if [[ "$MODELTAPS_PORT" == "0" ]]; then
  MODELTAPS_PORT="$(free_port)"
fi
MODELTAPS_BASE="http://${MODELTAPS_HOST}:${MODELTAPS_PORT}"

log "编译 Modeltaps（复用已有 web/build，不跑前端构建）"
[[ -d "$REPO_ROOT/web/build" ]] || die "缺少 web/build，请先在仓库根执行 make web"
( cd "$REPO_ROOT" && go build -o "$BIN" . ) || die "go build 失败"

mkdir -p "$DATA_DIR"
log "启动 Modeltaps：端口 ${MODELTAPS_PORT}，数据目录 out/modeltaps-data"
# exec 让子 shell 被二进制取代，$! 记下的就是 Modeltaps 本身的 pid；
# stdin/stdout/stderr 全部重定向，避免继承调用方的管道导致 `modeltaps-up.sh | tail` 收不到 EOF。
(
  cd "$DATA_DIR"
  exec env \
    PORT="$MODELTAPS_PORT" \
    SQLITE_PATH="$DATA_DIR/modeltaps.db" \
    ROOT_PASSWORD="$ROOT_PASSWORD" \
    SESSION_SECRET="modeltaps-authgear-it-session-secret" \
    USER_TOKEN_SECRET="modeltaps-authgear-it-user-token-secret" \
    "$BIN" </dev/null >"$OUT_DIR/modeltaps.log" 2>&1
) &
modeltaps_pid=$!
disown "$modeltaps_pid" 2>/dev/null || true
echo "$modeltaps_pid" >"$OUT_DIR/modeltaps.pid"
wait_http "${MODELTAPS_BASE}/api/status" 90 || {
  tail -30 "$OUT_DIR/modeltaps.log" >&2
  die "Modeltaps 未就绪，完整日志见 out/modeltaps.log"
}

log "以 root 登录后台"
rm -f "$COOKIES"
curl -fsS -c "$COOKIES" -X POST "${MODELTAPS_BASE}/api/user/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"root\",\"password\":\"${ROOT_PASSWORD}\"}" \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get("success") else print("login failed:", d.get("message")) or 1)' \
  || die "root 登录失败"

log "设置服务器地址为 ${MODELTAPS_BASE}（回调地址由它拼出）"
curl -fsS -b "$COOKIES" -X PUT "${MODELTAPS_BASE}/api/option/" \
  -H 'Content-Type: application/json' \
  -d "{\"key\":\"ServerAddress\",\"value\":\"${MODELTAPS_BASE}\"}" >/dev/null

# 端口/主机可能与 init-project.sh 写进 authgear.yaml 的不一致（随机端口场景）。
# CONFIG_SOURCE_WATCH=true，改完文件 AuthGear 会自动重载，不用重启容器。
if ! grep -q "${MODELTAPS_BASE}/oauth/oidc/${OIDC_SLUG}" "$VAR_DIR/app/authgear.yaml"; then
  log "同步 AuthGear 侧的 redirect_uri 与 post_logout_redirect_uri 到 ${MODELTAPS_BASE}"
  python3 - "$VAR_DIR/app/authgear.yaml" "$MODELTAPS_BASE" "$OIDC_SLUG" <<'PY'
import re, sys
path, base, slug = sys.argv[1], sys.argv[2], sys.argv[3]
text = open(path).read()
text = re.sub(r"http://[^/\s]+/oauth/oidc/" + re.escape(slug),
              f"{base}/oauth/oidc/{slug}", text)
text = re.sub(r"http://[^/\s]+/signed-out", f"{base}/signed-out", text)
open(path, "w").write(text)
PY
  sleep 3
fi

log "登记 OIDC 提供方 slug=${OIDC_SLUG}（标记为本站身份，/login 才会直达）"
payload="$(python3 -c '
import json, sys
slug, issuer, cid, secret, scopes, link = sys.argv[1:7]
print(json.dumps({
    "slug": slug, "display_name": "Authgear", "issuer": issuer,
    "client_id": cid, "client_secret": secret, "scopes": scopes,
    "username_claim": "", "display_name_claim": "", "avatar_claim": "",
    "link_by_verified_email": link == "true", "link_by_verified_phone": False,
    "disable_auto_register": False, "first_party": True,
    "enabled": True, "sort": 0,
}))' "$OIDC_SLUG" "$AUTHGEAR_ORIGIN" "$OIDC_CLIENT_ID" "$OIDC_CLIENT_SECRET" "$OIDC_SCOPES" "$LINK_BY_VERIFIED_EMAIL")"

existing_id="$(curl -fsS -b "$COOKIES" "${MODELTAPS_BASE}/api/oidc_provider/" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
for p in d.get('data') or []:
    if p.get('slug') == '${OIDC_SLUG}':
        print(p['id']); break
")"

if [[ -n "$existing_id" ]]; then
  method=PUT; url="${MODELTAPS_BASE}/api/oidc_provider/${existing_id}"
else
  method=POST; url="${MODELTAPS_BASE}/api/oidc_provider/"
fi
curl -fsS -b "$COOKIES" -X "$method" "$url" \
  -H 'Content-Type: application/json' -d "$payload" \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get("success") else print("provider failed:", d.get("message")) or 1)' \
  || die "登记提供方失败（discovery 不通？检查 ${AUTHGEAR_ORIGIN}）"

# 账号体系切到 external：/login 由服务端 302 到 AuthGear，本站不再渲染登录表单，
# root 仍可从 /login/admin 用密码进后台。
log "切换账号体系为 external（AccountSystem）"
curl -fsS -b "$COOKIES" -X PUT "${MODELTAPS_BASE}/api/option/" \
  -H 'Content-Type: application/json' \
  -d '{"key":"AccountSystem","value":"external"}' \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); sys.exit(0 if d.get("success") else print("option failed:", d.get("message")) or 1)' \
  || die "切换账号体系失败"

cat >"$OUT_DIR/modeltaps.env" <<EOF
MODELTAPS_PORT=${MODELTAPS_PORT}
MODELTAPS_BASE=${MODELTAPS_BASE}
MODELTAPS_ROOT_PASSWORD=${ROOT_PASSWORD}
MODELTAPS_COOKIES=${COOKIES}
MODELTAPS_DATA_DIR=${DATA_DIR}
OIDC_SLUG=${OIDC_SLUG}
OIDC_REDIRECT_URI=${MODELTAPS_BASE}/oauth/oidc/${OIDC_SLUG}
AUTHGEAR_ORIGIN=${AUTHGEAR_ORIGIN}
MAILPIT_API=${MAILPIT_API}
EOF

log "Modeltaps: ${MODELTAPS_BASE}   AuthGear: ${AUTHGEAR_ORIGIN}   Mailpit: ${MAILPIT_API}"
log "端口与凭据已写入 out/modeltaps.env"
