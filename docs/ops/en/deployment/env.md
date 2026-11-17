---
title: "Environment Variables"
layout: doc
outline: deep
lastUpdated: true
---

# Environment Variables

::: warning Note
Environment variables take precedence over the configuration file.
:::

This page may lag behind the code. For the complete set of configurable keys, refer to `config.example.yaml` in the root of the delivery package.

## Converting configuration keys to environment variables

Uppercase the key names from the configuration file and join nested levels with underscores. For example:

```yaml
# configuration file
user_token_secret: "<random string of at least 32 characters>"
logs:
  filename: "modeltaps.log" # log file name
```

becomes:

```bash
USER_TOKEN_SECRET="<random string of at least 32 characters>"
LOGS_FILENAME="modeltaps.log"
```

## Variable reference

1. `REDIS_CONN_STRING`: when set, Redis is used as the cache.
   - Example: `REDIS_CONN_STRING=redis://default:redispw@localhost:49153`
   - If database latency is already low there is no need to enable Redis; doing so may instead introduce stale data.
2. `SESSION_SECRET`: when set, a fixed session secret is used so cookies of signed-in users remain valid after a restart.
   - Generate a strong random string with `openssl rand -base64 48 | tr -d '\n'`; do not use a simple passphrase.
3. `SQL_DSN`: when set, the specified database is used instead of SQLite. Use MySQL or PostgreSQL.
   - Examples (angle brackets are placeholders — replace them with real values):
     - MySQL: `SQL_DSN=<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps`
     - PostgreSQL: `SQL_DSN=postgres://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/modeltaps` (support in progress, feedback welcome)
   - Create a dedicated database account for Modeltaps with a strong password; do not reuse superuser accounts such as `root`.
   - The `modeltaps` database must exist beforehand. Tables are created automatically.
   - Local database: add `--network="host"` to the deployment command so the container can reach MySQL on the host.
   - Cloud database: if the cloud server requires identity verification, append `?tls=skip-verify` to the connection parameters.
   - Adjust the following parameters to match your database (or keep the defaults):
     - `SQL_MAX_IDLE_CONNS`: maximum idle connections, defaults to `100`.
     - `SQL_MAX_OPEN_CONNS`: maximum open connections, defaults to `1000`.
       - Reduce this value if you see `Error 1040: Too many connections`.
     - `SQL_MAX_LIFETIME`: maximum connection lifetime in minutes, defaults to `60`.
4. `FRONTEND_BASE_URL`: when set, page requests are redirected to the given address. For slave nodes only.
   - Example: `FRONTEND_BASE_URL=https://<master server domain>`
5. `MEMORY_CACHE_ENABLED`: enables the in-memory cache, which introduces some delay in user quota updates. Accepts `true` or `false`; defaults to `false`.
   - Example: `MEMORY_CACHE_ENABLED=true`
6. `SYNC_FREQUENCY`: how often configuration is synced from the database when caching is enabled, in seconds. Defaults to `600`.
   - Example: `SYNC_FREQUENCY=60`
7. `NODE_TYPE`: the node type. Accepts `master` or `slave`; defaults to `master`.
   - Example: `NODE_TYPE=slave`
