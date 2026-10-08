---
title: "升級"
layout: doc
outline: deep
lastUpdated: true
---

# 自動升級

::: warning 請先登入私有鏡像倉庫
下面的升級腳本會執行 `docker pull ghcr.io/modeltaps/modeltaps:latest` 拉取私有鏡像。運行腳本的宿主機必須先完成 [登入私有鏡像倉庫](./index.md#登入私有鏡像倉庫)，否則拉取會因未授權而失敗。
:::

### 透過面板計劃任務

若使用寶塔等運維面板部署，可直接新增計劃任務。

新增計劃任務 -> Shell 腳本 -> 粘貼下方腳本內容即可。

使用 Docker Compose 部署時，請將腳本中的目錄改為 `docker-compose.yml` 所在目錄：

```
cd /www/wwwroot/
```

### 透過 Crontab

此腳本也可在 Crontab 中使用，但需確認 Crontab 能找到正確的 Docker 命令，建議使用絕對路徑。

配置 Crontab，每 5 分鐘執行一次腳本：

```
*/5 * * * * /path/to/modeltaps-auto-update.sh >> /path/to/modeltaps-auto-update.log 2>&1
```

### Docker Compose 腳本

```sh
# 拉取最新鏡像，並將輸出儲存在變數中
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# 檢查拉取命令是否成功執行
if [ $? -ne 0 ]; then
exit 1
fi

# 檢查輸出中是否包含特定字符串
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# 如果鏡像已經是最新的，則不執行任何操作
if [ $? -eq 0 ]; then
exit 0
fi

echo "檢測到 modeltaps 更新"

# 移除舊的容器
echo "已移除: $(docker rm -f modeltaps)"

# 需先切換到 `docker-compose.yml` 所在的目錄
cd /www/wwwroot/

# 運行新的容器
echo "已啟動: $(docker-compose up)"

# 打印更新時間和版本
echo "更新時間: $(date)"
echo "版本: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# 清理未使用的鏡像
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "已移除舊的鏡像."
```

### Docker 命令方式 腳本

```sh
# 拉取最新鏡像，並將輸出儲存在變數中
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# 檢查拉取命令是否成功執行
if [ $? -ne 0 ]; then
exit 1
fi

# 檢查輸出中是否包含特定字符串
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# 如果鏡像已經是最新的，則不執行任何操作
if [ $? -eq 0 ]; then
exit 0
fi

echo "檢測到 modeltaps 更新"

# 移除舊的容器
echo "已移除: $(docker rm -f modeltaps)"

# 需先切換到 `docker-compose.yml` 所在的目錄
cd /www/wwwroot/

# 運行新的容器 此處為 SQLite 的 docker 命令，可按實際部署方式調整
echo "已啟動: $(docker run -d -p 3000:3000 --name modeltaps --restart always -e TZ=Asia/Shanghai -v /home/ubuntu/data/modeltaps:/data ghcr.io/modeltaps/modeltaps:latest)"

# 打印更新時間和版本
echo "更新時間: $(date)"
echo "版本: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# 清理未使用的鏡像
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "已移除舊的鏡像."
```
