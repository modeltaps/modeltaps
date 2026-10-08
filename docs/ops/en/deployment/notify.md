---
title: "Notifications"
layout: doc
outline: deep
lastUpdated: true
---

# Notifications

Modeltaps sends a notification whenever a channel is disabled.

## Configuration file

Fill in the following fields in `config.yaml`. Leave any notification method you do not need empty.

```yaml
notify: # Notification settings; every configured method receives its own notification. Remove this section if you do not need notifications.
  email: # Email notifications (SMTP is configured in the admin console; enabled by default — disable it below if not needed)
    disable: false # Whether to disable email notifications
    smtp_to: "" # Recipient address (optional; falls back to the super administrator's email)
  dingTalk: # DingTalk bot notifications
    token: "" # The last segment of the webhook URL
    secret: "" # Secret (choose either secret or keyWord)
    keyWord: "" # Keyword (choose either secret or keyWord)
  lark: # Lark/Feishu bot notifications
    token: "" # The last segment of the webhook URL
    secret: "" # Secret (choose either secret or keyWord)
    keyWord: "" # Keyword (choose either secret or keyWord)
  pushdeer: # pushdeer notifications
    url: "https://api2.pushdeer.com" # pushdeer address (optional; required for self-hosted instances)
    pushkey: "" # pushkey
  telegram: # Telegram notifications
    bot_api_key: "" # Telegram bot API key
    chat_id: "" # Telegram chat_id
```

## Environment variables

### Email notifications

- `NOTIFY_EMAIL_DISABLE` — whether to disable email notifications, `true` or `false`
- `NOTIFY_EMAIL_SMTP_TO` — recipient address (optional; falls back to the super administrator's email)

### DingTalk notifications

- `NOTIFY_DINGTALK_TOKEN` — the last segment of the webhook URL
- `NOTIFY_DINGTALK_SECRET` — secret (choose either secret or keyWord)
- `NOTIFY_DINGTALK_KEYWORD` — keyword (choose either secret or keyWord)

### Lark/Feishu notifications

- `NOTIFY_LARK_TOKEN` — the last segment of the webhook URL
- `NOTIFY_LARK_SECRET` — secret (choose either secret or keyWord)
- `NOTIFY_LARK_KEYWORD` — keyword (choose either secret or keyWord)

### pushdeer notifications

- `NOTIFY_PUSHDEER_URL` — pushdeer address (optional; required for self-hosted instances)
- `NOTIFY_PUSHDEER_PUSHKEY` — pushdeer pushkey

### Telegram notifications

- `NOTIFY_TELEGRAM_BOT_API_KEY` — Telegram bot API key
- `NOTIFY_TELEGRAM_CHAT_ID` — Telegram chat_id
