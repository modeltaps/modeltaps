---
title: "环境变量"
layout: doc
outline: deep
lastUpdated: true
---

# 环境变量

::: warning 注意
环境变量优先级高于配置文件。
:::

本页面可能未及时更新，完整可配置键请以交付包根目录的配置文件示例 `config.example.yaml` 为准。

## 配置文件转环境变量

将配置文件中的变量名全部大写，遇到子集用下划线连接，例如：

```yaml
# 配置文件
user_token_secret: "<32 位以上的随机字符串>"
logs:
  filename: "modeltaps.log" # 日志文件名
```

转换为环境变量：

```bash
USER_TOKEN_SECRET="<32 位以上的随机字符串>"
LOGS_FILENAME="modeltaps.log"
```

## 环境变量说明

1. `REDIS_CONN_STRING`：设置之后将使用 Redis 作为缓存使用。
   - 例子：`REDIS_CONN_STRING=redis://default:redispw@localhost:49153`
   - 如果数据库访问延迟很低，没有必要启用 Redis，启用后反而会出现数据滞后的问题。
2. `SESSION_SECRET`：设置之后将使用固定的会话密钥，这样系统重新启动后已登录用户的 cookie 将依旧有效。
   - 取值请用 `openssl rand -base64 48 | tr -d '\n'` 生成强随机字符串，不要使用简单口令。
3. `SQL_DSN`：设置之后将使用指定数据库而非 SQLite，请使用 MySQL 或 PostgreSQL。
   - 例子（尖括号为占位符，请替换为实际值）：
     - MySQL：`SQL_DSN=<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps`
     - PostgreSQL：`SQL_DSN=postgres://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/modeltaps`（适配中，欢迎反馈）
   - 请为 Modeltaps 单独创建数据库账号并设置强密码，不要复用 `root` 等超级用户账号。
   - 注意需要提前建立数据库 `modeltaps`，无需手动建表，程序将自动建表。
   - 如果使用本地数据库：部署命令可添加 `--network="host"` 以使得容器内的程序可以访问到宿主机上的 MySQL。
   - 如果使用云数据库：如果云服务器需要验证身份，需要在连接参数中添加 `?tls=skip-verify`。
   - 请根据实际数据库配置修改下列参数（或者保持默认值）：
     - `SQL_MAX_IDLE_CONNS`：最大空闲连接数，默认为 `100`。
     - `SQL_MAX_OPEN_CONNS`：最大打开连接数，默认为 `1000`。
       - 如果报错 `Error 1040: Too many connections`，请适当减小该值。
     - `SQL_MAX_LIFETIME`：连接的最大生命周期，默认为 `60`，单位分钟。
4. `FRONTEND_BASE_URL`：设置之后将重定向页面请求到指定的地址，仅限从服务器设置。
   - 例子：`FRONTEND_BASE_URL=https://<主服务器域名>`
5. `MEMORY_CACHE_ENABLED`：启用内存缓存，会导致用户额度的更新存在一定的延迟，可选值为 `true` 和 `false`，未设置则默认为 `false`。
   - 例子：`MEMORY_CACHE_ENABLED=true`
6. `SYNC_FREQUENCY`：在启用缓存的情况下与数据库同步配置的频率，单位为秒，默认为 `600` 秒。
   - 例子：`SYNC_FREQUENCY=60`
7. `NODE_TYPE`：设置之后将指定节点类型，可选值为 `master` 和 `slave`，未设置则默认为 `master`。
   - 例子：`NODE_TYPE=slave`
8. `CHANNEL_UPDATE_FREQUENCY`：设置之后将定期更新渠道余额，单位为分钟，未设置则不进行更新。
   - 例子：`CHANNEL_UPDATE_FREQUENCY=1440`
9. `CHANNEL_TEST_FREQUENCY`：设置之后将定期检查渠道，单位为分钟，未设置则不进行检查。
   - 例子：`CHANNEL_TEST_FREQUENCY=1440`
10. `POLLING_INTERVAL`：批量更新渠道余额以及测试可用性时的请求间隔，单位为秒，默认无间隔。
    - 例子：`POLLING_INTERVAL=5`
11. `BATCH_UPDATE_ENABLED`：启用数据库批量更新聚合，会导致用户额度的更新存在一定的延迟可选值为 `true` 和 `false`，未设置则默认为 `false`。
    - 例子：`BATCH_UPDATE_ENABLED=true`
    - 若出现数据库连接数过多的问题，可以尝试启用该选项。
12. `BATCH_UPDATE_INTERVAL=5`：批量更新聚合的时间间隔，单位为秒，默认为 `5`。
    - 例子：`BATCH_UPDATE_INTERVAL=5`
13. 请求频率限制：
    - `GLOBAL_API_RATE_LIMIT`：全局 API 速率限制（除中继请求外），单 ip 三分钟内的最大请求数，默认为 `180`。
    - `GLOBAL_WEB_RATE_LIMIT`：全局 Web 速率限制，单 ip 三分钟内的最大请求数，默认为 `60`。
14. 编码器缓存设置：
    - `TIKTOKEN_CACHE_DIR`：默认程序启动时会联网下载一些通用的词元的编码，如：`gpt-3.5-turbo`，在一些网络环境不稳定，或者离线情况，可能会导致启动有问题，可以配置此目录缓存数据，可迁移到离线环境。
    - `DATA_GYM_CACHE_DIR`：目前该配置作用与 `TIKTOKEN_CACHE_DIR` 一致，但是优先级没有它高。
