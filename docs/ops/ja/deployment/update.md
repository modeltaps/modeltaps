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

::: danger アップグレード後はそのままダウングレードできません
API Key をハッシュで保存するバージョンにアップグレードすると、データベースには各キーのハッシュだけが残り、平文は消去されます。この操作は元に戻せず、旧バージョンにダウングレードするとすべての API Key が無効になります。アップグレード前にデータベースをバックアップし、ロールバックが必要な場合は旧バージョンと一緒にそのバックアップを復元してください。管理キーを導入するバージョンも、各アカウントのアクセストークンを同じようにハッシュへ移して平文を消去するため、ダウングレード後は古いアクセストークンがすべて使えなくなります。
:::

### 0.4.0 から 0.5.0 へのアップグレード

新しいバージョンを起動する前に、次の作業を行ってください。データベースは自動で移行されます。

- **Docker Compose：** `.env` に `REDIS_PASSWORD`（`openssl rand -hex 32`）を追加します。同梱の Redis にはパスワードが必須で、ないと `docker compose up` が停止します。
- **`ENCRYPTION_KEY` を設定**（`openssl rand -base64 32`）してください（未設定の場合）。初回起動時に `config.yaml` の `storage:` セクション（または `STORAGE_*`）がプラットフォーム設定へ移され、そのシークレットはキーがある場合のみ暗号化して保存されます。以降、このセクションは読み込まれません。
- **Host ヘッダーを書き換えるリバースプロキシの背後では**、設定でサーバーアドレスを指定してください。Cookie 認証による変更操作はサイト自身のオリジンから来る必要があります。
- **独自ルーティングポリシー**（個人・組織）を呼び出すクライアントは `policy/<名前>` を使う必要があります。プレフィックスなしの名前は、管理ポリシーかモデルを指すようになりました。
- **次のエンドポイントを GET で呼んでいるスクリプト**は POST に変更してください：`/api/channel/test`、`/api/channel/test/:id`、`/api/channel/check/:id`、`/api/channel/update_balance`、`/api/channel/update_balance/:id`、`/api/user/logout`、`/api/user/token`、`/api/verification`、`/api/reset_password`。
- **課金：**「チャネル原価を下回って課金しない」スイッチは削除されました。リクエストはカタログ価格 × グループ倍率で課金されます。利益を残すにはデフォルトグループの倍率を上げてください。

認証アプリとリカバリーコードを両方なくしたユーザーは、root が `DELETE /api/user/<id>/totp` で 2 段階認証を無効にできます。

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

### セルフチェック：users.email は NULL を許可する必要があります

メールアドレスのないアカウントは `users.email` に NULL を保存し、メールアドレスの一意制約はこれを前提にしています。この列を手動で `NOT NULL` に変更したことがある場合、Modeltaps は起動時に自動で NULL を許可する定義に戻し（元の型・文字セット・照合順序を保ち、データは変更しません）、実行した SQL をログに記録します。

データベースユーザーに `ALTER` 権限がない場合は自動修復が失敗して起動が停止し、実行すべき SQL がログに表示されます。`ALTER` 権限のあるユーザーでその SQL を実行してから、もう一度起動してください。

この列が NULL を許可しているかの確認方法（結果が `YES` なら問題ありません）：

```sql
-- MySQL
SELECT IS_NULLABLE, COLUMN_TYPE FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'email';

-- PostgreSQL
SELECT is_nullable FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'users' AND column_name = 'email';
```

手動で修復する場合（MySQL では `varchar(255)` を上で確認した `COLUMN_TYPE` に置き換え、元の文字セットと照合順序を保ってください）：

```sql
-- MySQL
ALTER TABLE `users` MODIFY COLUMN `email` varchar(255) NULL;

-- PostgreSQL
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
```
