---
title: "Notifications"
layout: doc
outline: deep
lastUpdated: true
---

# Notifications

Admins get a notice when a channel is disabled or recovers, a balance runs low, prices change,
models drift, credentials fail and so on. Notices are set up under
**Settings → Notifications & integrations → Notifications**, in three parts:

- **Notice kinds**: each kind can be turned off. Each kind is capped per hour; notices over the
  cap are dropped, and the first notice of the next hour says how many were.
- **Channels**: where notices go. Every notice goes to all enabled channels.
- **Mail server**: the SMTP server behind email channels, sign-up codes and quota warning emails.

## Channels

These types are supported, and a type can be added more than once (two different groups, say):

| Type | Settings |
| --- | --- |
| Email | Recipient; empty sends to the root admin's address. Needs the mail server |
| WeCom | The group bot's webhook URL |
| DingTalk | Bot access token (the part after `access_token=` in the webhook URL), and a signing secret or a keyword |
| Lark | Bot token (the last part of the webhook URL), and a signing secret or a keyword |
| PushDeer | PushKey; the server address when self-hosted |
| Telegram | Bot token and chat ID; a proxy when the server cannot reach Telegram directly |

While adding or editing a channel, **Send test** sends a test notice with what the form holds,
saved or not. Webhooks, tokens and secrets are not shown again once saved; leave them empty while
editing to keep them.

A fresh install starts with one email channel to the root admin's address, so notices arrive as
soon as the mail server is set up.

## Upgrading from the config file

Older versions set the channels under `notify:` in `config.yaml` (or the `NOTIFY_*` environment
variables). The first start after the upgrade moves them into the channels in the console; from
then on they are edited there, and the `notify:` section can be removed.

The **Telegram user bot** under Settings → Notifications & integrations → Integrations is
something else: it lets users link their account, check their balance and top up from Telegram.
Its key is still set under `tg:` in the config file and has nothing to do with the Telegram
notification channel.
