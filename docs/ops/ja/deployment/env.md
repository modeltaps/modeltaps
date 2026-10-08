---
title: "環境変数"
layout: doc
outline: deep
lastUpdated: true
---

# 環境変数

::: warning 注意
環境変数は設定ファイルより優先されます。
:::

本ページは更新が追いついていない場合があります。設定可能なキーの全一覧は、配布パッケージのルートにある設定ファイルの例 `config.example.yaml` を参照してください。

## 設定ファイルから環境変数への変換

設定ファイル内の変数名をすべて大文字にし、階層はアンダースコアでつなぎます。例：

```yaml
# 設定ファイル
user_token_secret: "<32 文字以上のランダム文字列>"
logs:
  filename: "modeltaps.log" # ログファイル名
```

環境変数に変換すると次のようになります。

```bash
USER_TOKEN_SECRET="<32 文字以上のランダム文字列>"
LOGS_FILENAME="modeltaps.log"
```

## 環境変数の説明

1. `REDIS_CONN_STRING`：設定すると Redis をキャッシュとして使用します。
   - 例：`REDIS_CONN_STRING=redis://default:redispw@localhost:49153`
   - データベースへのアクセスのレイテンシが十分低い場合、Redis を有効にする必要はありません。有効にすると逆にデータの反映遅延が発生します。
2. `SESSION_SECRET`：設定すると固定のセッション秘密鍵が使用され、システムを再起動してもログイン済みユーザーの cookie が有効なままになります。
   - 値は `openssl rand -base64 48 | tr -d '\n'` で強いランダム文字列を生成してください。単純なパスワードは使用しないでください。
3. `SQL_DSN`：設定すると SQLite ではなく指定したデータベースを使用します。MySQL または PostgreSQL を使用してください。
   - 例（山かっこはプレースホルダーです。実際の値に置き換えてください）：
     - MySQL：`SQL_DSN=<DB_USER>:<DB_PASSWORD>@tcp(<DB_HOST>:3306)/modeltaps`
     - PostgreSQL：`SQL_DSN=postgres://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/modeltaps`（対応作業中です。フィードバックを歓迎します）
   - Modeltaps 専用のデータベースアカウントを作成し、強力なパスワードを設定してください。`root` などのスーパーユーザーアカウントを流用しないでください。
   - データベース `modeltaps` は事前に作成しておく必要があります。テーブルは手動で作成する必要はなく、プログラムが自動的に作成します。
   - ローカルのデータベースを使用する場合：デプロイコマンドに `--network="host"` を追加すると、コンテナ内のプログラムがホスト上の MySQL にアクセスできます。
   - クラウドデータベースを使用する場合：クラウドサーバー側で認証が必要なときは、接続パラメータに `?tls=skip-verify` を追加してください。
   - 実際のデータベース構成に応じて以下のパラメータを変更してください（既定値のままでも構いません）。
     - `SQL_MAX_IDLE_CONNS`：最大アイドル接続数。既定は `100`。
     - `SQL_MAX_OPEN_CONNS`：最大オープン接続数。既定は `1000`。
       - `Error 1040: Too many connections` が発生する場合は、この値を適切に小さくしてください。
     - `SQL_MAX_LIFETIME`：接続の最大有効期間。既定は `60`、単位は分。
4. `FRONTEND_BASE_URL`：設定するとページリクエストを指定したアドレスへリダイレクトします。スレーブサーバーでのみ設定してください。
   - 例：`FRONTEND_BASE_URL=https://<マスターサーバーのドメイン>`
5. `MEMORY_CACHE_ENABLED`：メモリキャッシュを有効にします。ユーザークォータの更新に一定の遅延が生じます。指定可能な値は `true` と `false` で、未設定の場合は `false` です。
   - 例：`MEMORY_CACHE_ENABLED=true`
6. `SYNC_FREQUENCY`：キャッシュを有効にしている場合にデータベースと設定を同期する頻度。単位は秒で、既定は `600` 秒です。
   - 例：`SYNC_FREQUENCY=60`
7. `NODE_TYPE`：設定するとノードの種別を指定します。指定可能な値は `master` と `slave` で、未設定の場合は `master` です。
   - 例：`NODE_TYPE=slave`
