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
