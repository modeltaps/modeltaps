---
title: "部署说明"
layout: doc
outline: deep
lastUpdated: true
---

# 部署说明

::: warning 数据库独立性
Modeltaps 使用自有的数据库结构，请为其准备独立的数据库，不要与其他服务共用同一套数据表。
:::

## 配置说明

系统支持两种配置方式：

1. 环境变量
2. 配置文件 (config.yaml)

::: tip 配置优先级
环境变量 > 配置文件
:::

### 必要配置

- `USER_TOKEN_SECRET`: 必填，用于生成用户令牌的密钥，长度需大于 32 位，设置后请勿修改，否则会导致已签发的用户令牌失效
- `SESSION_SECRET`: 推荐填写，用于保持用户登录状态，如果不设置，每次重启后已登录用户需要重新登录

两者需要分别生成互不相同的强随机值：

```bash
openssl rand -base64 48 | tr -d '\n'; echo
```

::: danger 请勿直接使用文档中的示例值
本页出现的密钥、密码、数据库连接串**全部是占位符**（形如 `<...>`），仅用于说明格式，**不可用于生产**。真实值请写入宿主机上的 `.env` 文件（并执行 `chmod 600`）或密钥管理服务，不要直接写进命令行参数、`docker-compose.yml`，更不要提交到版本库。
:::

## 登录私有镜像仓库

Modeltaps 镜像发布在私有镜像仓库 `ghcr.io/modeltaps/modeltaps`。拉取前**必须先登录**，否则下面的 `docker pull` / `docker run` / `docker-compose up` 会因未授权而失败。

::: tip 权限说明
该镜像仓库启用了访问控制，仅授权账号可拉取。若登录后仍无法拉取，请联系交付方为对应账号授予该镜像的只读权限。
:::

1. **准备访问令牌**

   使用具备 `read:packages`（只读拉取包）权限的访问令牌，不要授予多余权限。

   ::: warning GHCR 只接受经典 PAT
   GitHub Container Registry 只认**经典（classic）Personal Access Token**，细粒度（fine-grained）令牌不支持 Packages 权限，拿它登录会一直报 `unauthorized`。
   :::

2. **登录（stdin 方式）**

   ```shell
   echo "$CR_PAT" | docker login ghcr.io -u <USERNAME> --password-stdin
   ```

   ::: warning 不要泄露 token
   请通过环境变量或标准输入把令牌传给 `docker login`，**不要**把它写进命令行参数、脚本或明文文件（否则会进入 shell 历史与进程列表）。示例中的 `$CR_PAT` 应来自本机安全设置的环境变量，`<USERNAME>` 替换为实际账号名。
   :::

3. **验证拉取**

   ```shell
   docker pull ghcr.io/modeltaps/modeltaps:latest
   ```

   能成功拉取即表示登录有效。

   ::: warning 用哪个系统用户登录，就只有那个用户能拉
   `docker login` 的凭据写在**执行登录的那个操作系统用户**的 `~/.docker/config.json` 里，用户之间不共用。因此要用**实际运行 `docker compose` 的那个用户**登录：习惯用 `sudo docker compose` 的话就得跑 `sudo docker login ghcr.io`（凭据落在 root 的 `/root/.docker/config.json`），否则拉取仍会报 `unauthorized`。
   :::

## Docker 部署

### 准备工作

1. 创建数据目录：

```bash
# 创建主数据目录
sudo mkdir -p /data/modeltaps
cd /data/modeltaps
```

::: tip 收紧数据目录权限
该目录将存放 SQLite 数据库、密钥文件与日志。`modeltaps.db` 由程序以 `0644` 权限创建，目录本身若对其他用户开放，宿主机上的任何用户都能直接读取数据库。写好 `.env` / `config.yaml` 后、启动容器前，请把目录属主改为容器内的用户（UID 10001）并设为仅属主可访问：

```bash
sudo chown -R 10001:10001 /data/modeltaps
sudo chmod 700 /data/modeltaps
```

