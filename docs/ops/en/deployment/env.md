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
8. `CHANNEL_UPDATE_FREQUENCY`: when set, channel balances are refreshed periodically, in minutes. No refresh happens when unset.
   - Example: `CHANNEL_UPDATE_FREQUENCY=1440`
9. `CHANNEL_TEST_FREQUENCY`: when set, channels are tested periodically, in minutes. No testing happens when unset.
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
      - Direct local requests (both the peer and the resolved client IP are loopback) are not counted; requests forwarded by a reverse proxy on the same host are still counted by the real client IP.
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
19. `USER_TOKEN_SECRET`: the signing secret for user tokens. Required, longer than 32 characters. Do not change it after the first run, or existing user tokens become invalid.
    - Generate it with `openssl rand -base64 48 | tr -d '\n'` and use a different value from `SESSION_SECRET`.
20. `HASHIDS_SALT`: the Sqids alphabet used to obfuscate user token information. Optional; when empty the default alphabet `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789` is used. If you set it, make sure the alphabet contains no duplicate characters.
21. `AUTO_PRICE_UPDATES`: automatic price updates. Accepts `true` or `false`; defaults to `false`. When enabled, every startup compares the database against the built-in default model prices and writes any missing prices into the database. Note that once enabled you cannot delete the built-in default prices — they are rewritten on the next restart. Use this only when your prices are meant to track the official ones.
22. `AUTO_PRICE_UPDATES_MODE`: the price update mode. `add` only adds prices that do not exist yet; `overwrite` replaces all price configuration; `update` only updates existing entries; `system` initializes prices from the built-in price table. Defaults to `system`. In production, `system` is recommended, combined with manually fetching data from the price update service in the web price management module and reviewing each entry before applying it.
23. `AUTO_PRICE_UPDATES_INTERVAL`: the automatic price update interval in minutes. Only effective when `AUTO_PRICE_UPDATES_MODE` is `add` or `overwrite`; the system periodically fetches price configuration from the price update server and applies it. Defaults to 1440.
24. `UPDATE_PRICE_SERVICE`: the price update service URL. When set, price data is pulled from this address. Empty by default, which disables the external price service (in that case use `AUTO_PRICE_UPDATES_MODE=system` so prices are initialized from the built-in table).
25. `USER_INVOICE_MONTH`: whether to enable monthly user invoices. When enabled, the system generates each user's previous-month summary invoice in the early hours of the 1st of every month. This can be resource-intensive with large volumes of data, so enable it with care. Defaults to `false`.
26. `ROOT_PASSWORD`: the initial password of the root account. Only takes effect when the database has no users yet (that is, on first startup). It must be 8–64 characters long, otherwise the program fails to start.
    - When unset, the program generates a 16-character strong random password and prints it exactly once in the first-run log.
    - Change the root password immediately after your first sign-in and clean up the startup log containing the initial password.
    - The variable **only takes effect when the database is first created**; changing or removing it afterwards does not affect the existing root password, so it is not a break-glass mechanism. Read [Administrator break-glass access](./index.md#administrator-break-glass-access) before turning password login off.
27. `ROOT_PASSWORD_ALLOW_INSECURE`: whether to skip the 8–64 character length check on `ROOT_PASSWORD`. Accepts `true` / `false`, defaults to `false`. **For local development only; never enable it in production.**
    - Local development convention: `ROOT_PASSWORD=root` plus `ROOT_PASSWORD_ALLOW_INSECURE=true`, so you can sign in with `root/root` after the first startup.
    - When enabled, a warning line is printed to the startup log. The switch has no effect when `ROOT_PASSWORD` is unset: the 16-character random password branch still applies.

## Additional configuration keys

The following commonly used keys are read by the code but not covered above. For the complete list and default values, refer to `config.example.yaml` in the root of the delivery package.

### Server

- `PORT`: listening port, defaults to `3000`.
- `GIN_MODE`: Gin run mode, `release` or `debug`. Defaults to `release`.
- `HTTPS`: whether the service is exposed over HTTPS (affects the cookie `Secure` flag and similar). Defaults to `false`.
- `TRUSTED_HEADER`: the request header used to resolve the real client IP, for example `CF-Connecting-IP`.
- `SHUTDOWN_TIMEOUT`: graceful shutdown timeout in seconds, defaults to `30`.
- `PPROF_ENABLED`: whether to enable pprof profiling, defaults to `false`.
- `LANGUAGE`: default language, defaults to `zh_CN`.
- `FAVICON`: custom favicon path.
- `GITHUB_PROXY`: proxy prefix for downloading external resources.

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
- `MODEL_DRIFT_AUTO_CHECK`: whether to detect model drift automatically, defaults to `false`.

### Logging, metrics and integrations

- `LOG_DIR`: log directory, defaults to `./logs`.
- `LOG_LEVEL`: log level; `debug` produces more detail.
- `LOGS_FILENAME` / `LOGS_MAX_SIZE` / `LOGS_MAX_AGE` / `LOGS_MAX_BACKUP` / `LOGS_COMPRESS`: log file name and rotation settings, defaulting to `modeltaps.log` / `100` (MB) / `7` (days) / `10` / `false`.
- `METRICS_USER` / `METRICS_PASSWORD`: Basic Auth for the `/metrics` endpoint; leave empty to disable authentication.
- `MCP_ENABLE`: whether to enable MCP, defaults to `false`.
- `UPTIME_KUMA_ENABLE` / `UPTIME_KUMA_DOMAIN` / `UPTIME_KUMA_STATUS_PAGE_NAME`: Uptime Kuma status page integration.
- `DISABLE_TOKEN_ENCODERS`: disable local token encoders and fall back to estimation, defaults to `false`.
- `TG_HTTP_PROXY`: HTTP/SOCKS5 proxy used to reach Telegram.

### Notifications, storage and search

The notification (`NOTIFY_*`), object storage (`STORAGE_*`) and web search (`SEARCH_*`) groups contain many keys. For key names and default values, refer directly to `config.example.yaml` in the root of the delivery package.

