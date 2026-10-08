# devenv —— 本地开发用的依赖服务

每个子目录是一个独立的 docker compose 栈，按需单独起停，互不依赖。
全部是 devenv 专用的弱口令 / 明文配置，**勿用于任何真实环境**。
各栈的完整说明（端口、可调变量、注意事项）写在对应 `docker-compose.yml` 的文件头注释里。

## OIDC 登录测试（Keycloak）

```bash
cd devenv/keycloak && docker compose up -d
```

控制台 http://localhost:8087/ （admin / admin），Modeltaps 后台要填的 issuer / client
见 `keycloak/docker-compose.yml` 头部。

## 邮件测试（Mailpit）

```bash
cd devenv/mailpit && docker compose up -d
```

Mailpit 是假 SMTP 服务器：收下所有信件但不投递，收到的邮件在 Web 界面里看，
用来验证邮箱验证码、找回密码、额度告警等邮件链路。

- Web 界面：http://localhost:8025
- SMTP：`localhost:1025`（无加密，接受任意账号 / 凭证）

Modeltaps 后台「系统设置 → 邮件」按下表填：

| 字段 | 值 |
| --- | --- |
| SMTP 服务器 | `localhost` |
| SMTP 端口 | `1025` |
| SMTP 账号 | 任意非空，如 `mailpit` |
| SMTP 凭证 | 任意非空，如 `mailpit` |
| 发件人地址 | 任意，如 `dev@modeltaps.local` |
| TLS 模式 | `none` |
