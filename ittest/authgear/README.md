# AuthGear + Mailpit 本地测试环境

自托管 AuthGear 作为身份提供方，用来验证 Modeltaps 的**外部账号体系**（`AccountSystem=external`）
链路。AuthGear 项目只配**邮箱一种登录标识**，两种登录组合：邮箱+密码、邮箱+验证码。
本目录完全自包含：不改 Modeltaps 源码，不接公网，`down.sh -v` 之后不留痕迹。

## 前置

Docker + docker compose v2、`go`、`python3`、`curl`。
仓库根需要已有 `web/build`（`modeltaps-up.sh` 只编译后端，不跑前端构建）。

## 三步

```bash
cd ittest/authgear
./up.sh            # postgres(带 pg_partman) / redis / mailpit + AuthGear 数据库迁移
./init-project.sh  # authgear init 生成密钥 + 写 authgear.yaml + 拉起 authgear
./modeltaps-up.sh     # 编译并启动 Modeltaps(SQLite)，登记 slug=authgear 的 OIDC 提供方
```

`modeltaps-up.sh` 会把 slug=authgear 的提供方标记为「本站身份」（`first_party`）并把站点的
`AccountSystem` 切成 `external`。跑完用浏览器打开 `http://127.0.0.1:3210/login`，服务端直接
302 到 AuthGear 登录页——本站不再渲染登录表单，也没有「使用 Authgear 登录」按钮。
root 的应急密码登录入口在 `http://127.0.0.1:3210/login/admin`（旧地址 `/login?local=1` 301 到这里）。
端口、root 密码、回调地址等写在 `out/modeltaps.env`。
三个脚本都幂等，可重复执行。

## 端口

| 服务 | 地址 | 覆盖用的环境变量 |
|---|---|---|
| Modeltaps | http://127.0.0.1:3210 | `MODELTAPS_HOST` / `MODELTAPS_PORT`（`MODELTAPS_PORT=0` 取随机空闲端口）|
| AuthGear（登录页 / issuer）| http://localhost:3110 | `AUTHGEAR_PORT` |
| AuthGear Admin GraphQL | http://localhost:3112 | `AUTHGEAR_ADMIN_PORT` |
| Mailpit Web / API | http://localhost:8126 | `MAILPIT_HTTP_PORT` |
| Mailpit SMTP | 1126 | `MAILPIT_SMTP_PORT` |

覆盖值可以写进本目录下的 `.env`（shell 赋值语句，已 gitignore）。

Modeltaps 挂 127.0.0.1、AuthGear 挂 localhost 只是为了让两边的请求一眼可分。Modeltaps 的会话
Cookie 已改名为 `modeltaps_session`（HTTPS 下 `__Host-modeltaps_session`），不再与 AuthGear 的
`session` 撞名，两边同挂 `localhost` 也不会互相覆盖——历史上那个「回调报 `state is empty
or not same`」的坑随改名一并消失。

## 取验证码

```bash
./otp.sh agmail1@example.test
```

只打印验证码本身。`DEV_MODE=true`（默认）时 AuthGear **不外发**邮件，整条正文写进容器
日志，`otp.sh` 从日志取；此时 Mailpit 收件箱始终是空的，它只是 `DEV_MODE=false` 时的兜底
（`otp.sh` 会自动回退到 Mailpit API，调用方无感知）。

注意：AuthGear 会把 SMTP 的 host / username / password 当敏感串，在日志里出现
即打码。测试用的邮箱域名和用户名不要和 `init-project.sh` 里那三个值重叠，否则
`otp.sh` 匹配不到 `recipient`。

## 清理

```bash
./down.sh      # 停容器，保留数据卷与 var/（下次起来还是同一个项目和同一批用户）
./down.sh -v   # 连数据卷、var/、out/ 一起删，回到干净状态
```

两者都会先停掉 `modeltaps-up.sh` 起的宿主机 Modeltaps 进程。

## 目录

- `docker-compose.yaml` — postgres / redis / mailpit / authgear，compose project 名 `modeltaps-ag-it`
- `postgres/Dockerfile` — 官方 postgres 镜像加装 `pg_partman`（AuthGear 的 audit log 迁移要用）
- `common.sh` — 端口、镜像、凭据等共用变量与工具函数
- `var/` — AuthGear 的 local_fs 配置源（`authgear.yaml` / `authgear.secrets.yaml`）
- `out/` — 编译产物、日志、pid、`modeltaps.env`
- `e2e/` — 基线用例：`baseline.mjs`（两种邮箱登录组合 + 同一账号两个邮箱）、`authgear-flow.mjs`（authflow v2 屏幕驱动）
- `cases/` — 安全边界用例：`security.mjs`（S1–S7）、`seclib.mjs`（共用工具）

`var/`、`out/`、`node_modules/` 都已 gitignore。

## 跑用例

```bash
cd ittest/authgear
pnpm install --filter modeltaps-authgear-ittest   # 只装 playwright（pnpm workspace，锁文件在仓库根目录）
pnpm exec playwright install chromium    # 首次还要下浏览器
node e2e/baseline.mjs                    # 基线三个用例（--gap 默认 90000ms，避开 OTP 冷却）
node cases/security.mjs --out out/security.json   # 安全九个用例，--only S2b,S7 可单跑
```

基线用例除了「注册 → 登录 → 退出 → 再登录仍是同一账号」，还核对退出走的是 RP-Initiated
Logout：登出接口下发 IdP 的结束会话地址，浏览器跳过去后回落本站 `/signed-out`。

安全用例里 S3（`preferred_username` 边界）依赖用户名标识，在这个只有邮箱的项目上按 SKIP
记录原因，不做判定。用例会临时改写 `var/app/authgear.yaml`（邮箱验证开关、给 client 追加一个
本地 `redirect_uri` 用来直接抓 id_token）与 Modeltaps 提供方的两个开关（`link_by_verified_email`、
`disable_auto_register`），跑完在 `finally` 里全部还原。
