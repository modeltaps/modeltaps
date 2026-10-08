---
title: "自動アップデート"
layout: doc
outline: deep
lastUpdated: true
---

# 自動アップデート

::: warning 先にプライベートイメージレジストリへログインしてください
以下のアップデートスクリプトは `docker pull ghcr.io/modeltaps/modeltaps:latest` を実行してプライベートイメージを取得します。スクリプトを実行するホストでは事前に [プライベートイメージレジストリへのログイン](./index.md#プライベートイメージレジストリへのログイン) を済ませる必要があります。ログインしていないと権限不足で取得に失敗します。
:::

### 管理パネルのスケジュールタスクで実行する

aaPanel（宝塔）などの運用管理パネルでデプロイしている場合は、スケジュールタスクを直接追加できます。

スケジュールタスクの追加 -> Shell スクリプト -> 下記のスクリプト内容を貼り付けます。

Docker Compose でデプロイしている場合は、スクリプト内のディレクトリを `docker-compose.yml` があるディレクトリに変更してください。

```
cd /www/wwwroot/
```

### Crontab で実行する

このスクリプトは Crontab でも利用できますが、Crontab から正しい Docker コマンドを見つけられることを確認する必要があります。絶対パスの使用を推奨します。

Crontab を設定し、5 分ごとにスクリプトを実行します。

```
*/5 * * * * /path/to/modeltaps-auto-update.sh >> /path/to/modeltaps-auto-update.log 2>&1
```

### Docker Compose 用スクリプト

```sh
# 最新イメージを取得し、出力を変数に格納する
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# 取得コマンドが正常に実行されたか確認する
if [ $? -ne 0 ]; then
exit 1
fi

# 出力に特定の文字列が含まれるか確認する
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# イメージが既に最新の場合は何も実行しない
if [ $? -eq 0 ]; then
exit 0
fi

echo "modeltaps の更新を検出しました"

# 古いコンテナを削除する
echo "削除しました: $(docker rm -f modeltaps)"

# 先に `docker-compose.yml` があるディレクトリに移動する必要がある
cd /www/wwwroot/

# 新しいコンテナを実行する
echo "起動しました: $(docker-compose up)"

# 更新時刻とバージョンを出力する
echo "更新時刻: $(date)"
echo "バージョン: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# 未使用のイメージを整理する
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "古いイメージを削除しました."
```

### Docker コマンド方式のスクリプト

```sh
# 最新イメージを取得し、出力を変数に格納する
output=$(docker pull ghcr.io/modeltaps/modeltaps:latest 2>&1)

# 取得コマンドが正常に実行されたか確認する
if [ $? -ne 0 ]; then
exit 1
fi

# 出力に特定の文字列が含まれるか確認する
echo "$output" | grep -q "Image is up to date for ghcr.io/modeltaps/modeltaps:latest"

# イメージが既に最新の場合は何も実行しない
if [ $? -eq 0 ]; then
exit 0
fi

echo "modeltaps の更新を検出しました"

# 古いコンテナを削除する
echo "削除しました: $(docker rm -f modeltaps)"

# 先に `docker-compose.yml` があるディレクトリに移動する必要がある
cd /www/wwwroot/

# 新しいコンテナを実行する。ここでは SQLite 用の docker コマンドを示しており、実際のデプロイ方式に応じて調整してください
echo "起動しました: $(docker run -d -p 3000:3000 --name modeltaps --restart always -e TZ=UTC -v /home/ubuntu/data/modeltaps:/data ghcr.io/modeltaps/modeltaps:latest)"

# 更新時刻とバージョンを出力する
echo "更新時刻: $(date)"
echo "バージョン: $(docker inspect ghcr.io/modeltaps/modeltaps:latest | grep 'org.opencontainers.image.version' | awk -F'"' '{print $4}')"

# 未使用のイメージを整理する
docker images | grep 'ghcr.io/modeltaps/modeltaps' | grep -v 'latest' | awk '{print $3}' | xargs -r docker rmi > /dev/null 2>&1
echo "古いイメージを削除しました."
```