8. `CHANNEL_UPDATE_FREQUENCY`：設定するとチャネルの残高を定期的に更新します。単位は分で、未設定の場合は更新しません。
   - 例：`CHANNEL_UPDATE_FREQUENCY=1440`
9. `CHANNEL_TEST_FREQUENCY`：設定するとチャネルを定期的に検査します。単位は分で、未設定の場合は検査しません。
   - 例：`CHANNEL_TEST_FREQUENCY=1440`
10. `POLLING_INTERVAL`：チャネル残高の一括更新および利用可否テスト時のリクエスト間隔。単位は秒で、既定では間隔を設けません。
    - 例：`POLLING_INTERVAL=5`
11. `BATCH_UPDATE_ENABLED`：データベースの一括更新の集約を有効にします。ユーザークォータの更新に一定の遅延が生じます。指定可能な値は `true` と `false` で、未設定の場合は `false` です。
    - 例：`BATCH_UPDATE_ENABLED=true`
    - データベースの接続数が多すぎる問題が発生する場合は、このオプションを有効にしてみてください。
12. `BATCH_UPDATE_INTERVAL=5`：一括更新の集約間隔。単位は秒で、既定は `5` です。
    - 例：`BATCH_UPDATE_INTERVAL=5`
13. リクエストのレート制限：
    - `GLOBAL_API_RATE_LIMIT`：グローバルな API レート制限（リレーリクエストを除く）。1 つの IP からの 3 分間の最大リクエスト数で、既定は `180` です。
    - `GLOBAL_WEB_RATE_LIMIT`：グローバルな Web レート制限。1 つの IP からの 3 分間の最大リクエスト数で、既定は `60` です。
14. エンコーダーのキャッシュ設定：
    - `TIKTOKEN_CACHE_DIR`：既定ではプログラム起動時に `gpt-3.5-turbo` などの一般的なトークンのエンコーディングをネットワーク経由でダウンロードします。ネットワーク環境が不安定な場合やオフライン環境では起動に問題が生じる可能性があるため、このディレクトリを設定してデータをキャッシュし、オフライン環境へ移行できます。
    - `DATA_GYM_CACHE_DIR`：現在この設定の役割は `TIKTOKEN_CACHE_DIR` と同じですが、優先度は `TIKTOKEN_CACHE_DIR` より低くなります。
