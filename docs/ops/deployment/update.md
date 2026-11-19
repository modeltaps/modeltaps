---
title: "自动升级"
layout: doc
outline: deep
lastUpdated: true
---

# 自动升级

::: warning 请先登录私有镜像仓库
下面的升级脚本会执行 `docker pull ghcr.io/modeltaps/modeltaps:latest` 拉取私有镜像。运行脚本的宿主机必须先完成 [登录私有镜像仓库](./index.md#登录私有镜像仓库)，否则拉取会因未授权而失败。
:::

::: danger 升级后不能直接降级
升级到存储 API Key 哈希的版本后，数据库里只保留 key 的哈希，明文已清空，这一步无法回滚。降级回旧版本会让所有 API Key 失效。升级前请备份数据库；需要回退时连同数据库备份一起恢复。引入管理密钥的版本同样把每个账户的访问令牌转为哈希并清空明文，降级后旧的访问令牌全部失效。
:::

### 从 0.4.0 升级到 0.5.0

启动新版本前先做这几件事；数据库会自动迁移。

- **Docker Compose：** 在 `.env` 里加上 `REDIS_PASSWORD`（`openssl rand -hex 32`）。自带的 Redis 必须设密码，没有它 `docker compose up` 会停下。
- **设置 `ENCRYPTION_KEY`**（`openssl rand -base64 32`），如果还没设。首次启动会把 `config.yaml` 的 `storage:` 段（或 `STORAGE_*`）搬进平台设置，其中的密钥只有设了这个变量才会加密存储。之后不再读取该段。
- **反向代理改写了 Host 头的**，请在设置里填写服务器地址：用 Cookie 登录的修改操作必须来自本站。
- **调用自建路由策略的客户端**（个人和组织）要改用 `policy/<名字>`；裸名字现在只指平台策略或模型。
- **用 GET 调用这些接口的脚本**要改成 POST：`/api/channel/test`、`/api/channel/test/:id`、`/api/channel/check/:id`、`/api/channel/update_balance`、`/api/channel/update_balance/:id`、`/api/user/logout`、`/api/user/token`、`/api/verification`、`/api/reset_password`。
- **计费：**「不低于渠道成本扣费」开关已删除。请求按目录价 × 分组倍率扣费；要保留利润，调高默认分组的倍率。

用户同时丢了验证器和恢复码时，root 可以用 `DELETE /api/user/<id>/totp` 为其关闭两步验证。

### 通过面板计划任务

若使用宝塔等运维面板部署，可直接添加计划任务。

添加计划任务 -> Shell 脚本 -> 粘贴下方脚本内容即可。

使用 Docker Compose 部署时，请将脚本中的目录改为 `docker-compose.yml` 所在目录：

```
cd /www/wwwroot/
```

### 通过 Crontab

此脚本也可在 Crontab 中使用，但需确认 Crontab 能找到正确的 Docker 命令，建议使用绝对路径。

配置 Crontab，每 5 分钟执行一次脚本：

```
*/5 * * * * /path/to/modeltaps-auto-update.sh >> /path/to/modeltaps-auto-update.log 2>&1
```

### Docker Compose 脚本

```sh
# 拉取最新镜像，并将输出存储在变量中
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# 检查拉取命令是否成功执行
if [ $? -ne 0 ]; then
exit 1
fi

# 检查输出中是否包含特定字符串
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# 如果镜像已经是最新的，则不执行任何操作
if [ $? -eq 0 ]; then
exit 0
fi

echo "检测到 modeltaps 更新"

# 移除旧的容器
echo "已移除: $(docker rm -f modeltaps)"

# 需先切换到 `docker-compose.yml` 所在的目录
cd /www/wwwroot/

# 运行新的容器
echo "已启动: $(docker-compose up)"

# 打印更新时间和版本
echo "更新时间: $(date)"
echo "版本: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# 清理未使用的镜像
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "已移除旧的镜像."
```

### Docker 命令方式 脚本

```sh
# 拉取最新镜像，并将输出存储在变量中
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# 检查拉取命令是否成功执行
if [ $? -ne 0 ]; then
exit 1
fi

# 检查输出中是否包含特定字符串
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# 如果镜像已经是最新的，则不执行任何操作
if [ $? -eq 0 ]; then
exit 0
fi

echo "检测到 modeltaps 更新"

# 移除旧的容器
echo "已移除: $(docker rm -f modeltaps)"

# 需先切换到 `docker-compose.yml` 所在的目录
cd /www/wwwroot/

# 运行新的容器 此处为 SQLite 的 docker 命令，可按实际部署方式调整
echo "已启动: $(docker run -d -p 3000:3000 --name modeltaps --restart always -e TZ=Asia/Shanghai -v /home/ubuntu/data/modeltaps:/data ghcr.io/modeltaps/modeltaps:latest)"

# 打印更新时间和版本
echo "更新时间: $(date)"
echo "版本: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# 清理未使用的镜像
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "已移除旧的镜像."
```

### 自查：users.email 必须允许 NULL

没有邮箱的账号在 `users.email` 里存的是 NULL，邮箱唯一约束依赖这一点。如果曾手动把这一列改成 `NOT NULL`，Modeltaps 启动时会自动把它改回允许 NULL（保留原有类型、字符集和排序规则，不改动数据），并在日志里记下执行的 SQL。

数据库账号没有 `ALTER` 权限时，自动修复会失败，启动随之终止，日志会给出需要执行的 SQL。用有 `ALTER` 权限的账号执行后重新启动即可。

自查这一列是否允许 NULL（结果为 `YES` 即正常）：

```sql
-- MySQL
SELECT IS_NULLABLE, COLUMN_TYPE FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'email';

-- PostgreSQL
SELECT is_nullable FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'users' AND column_name = 'email';
```

手动修复（MySQL 中把 `varchar(255)` 换成上面查到的 `COLUMN_TYPE`，并保留原有字符集与排序规则）：

```sql
-- MySQL
ALTER TABLE `users` MODIFY COLUMN `email` varchar(255) NULL;

-- PostgreSQL
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
```
