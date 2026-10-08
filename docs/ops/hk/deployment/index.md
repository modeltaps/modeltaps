---
title: "部署說明"
layout: doc
outline: deep
lastUpdated: true
---

# 部署說明

::: warning 數據庫獨立性
Modeltaps 使用自有的數據庫結構，請為其準備獨立的數據庫，不要與其他服務共用同一套數據表。
:::

## 配置說明

系統支援兩種配置方式：

1. 環境變數
2. 設定檔 (config.yaml)

::: tip 配置優先級
環境變數 > 設定檔
:::

### 必要配置

- `USER_TOKEN_SECRET`: 必填，用於生成用戶令牌的密鑰，長度需大於 32 位，設定後請勿修改，否則會導致已簽發的用戶令牌失效
- `SESSION_SECRET`: 推薦填寫，用於保持用戶登入狀態，如果不設定，每次重啟後已登入用戶需要重新登入

兩者需要分別生成互不相同的強隨機值：

```bash
openssl rand -base64 48 | tr -d '\n'; echo
```

::: danger 請勿直接使用文檔中的示例值
本頁出現的密鑰、密碼、數據庫連接串**全部是佔位符**（形如 `<...>`），僅用於說明格式，**不可用於生產**。真實值請寫入宿主機上的 `.env` 檔案（並執行 `chmod 600`）或密鑰管理服務，不要直接寫進命令列參數、`docker-compose.yml`，更不要提交到版本庫。
:::

## 登入私有鏡像倉庫

Modeltaps 鏡像發佈在私有鏡像倉庫 `ghcr.io/modeltaps/modeltaps`。拉取前**必須先登入**，否則下面的 `docker pull` / `docker run` / `docker-compose up` 會因未授權而失敗。

::: tip 權限說明
該鏡像倉庫啟用了訪問控制，僅授權帳戶可拉取。若登入後仍無法拉取，請聯絡交付方為對應帳戶授予該鏡像的唯讀權限。
:::

1. **準備訪問令牌**

   使用具備 `read:packages`（唯讀拉取包）權限的訪問令牌，不要授予多餘權限。

   ::: warning GHCR 只接受經典 PAT
   GitHub Container Registry 只認**經典（classic）Personal Access Token**，細粒度（fine-grained）令牌不支援 Packages 權限，拿它登入會一直報 `unauthorized`。
   :::

2. **登入（stdin 方式）**

   ```shell
   echo "$CR_PAT" | docker login ghcr.io -u <USERNAME> --password-stdin
   ```

   ::: warning 不要洩露 token
   請透過環境變數或標準輸入把令牌傳給 `docker login`，**不要**把它寫進命令列參數、腳本或明文檔案（否則會進入 shell 歷史與程序列表）。示例中的 `$CR_PAT` 應來自本機安全設定的環境變數，`<USERNAME>` 替換為實際帳戶名。
   :::

3. **驗證拉取**

   ```shell
   docker pull ghcr.io/modeltaps/modeltaps:latest
   ```

   能成功拉取即表示登入有效。

   ::: warning 用哪個系統用戶登入，就只有那個用戶能拉
   `docker login` 的憑證寫在**執行登入的那個作業系統用戶**的 `~/.docker/config.json` 內，用戶之間不共用。因此要用**實際運行 `docker compose` 的那個用戶**登入：習慣用 `sudo docker compose` 的話就得跑 `sudo docker login ghcr.io`（憑證落在 root 的 `/root/.docker/config.json`），否則拉取仍會報 `unauthorized`。
   :::

## Docker 部署

### 準備工作

1. 建立數據目錄：

```bash
# 建立主數據目錄
sudo mkdir -p /data/modeltaps
cd /data/modeltaps
```

::: tip 收緊數據目錄權限
該目錄將存放 SQLite 數據庫、密鑰檔案與日誌。`modeltaps.db` 由程式以 `0644` 權限建立，目錄本身若對其他用戶開放，宿主機上的任何用戶都能直接讀取數據庫。寫好 `.env` / `config.yaml` 後、啟動容器前，請把目錄屬主改為容器內的用戶（UID 10001）並設為僅屬主可存取：