8. `CHANNEL_UPDATE_FREQUENCY`: how often channel balances are refreshed, in minutes. Defaults to 60; 0 turns the refresh off. Only channel types with a balance API are refreshed (OpenRouter, Vercel AI Gateway, Requesty, DeepSeek, Moonshot, SiliconFlow); a channel whose balance drops below its low-balance alert threshold triggers a notification, and a balance of 0 disables the channel. Subscription channels (Codex, Claude Code) have their usage windows checked on the same schedule (Codex through its usage endpoint, Claude Code from the usage its responses report), and each window that reaches an alert tier (50%, 75% and 100% by default, set in the notification settings or on the channel's monitoring tab) sends one subscription usage notice per tier, again after the window resets. OpenRouter reports the remaining amount of the key, so set a spend limit on the key in the OpenRouter dashboard. Requesty only shows the balance to a key with manage permission: enter the channel key as `apiKey|manageKey`, or use one key that has both permissions.
   - Example: `CHANNEL_UPDATE_FREQUENCY=1440`
9. `CHANNEL_TEST_FREQUENCY`: when set, every channel is tested at this interval (minutes), overriding the scheduled test in System settings → Gateway Behavior → Channel Auto Disable & Recovery. When unset, the admin setting applies; by default only auto-disabled channels are tested every 10 minutes and re-enabled when they pass.
   - Example: `CHANNEL_TEST_FREQUENCY=1440`
10. `POLLING_INTERVAL`: the interval between requests when refreshing channel balances or testing availability in bulk, in seconds. No interval by default.
    - Example: `POLLING_INTERVAL=5`
11. `BATCH_UPDATE_ENABLED`: enables batched database updates, which introduces some delay in user quota updates. Accepts `true` or `false`; defaults to `false`.
    - Example: `BATCH_UPDATE_ENABLED=true`
    - Try enabling this if you run into too many database connections.
12. `BATCH_UPDATE_INTERVAL=5`: the batching interval, in seconds. Defaults to `5`.
    - Example: `BATCH_UPDATE_INTERVAL=5`
13. Rate limiting:
    - `GLOBAL_API_RATE_LIMIT`: global API rate limit (excluding relay requests), the maximum number of requests within three minutes. Defaults to `300`.
    - `GLOBAL_WEB_RATE_LIMIT`: global web rate limit, the maximum number of requests within three minutes. Defaults to `300`.
    - How requests are counted:
      - **When a signed-in user is recognized** (a valid web session is present), each request takes from two buckets: the **user** quota and the **source IP** ceiling; whichever fills first returns `429`. The user quota is the value configured above and the IP ceiling is four times that. Users behind the same NAT or office gateway get separate quotas and do not crowd each other out, while registering more accounts on one machine does not multiply the quota.
      - **When no signed-in user is recognized** (anonymous access, or token-authenticated endpoints such as `/dashboard`), requests are counted per source IP only, at exactly the configured value.
      - IPv6 sources are counted per `/64` block: switching addresses within one block does not give a fresh quota.
      - Direct local requests (a loopback peer and no `X-Forwarded-For` / `X-Real-IP` / `TRUSTED_HEADER` header) are not counted. Requests forwarded by a proxy in `TRUSTED_PROXIES` are counted by the client IP it reports, also when that is a loopback address.
      - When a reverse proxy on the same host is missing from `TRUSTED_PROXIES`, every client looks local: requests it forwards are not rate limited (otherwise the whole site would share one bucket), and the system log warns about it once an hour. Add the proxy to `TRUSTED_PROXIES`.
    - `GLOBAL_RATE_LIMIT_WHITELIST`: source addresses exempt from rate limiting, as IPs or CIDRs separated by commas. Empty by default. Intended for trusted high-volume callers such as admin scripts and monitoring probes.
      - Example: `GLOBAL_RATE_LIMIT_WHITELIST=10.0.0.5,192.168.0.0/16`
      - It only applies to the two global limits above. Sensitive endpoints (sign-in, registration, password change, OAuth callbacks) and uploads / downloads have their own limits and are **not exempted**, so brute-force protection stays in place.
      - The whitelist matches the client IP the server resolves, whose trust is governed by `TRUSTED_PROXIES`; spoofed request headers cannot obtain an exemption on a directly exposed instance.
    - A rate-limited request gets `429` with `Retry-After` (seconds) and `X-RateLimit-Limit` / `X-RateLimit-Remaining` / `X-RateLimit-Reset` headers (readable by cross-origin callers), and a JSON error body that includes the remaining wait in seconds and the request id.
14. Encoder cache settings:
    - `TIKTOKEN_CACHE_DIR`: by default the program downloads common token encodings at startup (for example for `gpt-3.5-turbo`), which can fail on unstable networks or offline hosts. Point this at a directory to cache the data; the cache can be moved to an offline environment.
    - `DATA_GYM_CACHE_DIR`: currently equivalent to `TIKTOKEN_CACHE_DIR`, but with lower precedence.
15. `RELAY_TIMEOUT`: relay timeout in seconds. No timeout is applied by default.
16. `SQLITE_BUSY_TIMEOUT`: SQLite lock wait timeout in milliseconds. Defaults to `3000`.
17. `TG_BOT_API_KEY`: the Telegram bot API key, obtainable from [BotFather](https://t.me/BotFather).
18. `TG_WEBHOOK_SECRET`: (optional) a webhook secret of your choosing. When set, messages are received via `webhook` instead of polling.
    - Telegram delivers to the fixed address `/api/telegram/webhook`, checked by this secret alone. Earlier versions put the bot token in the delivery address, so access logs may hold `/api/telegram/<token>`; if anyone else could read those logs, rotate the token in BotFather.
19. `USER_TOKEN_SECRET`: the signing secret for user tokens. Required, longer than 32 characters. Do not change it after the first run, or existing user tokens become invalid.
    - Generate it with `openssl rand -base64 48 | tr -d '\n'` and use a different value from `SESSION_SECRET`.
    - Checked at startup: the gateway refuses to start when it is still the example placeholder (such as `<change-me-…>`) or shorter than 32 characters. `SESSION_SECRET` is checked the same way when it is set. An existing deployment with a shorter secret that cannot rotate right away can set `USER_TOKEN_SECRET_ALLOW_SHORT=true` to start anyway with a warning in the log; rotating the secret invalidates every API key issued so far.
20. `HASHIDS_SALT`: the Sqids alphabet used to obfuscate user token information. Optional; when empty the default alphabet `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789` is used. If you set it, make sure the alphabet contains no duplicate characters.
21. `AUTO_PRICE_UPDATES`: automatic price updates. Accepts `true` or `false`; defaults to `false`. When enabled, every startup compares the database against the built-in default model prices and writes any missing prices into the database. Note that once enabled you cannot delete the built-in default prices — they are rewritten on the next restart. Use this only when your prices are meant to track the official ones.
22. `AUTO_PRICE_UPDATES_MODE`: the price update mode. `add` only adds prices that do not exist yet; `overwrite` replaces all price configuration; `update` only updates existing entries; `system` initializes prices from the built-in price table. Defaults to `system`. In production, `system` is recommended, combined with manually fetching data from the price update service in the web price management module and reviewing each entry before applying it.
23. `AUTO_PRICE_UPDATES_INTERVAL`: the automatic price update interval in minutes. Only effective when `AUTO_PRICE_UPDATES_MODE` is `add` or `overwrite`; the system periodically fetches price configuration from the price update server and applies it. Defaults to 1440.
24. `UPDATE_PRICE_SERVICE`: the price update service URL. When set, price data is pulled from this address. Empty by default, which disables the external price service (in that case use `AUTO_PRICE_UPDATES_MODE=system` so prices are initialized from the built-in table).
25. `USER_INVOICE_MONTH`: whether to enable monthly user invoices. When enabled, the system generates each user's previous-month summary invoice in the early hours of the 1st of every month. This can be resource-intensive with large volumes of data, so enable it with care. Defaults to `false`.
26. `ROOT_PASSWORD`: the initial password of the super admin account seeded on first startup (username `admin`, role root). Only takes effect when the database has no users yet (that is, on first startup). It must be 8–64 characters long, otherwise the program fails to start.
    - When unset, the program generates a 16-character strong random password and prints it exactly once in the first-run log.
    - Change the root password immediately after your first sign-in and clean up the startup log containing the initial password.
    - The variable **only takes effect when the database is first created**; changing or removing it afterwards does not affect the existing root password, so it is not a break-glass mechanism. Read [Administrator break-glass access](./index.md#administrator-break-glass-access) before turning password login off.
27. `ROOT_PASSWORD_ALLOW_INSECURE`: whether to skip the 8–64 character length check on `ROOT_PASSWORD`. Accepts `true` / `false`, defaults to `false`. **For local development only; never enable it in production.**
    - Local development convention: `ROOT_PASSWORD=admin` plus `ROOT_PASSWORD_ALLOW_INSECURE=true`, so you can sign in with `admin/admin` after the first startup. Databases created before this change seeded `root` instead; keep signing in with that account.
    - When enabled, a warning line is printed to the startup log. The switch has no effect when `ROOT_PASSWORD` is unset: the 16-character random password branch still applies.
28. `ENCRYPTION_KEY`: the master key that encrypts stored credentials in the database (AES-256-GCM). When it is set, upstream channel keys (including the refresh tokens of OAuth channels such as Codex and Claude Code), channel management keys, payment gateway configurations, notification channel secrets, OIDC provider client secrets and the secret system settings (SMTP token, GitHub / OIDC / LinuxDo client secrets, WeChat server token, Turnstile secret key, CF Worker image key) are encrypted before they are written to the database.
    - The value is 32 random bytes in base64 or hex; generate it with `openssl rand -base64 32`. The program refuses to start when the value is malformed or still a placeholder from the docs.
    - When unset, values are stored in plaintext as before; the startup log prints a warning and the admin settings (System settings → Site) show a notice.
    - After you set it and restart, the master node encrypts the existing plaintext values in batches and logs how many it encrypted per column; later starts do not process them again. Every node of a multi-node deployment must use the same value.
    - **Back up this key**, separately from the database backups: encrypted data cannot be recovered without it, only cleared (the values start with `enc:v1:`) and entered again. When the database holds encrypted values but no configured key opens them (unset or wrong), the program refuses to start rather than serve channels whose keys it cannot read.
    - Do not change it casually; to rotate it, use `ENCRYPTION_KEY_PREVIOUS` below.
29. `ENCRYPTION_KEY_PREVIOUS`: the old key while rotating `ENCRYPTION_KEY`, used only to decrypt. Move the old value to `ENCRYPTION_KEY_PREVIOUS`, put a newly generated value in `ENCRYPTION_KEY` and restart every node; at startup the master node re-encrypts the data still under the old key with the new one. Once the log shows the `re-encrypted` line, remove `ENCRYPTION_KEY_PREVIOUS` and restart again.

## Additional configuration keys

The following commonly used keys are read by the code but not covered above. For the complete list and default values, refer to `config.example.yaml` in the root of the delivery package.

### Server

- `PORT`: listening port, defaults to `3000`.
- `GIN_MODE`: Gin run mode, `release` or `debug`. Defaults to `release`.
- `HTTPS`: whether the service is exposed over HTTPS (affects the cookie `Secure` flag and similar). Defaults to `false`.
- `TRUSTED_HEADER`: the request header used to resolve the real client IP, for example `CF-Connecting-IP`.
- Cross-site request check (no variable, always on): write requests (POST / PUT / PATCH / DELETE) signed in with a console session must come from this site's pages. Their `Origin` (or `Referer` when `Origin` is missing) must match the server address configured in the admin settings, `FRONTEND_BASE_URL`, the request's Host, or the `X-Forwarded-Host` sent by a proxy in `TRUSTED_PROXIES`; otherwise they get `403`. Calls with a management key, payment notifications and webhooks are not affected. If your reverse proxy rewrites Host without sending `X-Forwarded-Host`, set the server address in the admin settings.
- `SHUTDOWN_TIMEOUT`: graceful shutdown timeout in seconds, defaults to `30`.
- `API_MAX_BODY_MB`: request body limit of the management API (`/api`), the dashboard routes and MCP, in MB. Larger bodies get `413`. Defaults to `2`; `0` removes the limit. Bulk admin imports (price sync, batch price updates, channel keys, settings) accept 32 MB, or this value if it is larger; payment notifications always accept 1 MB. Relay routes use `RELAY_MAX_BODY_MB` (default `128`) instead.
- `SERVER_BODY_READ_TIMEOUT`: how long a client may take to send a request body to the management API, the dashboard routes and MCP, in seconds. Defaults to `60`; `0` removes the bound. Relay routes are not bounded, so long uploads and streamed responses are unaffected.
- `PPROF_ENABLED`: whether to enable pprof profiling, defaults to `false`. When enabled, `/debug/pprof/` is open to the super admin only: sign in as the super admin and open it in the browser, or send the super admin's legacy access token in the `Authorization` header. Management keys are refused.
- `LANGUAGE`: default language, defaults to `zh_CN`.
- `FAVICON`: custom favicon path.
- `GITHUB_PROXY`: proxy prefix for downloading external resources.

### Login security

- `LOGIN_MAX_FAILURES`: failed sign-ins allowed for one account from one IP, defaults to `5`. Once reached, that IP cannot sign in to the account for a while; other addresses are not affected, so someone guessing from one address cannot lock the owner out. Two wider limits are 4 times this value (`20` by default): failures for one account across all IPs (stops guessing from rotating addresses; once reached, the account is locked for every address) and failures from one IP across all accounts (stops one address trying passwords on many accounts). Wrong passwords, wrong email codes and wrong two-step codes all count; IPv6 addresses count per /64. A locked sign-in gets the same message as a wrong password. A successful sign-in clears the account's counters but not the IP's.
- `LOGIN_LOCKOUT_MINUTES`: lockout window in minutes, defaults to `15`, counted from the latest failure.

> With Redis enabled the failure counters are shared across replicas; without Redis they are kept per process, so with several replicas the effective limits are multiplied by the number of replicas.

### Database and Redis

- `SQLITE_PATH`: SQLite database file path, defaults to `modeltaps.db`.
- `BRAND_ICON_DIR`: local directory of the synced brand icon layer, one `{dir}/{version}/` per version, defaults to `data/brand-icons`.
- `REDIS_DB`: Redis DB index (used when the connection string does not include one), defaults to `0`.
- `REDIS_POOL_SIZE`: connection pool size, defaults to `100`.
- `REDIS_MIN_IDLE_CONNS`: minimum idle connections, defaults to `10`.
- `REDIS_POOL_TIMEOUT` / `REDIS_READ_TIMEOUT` / `REDIS_WRITE_TIMEOUT`: pool wait / read / write timeouts in seconds, defaulting to `5` / `2` / `2`.

### HTTP client and relay timeouts

- `CONNECT_TIMEOUT`: connection timeout in seconds, defaults to `5`.
- `RELAY_REQUEST_TIMEOUT`: timeout for a single relayed request, in seconds. Defaults to `300`.
- `STREAM_IDLE_TIMEOUT`: streaming idle timeout. The timer resets whenever data arrives during a streamed channel response; if the stream stays silent longer than this, it is aborted. Complements the overall wall-clock timeout and targets "stuck streams" precisely. In seconds, defaults to `300`; set to `0` to disable.
- `RESPONSE_HEADER_TIMEOUT`: response header timeout in seconds, defaults to `120`.
- `TLS_HANDSHAKE_TIMEOUT`: TLS handshake timeout in seconds, defaults to `30`.
- `TLS_INSECURE_SKIP_VERIFY`: skip TLS certificate verification (insecure), defaults to `false`.
- `MAX_CONNS_PER_HOST` / `MAX_IDLE_CONNS` / `MAX_IDLE_CONNS_PER_HOST`: connection limits, defaulting to `0` (unlimited) / `1000` / `200`.

### Pricing and billing

- `CATALOG_PRICING_URL`: source URL for syncing model catalog prices.
- `CATALOG_PRICING_AUTO_SYNC`: whether to sync catalog prices automatically, defaults to `true`.
- `CHANNEL_PRICING_AUTO_SYNC`: whether to sync channel prices automatically, defaults to `true`.
- `UNPRICED_MODEL_POLICY`: policy for unpriced models, defaults to `block`.
- `UNPRICED_MODEL_DEFAULT_RATIO`: default ratio for unpriced models, defaults to `30.0`.
- `MODEL_DRIFT_AUTO_CHECK`: whether to compare channel models with the upstream model lists every day (models removed or added upstream), defaults to `true`. The check keeps each channel's model list in step with its upstream: models new upstream join the list (switched off unless the channel option "Switch new upstream models on automatically" is on), models the upstream stops listing stay and are marked gone, and models an admin removed are not added back. Once supplies route requests (`ModelSupplyEnabled`), the list itself routes nothing: new models join switched on, and those that belong to no supply yet are listed under "New upstream models" on the models page, where an admin attaches them to a public model or ignores them.

### Logging, metrics and integrations

- `LOG_DIR`: log directory, defaults to `./logs`.
- `LOG_LEVEL`: log level; `debug` produces more detail.
- `LOGS_FILENAME` / `LOGS_MAX_SIZE` / `LOGS_MAX_AGE` / `LOGS_MAX_BACKUP` / `LOGS_COMPRESS`: log file name and rotation settings, defaulting to `modeltaps.log` / `100` (MB) / `7` (days) / `10` / `false`.
- `LOGS_FORMAT`: log format, `text` (default) or `json` (one JSON object per line, for collectors such as Loki or ELK).
- `LOGS_ACCESS_LOG`: whether to write one access log line per HTTP request, default `true`. Successful requests for front-end static files and from the system log page itself are left out; failed requests and errors are always logged.
- Logs go to the log file and to standard error. The admin "System logs" page lists only the warnings and errors that recurred since the last start, plus the recent logs kept in memory; to search the full history, collect standard error or the log directory with `docker logs`, journald or a log platform.
- `METRICS_USER` / `METRICS_PASSWORD`: Basic Auth for the `/metrics` endpoint; leave empty to disable authentication.
- `MCP_ENABLE`: whether to enable MCP, defaults to `false`.
  - Endpoints: `POST /mcp` (Streamable HTTP), `GET /mcp/sse` + `POST /mcp/message` (SSE). Send a management key (read-only is enough) in an `Authorization: Bearer <management key>` header; the old `/mcp/<token>` path form has been removed. Deprecated access tokens keep working until the next release; API keys are refused (`401`).
- `UPTIME_KUMA_ENABLE` / `UPTIME_KUMA_DOMAIN` / `UPTIME_KUMA_STATUS_PAGE_NAME`: Uptime Kuma status page integration.
- `DISABLE_TOKEN_ENCODERS`: disable local token encoders and fall back to estimation, defaults to `false`.
- `TG_HTTP_PROXY`: HTTP/SOCKS5 proxy used to reach Telegram.

### Mail sending

When unset, mail services are configured in the console under Platform admin → Mail. Several can be stored and one is in use: the first one added goes into use, using another replaces it, and the one in use can only be deleted after switching to another, so a new service can be set up and tested before switching. When `SMTP_URL` is set the deployment wins: the console turns read-only and can only send a test email.

- `SMTP_URL`: `smtps://user:password@host:465` (implicit TLS) or `smtp://user:password@host:587`; add `?tls=starttls` and so on to choose the encryption (`auto`, `ssl`, `starttls`, `starttls_opportunistic`, `none`). URL-encode special characters in the password.
- `SMTP_FROM`: sender address, required when `SMTP_URL` is set.
- `SMTP_FROM_NAME` / `SMTP_REPLY_TO`: sender name and reply-to address, optional.

For example with Resend: `SMTP_URL=smtps://resend:re_xxx@smtp.resend.com:465` and `SMTP_FROM=noreply@example.com`.

### Storage and search

The object storage (`STORAGE_*`) and web search (`SEARCH_*`) groups contain many keys. For key names and default values, refer directly to `config.example.yaml` in the root of the delivery package. Notification channels (`NOTIFY_*`) moved to the console, see [Notifications](./notify.md).

