---
title: "通知設定"
layout: doc
outline: deep
lastUpdated: true
---

# 通知設定

チャネルが無効化されると、システムから通知が送信されます。

## 設定ファイルでの設定

config.yaml に以下のフィールドを記入してください。有効にする必要のない通知方式は空のままで構いません。

```yaml
notify: # 通知設定。設定した通知方式の数だけ同時に通知が送信されます。通知が不要な場合はこの設定を削除できます
  email: # メール通知（SMTP の詳細は管理画面で設定します。既定で有効です。無効にする場合は下記で指定してください）
    disable: false # メール通知を無効にするか
    smtp_to: "" # 宛先アドレス（空でも可。空の場合はスーパー管理者のメールアドレスを使用します）
  dingTalk: # DingTalk ボット通知
    token: "" # webhook URL の末尾の文字列
    secret: "" # シークレット（secret / keyWord のいずれか一方）
    keyWord: "" # キーワード（secret / keyWord のいずれか一方）
  lark: # Lark（Feishu）ボット通知
    token: "" # webhook URL の末尾の文字列
    secret: "" # シークレット（secret / keyWord のいずれか一方）
    keyWord: "" # キーワード（secret / keyWord のいずれか一方）
  pushdeer: # pushdeer 通知
    url: "https://api2.pushdeer.com" # pushdeer のアドレス（空でも可。セルフホストの場合は記入してください）
    pushkey: "" # pushkey
  telegram: # Telegram 通知
    bot_api_key: "" # Telegram bot の API キー
    chat_id: "" # Telegram chat_id
```

## 環境変数での設定

### メール通知

- `NOTIFY_EMAIL_DISABLE` メール通知を無効にするか。`true` または `false`
- `NOTIFY_EMAIL_SMTP_TO` 宛先アドレス（空でも可。空の場合はスーパー管理者のメールアドレスを使用します）

### DingTalk 通知

- `NOTIFY_DINGTALK_TOKEN` webhook URL の末尾の文字列
- `NOTIFY_DINGTALK_SECRET` シークレット（secret / keyWord のいずれか一方）
- `NOTIFY_DINGTALK_KEYWORD` キーワード（secret / keyWord のいずれか一方）

### Lark（Feishu）通知

- `NOTIFY_LARK_TOKEN` webhook URL の末尾の文字列
- `NOTIFY_LARK_SECRET` シークレット（secret / keyWord のいずれか一方）
- `NOTIFY_LARK_KEYWORD` キーワード（secret / keyWord のいずれか一方）

### pushdeer 通知

- `NOTIFY_PUSHDEER_URL` pushdeer のアドレス（空でも可。セルフホストの場合は記入してください）
- `NOTIFY_PUSHDEER_PUSHKEY` pushdeer の pushkey

### Telegram 通知

- `NOTIFY_TELEGRAM_BOT_API_KEY` Telegram bot の API キー
- `NOTIFY_TELEGRAM_CHAT_ID` Telegram chat_id
