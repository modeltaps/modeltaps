---
title: "画像ストレージ設定"
layout: doc
outline: deep
lastUpdated: true
---

# 画像ストレージ設定

一部のプロバイダーの画像生成 API は URL を返しません。そのため画像ストレージを設定しておくと、システムがまず画像をストレージへアップロードし、そのリンクを返します。

`gemini` の画像出力対応モデルを利用する場合、画像ストレージの設定が正しくないと画像の返却に失敗します（gemini ネイティブ API では設定は不要です）。

画像ストレージは複数設定できます。アップロードに失敗した場合、システムは自動的に次のストレージへアップロードします。

```yaml
storage: # ストレージ設定（任意。主に画像生成向け。URL を返さず base64 画像のみを返すプロバイダーがあり、設定しておくと URL 形式で画像生成結果を返せます）
  smms: # sm.ms 画像ストレージの設定
    secret: "" # sm.ms API キー
  imgur:
    client_id: "" # imgur client_id
  alioss: # Alibaba Cloud OSS オブジェクトストレージ
    endpoint: "" # Endpoint（リージョンのエンドポイント）。例：oss-cn-beijing.aliyuncs.com
    region: "" # リージョン ID。例：cn-beijing。空の場合は標準 endpoint（oss-<リージョン>.aliyuncs.com）から導出します。カスタムドメインや転送アクセラレーションの endpoint では必須です
    bucketName: "" # バケット名。例：zerodeng-superai
    accessKeyId: "" # Alibaba Cloud の認可 KEY。コンソールの RAM 管理画面で取得します
    accessKeySecret: "" # Alibaba Cloud の認可 SECRET。コンソールの RAM 管理画面で取得します
  s3: # AWS S3 互換プロトコル
    endpoint: "" # Endpoint（リージョンのエンドポイント）。例：https://xxxxxx.r2.cloudflarestorage.com
    region: "" # 署名リージョン。例：us-east-1。空の場合は AWS、Backblaze B2、Wasabi の endpoint から導出し、それ以外（Cloudflare R2 など）は auto を使います
    cdnurl: "" # 公開アクセス用ドメイン。例：https://pub-xxxxx.r2.dev。未設定の場合は endpoint を使用します
    bucketName: "" # バケット名。例：zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```

Alibaba Cloud OSS のアップロードと署名付きリンクは V4 署名を使います（2025 年以降に作成されたアカウントとバケットは V1 署名を受け付けません）。V4 署名にはバケットのリージョンが必要です。endpoint が `oss-cn-beijing.aliyuncs.com` のような標準形式なら `region` は空のままで構いません。それ以外の場合は設定してください。未設定だと起動時にエラーがログに出力され、Alibaba Cloud OSS は無効のままになります。

S3 の署名にはリージョンが必要です。Cloudflare R2 は `auto` を使うので空のままで構いません。AWS S3、Backblaze B2、Wasabi の標準 endpoint（`https://s3.us-west-2.amazonaws.com`、`https://s3.us-west-004.backblazeb2.com` など）からは自動で導出します。その他のサービスでは各社のドキュメントに従って `region` を設定してください。呼び出し側には `cdnurl` 配下の通常のリンクを返すため、バケットは公開読み取りを許可する必要があります。`expirationDays` はオブジェクトの Expires ヘッダーを設定するだけでファイルは削除しません。古いファイルを自動で削除するには、ストレージ側でライフサイクルルールを設定してください。