完整步骤见 [目录权限收紧](#目录权限收紧)。
:::

2. 确保 Docker 已正确安装并启动：

```bash
# 检查 Docker 状态
sudo systemctl status docker
# 如果未启动，则启动 Docker
sudo systemctl start docker
```

::: warning 注意

- `-p 3000:3000` 中的第一个 `3000` 是宿主机的端口，可以根据需要进行修改。
- 数据和日志将会保存在宿主机的 `/data/modeltaps` 目录，请确保该目录存在且具有写入权限，或者更改为合适的目录。该目录内含 SQLite 数据库与日志文件，属于敏感数据，请按 [目录权限收紧](#目录权限收紧) 设置属主与权限。
- 如果启动失败，请添加 `--privileged=true`。
- 并发量较大时，**务必**设置 `SQL_DSN`。
  :::

### 使用环境变量部署

更多环境变量说明请参考 [环境变量](./env.md)。

推荐先把密钥写进宿主机的 `.env` 文件，再用 `--env-file` 注入容器，这样密钥不会进入 shell 历史与进程列表：

```bash
# /data/modeltaps/.env —— 下列尖括号内容均为占位符，请替换为你自己生成的值
TZ=Asia/Shanghai
USER_TOKEN_SECRET=<openssl rand -base64 48 生成的随机值>
SESSION_SECRET=<另行生成的一个随机值>
```

写好后收紧文件权限，仅属主可读写：

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

在 `.env` 中追加 `SQL_DSN`，容器启动命令与 SQLite 完全一致：

```bash
SQL_DSN=<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps
```

#### 使用 PostgreSQL

```bash
SQL_DSN=postgres://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/modeltaps
```

::: warning 数据库账号
请为 Modeltaps 单独创建数据库账号并设置强密码，不要复用 `root` 等超级用户账号。
:::

部署完毕后，访问 `http://localhost:3000` 即可。首次登录后请先 [设置服务器地址](#设置服务器地址)。

### 使用配置文件部署

1. 准备配置文件模板：配置文件示例 `config.example.yaml` 随交付包一同提供（位于交付包根目录），把它复制到数据目录并改名为 `config.yaml`：

```bash
cd /data/modeltaps
cp <交付包目录>/config.example.yaml ./config.yaml
```

2. 根据需要修改配置文件内容，常用配置项包括：

```yaml
# 必要配置（尖括号内容为占位符，请替换为自行生成的强随机值，勿用于生产）
user_token_secret: "<openssl rand -base64 48 生成的随机值>" # 用户令牌密钥
session_secret: "<另行生成的一个随机值>" # 会话密钥

# 数据库配置
sql_dsn: "<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps" # MySQL 配置示例
```

::: warning 配置文件含明文密钥
`config.yaml` 会以明文保存密钥，请执行 `chmod 600 config.yaml` 收紧权限，并确保它不会被提交到版本库。若不希望密钥落盘，可改用上一节的 `.env` + 环境变量方式。
:::

3. 运行容器

```shell
docker run -d -p 3000:3000 \
  --name modeltaps \
  --restart always \
  -e TZ=Asia/Shanghai \
  -v /data/modeltaps:/data \
  ghcr.io/modeltaps/modeltaps:latest
```

## Docker Compose 部署

::: warning 请先登录私有镜像仓库
私有镜像需要先完成 [登录私有镜像仓库](#登录私有镜像仓库)，否则 `docker-compose up` 拉取镜像会失败。
:::

### 准备工作

1. 创建必要的目录结构：

```bash
# 创建主目录
sudo mkdir -p /data/modeltaps
cd /data/modeltaps
# 创建子目录
mkdir data
```

::: tip 收紧数据目录权限
`data` 子目录挂载为容器内的 `/data`，将存放 SQLite 数据库、配置文件与日志；`modeltaps.db` 由程序以 `0644` 权限创建，目录若对其他用户开放，宿主机上的任何用户都能直接读取数据库。**首次启动前**请把它交给容器内的用户（UID 10001）并设为仅属主可访问：

```bash
sudo chown -R 10001:10001 data
sudo chmod 700 data
```

完整步骤见 [目录权限收紧](#目录权限收紧)。
:::

2. 准备编排文件与环境变量模板：`docker-compose.yml` 与 `.env.example` 位于交付包根目录，复制到当前目录，并把环境变量模板另存为 `.env`：

```bash
cp <交付包目录>/docker-compose.yml ./
cp <交付包目录>/.env.example ./.env
```

3. 编辑 `.env` 填入密钥，**不要**直接修改 `docker-compose.yml` 中的密钥：

`docker-compose.yml` 中的密钥全部写成 `${VAR:?...}`，会自动从同目录的 `.env` 读取；任一项缺失时 `docker-compose up` 会直接报错退出。至少需要填写 `SESSION_SECRET`、`USER_TOKEN_SECRET`、`MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD`，取值请用命令生成：

```bash
# 会话与令牌密钥
openssl rand -base64 48 | tr -d '\n'; echo
# 数据库口令
openssl rand -base64 24 | tr -d '\n'; echo
```

写好后收紧文件权限：

```bash
chmod 600 .env
```

::: warning 不要提交 .env
`.env` 内含明文密钥，请确保它已被 `.gitignore` 忽略，也不要随部署包一起分发。
:::

如果改用配置文件，执行下面命令，并删除 `docker-compose.yml` 文件中的 `SQL_DSN`/`REDIS_CONN_STRING`/`SESSION_SECRET` / `USER_TOKEN_SECRET` 参数：

```shell
# 复制应用配置文件模板
cp <交付包目录>/config.example.yaml ./data/config.yaml
```

### 启动服务

```shell
docker-compose up -d
```

启动服务后，可通过以下命令查看部署状态：

```shell
docker-compose ps
```

请确保所有的服务都已经成功启动，并且状态为 'Up'。

部署完毕后，访问 `http://localhost:3000` 即可。首次登录后请先 [设置服务器地址](#设置服务器地址)。

### 可选：同时启动本地文档站

`docker-compose.yml` 中还有一个默认**不启动**的 `docs` 服务，用于在内网或离线环境自建一份本文档站。它由 profile 控制，普通的 `docker-compose up -d` 完全不受影响：

```shell
docker compose --profile docs up -d
```

启动后访问 `http://localhost:3001` 即可（`/llms.txt`、`/llms-full.txt` 也一并提供）。

::: warning 需要完整交付包
该服务是本地构建（构建上下文为交付包中的 `docs/` 目录），而非拉取镜像，因此只能在拥有完整交付包的机器上使用；仅取用 `docker-compose.yml` 时无法启动该服务。首次构建需要联网安装文档站依赖。
:::

只停掉文档站、保留网关服务：

```shell
docker compose --profile docs stop docs
```

## 手动部署

除容器镜像外，交付包还提供对应平台的 `modeltaps` 可执行文件，可直接在宿主机上运行。

1. **放置可执行文件**：把交付包中的 `modeltaps` 可执行文件复制到安装目录（例如 `/opt/modeltaps`）。

2. **运行应用**：添加执行权限并运行：

   ```shell
   chmod u+x modeltaps
   ./modeltaps --port 3000 --log-dir ./logs
   ```

3. **访问应用**：在浏览器中访问 `http://localhost:3000` 并登录。初始账号用户名为 `root`，初始密码取决于首次启动时的配置：

   - 设置了 `ROOT_PASSWORD`（长度 8–64 位）：使用该值作为 root 初始密码；长度不合规时程序会启动失败。
   - 未设置 `ROOT_PASSWORD`：程序会用 `crypto/rand` 生成 16 位强随机密码，并**仅在首次启动日志中打印一次**，形如：

     ```text
     no user exists, create a root user for you: username is root, initial root password: <随机密码>
     ```

   本地开发约定：设置 `ROOT_PASSWORD=root` 与 `ROOT_PASSWORD_ALLOW_INSECURE=true`，首启后即可用 `root/root` 登录；该开关仅限本地开发，生产环境严禁开启。

   ::: warning 初始密码
   该逻辑只在数据库中还没有任何用户时生效。请在首次登录后立即修改 root 密码，并清理含有初始密码的启动日志（见 [目录权限收紧](#目录权限收紧)）。
   :::

运行方式与配置项和容器部署一致：既可用 `.env` / 环境变量注入，也可用 `--config ./config.yaml` 指定配置文件（见 [命令行参数](./cli.md)）。生产环境请按 [裸机（手动）部署](#裸机-手动-部署) 收紧目录与文件权限。

## 设置服务器地址

服务器地址的出厂默认值是 `http://localhost:3000`。只要站点通过其他域名、IP 或端口对外访问（包括经反向代理访问），首次登录后就要立即修改：

1. 以 root 或管理员账号登录，进入「系统设置 → 站点」，找到「服务器设置」卡片。
2. 在「服务器地址」中填写用户实际访问站点用的完整地址，包含协议，非默认端口也要写上，例如 `https://yourdomain.com`。
3. 点击「保存」。

后端用这个地址拼接所有对外链接，保持默认值时以下内容都会指向 `localhost`，用户无法使用：

- 「API Key」页面显示的 Base URL 与示例命令；
- 找回密码等邮件中的链接；
- OAuth / OIDC 登录回调与支付回调；
- 通行密钥（WebAuthn）绑定的站点域名。

仍为默认值时，该卡片会显示警告并给出当前浏览器访问的地址作为建议值。

::: warning 先设地址，再注册通行密钥
通行密钥与注册时服务器地址的域名绑定，之后更换域名会让已注册的通行密钥失效。请先设好服务器地址并重启一次服务，再按 [管理员逃生口](#管理员逃生口) 为 root 注册通行密钥。
:::

## 目录权限收紧

日志目录与数据目录都会落盘敏感内容，默认权限**不足以**防止同一台机器上的其他用户读取，部署后请手动收紧。

### 为什么要收紧

**1. 日志目录会写入 root 初始密码**

首次启动时（数据库中还没有任何用户），若未设置 `ROOT_PASSWORD`，程序会生成 16 位随机密码并**以明文打印一次**：

```text
no user exists, create a root user for you: username is root, initial root password: <随机密码>
```

这条日志会同时写到两个地方：

- **日志文件**：路径为 `LOG_DIR` / `LOGS_FILENAME`（默认 `./logs` / `modeltaps.log`，见 [环境变量](./env.md)、[命令行参数](./cli.md)）。容器镜像的工作目录是 `/data`，因此默认落在 `/data/logs/modeltaps.log`，即宿主机挂载点下的 `logs/modeltaps.log`。
- **进程标准错误**：即 `docker logs modeltaps` / systemd journal 能看到的输出。

权限现状需要注意两点：

- 日志目录若不存在，由程序自行创建，创建时请求的权限为 `0777`（实际值受 umask 影响，通常为 `0755`），**同机其他用户可进入并读取**。
- 日志文件由 lumberjack 新建时为 `0600`，但轮转时会**沿用已存在文件的权限**——如果你事先手工创建过一个宽权限的 `modeltaps.log`，这个宽权限会被一直继承下去。

**2. 数据目录含数据库与明文配置**

挂载点（示例中的 `/data/modeltaps`）下通常包含：

- `modeltaps.db`：SQLite 数据库（`SQLITE_PATH`，默认相对工作目录，即 `/data/modeltaps.db`），内含用户密码哈希、用户令牌、渠道 API Key 等；程序以 `0644` 权限创建该文件，同机其他用户默认可读；
- `config.yaml`：明文保存 `user_token_secret`、`session_secret`、`sql_dsn` 等；
- `.env`：明文密钥；
- `logs/`：上面提到的日志文件。

::: tip 图片上传不落本地盘
图床上传（`STORAGE_*`）一律直传远端服务（sm.ms / imgur / 阿里云 OSS / S3 协议），Modeltaps **不会**在本机创建上传目录，因此无需为上传内容单独设权限——需要保护的是保存这些图床凭据的 `config.yaml` / `.env`。详见 [图床配置](./storage.md)。
:::

### Docker / Docker Compose 部署

镜像内以固定的非 root 用户 `modeltaps`（UID/GID 均为 `10001`）运行，宿主机挂载目录的属主需与之对齐，否则容器无写权限：

```bash
# 属主对齐到容器内的 modeltaps(10001)
sudo chown -R 10001:10001 /data/modeltaps

# 挂载根目录与日志目录：仅属主可读写执行
sudo chmod 700 /data/modeltaps
sudo chmod 700 /data/modeltaps/logs

# 敏感文件：仅属主可读写（文件不存在时可跳过对应项）
sudo chmod 600 /data/modeltaps/.env \
               /data/modeltaps/config.yaml \
               /data/modeltaps/modeltaps.db \
               /data/modeltaps/logs/modeltaps.log
```

目录设为 `700` 后，宿主机上的普通运维账号需要 `sudo` 才能查看这些文件，这正是预期效果。

### 裸机（手动）部署

不要用 root 直接跑，建议建一个无登录 shell 的专用系统用户，并把安装目录交给它：

```bash
# 创建专用系统用户
sudo useradd --system --home-dir /opt/modeltaps --shell /usr/sbin/nologin modeltaps

# 属主归专用用户，目录仅属主可进入
sudo chown -R modeltaps:modeltaps /opt/modeltaps
sudo chmod 750 /opt/modeltaps
sudo chmod 700 /opt/modeltaps/logs

# 敏感文件仅属主可读写
sudo chmod 600 /opt/modeltaps/config.yaml \
               /opt/modeltaps/modeltaps.db \
               /opt/modeltaps/logs/modeltaps.log
```

以该用户启动（`--log-dir` 显式指向已收紧的目录，避免程序用宽 umask 新建）：

```bash
sudo -u modeltaps /opt/modeltaps/modeltaps --port 3000 --log-dir /opt/modeltaps/logs
```

::: tip 先建目录再启动
先用上述命令把 `logs` 目录建好并设为 `700`，程序即不会走"自动创建 `0777`"这条路径。
:::

### 清理已写入的初始密码

改完 root 密码后，初始密码在日志里仍是明文，需要一并清掉：

```bash
# 1. 确认日志中是否存在该行
sudo grep -l "initial root password" /data/modeltaps/logs/*.log

# 2. 删除含该行的日志文件（含轮转产生的历史文件）
sudo rm -f /data/modeltaps/logs/modeltaps.log /data/modeltaps/logs/modeltaps-*.log
```

容器的标准输出另存于 Docker 自己的日志文件，`rm` 上面的文件并不会清掉它。确认与清理方式：

```bash
# 确认
docker logs modeltaps 2>&1 | grep "initial root password"
# 清理：重建容器（数据在挂载卷中，不会丢）
docker rm -f modeltaps && docker run -d ...   # 用原来的启动命令重新创建
```

::: tip 更彻底的做法：首启前就设好 ROOT_PASSWORD
在**首次启动前**通过 `.env` 设置 `ROOT_PASSWORD`（8–64 位，长度不合规程序会启动失败），程序即不会生成随机密码，日志中只会出现 `password is set via ROOT_PASSWORD env`，不含密码本身。参见 [环境变量](./env.md#环境变量说明) 的 `ROOT_PASSWORD` 条目。
:::

## 管理员逃生口

站点把账号体系切换为外部**身份提供方**后，普通用户只能经身份提供方登录。为避免身份提供方不可用（密钥轮换、域名变更、停止服务、配置出错）时无人能进入后台，Modeltaps 为 root 账号保留了一条不依赖身份提供方的登录路径。

- **逃生入口**：外部身份提供方模式下，访问 `/login/admin` 会显示管理员登录表单（密码或通行密钥），且只有 root 账号能通过；其他账号在该表单提交一律拒绝。该入口沿用密码登录的全部校验（账号锁定、失败计数、人机校验），安全等级不降低。旧地址 `/login?local=1` 永久跳转到这里。
- **可以关闭**：后台「系统设置 → 登录方式 → 管理员应急登录」（`AdminLoginEnabled`，默认开）能关掉该入口；关闭后 `/login/admin` 与 root 的本站密码、通行密钥一并失效，只剩身份提供方一条路，请确认提供方自身有灾备再关。
- **切换前置**：切换到外部身份提供方**之前**，请先为 root 设置一个强密码（建议 16 位以上，保存在密码管理器中），并在「设置 → 登录与安全 → 登录验证 → 通行密钥」注册至少一枚通行密钥，作为第二条不依赖身份提供方的路径。
- **随时轮换**：外部模式下 root 仍可在「设置 → 登录与安全 → 管理员应急登录」修改该密码、增删通行密钥；其余账号在此状态下不能设置或修改本站密码。
- **`ROOT_PASSWORD` 只是初始值**：该环境变量仅在首次建库（数据库中还没有任何用户）时生效，之后修改或删除都不影响已存在的 root 密码，因此不能当作逃生手段。参见 [环境变量](./env.md#环境变量说明) 的 `ROOT_PASSWORD` 条目。

## 多机部署

### 准备工作

1. 确保所有服务器都安装了必要的组件：

- Docker 或 手动部署所需的组件
- Redis（如果需要使用缓存）
- MySQL 客户端（如果使用远程 MySQL）

2. 网络配置：

- 确保所有服务器能够访问主数据库
- 如果使用 Redis，确保可以访问 Redis 服务器
- 检查服务器间的防火墙设置

1. 所有服务器 `SESSION_SECRET` 设置一样的值。
2. 必须设置 `SQL_DSN`，使用 MySQL 数据库而非 SQLite，所有服务器连接同一个数据库。
3. 所有从服务器必须设置 `NODE_TYPE` 为 `slave`，不设置则默认为主服务器。
4. 设置 `SYNC_FREQUENCY` 后服务器将定期从数据库同步配置，在使用远程数据库的情况下，推荐设置该项并启用 Redis，无论主从。
5. 从服务器可以选择设置 `FRONTEND_BASE_URL`，以重定向页面请求到主服务器。
6. 从服务器上**分别**装好 Redis，设置好 `REDIS_CONN_STRING`，这样可以做到在缓存未过期的情况下数据库零访问，可以减少延迟。
7. 如果主服务器访问数据库延迟也比较高，则也需要启用 Redis，并设置 `SYNC_FREQUENCY`，以定期从数据库同步配置。
