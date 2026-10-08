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
    bucketName: "" # バケット名。例：zerodeng-superai
    accessKeyId: "" # Alibaba Cloud の認可 KEY。コンソールの RAM 管理画面で取得します
    accessKeySecret: "" # Alibaba Cloud の認可 SECRET。コンソールの RAM 管理画面で取得します
  s3: # AWS S3 互換プロトコル
    endpoint: "" # Endpoint（リージョンのエンドポイント）。例：https://xxxxxx.r2.cloudflarestorage.com
    cdnurl: "" # 公開アクセス用ドメイン。例：https://pub-xxxxx.r2.dev。未設定の場合は endpoint を使用します
    bucketName: "" # バケット名。例：zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```