15. `RELAY_TIMEOUT`：リレーのタイムアウト設定。単位は秒で、既定ではタイムアウトを設定しません。
16. `SQLITE_BUSY_TIMEOUT`：SQLite のロック待機タイムアウト設定。単位はミリ秒で、既定は `3000` です。
17. `TG_BOT_API_KEY`：Telegram bot の API キー。[BotFather](https://t.me/BotFather) で取得できます。
18. `TG_WEBHOOK_SECRET`：（任意）webhook のシークレット。任意の値を設定できます。設定すると `webhook` 方式でメッセージを受信し、設定しない場合はポーリング（Polling）方式を使用します。
19. `USER_TOKEN_SECRET`：ユーザートークンの署名秘密鍵を設定します。必須で、32 文字を超える長さが必要です。設定後は変更しないでください。変更するとユーザートークンが無効になります。
    - 値は `openssl rand -base64 48 | tr -d '\n'` で生成し、`SESSION_SECRET` とは異なる値を使用してください。
20. `HASHIDS_SALT`：Sqids のアルファベット。ユーザートークン情報を難読化するために使用します。空でも構いません。空の場合は既定のアルファベット `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789` を使用します。設定する場合は、アルファベット内に重複する文字がないようにしてください。
21. `AUTO_PRICE_UPDATES`：価格の自動更新。指定可能な値は `true` と `false` で、未設定の場合は `false` です。有効にすると、プログラムの起動ごとにデータベース内のデータとプログラム内蔵の既定モデル価格を比較し、データベースにモデル価格の欠落があれば自動的にデータベースへ同期します。注意：有効にするとプログラム内蔵の既定モデル価格は削除できず、削除しても再起動時に再度書き込まれます。このオプションは価格を公式と一致させたい場合に適しています。
22. `AUTO_PRICE_UPDATES_MODE`：価格更新モード。指定可能な値は、`add`：システムに存在しない価格のみ追加する、`overwrite`：システムのすべての価格設定を上書きする、`update`：既存のデータのみ更新する、`system`：プログラム内蔵の価格表で価格設定を初期化する、で、既定は `system` です。本番環境では `system` モードを使用し、Web の価格管理モジュールで価格更新サービスのデータを手動取得した上で、1 件ずつ確認しながら更新することを推奨します。
23. `AUTO_PRICE_UPDATES_INTERVAL`：価格の自動更新間隔。単位は分で、`AUTO_PRICE_UPDATES_MODE` が `add`、`overwrite` の場合のみ有効です。システムはこの間隔で価格更新サーバーから価格設定を取得し、システムの価格を更新します。既定値：1440
24. `UPDATE_PRICE_SERVICE`：価格更新サービスのアドレス。設定するとこのアドレスから価格データを取得して価格を更新します。既定は空で、空の場合は外部の価格サービスを使用しません（この場合 `AUTO_PRICE_UPDATES_MODE` は `system`、すなわちプログラム内蔵の価格表で初期化する設定にしてください）。
25. `USER_INVOICE_MONTH`：ユーザーの月次請求書機能を有効にするか。有効にすると、システムは毎月 1 日の未明にユーザーの前月分のデータ集計請求書を生成します。データ量が多い場合はリソース消費が大きくなるため、慎重に有効化してください。既定は `false` です。
26. `ROOT_PASSWORD`：root アカウントの初期パスワード。データベースにまだユーザーが存在しない場合（すなわち初回起動時）にのみ有効です。長さは 8〜64 文字である必要があり、範囲外の場合はプログラムが起動に失敗します。
    - 未設定の場合、プログラムは 16 文字の強いランダムパスワードを生成し、初回起動時のログに 1 度だけ出力します。
    - 初回ログイン後は直ちに root のパスワードを変更し、初期パスワードを含む起動ログを削除してください。
    - この変数は**データベースの初回作成時にのみ有効**で、その後に変更・削除しても既存の root パスワードには影響しないため、緊急手段としては使えません。パスワードログインを無効にする前に [管理者の緊急ログイン経路](./index.md#管理者の緊急ログイン経路) を読んでください。
27. `ROOT_PASSWORD_ALLOW_INSECURE`：`ROOT_PASSWORD` の 8〜64 文字の長さチェックをスキップするか。`true` / `false` を指定でき、既定は `false` です。**ローカル開発環境専用で、本番環境では絶対に有効化しないでください。**
    - ローカル開発の取り決め：`ROOT_PASSWORD=root` と `ROOT_PASSWORD_ALLOW_INSECURE=true` を設定すると、初回起動後に `root/root` でログインできます。
    - 有効にすると起動ログに警告が 1 行出力されます。`ROOT_PASSWORD` が未設定の場合このスイッチは効かず、16 文字のランダムパスワードの分岐のままです。

## その他の設定項目

以下は上記で展開していないものの、コードが実際に読み取る主要な設定項目です。完全な一覧と既定値は、配布パッケージのルートにある `config.example.yaml` を参照してください。

### サーバー

- `PORT`：待ち受けポート。既定は `3000`。
- `GIN_MODE`：Gin の実行モード。`release` / `debug`。既定は `release`。
- `HTTPS`：HTTPS で外部公開するか（cookie の `Secure` などに影響します）。既定は `false`。
- `TRUSTED_HEADER`：実際のクライアント IP を取得するリクエストヘッダー。例：`CF-Connecting-IP`。
- `SHUTDOWN_TIMEOUT`：グレースフルシャットダウンのタイムアウト。単位は秒、既定は `30`。
- `PPROF_ENABLED`：pprof によるパフォーマンス分析を有効にするか。既定は `false`。
- `LANGUAGE`：既定の言語。既定は `zh_CN`。
- `FAVICON`：カスタム favicon のパス。
- `GITHUB_PROXY`：外部リソースをダウンロードする際のプロキシのプレフィックス。

### データベースと Redis

- `SQLITE_PATH`：SQLite データベースファイルのパス。既定は `modeltaps.db`。
- `BRAND_ICON_DIR`：ブランドアイコン同期レイヤーのローカルディレクトリ。バージョンごとに `{dir}/{version}/` に保存されます。既定は `data/brand-icons`。
- `REDIS_DB`：Redis の DB 番号（接続文字列に DB が含まれていない場合に有効）。既定は `0`。
- `REDIS_POOL_SIZE`：コネクションプールのサイズ。既定は `100`。
- `REDIS_MIN_IDLE_CONNS`：最小アイドル接続数。既定は `10`。
- `REDIS_POOL_TIMEOUT` / `REDIS_READ_TIMEOUT` / `REDIS_WRITE_TIMEOUT`：コネクションプールの待機 / 読み取り / 書き込みタイムアウト。単位は秒、既定は `5` / `2` / `2`。

### HTTP クライアント / リレーのタイムアウト

- `CONNECT_TIMEOUT`：接続タイムアウト。単位は秒、既定は `5`。
- `RELAY_REQUEST_TIMEOUT`：リレー 1 回あたりのリクエストタイムアウト。単位は秒、既定は `300`。
- `STREAM_IDLE_TIMEOUT`：ストリーミングのアイドルタイムアウト。チャネルのストリーミング応答中はデータを受信するたびに計測がリセットされ、無音状態がこの時間を超えるとストリームを中断します（実時間の総タイムアウトと補完的に機能し、「応答が止まったストリーム」を正確に処理するために使用します）。単位は秒、既定は `300`。`0` にすると無効になります。
- `RESPONSE_HEADER_TIMEOUT`：レスポンスヘッダーのタイムアウト。単位は秒、既定は `120`。
- `TLS_HANDSHAKE_TIMEOUT`：TLS ハンドシェイクのタイムアウト。単位は秒、既定は `30`。
- `TLS_INSECURE_SKIP_VERIFY`：TLS 証明書の検証をスキップします（安全ではありません）。既定は `false`。
- `MAX_CONNS_PER_HOST` / `MAX_IDLE_CONNS` / `MAX_IDLE_CONNS_PER_HOST`：接続数の上限。既定は `0`（無制限）/ `1000` / `200`。

### 価格 / 課金

- `CATALOG_PRICING_URL`：モデルカタログ価格の同期元アドレス。
- `CATALOG_PRICING_AUTO_SYNC`：カタログ価格を自動同期するか。既定は `true`。
- `CHANNEL_PRICING_AUTO_SYNC`：チャネル価格を自動同期するか。既定は `true`。
- `UNPRICED_MODEL_POLICY`：価格未設定モデルに対するポリシー。既定は `block`。
- `UNPRICED_MODEL_DEFAULT_RATIO`：価格未設定モデルの既定倍率。既定は `30.0`。
- `MODEL_DRIFT_AUTO_CHECK`：モデルのドリフトを自動検出するか。既定は `false`。

### ログ / メトリクス / 連携

- `LOG_DIR`：ログディレクトリ。既定は `./logs`。
- `LOG_LEVEL`：ログレベル。`debug` にするとより詳細に出力されます。
- `LOGS_FILENAME` / `LOGS_MAX_SIZE` / `LOGS_MAX_AGE` / `LOGS_MAX_BACKUP` / `LOGS_COMPRESS`：ログファイル名およびローテーション設定。既定は `modeltaps.log` / `100`（MB）/ `7`（日）/ `10` / `false`。
- `METRICS_USER` / `METRICS_PASSWORD`：`/metrics` エンドポイントの Basic 認証。空にすると認証を行いません。
- `MCP_ENABLE`：MCP を有効にするか。既定は `false`。
- `UPTIME_KUMA_ENABLE` / `UPTIME_KUMA_DOMAIN` / `UPTIME_KUMA_STATUS_PAGE_NAME`：Uptime Kuma ステータスページ連携。
- `DISABLE_TOKEN_ENCODERS`：ローカルのトークンエンコーダーを無効にします（推定値に切り替えます）。既定は `false`。
- `TG_HTTP_PROXY`：Telegram へアクセスする際の HTTP/SOCKS5 プロキシ。

### 通知 / ストレージ / 検索

通知（`NOTIFY_*`）、オブジェクトストレージによる画像ストレージ（`STORAGE_*`）、Web 検索（`SEARCH_*`）などのグループには設定項目が多数あります。キー名と既定値は、配布パッケージのルートにある `config.example.yaml` を直接参照してください。

