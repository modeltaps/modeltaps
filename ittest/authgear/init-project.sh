#!/usr/bin/env bash
# 生成 AuthGear 项目配置（local_fs 配置源）并拉起 authgear。
# 幂等：已存在配置时只覆盖 authgear.yaml，密钥材料（authgear.secrets.yaml）保留不变。
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd docker curl python3

[[ -d "$VAR_DIR/app" ]] || die "请先执行 ./up.sh"

APP_DIR="$VAR_DIR/app"

if [[ ! -f "$APP_DIR/authgear.secrets.yaml" ]]; then
  log "authgear init 生成密钥材料"
  dc run --rm --no-deps -T --entrypoint authgear -v "$APP_DIR:/work" -w /work authgear \
    init --interactive=false \
    --purpose=project \
    --app-id="$AUTHGEAR_APP_ID" \
    --public-origin="$AUTHGEAR_ORIGIN" \
    --phone-otp-mode=sms \
    --search-implementation=postgresql \
    --database-url='postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable' \
    --audit-database-url='postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable' \
    --search-database-url='postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable' \
    --redis-url='redis://redis:6379/0' \
    --analytic-redis-url='redis://redis:6379/1' \
    -o /work >"$OUT_DIR/init.log" 2>&1 || {
      tail -30 "$OUT_DIR/init.log" >&2
      die "authgear init 失败，完整输出见 out/init.log"
    }
  # authgear init 不会写 mail.smtp 与 oauth.client_secrets，这两段补在下面。
  # secrets 是一个扁平列表，直接追加即可。
  CLIENT_SECRET_K="$(python3 -c 'import base64,sys; print(base64.urlsafe_b64encode(sys.argv[1].encode()).decode().rstrip("="))' "$OIDC_CLIENT_SECRET")"
  cat >>"$APP_DIR/authgear.secrets.yaml" <<EOF
- key: mail.smtp
  data:
    host: mailpit
    port: 1025
    mode: normal
    # AuthGear 会把 SMTP host/username/password 当敏感串，在日志里出现即打码。
    # 这三个值必须不与测试用的邮箱域名/用户名重叠，否则 otp.sh 匹配不到 recipient。
    username: smtpuser
    password: smtppass
    sender: no-reply@authgear.test
- key: oauth.client_secrets
  data:
    items:
    - client_id: ${OIDC_CLIENT_ID}
      keys:
      - kty: oct
        kid: ${OIDC_CLIENT_ID}-secret
        k: ${CLIENT_SECRET_K}
EOF
else
  log "复用已有 authgear.secrets.yaml"
fi

log "写入 authgear.yaml（只有 email 一种 login_id，password + 邮箱验证码两种认证器）"
cat >"$APP_DIR/authgear.yaml" <<EOF
id: ${AUTHGEAR_APP_ID}
http:
  public_origin: ${AUTHGEAR_ORIGIN}
search:
  implementation: postgresql
ui:
  signup_login_flow_enabled: true
authentication:
  identities:
  - login_id
  primary_authenticators:
  - password
  - oob_otp_email
# 邮箱是唯一的登录标识：Modeltaps 侧的账号体系按邮箱收敛，用户名与手机号都不参与登录。
identity:
  login_id:
    keys:
    - key: email
      type: email
# 邮箱验证默认开启：安全用例会另外准备一份关闭验证的副本来观察 email_verified=false。
verification:
  claims:
    email:
      enabled: true
      required: true
oauth:
  clients:
  - client_id: ${OIDC_CLIENT_ID}
    name: Modeltaps
    client_name: Modeltaps
    x_application_type: confidential
    grant_types:
    - authorization_code
    - refresh_token
    response_types:
    - code
    redirect_uris:
    - http://${MODELTAPS_HOST}:${MODELTAPS_PORT}/oauth/oidc/${OIDC_SLUG}
    # 退出登录走 RP-Initiated Logout，IdP 结束会话后回跳本站已登出页。
    post_logout_redirect_uris:
    - http://${MODELTAPS_HOST}:${MODELTAPS_PORT}/signed-out
EOF

log "拉起 authgear"
dc up -d authgear
wait_http "${AUTHGEAR_ORIGIN}/.well-known/openid-configuration" 120 \
  || { dc logs --tail 40 authgear >&2; die "AuthGear 未就绪"; }

cat >"$OUT_DIR/authgear.env" <<EOF
AUTHGEAR_ORIGIN=${AUTHGEAR_ORIGIN}
AUTHGEAR_ADMIN_API=http://localhost:${AUTHGEAR_ADMIN_PORT}
AUTHGEAR_APP_ID=${AUTHGEAR_APP_ID}
OIDC_CLIENT_ID=${OIDC_CLIENT_ID}
OIDC_CLIENT_SECRET=${OIDC_CLIENT_SECRET}
MAILPIT_API=${MAILPIT_API}
EOF

log "AuthGear 就绪：${AUTHGEAR_ORIGIN}"
log "完成。下一步：./modeltaps-up.sh"