15. `RELAY_TIMEOUT`：中继超时设置，单位为秒，默认不设置超时时间。
16. `SQLITE_BUSY_TIMEOUT`：SQLite 锁等待超时设置，单位为毫秒，默认 `3000`。
17. `TG_BOT_API_KEY`：Telegram bot 的 API 密钥，可在 [BotFather](https://t.me/BotFather) 获取。
18. `TG_WEBHOOK_SECRET`：（可选）webhook 密钥，可自定义。设置该密钥后将使用 `webhook` 方式接收消息，否则使用轮询（Polling）方式。
19. `USER_TOKEN_SECRET` ： 设置用户令牌签名密钥，必填，大于 32 位以上， 设置后请勿修改，否则会导致用户令牌失效。
    - 取值请用 `openssl rand -base64 48 | tr -d '\n'` 生成，且与 `SESSION_SECRET` 使用不同的值。
20. `HASHIDS_SALT` ：Sqids 字母表，用于混淆用户令牌信息， 可空，如为空则使用默认字母表`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`，如设置，则需要保证字母表中无重复字符。
21. `AUTO_PRICE_UPDATES`：自动更新价格，可选值为 `true` 和 `false`，未设置则默认为 `false`。开启后每次启动程序时，会比对数据库中的数据与程序内置的默认模型价格，若数据库中的模型价格有缺失将自动同步到数据库中。需注意：开启后无法删除程序内置的默认模型价格，删除后重启会重新写入，该选项适用于价格与官方保持一致的场景。
22. `AUTO_PRICE_UPDATES_MODE`：价格更新模式，可选值为 `add`：仅增加系统不存在的价格；`overwrite`：覆盖系统所有价格配置；`update`：仅更新现有数据；`system`：使用程序内置价格表初始化价格配置，默认为 `system`。生产环境建议使用 `system` 模式，并在 Web 端价格管理模块手动获取价格更新服务数据后逐条核对更新。
23. `AUTO_PRICE_UPDATES_INTERVAL` ：价格自动更新时间，单位分钟，仅`AUTO_PRICE_UPDATES_MODE`为`add`、`overwrite`时生效，系统将按照此时间周期性从价格更新服务器获取价格配置并更新系统价格。默认值：1440
24. `UPDATE_PRICE_SERVICE` ：价格更新服务地址，设置之后将从该地址拉取价格数据更新价格。默认为空，为空时不启用外部价格服务（此时 `AUTO_PRICE_UPDATES_MODE` 请使用 `system`，即以程序内置价格表初始化）。
25. `USER_INVOICE_MONTH`：是否开启用户月度账单功能，开启后系统每月 1 日凌晨生成用户上一月度的数据汇总账单；数据量较大时资源消耗较高，请谨慎开启，默认 `false`。
26. `ROOT_PASSWORD` ：root 账号的初始密码，仅在数据库中还没有任何用户（即首次启动）时生效，长度需为 8–64 位，超出范围程序会启动失败。
    - 不设置时，程序会生成 16 位强随机密码，并仅在首次启动日志中打印一次。
    - 请在首次登录后立即修改 root 密码，并清理含有初始密码的启动日志。
    - 该变量**仅首次建库生效**，之后修改或删除都不影响已存在的 root 密码，不能当作逃生手段；关闭密码登录前请先读[管理员逃生口](./index.md#管理员逃生口)。
27. `ROOT_PASSWORD_ALLOW_INSECURE`：是否跳过 `ROOT_PASSWORD` 的 8–64 位长度校验，可选值 `true` / `false`，默认 `false`。**仅限本地开发环境，生产环境严禁开启。**
    - 本地开发约定：`ROOT_PASSWORD=root` + `ROOT_PASSWORD_ALLOW_INSECURE=true`，首次启动后即可用 `root/root` 登录。
    - 开启后启动日志会打印一行警告；未设置 `ROOT_PASSWORD` 时该开关不生效，仍走 16 位随机密码分支。

## 更多配置项

以下为上方未展开、但代码实际读取的常用配置项，完整列表与默认值请以交付包根目录的 `config.example.yaml` 为准。

### 服务器

- `PORT`：监听端口，默认 `3000`。
- `GIN_MODE`：Gin 运行模式，`release` / `debug`，默认 `release`。
- `HTTPS`：是否以 HTTPS 对外（影响 cookie `Secure` 等），默认 `false`。
- `TRUSTED_HEADER`：获取真实客户端 IP 的请求头，例如 `CF-Connecting-IP`。**仅当请求来自 `TRUSTED_PROXIES` 内的地址时才被采信**，否则忽略。
- `TRUSTED_PROXIES`：反向代理的 IP / CIDR 列表，逗号分隔（配置文件中写作 YAML 列表）。默认空 = 不信任任何代理，客户端 IP 一律取直连对端地址。详见下方[反向代理与真实客户端 IP](#反向代理与真实客户端-ip)。
- `SHUTDOWN_TIMEOUT`：优雅关闭超时，单位秒，默认 `30`。
- `PPROF_ENABLED`：是否开启 pprof 性能分析，默认 `false`。
- `LANGUAGE`：默认语言，默认 `zh_CN`。
- `FAVICON`：自定义 favicon 路径。
- `GITHUB_PROXY`：外部资源下载的代理前缀。

### 登录安全

- `LOGIN_MAX_FAILURES`：同一账号连续登录失败次数上限（跨来源 IP 统计），默认 `5`。达到后账号进入锁定，提示语与普通认证失败一致。
- `LOGIN_LOCKOUT_MINUTES`：账号锁定时长，单位分钟，默认 `15`。窗口从最近一次失败起算。
- `TURNSTILE_SESSION_MINUTES`：会话内 Turnstile 校验结果的有效期，单位分钟，默认 `10`。需覆盖"发送验证码 → 注册"两步流程。
- `SESSION_IDLE_DAYS`：登录会话空闲多少天没有任何请求即失效，默认 `7`（配置文件 `session.idle_days`）。
- `SESSION_MAX_DAYS`：登录会话最长有效天数，无论是否活跃，同时是会话 cookie 的 `Max-Age`，默认 `30`（配置文件 `session.max_days`）。

> 启用 Redis 时失败计数在多副本间共享；未启用 Redis 时为进程内计数，多副本下实际阈值会按副本数放大。

> 登录方式（账号体系、本站账号开关、社交登录、OIDC 提供方）不通过环境变量配置，统一在后台
> 「系统设置 → 登录方式」管理。账号体系二选一见下方 [账号体系](#账号体系)，提供方字段、回调地址与
> 账号关联规则见 [OIDC 登录提供方](#oidc-登录提供方)。

### 数据库与 Redis

- `SQLITE_PATH`：SQLite 数据库文件路径，默认 `modeltaps.db`。
- `BRAND_ICON_DIR`：品牌图标同步层的本地目录，每个版本存为 `{目录}/{version}/`，默认 `data/brand-icons`。
- `REDIS_DB`：Redis DB 序号（连接串未含 DB 时生效），默认 `0`。
- `REDIS_POOL_SIZE`：连接池大小，默认 `100`。
- `REDIS_MIN_IDLE_CONNS`：最小空闲连接数，默认 `10`。
- `REDIS_POOL_TIMEOUT` / `REDIS_READ_TIMEOUT` / `REDIS_WRITE_TIMEOUT`：连接池等待 / 读 / 写超时，单位秒，默认 `5` / `2` / `2`。

### HTTP 客户端 / 中继超时

- `CONNECT_TIMEOUT`：连接超时，单位秒，默认 `5`。
- `RELAY_REQUEST_TIMEOUT`：中继单次请求超时，单位秒，默认 `300`。
- `STREAM_IDLE_TIMEOUT`：流式空闲超时，渠道流式响应期间每收到数据即重置计时，静默超过该时长则中止流（与墙钟总超时互补，用于精准处理"卡死流"），单位秒，默认 `300`，设为 `0` 禁用。
- `RESPONSE_HEADER_TIMEOUT`：响应头超时，单位秒，默认 `120`。
- `TLS_HANDSHAKE_TIMEOUT`：TLS 握手超时，单位秒，默认 `30`。
- `TLS_INSECURE_SKIP_VERIFY`：跳过 TLS 证书校验（不安全），默认 `false`。
- `MAX_CONNS_PER_HOST` / `MAX_IDLE_CONNS` / `MAX_IDLE_CONNS_PER_HOST`：连接数上限，默认 `0`（不限）/ `1000` / `200`。

### 价格 / 计费

- `CATALOG_PRICING_URL`：模型目录价格同步源地址。
- `CATALOG_PRICING_AUTO_SYNC`：是否自动同步目录价格，默认 `true`。
- `CHANNEL_PRICING_AUTO_SYNC`：是否自动同步渠道价格，默认 `true`。
- `UNPRICED_MODEL_POLICY`：未定价模型策略，默认 `block`。
- `UNPRICED_MODEL_DEFAULT_RATIO`：未定价模型默认倍率，默认 `30.0`。
- `MODEL_DRIFT_AUTO_CHECK`：是否自动检测模型漂移，默认 `false`。

### 日志 / 指标 / 集成

- `LOG_DIR`：日志目录，默认 `./logs`。
- `LOG_LEVEL`：日志级别，`debug` 输出更详细。
- `LOGS_FILENAME` / `LOGS_MAX_SIZE` / `LOGS_MAX_AGE` / `LOGS_MAX_BACKUP` / `LOGS_COMPRESS`：日志文件名及轮转设置，默认 `modeltaps.log` / `100`(MB) / `7`(天) / `10` / `false`。
- `METRICS_USER` / `METRICS_PASSWORD`：`/metrics` 接口 Basic Auth，留空则不鉴权。
- `MCP_ENABLE`：是否启用 MCP，默认 `false`。
- `UPTIME_KUMA_ENABLE` / `UPTIME_KUMA_DOMAIN` / `UPTIME_KUMA_STATUS_PAGE_NAME`：Uptime Kuma 状态页集成。
- `DISABLE_TOKEN_ENCODERS`：禁用本地 token 编码器（改用估算），默认 `false`。
- `TG_HTTP_PROXY`：访问 Telegram 的 HTTP/SOCKS5 代理。

### 通知 / 存储 / 搜索

通知（`NOTIFY_*`）、对象存储图床（`STORAGE_*`）、联网搜索（`SEARCH_*`）等分组配置项较多，键名与默认值请直接参考交付包根目录的 `config.example.yaml`。

## 账号体系

后台「系统设置 → 登录方式」顶部是账号体系二选一（option `AccountSystem`，经 `/api/status` 的
`account_system` 公开下发）。两种模式互斥，第三方登录、手机号、两步验证只在**管账号的那一方**配置，
不会两边都配：

| 模式 | 值 | 谁管账号 | `/login` |
| --- | --- | --- | --- |
| 内置账号（默认） | `builtin` | 本站：密码、邮箱验证码、通行密钥，可搭配 GitHub / 微信 / 飞书 / LinuxDo 社交登录 | 本站登录表单 |
| 外部身份提供方 | `external` | 标记为「作为本站账号登录」的 OIDC 提供方（Authgear、Keycloak、Logto、Auth0 等） | 浏览器导航直接 302 到提供方登录页，本站没有任何中间页 |

- **外部模式下**本站的密码登录、注册、找回密码、邮箱验证码、社交登录与通行密钥（非 root）一律关闭，
  不管这几组后台开关怎么填；只有 root 保留 [管理员逃生口](./index.md#管理员逃生口) `/login/admin`
  （后台开关 `AdminLoginEnabled`，默认开）。直接访问 `/register`、`/reset` 一律并入 `/login`。
- **内置模式下** OIDC 提供方不参与登录：即便表里还有启用的行，授权与回调请求都会被拒绝，登录页也不
  渲染提供方按钮。提供方是账号体系，不是第三方登录；想用它登录就切到外部模式。
- 切换到外部模式前，后台会校验已经存在一个启用且标记为「本站身份」的提供方；只启用了一个提供方时
  不标记也会被当作本站身份，启用了多个则必须恰好标记一个，否则 `/login` 只显示「尚未配置身份提供方」。
- 切回内置模式前，请先在「登录方式 → 本站账号」确认至少开着一种登录方式；由提供方建号的用户没有
  本站密码，要走找回密码。

`/api/status` 下发的登录开关都是按账号体系推导后的**有效值**，前端只看这些字段：

| 字段 | 含义 |
| --- | --- |
| `account_system` | `builtin` / `external` |
| `password_login` / `password_register` | 内置模式取后台开关，外部模式恒 `false` |
| `email_code_login` | 内置模式 + 后台「允许通过邮箱验证码登录」（`EmailCodeLoginEnabled`）+ 已配置 SMTP，否则 `false` |
| `passkey_login` | 内置模式取后台「允许使用通行密钥」（`PasskeyLoginEnabled`），外部模式恒 `false` |
| `github_oauth` / `wechat_login` / `lark_login` / `linuxDo_oauth` | 内置模式取各自开关，外部模式恒 `false` |
| `admin_login_enabled` | 外部模式 + `AdminLoginEnabled`，内置模式恒 `false` |
| `oidc_providers[]` | 外部模式下承担本站身份的那**一个**提供方（含 `first_party` 与四个设置页深链），内置模式为空数组 |

### 邮箱验证码登录

内置模式独有。开启后登录页在密码之外多一个「用邮箱验证码登录」入口：`POST /api/user/login_code`
发码（同一邮箱 60 秒内只发一次，未绑定该邮箱的地址静默不发，不暴露账号是否存在）、
`POST /api/user/login/code` 验码登录。验证码错误计入与密码登录共用的账号锁（`LOGIN_MAX_FAILURES` /
`LOGIN_LOCKOUT_MINUTES`）。用户可在「设置 → 登录与安全 → 登录验证」自行关闭本人的验证码登录。

### 登录会话

登录态存在表 `user_sessions`，cookie（HTTPS 下 `__Host-modeltaps_session`，HTTP 下 `modeltaps_session`）
只存会话键。空闲超过 `SESSION_IDLE_DAYS` 或距登录超过 `SESSION_MAX_DAYS` 即失效；用户可在
「设置 → 登录与安全 → 登录会话」查看各设备并单独结束或「退出其他设备」；修改 / 重置密码、封禁、
删号、删除提供方会自动结束相应会话。经提供方登录的会话把 `id_token` 存在会话行上，退出时用于
结束 IdP 侧会话（见下方[退出登录](#退出登录)）。

cookie 改名后不再与同域名部署的身份提供方互相覆盖（Authgear 等也用名为 `session` 的 cookie）。

## OIDC 登录提供方

OIDC 登录提供方存在数据库里，由管理员在后台「系统设置 → 登录方式 → OIDC 登录提供方」增删改，
不需要重启，保存后立即生效。任何支持 OIDC 的服务都可以（Authgear、Keycloak、Logto、Auth0、Okta 等）。
提供方只在[外部身份提供方](#账号体系)模式下参与登录：承担登录的那一个标记为「作为本站账号登录」，
`/login` 直接跳到它的登录页；其余启用的提供方只保留在库里，不出现在登录页。

### 字段说明

| 表单字段 | 提交字段 | 说明 |
| --- | --- | --- |
| 标识（slug） | `slug` | 只能是小写字母、数字与连字符，长度 1–32，全站唯一。进 URL，**创建后不可修改** |
| 重定向 URL | — | 只读，由 `slug` 与「服务器地址」拼出，需登记到 IdP 侧 |
| 显示名 | `display_name` | 登录按钮文案，留空时回落为 `slug` |
| OIDC 发行者（Issuer） | `issuer` | 必填，须为 `https://` URL（仅 `localhost` / `127.0.0.1` / `::1` 允许 `http`），不能带查询参数或片段，末尾 `/` 会被去掉 |
| 客户端 ID | `client_id` | 必填 |
| 客户端密钥 | `client_secret` | 创建时填写。编辑时**留空表示不修改**；接口任何情况下都不回显密钥 |
| 权限范围（Scopes） | `scopes` | 必填且必须包含 `openid`，逗号或空格分隔，默认 `openid,email,profile` |
| 用户名 Claim | `username_claim` | 留空按 `preferred_username` |
| 显示名 Claim | `display_name_claim` | 留空按 `name` |
| 头像 Claim | `avatar_claim` | 留空按 `picture` |
| 排序 | `sort` | 数字越小越靠前，默认 `0` |
| 按已验证邮箱关联已有账号 | `link_by_verified_email` | 见下方[账号关联规则](#账号关联规则)，默认关闭 |
| 按已验证手机号关联已有账号 | `link_by_verified_phone` | 见下方[账号关联规则](#账号关联规则)，默认关闭 |
| 禁止首登自动建号 | `disable_auto_register` | 开启后未关联到已有账号的首次登录被拒绝，不建号也不写身份行；迁移期专用，见下方「从密码登录迁移到 Authgear / OIDC」，默认关闭 |
| 作为本站账号登录 | `first_party` | 外部模式下承担 `/login` 直达与账号安全页跳转的那一个提供方；只启用一个提供方时可不标记，启用多个时必须恰好标记一个。`/api/status` 的 `oidc_providers[]` 只下发这一个，默认关闭 |
| 账号设置页地址 | `account_settings_url` | 提供方侧的账号总设置页，须为 `https://` 绝对 URL；留空表示没有。外部模式下账号安全页用它给出「在身份提供方管理」外链 |
| 修改密码页地址 | `password_url` | 提供方侧改密页深链，账号安全页「密码」行按此跳转；留空则不显示该行 |
| 两步验证页地址 | `mfa_url` | 提供方侧 MFA 设置页深链，账号安全页「两步验证」行按此跳转；留空则不显示该行 |
| 通行密钥页地址 | `passkey_url` | 提供方侧通行密钥设置页深链，账号安全页「通行密钥」行按此跳转；留空则不显示该行 |
| 第三方登录与账号标识页地址 | `identity_url` | 提供方侧身份 / 账号标识设置页深链，账号安全页「账号标识」卡的「邮箱」「手机号」「第三方账号」各行按此跳转；留空则不显示 |
| 启用 | `enabled` | 关闭后该提供方不再承担登录，回调也会被拒绝；外部模式下唯一的本站身份提供方被停用后 `/login` 只显示「尚未配置身份提供方」，root 走 `/login/admin` |

四个设置页深链以 Authgear 为例分别是 `https://<authgear 域名>/settings/change_password`、
`/settings/mfa`、`/settings/passkey`、`/settings/identity`；Keycloak 账号控制台对应
`https://<keycloak>/realms/<realm>/account/#/security/signingin`（密码、MFA、通行密钥同一页，四个字段可填同一地址）
与 `.../account/#/personal-info`（邮箱、手机号等账号标识）。

三个 claim 名留空即按 OIDC 标准值兜底，因此只下发标准 claim 的 IdP 开箱可用。取到的值不是
字符串（或 claim 缺失）时按空处理：用户名会退回系统兜底用户名，显示名与头像留空。

### 回调地址

| 位置 | 地址 |
| --- | --- |
| 前端回调页（登记到 IdP 的 `redirect_uri`） | `{ServerAddress}/oauth/oidc/{slug}` |
| 后端接口（前端页面转调，无需登记） | `{ServerAddress}/api/oauth/oidc/{slug}` |
| 退出落地页（登记到 IdP 的 `post_logout_redirect_uri`） | `{ServerAddress}/signed-out` |

**登记到 IdP 的重定向 URI 是前端回调页**，也就是编辑抽屉里「重定向 URL」只读框显示的那一条；
前端页面拿到 `code` / `state` 后再调后端 `/api/oauth/oidc/{slug}` 完成换取与登录。

`ServerAddress` 取后台「系统设置 → 通用」里配置的服务器地址，授权请求里的 `redirect_uri`
就由它拼出，所以它必须与 IdP 侧登记的一致。

`slug` 为 `oidc` 时（存量单提供方升级后就是这个 slug）是唯一的例外：后端发出的 `redirect_uri` 是
**`{ServerAddress}/oauth/oidc`**——去掉了末尾的 `/oidc`，这样存量部署升级后不用改 IdP 侧的
重定向白名单。后台表单里的只读框同样显示这条地址。其余 slug 一律用 `/oauth/oidc/{slug}`。

### 退出登录

经 OIDC 登录的会话退出时，除了清掉本站会话，还会按 RP-Initiated Logout 把浏览器整页跳到该提供方
的 `end_session_endpoint`（由 discovery 自动发现，IdP 未声明该端点时退出就只清本站会话），带上
`id_token_hint` 与 `post_logout_redirect_uri={ServerAddress}/signed-out`，由 IdP 结束会话后回到本站
的已退出页。

::: warning 必须在 IdP 登记 Post Logout Redirect URI
必须在 IdP 侧把 `{ServerAddress}/signed-out` 登记为 Post Logout Redirect URI，否则退出时 IdP 拒绝
回跳，用户会停在 IdP 的报错页上。另外，退出请求不带 `id_token_hint` 时 Authgear 会显示一个确认页，
要用户再点一次才登出；本站一律携带 `id_token_hint`，所以正常配置下是静默登出。
:::

不走 IdP 的会话（密码登录等）退出后直接落到本站 `/signed-out`，不做任何外跳。

### 账号关联规则

回调后按以下顺序关联账号。各提供方互不干扰：同一个 `sub` 在不同提供方是两个独立身份。

1. 按「提供方 + `id_token` 的 `sub`」查已绑定身份，命中即登录（账号被封禁时拒绝）。
2. 未命中时，仅当该提供方开启了「按已验证邮箱关联」，且 `id_token` 同时携带 `email` 与
   `email_verified`（布尔 `true` 或字符串 `"true"`），且该邮箱归一化后唯一对应一个已有账号
   （排除组织影子记账账户），**且该账号在本站的邮箱本身也是已验证状态**时，才把该身份写入
   该账号并登录。「已验证」是双向要求：IdP 侧验证过还不够，站内那个邮箱也得有可信来源——
   邮箱验证码注册 / 绑定、或此前由 IdP 的已验证 claim 写入才算，管理员在后台用户编辑页直接
   代填的邮箱不算（那里不发验证码），否则在任意账号上填一个别人的邮箱就能把它变成登录入口。
   账号安全页「账号标识」卡上邮箱行的「已验证 / 未验证」角标显示的就是这个状态。
   站内邮箱未验证时按「未命中」处理，继续走第 3 步。若该账号在此提供方下已绑定其它身份，
   则拒绝自动关联并提示用户先登录后在账号绑定页手动绑定。

   升级到带该判定的版本时，存量非空邮箱一律按已验证回填（历史上没有记录来源，绝大多数出自
   验证码或 IdP，不能让全站用户重新验证）；此后写入的邮箱才按真实来源区分。
3. 邮箱未命中时，同样的规则再按手机号走一遍：仅当该提供方开启了「按已验证手机号关联」，
   且 `id_token` 同时携带 `phone_number` 与 `phone_number_verified`（布尔 `true` 或字符串
   `"true"`），且该号码去掉首尾空白后唯一对应一个已有账号时才关联。**邮箱优先于手机号**：
   两者都命中且指向不同账号时按邮箱关联。
4. 以上都不成立时：该提供方若开启了「禁止首登自动建号」，则直接拒绝——不建号、不写身份行，
   回调返回 `OIDC_REGISTER_DISABLED:` 前缀的提示；开关关闭（默认）时按新用户注册，
   站点未开放注册时返回 `OIDC_REGISTER_CLOSED:` 前缀的提示。新账号用户名取用户名 Claim 去掉
   `@` 及其后部分、再截到 12 个字符；结果为空或与已有用户名重复时回落为 `oidc_<序号>`。
   注册时只有已验证且未被占用的邮箱 / 手机号才会回填到新账号，否则新账号不带邮箱，需用户自行在
   个人设置中绑定并验证。

已登录用户访问回调时走绑定：先校验会话对应的账号仍存在（未被删除）且未被封禁，不满足则直接
拒绝、不写任何绑定；通过后该身份未被占用则绑定到当前账号，已被其它账号占用则拒绝。

删除用户时会一并删除其 OIDC 身份，该 IdP 账号可重新注册。

关联成功（含注册与绑定）后，IdP 下发的已验证邮箱 / 手机号按提供方身份分两档写回本站账号：
**标记了「作为本站账号登录」（`first_party`）的提供方以 IdP 为准**——本地为空、或与 IdP 下发的值
不同，都写入本站，用户在 IdP 侧改了邮箱或手机号，下次登录本站就跟着更新；**其余提供方只回填、
不覆盖**——仅本地为空时写入，本地已有值一律保留，避免第三方静默改掉本站的账号标识。两档都不抢
已归属其它账号的值（跳过并记日志，只记 user_id 与提供方 slug，标识明文不入日志）；IdP 未下发该
claim、或 claim 未标记已验证时，本地已有值也不会被清空。

::: warning 「按已验证邮箱 / 手机号关联」是账号接管面
开启后，任何能在该 IdP 侧把邮箱（或手机号）验证成站内已有账号标识的人，都能直接登入那个站内账号。
只有当 IdP 的标识归属完全由你掌控（企业目录、内部 IdP）时才开启；面向公开注册的 IdP 请保持关闭，
让用户自己登录后在个人设置里绑定。

上面第 2、3 步的「已绑定其它身份则拒绝」封住了「IdP 侧标识被释放后由他人重新验证」的接管路径：
原主体已在该账号留下身份行，新主体再拿同一标识登录会被拒绝并要求手动绑定。但对**从未绑定过
该提供方**的账号（例如纯用户名/密码老账号），按已验证标识关联仍是接管面，上述前提依然成立。

`email_verified` / `phone_number_verified` 非真值（含缺失）的标识一律视为未验证，
既不参与关联也不回填。

本站生产在满足上述前提的情况下仍然开启了按已验证邮箱关联，理由与回滚方式见下方
[生产为什么开启按已验证邮箱关联](#生产为什么开启按已验证邮箱关联)。
:::

::: danger 永远不按用户名 claim 关联
系统**不会**用「用户名 Claim」去匹配已有账号，该 claim 只用于新账号的用户名。
它由 IdP 侧自由设置，一旦用于关联，任何能在 IdP 注册的人都可以靠改同名用户名接管站内账号。
:::

#### 生产为什么开启按已验证邮箱关联

Modeltaps 自己的生产部署在本站身份提供方上开启了 `link_by_verified_email`。上面的警告块讲的是这个
开关的风险，这里记录为什么在满足前提时仍然值得开，以及关掉它要付出什么。

- **理由**：同一个人无论用密码、邮箱验证码、第三方登录还是身份提供方进来，只要邮箱验证过就落到
  同一个账号。这正是账号安全页「账号标识」卡对用户的承诺——卡上每一项都指向这个账号，用其中任何
  一个登录进来的都是同一个账号。关掉这个开关，这句话对「首次经 IdP 登录的老账号」就不成立。
- **前提**：IdP 必须真正验证邮箱（`email_verified` 只在验证流程走完后为真），且邮箱归属完全由运维
  掌控——企业目录、内部 IdP、管理员导入或邀请制。面向公开注册的 IdP 不满足这个前提：任何人都能在
  那边注册并验证一个站内已有邮箱，此时必须保持关闭，只走用户自己登录后手动绑定。
- **关掉的后果**：同一邮箱的 IdP 主体首次登录时会另建一个新账号，额度、令牌、日志与组织归属随之
  分裂成两份，而本站没有账号合并工具，只能由管理员逐个手工处理。开关只影响此后的**首次**关联：
  已经写入的身份行不受影响，已关联的账号照常登录。
- **回滚**：在后台「系统设置 → 登录方式 → OIDC 登录提供方」里关掉该提供方的这个开关，保存后立即
  生效，不需要重启，也不涉及任何数据迁移；此前自动关联出来的身份行继续有效。

### 连通性测试、停用与删除

- **保存时自动发现**：新建 / 编辑保存时会按 `issuer` 跑一次标准 OIDC discovery（超时 10 秒）。
  失败时保存被拒并原样返回错误摘要；确需先落库可点「仍保存为停用」，此时记录一律以
  `enabled=false` 落库，不会把不可用的提供方挂到登录页上。
- **测试连通性**：列表里的「测试连通性」按已保存的 `issuer` 重跑一次 discovery，用来排查
  IdP 侧变更（换域名、证书过期、内网不可达）。
- **停用**：切换状态不做 discovery，IdP 已经完全不可达时也能停。停用只是摘掉登录入口与回调，
  身份绑定关系保留，重新启用后用户照常登录。
- **删除**：该提供方下**只要还有已绑定的用户身份就会被拒绝**（HTTP 409，提示还剩多少个身份）。
  删除会连带清掉这些绑定、相关用户再也无法用该 IdP 登录。排障请一律用停用；确需删除，
  先让用户各自解绑。

### 示例：对接 Authgear

Authgear 下发标准 claim，三个 claim 名保持留空即可。

1. Authgear Portal → **Applications** → **Add Application**，类型选 **Web app**（有后端、可保存密钥）。
2. **Redirect URI** 填 `https://<你的域名>/oauth/oidc/authgear`（`authgear` 就是下一步要填的 slug）。
3. 同一页的 **Post Logout Redirect URI** 填 `https://<你的域名>/signed-out`。不登记这条，退出时
   Authgear 会拒绝回跳，用户停在 Authgear 的报错页上。
4. 记下应用的 **Client ID** 与 **Client Secret**，以及 Authgear 项目的 endpoint
   （形如 `https://<project>.authgear.cloud`，即 issuer）。
5. 回到 Modeltaps 后台「系统设置 → 登录方式 → OIDC 登录提供方 → 新建」，填：
   - 标识（slug）：`authgear`
   - 显示名：`Authgear`
   - OIDC 发行者：第 4 步的 endpoint
   - 客户端 ID / 客户端密钥：第 4 步的两个值
   - 权限范围：`openid,email,profile`
   - 三个 claim 名：留空（用 `preferred_username` / `name` / `picture`）
6. 确认表单里的「重定向 URL」与第 2 步登记的完全一致，保存。保存即触发 discovery，成功说明
   issuer 可达。
7. Authgear Portal → **UI Settings → Language** 启用 `zh-CN`、`zh-HK`、`en`、`ja`。本站跳转时会带上
   当前界面语言（`ui_locales`），未启用的语言 Authgear 会回落到 Primary Language。
8. 退出登录，在登录页点「使用 Authgear 登录」验证整条链路；再退出一次，确认浏览器经 Authgear
   跳回本站 `/signed-out`（未登记第 3 步的地址时这一步会停在 Authgear 报错页）。

#### 多个产品共用同一个 Authgear 项目

一个 Authgear 项目就是**一套用户目录加一套会话**，项目下可以建多个 Application，一个产品用一个。
多产品共用一个项目时要清楚哪些东西是项目级的、哪些是应用级的。

- **用户与会话是项目级的**：登录方式、验证策略、用户目录只有一份。用户在 A 产品登录后再访问
  B 产品，浏览器带着同一份项目会话过去，授权页通常直接放行，不用再输一次密码。
- **一个产品一个 Application**：各自有独立的 Client ID / Client Secret 与重定向 URI 白名单
  （参见 `ittest/authgear/init-project.sh` 里 `oauth.clients` 数组的写法，一个产品一个条目），
  轮换或泄露互不牵连。本站后台只登记属于自己的那一对。
- **issuer 与 `sub` 是同一份**：各产品填的 OIDC 发行者都是同一个项目 endpoint，同一个人在各产品
  拿到的 `sub` 也是同一个值。本站身份行按 `user_oidc_identities.subject` 认人，所以用户在别的
  产品注册过并不会让他在本站已有账号——首登仍按[账号关联规则](#账号关联规则)走关联或建号——但一旦
  绑定，这条身份行锁定的就是那个跨产品唯一的主体。
- **退出登录会波及其它产品**：本站退出走 RP-Initiated Logout，IdP 结束的是**项目级会话**，
  同项目下其它产品的登录态随之失效，下次访问要重新登录。只想退本站时别走这条链路。
- **界面语言是项目级设置**：`UI Settings → Language` 的启用列表对项目下所有 Application 生效，
  某个产品想单独多支持一种语言，只能在项目级一起开。

### 示例：对接 Authany

Authany（`authany.com`）是托管版 Authgear，Portal 里的每一步都与上面的 Authgear 示例相同——
建 Web app、登记两个回跳地址、记下 Client ID / Secret、在本站后台新建提供方、启用界面语言。
本站生产（`modeltaps.com`）用的就是它，下面只记与通用写法不同的四处，其余照上面的 8 步做。

1. **issuer 是自定义域名**。「OIDC 发行者」填 `https://auth.modeltaps.com`：项目的自定义域名，
   CNAME 指向默认域名 `modeltaps.authanyid.com`（默认域名现在会 307 跳到自定义域名）。discovery、
   授权与 `end_session_endpoint` 全部走这个域名。

   ::: warning 自定义域名与本站同父域时，`http.cookie_domain` 必须是主机名
   Authgear 的会话 Cookie 名为 `session`，域取项目配置 `http.cookie_domain`。自定义域名激活时
   这个值可能被派生成 eTLD+1，`auth.modeltaps.com` 就会得到 `Domain=modeltaps.com`，浏览器把它一并发给
   本站，与本站同名 Cookie 相互覆盖，OIDC 回调会间歇性报 `state is empty or not same`。
   激活域名后务必把该项目的 `http.cookie_domain` 显式设为主机名本身（这里是 `auth.modeltaps.com`），
   本站生产就是这么配的。默认域名（`*.authanyid.com`）建项目时即为主机名，没有这个问题。
   :::

2. **三个提供方开关按下表打开**（其余保持默认关闭）。

   | 开关 | 生产取值 | 为什么 |
   | --- | --- | --- |
   | 作为本站账号登录（`first_party`） | 开 | Authany 是本站的第一方 IdP，由它承担 `/login` 直达与账号安全页的深链跳转 |
   | 按已验证邮箱关联已有账号（`link_by_verified_email`） | 开 | 同一个人无论此前用什么方式进来，只要邮箱已验证就落到同一个账号；前提与回滚见[生产为什么开启按已验证邮箱关联](#生产为什么开启按已验证邮箱关联) |
   | 禁止首登自动建号（`disable_auto_register`） | 开 | 迁移期不接受 IdP 侧新主体自动建号，未关联到已有账号的首登一律拒绝 |

3. **四个设置页深链指向 `https://auth.modeltaps.com/settings` 下的子页**，账号安全页按它们外跳：

   - 账号设置页地址（`account_settings_url`）：`https://auth.modeltaps.com/settings`
   - 修改密码页地址（`password_url`）：`https://auth.modeltaps.com/settings/change_password`
   - 两步验证页地址（`mfa_url`）：`https://auth.modeltaps.com/settings/mfa`
   - 通行密钥页地址（`passkey_url`）：`https://auth.modeltaps.com/settings/passkey`
   - 第三方登录与账号标识页地址（`identity_url`）：`https://auth.modeltaps.com/settings/identity`

4. **两个回跳地址按 slug `authany` 登记在 Authany 侧的应用上**：重定向 URI
   `https://modeltaps.com/oauth/oidc/authany`，Post Logout Redirect URI `https://modeltaps.com/signed-out`。
   两者都要与本站「服务器地址」拼出的结果完全一致，后者缺失时退出会停在 IdP 的报错页。

### 示例：对接 Logto

Logto 在 `profile` scope 下同时下发标准 claim（`preferred_username`、`name`、`picture`）与自有的
`username` claim。用标准 claim 即可，但如果你要求站内用户名与 Logto 用户名严格一致，可把
「用户名 Claim」改成 `username`。

1. Logto Console → **Applications** → **Create application**，类型选 **Traditional web**。
2. **Redirect URIs** 填 `https://<你的域名>/oauth/oidc/logto`。
3. 在应用的 **Settings** 里记下 **App ID**、**App secrets**，以及 **Endpoint**
   （形如 `https://<tenant-id>.logto.app`，即 issuer）。
4. Modeltaps 后台新建提供方：
   - 标识（slug）：`logto`
   - 显示名：`Logto`
   - OIDC 发行者：第 3 步的 Endpoint
   - 客户端 ID：App ID；客户端密钥：App secret
   - 权限范围：`openid,email,profile`（`email` 决定是否下发 `email` / `email_verified`，
     `profile` 决定是否下发 `preferred_username` / `name` / `picture` / `username`）
   - 用户名 Claim：留空用 `preferred_username`，或填 `username` 取 Logto 用户名
   - 显示名 Claim / 头像 Claim：留空
5. 保存后用「测试连通性」确认 discovery 通过，再到登录页实测一次。

> Logto 的 `preferred_username` 在用户未单独设置时会回落为其 username，所以留空也能拿到可用值。
> 若用户在 Logto 侧既没有 username 也没有 preferred_username，注册出来的账号会使用系统兜底用户名，
> 用户可自行改名。

### 从密码登录迁移到 Authgear / OIDC

把已有的密码用户迁到 Authgear（或任意 OIDC 提供方）时，最大的坑是没绑邮箱、或邮箱与 IdP 不一致的
老用户直接点 OIDC 登录，会被当成新用户建出一个空账号，而系统没有账号合并工具。按下面的顺序做可以
避免这种孤岛账号。

#### 推荐迁移顺序

1. 保留密码登录，配置 Authgear 提供方并开启「按已验证邮箱关联」。
2. 开启「禁止首登自动建号」（迁移期）。
3. 通知用户：用原方式登录 → 个人设置 → 账号绑定 → 绑定 Authgear。
4. 管理后台核查「无 OIDC 身份的活跃用户」直到归零（或可接受）。
5. 在「登录方式」把账号体系切换到**外部身份提供方**（切换前先读[管理员逃生口](./index.md#管理员逃生口)，
   给 root 设好应急密码与通行密钥）；如需开放新用户注册，再关闭「禁止首登自动建号」。

::: warning 第 1 步的前提：先切换账号体系再谈「保留密码登录」是不成立的
内置模式下提供方不参与登录，所以「保留密码登录 + 让用户点 OIDC 按钮绑定」这一步在现版本要换个做法：
迁移期先在**外部模式**下让用户经提供方登录，靠「按已验证邮箱关联」自动并到老账号，或先关「禁止首登自动建号」
让绑定发生在提供方一侧；老用户没有邮箱或邮箱不一致的，由管理员在用户页核对后手动处理。
:::

切换到外部模式后，浏览器导航到 `/login`（请求头 `Accept` 含 `text/html`）由服务端直接回 302 到 IdP 授权页，
不会先返回本站登录页或「正在跳转」的过渡页；非浏览器请求（如 `curl` 默认的 `Accept: */*`）仍照常返回前端页面。
本站的密码、注册、找回、社交登录与通行密钥（非 root）随之关闭，`/register`、`/reset` 并入 `/login`；
`/login/admin` 为 root 保留管理员登录表单。这些状态经 `/api/status` 的 `account_system`、`password_login`、
`password_register` 等有效值下发，旧前端缓存刷新后即生效。

退出登录不受影响：登出接口会下发 IdP 的结束会话地址，浏览器跳过去清掉 IdP 侧的 SSO 会话后回本站
`/signed-out`，不会被立即登回。IdP 不可达时 root 从 `/login/admin` 登录后在「登录方式」切回内置账号，
或用 root 的 access token 调 `PUT /api/option/` 把 `AccountSystem` 写回 `builtin`。

第 1 步开启「按已验证邮箱关联」是为了让邮箱一致的老用户零操作完成迁移，但它同时是账号接管面，
前提与限制见上方[账号关联规则](#账号关联规则)里的警告块；面向公开注册的 IdP 请保持关闭，
只靠第 3 步的手动绑定迁移。

#### 「禁止首登自动建号」的语义

逐提供方的开关（提交字段 `disable_auto_register`），默认关闭，关闭时行为与开关引入前完全一致。

开启后，回调走完[账号关联规则](#账号关联规则)前两步仍未关联到任何已有账号时直接拒绝：
不建用户、不写身份行，响应以 `OIDC_REGISTER_DISABLED:` 前缀返回，登录页据此展示引导文案。

开关只作用于**未登录**状态下的首次登录。已登录用户在个人设置里绑定该提供方走的是绑定分支，
不受影响——这正是迁移期要用户走的路径。

#### 回调失败的错误码约定

回调失败时后端只下发稳定错误码前缀 + 一句非技术性说明，前端按前缀匹配决定展示与出口：

| 前缀 | 触发条件 |
|---|---|
| `OIDC_LINK_CONFLICT:` | 已验证邮箱命中的账号在该提供方下已绑定另一个 `sub` |
| `OIDC_REGISTER_DISABLED:` | 该提供方开启了「禁止首登自动建号」 |
| `OIDC_REGISTER_CLOSED:` | 站点未开放新用户注册 |

冒号后的说明按提供方的「本站身份」（`first_party`）开关调整：开启时不出现提供方显示名、`slug`
与「OIDC」字样；关闭时保留显示名（未配显示名则退回 `slug`）。收尾建议按密码登录开关调整：
密码登录开启时提示「先用原方式登录，再在『账号绑定』页手动绑定」，关闭时站内已无其它登录方式，
改为提示联系管理员。

#### 核查「无 OIDC 身份的活跃用户」

管理后台「用户」页顶部有三张核查卡片：活跃用户总数、已绑 OIDC、未绑 OIDC；
对应接口是 `GET /api/user/oidc_coverage`（仅管理员），另按提供方返回各自已绑人数。

口径：**活跃用户** = 未删除 + 已启用 + 非组织影子账户；**已绑**只计**启用中**的提供方——
只绑了已停用提供方的用户计入未绑。

同一页的筛选条里有「登录方式 → 未绑 OIDC」（`login_method=no_oidc`），用来列出还没迁的人，
可与关键字、状态等既有筛选叠加。注意列表筛选沿用列表自身的语义，不预先排除封禁用户，
需要严格对齐卡片口径时再叠加一个状态筛选。

#### 手机号字段

`users.phone_number` 与邮箱同等对待，是**可关联标识**：列上有唯一索引 `idx_users_phone_unique`，
一个号码最多对应一个账号；空号码以 `NULL` 落库，不参与唯一约束，因此任意多个无手机号的账号可共存。
开启了「按已验证手机号关联」的提供方可以据此把 IdP 身份关联到已有账号（见上方
[账号关联规则](#账号关联规则)）。

来源是 IdP 下发的 `phone_number` claim：只有 `phone_number_verified` 为真、且号码去掉首尾空白后
不超过 32 个字符时才会写入。登录命中、注册、绑定三处规则一致，并与邮箱共用同一套镜像口径（见上方
[账号关联规则](#账号关联规则)）：「作为本站账号登录」的提供方以 IdP 为准，本地为空或与 IdP 不同都
写入；其余提供方只在本地为空时回填，已有值一律不覆盖。两档都不抢已被其它账号占用的号码；未验证、
缺失或超长一律不写，也不清空本地已有值。

用户不能自改，前台个人设置与后台用户详情都是只读展示。它**不是本站密码登录的标识**，
Modeltaps 也不发短信——手机号登录与短信验证码全部由 IdP 负责。

## 反向代理与真实客户端 IP

限流、回环豁免等按 IP 计的逻辑取的是「客户端 IP」。为避免任何人都能用 `X-Forwarded-For`
之类的请求头伪造 IP 绕过限流，Modeltaps **默认不信任任何代理**：`TRUSTED_PROXIES` 为空时，
客户端 IP 一律取 TCP 直连对端地址，`TRUSTED_HEADER` 即使配了也会被忽略。

::: warning 部署在反向代理后面必须配置 TRUSTED_PROXIES
否则所有请求的客户端 IP 都会是反代自身的地址，限流会把全站流量算到同一个桶里。
:::

### Cloudflare

回源到 Modeltaps 的是 Cloudflare 边缘节点，需把 Cloudflare 的回源网段列入信任，并采信
`CF-Connecting-IP`：

```yaml
trusted_header: "CF-Connecting-IP"
trusted_proxies:
  - 173.245.48.0/20
  - 103.21.244.0/22
  # ... 其余网段见 https://www.cloudflare.com/ips/
```

网段清单以 Cloudflare 官方 <https://www.cloudflare.com/ips/> 为准（IPv4 与 IPv6 都要填），
并随官方变更更新。若 Cloudflare 后面还套了自建 Nginx 再回源，则应填 Nginx 的地址而非 Cloudflare 网段。

### Nginx / 其他反向代理

填反代到 Modeltaps 的网关地址（同机或 Docker 同网络），并让反代透传 `X-Forwarded-For`：

```yaml
trusted_header: "" # 留空则按默认的 X-Forwarded-For / X-Real-IP 取链上最右侧的不可信地址
trusted_proxies:
  - 127.0.0.1
  - 172.18.0.0/16 # Docker 网络示例，按实际网段填写
```

对应 Nginx 侧：

```nginx
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header X-Real-IP $remote_addr;
```

### 直连（无反向代理）

保持默认即可：`TRUSTED_PROXIES` 与 `TRUSTED_HEADER` 均留空，客户端 IP 即直连地址，
限流按真实来源 IP 计。

