---
title: "環境變數"
layout: doc
outline: deep
lastUpdated: true
---

# 環境變數

::: warning 注意
環境變數優先級高於設定檔。
:::

本頁面可能未及時更新，完整可配置鍵請以交付包根目錄的設定檔示例 `config.example.yaml` 為準。

## 設定檔轉環境變數

將設定檔中的變數名全部大寫，遇到子集用底線連接，例如：

```yaml
# 設定檔
user_token_secret: "<32 位以上的隨機字符串>"
logs:
  filename: "modeltaps.log" # 日誌檔案名
```

轉換為環境變數：

```bash
USER_TOKEN_SECRET="<32 位以上的隨機字符串>"
LOGS_FILENAME="modeltaps.log"
```

## 環境變數說明

1. `REDIS_CONN_STRING`：設定之後將使用 Redis 作為快取使用。
   - 例子：`REDIS_CONN_STRING=redis://default:redispw@localhost:49153`
   - 如果數據庫訪問延遲很低，沒有必要啟用 Redis，啟用後反而會出現數據滯後的問題。
2. `SESSION_SECRET`：設定之後將使用固定的會話密鑰，這樣系統重新啟動後已登入用戶的 cookie 將依舊有效。
   - 取值請用 `openssl rand -base64 48 | tr -d '\n'` 生成強隨機字符串，不要使用簡單口令。
3. `SQL_DSN`：設定之後將使用指定數據庫而非 SQLite，請使用 MySQL 或 PostgreSQL。
   - 例子（尖括號為佔位符，請替換為實際值）：
     - MySQL：`SQL_DSN=<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps`
     - PostgreSQL：`SQL_DSN=postgres://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/modeltaps`（適配中，歡迎反饋）
   - 請為 Modeltaps 單獨建立數據庫帳戶並設定強密碼，不要複用 `root` 等超級用戶帳戶。
   - 注意需要提前建立數據庫 `modeltaps`，無需手動建表，程式將自動建表。
   - 如果使用本地數據庫：部署命令可新增 `--network="host"` 以使得容器內的程式可以訪問到宿主機上的 MySQL。
   - 如果使用雲數據庫：如果雲伺服器需要驗證身份，需要在連接參數中新增 `?tls=skip-verify`。
   - 請根據實際數據庫配置修改下列參數（或者保持預設值）：
     - `SQL_MAX_IDLE_CONNS`：最大空閒連接數，預設為 `100`。
     - `SQL_MAX_OPEN_CONNS`：最大打開連接數，預設為 `1000`。
       - 如果報錯 `Error 1040: Too many connections`，請適當減小該值。
     - `SQL_MAX_LIFETIME`：連接的最大生命週期，預設為 `60`，單位分鐘。
4. `FRONTEND_BASE_URL`：設定之後將重定向頁面請求到指定的地址，僅限從伺服器設定。
   - 例子：`FRONTEND_BASE_URL=https://<主伺服器域名>`
5. `MEMORY_CACHE_ENABLED`：啟用內存快取，會導致用戶額度的更新存在一定的延遲，可選值為 `true` 和 `false`，未設定則預設為 `false`。
   - 例子：`MEMORY_CACHE_ENABLED=true`
6. `SYNC_FREQUENCY`：在啟用快取的情況下與數據庫同步配置的頻率，單位為秒，預設為 `600` 秒。
   - 例子：`SYNC_FREQUENCY=60`
7. `NODE_TYPE`：設定之後將指定節點類型，可選值為 `master` 和 `slave`，未設定則預設為 `master`。
   - 例子：`NODE_TYPE=slave`
8. `CHANNEL_UPDATE_FREQUENCY`：設定之後將定期更新渠道餘額，單位為分鐘，未設定則不進行更新。
   - 例子：`CHANNEL_UPDATE_FREQUENCY=1440`
9. `CHANNEL_TEST_FREQUENCY`：設定之後將定期檢查渠道，單位為分鐘，未設定則不進行檢查。
   - 例子：`CHANNEL_TEST_FREQUENCY=1440`
10. `POLLING_INTERVAL`：批量更新渠道餘額以及測試可用性時的請求間隔，單位為秒，預設無間隔。
    - 例子：`POLLING_INTERVAL=5`
11. `BATCH_UPDATE_ENABLED`：啟用數據庫批量更新聚合，會導致用戶額度的更新存在一定的延遲可選值為 `true` 和 `false`，未設定則預設為 `false`。
    - 例子：`BATCH_UPDATE_ENABLED=true`
    - 若出現數據庫連接數過多的問題，可以嘗試啟用該選項。
12. `BATCH_UPDATE_INTERVAL=5`：批量更新聚合的時間間隔，單位為秒，預設為 `5`。
    - 例子：`BATCH_UPDATE_INTERVAL=5`
