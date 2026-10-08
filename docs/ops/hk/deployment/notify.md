---
title: "訊息通知"
layout: doc
outline: deep
lastUpdated: true
---

# 訊息通知

當渠道被禁用時，系統會發送通知。

## 設定檔配置

請在 config.yaml 中填寫以下字段內容，無需開啟的通知方式保持為空即可。

```yaml
notify: # 通知設定，配置幾個通知方式即會同時發送幾次通知；如不需要通知，可刪除該配置
  email: # 郵件通知 (具體 SMTP 配置在後台設定，預設為開啟狀態，如需關閉請在下面禁用)
    disable: false # 是否禁用郵件通知
    smtp_to: "" # 收件人地址 (可空，如果為空則使用超級管理員郵箱)
  dingTalk: # 釘釘機械人通知
    token: "" # webhook 地址最後一串字元
    secret: "" # 密鑰 (secret/keyWord 二選一)
    keyWord: "" # 關鍵字 (secret/keyWord 二選一)
  lark: # 飛書機械人通知
    token: "" # webhook 地址最後一串字元
    secret: "" # 密鑰 (secret/keyWord 二選一)
    keyWord: "" # 關鍵字 (secret/keyWord 二選一)
  pushdeer: # pushdeer 通知
    url: "https://api2.pushdeer.com" # pushdeer地址 (可空，如果自建需填寫)
    pushkey: "" # pushkey
  telegram: # Telegram 通知
    bot_api_key: "" # Telegram bot 的 API 密鑰
    chat_id: "" # Telegram chat_id
```

## 環境變數配置

### email 通知

- `NOTIFY_EMAIL_DISABLE` 是否禁用郵件通知, `true` 或者 `false`
- `NOTIFY_EMAIL_SMTP_TO` 收件人地址 (可空，如果為空則使用超級管理員郵箱)

### 釘釘通知

- `NOTIFY_DINGTALK_TOKEN` webhook 地址最後一串字元
- `NOTIFY_DINGTALK_SECRET` 密鑰 (secret/keyWord 二選一)
- `NOTIFY_DINGTALK_KEYWORD` 關鍵字 (secret/keyWord 二選一)

### 飛書通知

- `NOTIFY_LARK_TOKEN` webhook 地址最後一串字元
- `NOTIFY_LARK_SECRET` 密鑰 (secret/keyWord 二選一)
- `NOTIFY_LARK_KEYWORD` 關鍵字 (secret/keyWord 二選一)

### pushdeer 通知

- `NOTIFY_PUSHDEER_URL` pushdeer 地址 (可空，如自建需填寫)
- `NOTIFY_PUSHDEER_PUSHKEY` pushdeer pushkey

### telegram 通知

- `NOTIFY_TELEGRAM_BOT_API_KEY` Telegram bot 的 API 密鑰
- `NOTIFY_TELEGRAM_CHAT_ID` Telegram chat_id