```bash
sudo chown -R 10001:10001 /data/modeltaps
sudo chmod 700 /data/modeltaps
```

完整步驟見 [目錄權限收緊](#目錄權限收緊)。
:::

2. 確保 Docker 已正確安裝並啟動：

```bash
# 檢查 Docker 狀態
sudo systemctl status docker
# 如果未啟動，則啟動 Docker
sudo systemctl start docker
```

::: warning 注意

- `-p 3000:3000` 中的第一個 `3000` 是宿主機的端口，可以根據需要進行修改。
- 數據和日誌將會儲存在宿主機的 `/data/modeltaps` 目錄，請確保該目錄存在且具有寫入權限，或者更改為合適的目錄。該目錄內含 SQLite 數據庫與日誌檔案，屬於敏感數據，請按 [目錄權限收緊](#目錄權限收緊) 設定擁有者與權限。
- 如果啟動失敗，請新增 `--privileged=true`。
- 併發量較大時，**務必**設定 `SQL_DSN`。
  :::

### 使用環境變數部署

更多環境變數說明請參考 [環境變數](./env.md)。

推薦先把密鑰寫進宿主機的 `.env` 檔案，再用 `--env-file` 注入容器，這樣密鑰不會進入 shell 歷史與程序列表：

```bash
# /data/modeltaps/.env —— 下列尖括號內容均為佔位符，請替換為你自己生成的值
TZ=Asia/Shanghai
USER_TOKEN_SECRET=<openssl rand -base64 48 生成的隨機值>
SESSION_SECRET=<另行生成的一個隨機值>
```

寫好後收緊檔案權限，僅擁有者可讀寫：

```bash
chmod 600 /data/modeltaps/.env
```

#### 使用 SQLite

```shell
docker run -d -p 3000:3000 \
  --name modeltaps \
  --restart always \
  --env-file /data/modeltaps/.env \
  -v /data/modeltaps:/data \
  ghcr.io/modeltaps/modeltaps:latest
```

#### 使用 MySQL

在 `.env` 中追加 `SQL_DSN`，容器啟動命令與 SQLite 完全一致：

```bash
SQL_DSN=<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps
```

#### 使用 PostgreSQL

```bash
SQL_DSN=postgres://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/modeltaps
```

::: warning 數據庫帳戶
請為 Modeltaps 單獨建立數據庫帳戶並設定強密碼，不要複用 `root` 等超級用戶帳戶。
:::

部署完畢後，訪問 `http://localhost:3000` 即可。首次登入後請先 [設定伺服器地址](#設定伺服器地址)。

### 使用設定檔部署

1. 準備設定檔模板：設定檔示例 `config.example.yaml` 隨交付包一同提供（位於交付包根目錄），把它複製到數據目錄並改名為 `config.yaml`：

```bash
cd /data/modeltaps
cp <交付包目錄>/config.example.yaml ./config.yaml
```

2. 根據需要修改設定檔內容，常用配置項包括：

```yaml
# 必要配置（尖括號內容為佔位符，請替換為自行生成的強隨機值，勿用於生產）
user_token_secret: "<openssl rand -base64 48 生成的隨機值>" # 用戶令牌密鑰
session_secret: "<另行生成的一個隨機值>" # 會話密鑰

# 數據庫配置
sql_dsn: "<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps" # MySQL 配置示例
```

::: warning 設定檔含明文密鑰
`config.yaml` 會以明文儲存密鑰，請執行 `chmod 600 config.yaml` 收緊權限，並確保它不會被提交到版本庫。若不希望密鑰落盤，可改用上一節的 `.env` + 環境變數方式。
:::

3. 運行容器

```shell
docker run -d -p 3000:3000 \
  --name modeltaps \
  --restart always \
  -e TZ=Asia/Shanghai \
  -v /data/modeltaps:/data \
  ghcr.io/modeltaps/modeltaps:latest
```

## Docker Compose 部署

::: warning 請先登入私有鏡像倉庫
私有鏡像需要先完成 [登入私有鏡像倉庫](#登入私有鏡像倉庫)，否則 `docker-compose up` 拉取鏡像會失敗。
:::

### 準備工作

1. 建立必要的目錄結構：

```bash
# 建立主目錄
sudo mkdir -p /data/modeltaps
cd /data/modeltaps
# 建立子目錄
mkdir data
```

::: tip 收緊數據目錄權限
`data` 子目錄掛載為容器內的 `/data`，將存放 SQLite 數據庫、設定檔與日誌；`modeltaps.db` 由程式以 `0644` 權限建立，目錄若對其他用戶開放，宿主機上的任何用戶都能直接讀取數據庫。**首次啟動前**請把它交給容器內的用戶（UID 10001）並設為僅屬主可存取：

```bash
sudo chown -R 10001:10001 data
sudo chmod 700 data
```

完整步驟見 [目錄權限收緊](#目錄權限收緊)。
:::

2. 準備編排檔案與環境變數模板：`docker-compose.yml` 與 `.env.example` 位於交付包根目錄，複製到當前目錄，並把環境變數模板另存為 `.env`：

```bash
cp <交付包目錄>/docker-compose.yml ./
cp <交付包目錄>/.env.example ./.env
```

3. 編輯 `.env` 填入密鑰，**不要**直接修改 `docker-compose.yml` 中的密鑰：

`docker-compose.yml` 中的密鑰全部寫成 `${VAR:?...}`，會自動從同目錄的 `.env` 讀取；任一項缺失時 `docker-compose up` 會直接報錯退出。至少需要填寫 `SESSION_SECRET`、`USER_TOKEN_SECRET`、`MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD`，取值請用命令生成：

```bash
# 會話與令牌密鑰
openssl rand -base64 48 | tr -d '\n'; echo
# 數據庫口令
openssl rand -base64 24 | tr -d '\n'; echo
```

寫好後收緊檔案權限：

```bash
chmod 600 .env
```

::: warning 不要提交 .env
`.env` 內含明文密鑰，請確保它已被 `.gitignore` 忽略，也不要隨部署包一起分發。
:::

如果改用設定檔，執行下面命令，並刪除 `docker-compose.yml` 檔案中的 `SQL_DSN`/`REDIS_CONN_STRING`/`SESSION_SECRET` / `USER_TOKEN_SECRET` 參數：

```shell
# 複製應用設定檔模板
cp <交付包目錄>/config.example.yaml ./data/config.yaml
```

### 啟動服務

```shell
docker-compose up -d
```

啟動服務後，可透過以下命令查看部署狀態：

```shell
docker-compose ps
```

請確保所有的服務都已經成功啟動，並且狀態為 'Up'。

部署完畢後，訪問 `http://localhost:3000` 即可。首次登入後請先 [設定伺服器地址](#設定伺服器地址)。

### 可選：同時啟動本地文檔站

`docker-compose.yml` 中還有一個預設**不啟動**的 `docs` 服務，用於在內網或離線環境自建一份本文檔站。它由 profile 控制，普通的 `docker-compose up -d` 完全不受影響：

```shell
docker compose --profile docs up -d
```

啟動後訪問 `http://localhost:3001` 即可（`/llms.txt`、`/llms-full.txt` 也一併提供）。

::: warning 需要完整交付包
該服務是本地構建（構建上下文為交付包中的 `docs/` 目錄），而非拉取鏡像，因此只能在擁有完整交付包的機器上使用；僅取用 `docker-compose.yml` 時無法啟動該服務。首次構建需要聯網安裝文檔站依賴。
:::

只停掉文檔站、保留網關服務：

```shell
docker compose --profile docs stop docs
```

## 手動部署

除容器鏡像外，交付包還提供對應平台的 `modeltaps` 可執行檔案，可直接在宿主機上運行。

1. **放置可執行檔案**：把交付包中的 `modeltaps` 可執行檔案複製到安裝目錄（例如 `/opt/modeltaps`）。

2. **運行應用**：新增執行權限並運行：

   ```shell
   chmod u+x modeltaps
   ./modeltaps --port 3000 --log-dir ./logs
   ```

3. **訪問應用**：在瀏覽器中訪問 `http://localhost:3000` 並登入。初始帳戶用戶名為 `root`，初始密碼取決於首次啟動時的配置：

   - 設定了 `ROOT_PASSWORD`（長度 8–64 位）：使用該值作為 root 初始密碼；長度不合規時程式會啟動失敗。
   - 未設定 `ROOT_PASSWORD`：程式會用 `crypto/rand` 生成 16 位強隨機密碼，並**僅在首次啟動日誌中打印一次**，形如：

     ```text
     no user exists, create a root user for you: username is root, initial root password: <隨機密碼>
     ```

   本地開發約定：設定 `ROOT_PASSWORD=root` 與 `ROOT_PASSWORD_ALLOW_INSECURE=true`，首次啟動後即可用 `root/root` 登入；該開關僅限本地開發，生產環境嚴禁開啟。

   ::: warning 初始密碼
   該邏輯只在數據庫中還沒有任何用戶時生效。請在首次登入後立即修改 root 密碼，並清理含有初始密碼的啟動日誌（見 [目錄權限收緊](#目錄權限收緊)）。
   :::

運行方式與配置項和容器部署一致：既可用 `.env` / 環境變數注入，也可用 `--config ./config.yaml` 指定設定檔（見 [命令列參數](./cli.md)）。生產環境請按 [裸機（手動）部署](#裸機-手動-部署) 收緊目錄與檔案權限。

## 設定伺服器地址

伺服器地址的出廠預設值是 `http://localhost:3000`。只要站點透過其他網域、IP 或連接埠對外訪問（包括經反向代理訪問），首次登入後就要立即修改：

1. 以 root 或管理員帳戶登入，進入「系統設定 → 站點」，找到「伺服器設定」卡片。
2. 在「伺服器地址」中填寫用戶實際訪問站點用的完整地址，包含協定，非預設連接埠也要寫上，例如 `https://yourdomain.com`。
3. 點擊「保存」。

後端用這個地址拼接所有對外連結，保持預設值時以下內容都會指向 `localhost`，用戶無法使用：

- 「API Key」頁面顯示的 Base URL 與示例命令；
- 找回密碼等郵件中的連結；
- OAuth / OIDC 登入回調與支付回調；
- 通行密鑰（WebAuthn）綁定的站點網域。

仍為預設值時，該卡片會顯示警告，並以目前瀏覽器訪問的地址作為建議值。

::: warning 先設地址，再註冊通行密鑰
通行密鑰與註冊時伺服器地址的網域綁定，之後更換網域會令已註冊的通行密鑰失效。請先設好伺服器地址並重啟一次服務，再按 [管理員逃生口](#管理員逃生口) 為 root 註冊通行密鑰。
:::

## 目錄權限收緊

日誌目錄與數據目錄都會落盤敏感內容，預設權限**不足以**防止同一台機器上的其他用戶讀取，部署後請手動收緊。

### 為什麼要收緊

**1. 日誌目錄會寫入 root 初始密碼**

首次啟動時（數據庫中還沒有任何用戶），若未設定 `ROOT_PASSWORD`，程式會生成 16 位隨機密碼並**以明文打印一次**：

```text
no user exists, create a root user for you: username is root, initial root password: <隨機密碼>
```

這條日誌會同時寫到兩個地方：

- **日誌檔案**：路徑為 `LOG_DIR` / `LOGS_FILENAME`（預設 `./logs` / `modeltaps.log`，見 [環境變數](./env.md)、[命令列參數](./cli.md)）。容器鏡像的工作目錄是 `/data`，因此預設落在 `/data/logs/modeltaps.log`，即宿主機掛載點下的 `logs/modeltaps.log`。
- **程序標準錯誤**：即 `docker logs modeltaps` / systemd journal 能看到的輸出。

權限現狀需要注意兩點：

- 日誌目錄若不存在，由程式自行建立，建立時請求的權限為 `0777`（實際值受 umask 影響，通常為 `0755`），**同機其他用戶可進入並讀取**。
- 日誌檔案由 lumberjack 新建時為 `0600`，但輪轉時會**沿用已存在檔案的權限**——如果你事先手工建立過一個寬權限的 `modeltaps.log`，這個寬權限會被一直繼承下去。

**2. 數據目錄含數據庫與明文配置**

掛載點（示例中的 `/data/modeltaps`）下通常包含：

- `modeltaps.db`：SQLite 數據庫（`SQLITE_PATH`，預設相對工作目錄，即 `/data/modeltaps.db`），內含用戶密碼哈希、用戶令牌、渠道 API Key 等；程式以 `0644` 權限建立該檔案，同機其他用戶預設可讀；
- `config.yaml`：明文儲存 `user_token_secret`、`session_secret`、`sql_dsn` 等；
- `.env`：明文密鑰；
- `logs/`：上面提到的日誌檔案。

::: tip 圖片上傳不落本地盤
圖床上傳（`STORAGE_*`）一律直傳遠端服務（sm.ms / imgur / 阿里雲 OSS / S3 協議），Modeltaps **不會**在本機建立上傳目錄，因此無需為上傳內容單獨設權限——需要保護的是儲存這些圖床憑據的 `config.yaml` / `.env`。詳見 [圖床設定](./storage.md)。
:::

### Docker / Docker Compose 部署

鏡像內以固定的非 root 用戶 `modeltaps`（UID/GID 均為 `10001`）運行，宿主機掛載目錄的擁有者需與之對齊，否則容器無寫權限：

```bash
# 擁有者對齊到容器內的 modeltaps(10001)
sudo chown -R 10001:10001 /data/modeltaps

# 掛載根目錄與日誌目錄：僅擁有者可讀寫執行
sudo chmod 700 /data/modeltaps
sudo chmod 700 /data/modeltaps/logs

# 敏感檔案：僅擁有者可讀寫（檔案不存在時可跳過對應項）
sudo chmod 600 /data/modeltaps/.env \
               /data/modeltaps/config.yaml \
               /data/modeltaps/modeltaps.db \
               /data/modeltaps/logs/modeltaps.log
```

目錄設為 `700` 後，宿主機上的普通運維帳戶需要 `sudo` 才能查看這些檔案，這正是預期效果。

### 裸機（手動）部署

不要用 root 直接跑，建議建一個無登入 shell 的專用系統用戶，並把安裝目錄交給它：

```bash
# 建立專用系統用戶
sudo useradd --system --home-dir /opt/modeltaps --shell /usr/sbin/nologin modeltaps

# 擁有者歸專用用戶，目錄僅擁有者可進入
sudo chown -R modeltaps:modeltaps /opt/modeltaps
sudo chmod 750 /opt/modeltaps
sudo chmod 700 /opt/modeltaps/logs

# 敏感檔案僅擁有者可讀寫
sudo chmod 600 /opt/modeltaps/config.yaml \
               /opt/modeltaps/modeltaps.db \
               /opt/modeltaps/logs/modeltaps.log
```

以該用戶啟動（`--log-dir` 顯式指向已收緊的目錄，避免程式用寬 umask 新建）：

```bash
sudo -u modeltaps /opt/modeltaps/modeltaps --port 3000 --log-dir /opt/modeltaps/logs
```

::: tip 先建目錄再啟動
先用上述命令把 `logs` 目錄建好並設為 `700`，程式即不會走"自動建立 `0777`"這條路徑。
:::

### 清理已寫入的初始密碼

改完 root 密碼後，初始密碼在日誌裡仍是明文，需要一併清掉：

```bash
# 1. 確認日誌中是否存在該行
sudo grep -l "initial root password" /data/modeltaps/logs/*.log

# 2. 刪除含該行的日誌檔案（含輪轉產生的歷史檔案）
sudo rm -f /data/modeltaps/logs/modeltaps.log /data/modeltaps/logs/modeltaps-*.log
```

容器的標準輸出另存於 Docker 自己的日誌檔案，`rm` 上面的檔案並不會清掉它。確認與清理方式：

```bash
# 確認
docker logs modeltaps 2>&1 | grep "initial root password"
# 清理：重建容器（數據在掛載卷中，不會丟）
docker rm -f modeltaps && docker run -d ...   # 用原來的啟動命令重新建立
```

::: tip 更徹底的做法：首啟前就設好 ROOT_PASSWORD
在**首次啟動前**透過 `.env` 設定 `ROOT_PASSWORD`（8–64 位，長度不合規程式會啟動失敗），程式即不會生成隨機密碼，日誌中只會出現 `password is set via ROOT_PASSWORD env`，不含密碼本身。參見 [環境變數](./env.md#環境變數說明) 的 `ROOT_PASSWORD` 條目。
:::

## 管理員逃生口

網站把帳戶體系切換為外部**身份提供方**後，普通用戶只能經身份提供方登入。為免身份提供方不可用（密鑰輪換、域名變更、停止服務、配置出錯）時無人能進入後台，Modeltaps 為 root 帳戶保留了一條不依賴身份提供方的登入路徑。

- **逃生入口**：外部身份提供方模式下，訪問 `/login/admin` 會顯示管理員登入表單（密碼或通行密鑰），而且只有 root 帳戶能通過；其他帳戶在該表單提交一律拒絕。該入口沿用密碼登入的全部校驗（帳戶鎖定、失敗計數、人機校驗），安全等級不降低。舊地址 `/login?local=1` 永久跳轉到這裡。
- **可以關閉**：後台「系統設定 → 登入方式 → 管理員應急登入」（`AdminLoginEnabled`，預設開啟）能關掉該入口；關閉後 `/login/admin` 與 root 的本站密碼、通行密鑰一併失效，只剩身份提供方一條路，請確認提供方自身有災備再關。
- **切換前置**：切換到外部身份提供方**之前**，請先為 root 設定一個強密碼（建議 16 位以上，保存在密碼管理器中），並在「設定 → 登入與安全 → 登入方式 → 通行密鑰」註冊至少一枚通行密鑰，作為第二條不依賴身份提供方的路徑。
- **隨時輪換**：外部模式下 root 仍可在「設定 → 登入與安全 → 管理員應急登入」修改該密碼、增刪通行密鑰；其餘帳戶在此狀態下不能設定或修改本站密碼。
- **`ROOT_PASSWORD` 只是初始值**：該環境變數僅在首次建庫（數據庫中還沒有任何用戶）時生效，之後修改或刪除都不影響已存在的 root 密碼，因此不能當作逃生手段。參見 [環境變數](./env.md#環境變數說明) 的 `ROOT_PASSWORD` 條目。

## 多機部署

### 準備工作

1. 確保所有伺服器都安裝了必要的組件：

- Docker 或 手動部署所需的組件
- Redis（如果需要使用快取）
- MySQL 客戶端（如果使用遠端 MySQL）

2. 網絡配置：

- 確保所有伺服器能夠訪問主數據庫
- 如果使用 Redis，確保可以訪問 Redis 伺服器
- 檢查伺服器間的防火牆設定

1. 所有伺服器 `SESSION_SECRET` 設定一樣的值。
2. 必須設定 `SQL_DSN`，使用 MySQL 數據庫而非 SQLite，所有伺服器連接同一個數據庫。
3. 所有從伺服器必須設定 `NODE_TYPE` 為 `slave`，不設定則預設為主伺服器。
4. 設定 `SYNC_FREQUENCY` 後伺服器將定期從數據庫同步配置，在使用遠端數據庫的情況下，推薦設定該項並啟用 Redis，無論主從。
5. 從伺服器可以選擇設定 `FRONTEND_BASE_URL`，以重定向頁面請求到主伺服器。
6. 從伺服器上**分別**裝好 Redis，設定好 `REDIS_CONN_STRING`，這樣可以做到在快取未過期的情況下數據庫零訪問，可以減少延遲。
7. 如果主伺服器訪問數據庫延遲也比較高，則也需要啟用 Redis，並設定 `SYNC_FREQUENCY`，以定期從數據庫同步配置。