13. 請求頻率限制：
    - `GLOBAL_API_RATE_LIMIT`：全局 API 速率限制（除中繼請求外），單 ip 三分鐘內的最大請求數，預設為 `180`。
    - `GLOBAL_WEB_RATE_LIMIT`：全局 Web 速率限制，單 ip 三分鐘內的最大請求數，預設為 `60`。
14. 編碼器快取設定：
    - `TIKTOKEN_CACHE_DIR`：預設程式啟動時會聯網下載一些通用的詞元的編碼，如：`gpt-3.5-turbo`，在一些網絡環境不穩定，或者離線情況，可能會導致啟動有問題，可以配置此目錄快取數據，可遷移到離線環境。
    - `DATA_GYM_CACHE_DIR`：目前該配置作用與 `TIKTOKEN_CACHE_DIR` 一致，但是優先級沒有它高。
15. `RELAY_TIMEOUT`：中繼逾時設定，單位為秒，預設不設定逾時時間。
16. `SQLITE_BUSY_TIMEOUT`：SQLite 鎖等待逾時設定，單位為毫秒，預設 `3000`。
17. `TG_BOT_API_KEY`：Telegram bot 的 API 密鑰，可在 [BotFather](https://t.me/BotFather) 獲取。
18. `TG_WEBHOOK_SECRET`：（可選）webhook 密鑰，可自訂。設定該密鑰後將使用 `webhook` 方式接收訊息，否則使用輪詢（Polling）方式。
19. `USER_TOKEN_SECRET` ： 設定用戶令牌簽名密鑰，必填，大於 32 位以上， 設定後請勿修改，否則會導致用戶令牌失效。
    - 取值請用 `openssl rand -base64 48 | tr -d '\n'` 生成，且與 `SESSION_SECRET` 使用不同的值。
20. `HASHIDS_SALT` ：Sqids 字母表，用於混淆用戶令牌信息， 可空，如為空則使用預設字母表`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789`，如設定，則需要保證字母表中無重複字元。
21. `AUTO_PRICE_UPDATES`：自動更新價格，可選值為 `true` 和 `false`，未設定則預設為 `false`。開啟後每次啟動程式時，會比對數據庫中的數據與程式內建的預設模型價格，若數據庫中的模型價格有缺失將自動同步到數據庫中。需注意：開啟後無法刪除程式內建的預設模型價格，刪除後重啟會重新寫入，該選項適用於價格與官方保持一致的場景。
22. `AUTO_PRICE_UPDATES_MODE`：價格更新模式，可選值為 `add`：僅增加系統不存在的價格；`overwrite`：覆蓋系統所有價格配置；`update`：僅更新現有數據；`system`：使用程式內建價格表初始化價格配置，預設為 `system`。生產環境建議使用 `system` 模式，並在 Web 端價格管理模組手動獲取價格更新服務數據後逐條核對更新。
23. `AUTO_PRICE_UPDATES_INTERVAL` ：價格自動更新時間，單位分鐘，僅`AUTO_PRICE_UPDATES_MODE`為`add`、`overwrite`時生效，系統將按照此時間週期性從價格更新伺服器獲取價格配置並更新系統價格。預設值：1440
24. `UPDATE_PRICE_SERVICE` ：價格更新服務地址，設定之後將從該地址拉取價格數據更新價格。預設為空，為空時不啟用外部價格服務（此時 `AUTO_PRICE_UPDATES_MODE` 請使用 `system`，即以程式內建價格表初始化）。
25. `USER_INVOICE_MONTH`：是否開啟用戶月度賬單功能，開啟後系統每月 1 日凌晨生成用戶上一月度的數據匯總賬單；數據量較大時資源消耗較高，請謹慎開啟，預設 `false`。
26. `ROOT_PASSWORD` ：root 帳戶的初始密碼，僅在數據庫中還沒有任何用戶（即首次啟動）時生效，長度需為 8–64 位，超出範圍程式會啟動失敗。
    - 不設定時，程式會生成 16 位強隨機密碼，並僅在首次啟動日誌中打印一次。
    - 請在首次登入後立即修改 root 密碼，並清理含有初始密碼的啟動日誌。
    - 該變數**僅首次建庫生效**，之後修改或刪除都不影響已存在的 root 密碼，不能當作逃生手段；關閉密碼登入前請先讀[管理員逃生口](./index.md#管理員逃生口)。
27. `ROOT_PASSWORD_ALLOW_INSECURE`：是否跳過 `ROOT_PASSWORD` 的 8–64 位長度校驗，可選值 `true` / `false`，預設 `false`。**僅限本地開發環境，生產環境嚴禁開啟。**
    - 本地開發約定：`ROOT_PASSWORD=root` + `ROOT_PASSWORD_ALLOW_INSECURE=true`，首次啟動後即可用 `root/root` 登入。
    - 開啟後啟動日誌會打印一行警告；未設定 `ROOT_PASSWORD` 時該開關不生效，仍走 16 位隨機密碼分支。

## 更多配置項

以下為上方未展開、但代碼實際讀取的常用配置項，完整列表與預設值請以交付包根目錄的 `config.example.yaml` 為準。

### 伺服器

- `PORT`：監聽端口，預設 `3000`。
- `GIN_MODE`：Gin 運行模式，`release` / `debug`，預設 `release`。
- `HTTPS`：是否以 HTTPS 對外（影響 cookie `Secure` 等），預設 `false`。
- `TRUSTED_HEADER`：獲取真實客戶端 IP 的請求頭，例如 `CF-Connecting-IP`。
- `SHUTDOWN_TIMEOUT`：優雅關閉逾時，單位秒，預設 `30`。
- `PPROF_ENABLED`：是否開啟 pprof 性能分析，預設 `false`。
- `LANGUAGE`：預設語言，預設 `zh_CN`。
- `FAVICON`：自訂 favicon 路徑。
- `GITHUB_PROXY`：外部資源下載的代理前綴。

### 數據庫與 Redis

- `SQLITE_PATH`：SQLite 數據庫檔案路徑，預設 `modeltaps.db`。
- `BRAND_ICON_DIR`：品牌圖示同步層的本機目錄，每個版本存為 `{目錄}/{version}/`，預設 `data/brand-icons`。
- `REDIS_DB`：Redis DB 序號（連接串未含 DB 時生效），預設 `0`。
- `REDIS_POOL_SIZE`：連接池大小，預設 `100`。
- `REDIS_MIN_IDLE_CONNS`：最小空閒連接數，預設 `10`。
- `REDIS_POOL_TIMEOUT` / `REDIS_READ_TIMEOUT` / `REDIS_WRITE_TIMEOUT`：連接池等待 / 讀 / 寫逾時，單位秒，預設 `5` / `2` / `2`。

### HTTP 客戶端 / 中繼逾時

- `CONNECT_TIMEOUT`：連接逾時，單位秒，預設 `5`。
- `RELAY_REQUEST_TIMEOUT`：中繼單次請求逾時，單位秒，預設 `300`。
- `STREAM_IDLE_TIMEOUT`：串流空閒逾時，渠道串流回應期間每收到數據即重置計時，靜默超過該時長則中止串流（與牆鐘總逾時互補，用於精準處理"卡死串流"），單位秒，預設 `300`，設為 `0` 禁用。
- `RESPONSE_HEADER_TIMEOUT`：回應頭逾時，單位秒，預設 `120`。
- `TLS_HANDSHAKE_TIMEOUT`：TLS 握手逾時，單位秒，預設 `30`。
- `TLS_INSECURE_SKIP_VERIFY`：跳過 TLS 憑證校驗（不安全），預設 `false`。
- `MAX_CONNS_PER_HOST` / `MAX_IDLE_CONNS` / `MAX_IDLE_CONNS_PER_HOST`：連接數上限，預設 `0`（不限）/ `1000` / `200`。

### 價格 / 計費

- `CATALOG_PRICING_URL`：模型目錄價格同步源地址。
- `CATALOG_PRICING_AUTO_SYNC`：是否自動同步目錄價格，預設 `true`。
- `CHANNEL_PRICING_AUTO_SYNC`：是否自動同步渠道價格，預設 `true`。
- `UNPRICED_MODEL_POLICY`：未定價模型策略，預設 `block`。
- `UNPRICED_MODEL_DEFAULT_RATIO`：未定價模型預設倍率，預設 `30.0`。
- `MODEL_DRIFT_AUTO_CHECK`：是否自動檢測模型漂移，預設 `false`。

### 日誌 / 指標 / 集成

- `LOG_DIR`：日誌目錄，預設 `./logs`。
- `LOG_LEVEL`：日誌級別，`debug` 輸出更詳細。
- `LOGS_FILENAME` / `LOGS_MAX_SIZE` / `LOGS_MAX_AGE` / `LOGS_MAX_BACKUP` / `LOGS_COMPRESS`：日誌檔案名及輪轉設定，預設 `modeltaps.log` / `100`(MB) / `7`(天) / `10` / `false`。
- `METRICS_USER` / `METRICS_PASSWORD`：`/metrics` 介面 Basic Auth，留空則不鑑權。
- `MCP_ENABLE`：是否啟用 MCP，預設 `false`。
- `UPTIME_KUMA_ENABLE` / `UPTIME_KUMA_DOMAIN` / `UPTIME_KUMA_STATUS_PAGE_NAME`：Uptime Kuma 狀態頁集成。
- `DISABLE_TOKEN_ENCODERS`：禁用本地 token 編碼器（改用估算），預設 `false`。
- `TG_HTTP_PROXY`：訪問 Telegram 的 HTTP/SOCKS5 代理。

### 通知 / 儲存 / 搜尋

通知（`NOTIFY_*`）、對象存儲圖床（`STORAGE_*`）、聯網搜索（`SEARCH_*`）等分組配置項較多，鍵名與預設值請直接參考交付包根目錄的 `config.example.yaml`。

