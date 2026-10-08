---
title: "API Keys and Quota"
layout: doc
outline: deep
lastUpdated: true
---

# API Keys and Quota

An API key is the credential you use to call Modeltaps. We recommend creating a separate API key per application and per environment: logs and usage then clearly show who is spending, and you can disable a single key without affecting anything else.

## The API key list

The **API Keys** page lists all of your API keys. The columns shown by default are:

| Column | Meaning |
| --- | --- |
| API key | Name and key (the key is masked by default; you can reveal or copy it) |
| Group | The channel group this API key uses; shows **Follow user** when unset |
| Expiry | **Never**, the remaining time, or **Expired** |
| Last used | Time of the last successful call, or **Never used** |
| Usage | Total quota consumed, with today's usage alongside |
| Limit | Remaining quota or **Unlimited**; when a period is set, shows the period cap and the next reset time |
| Status | A switch to enable / disable the API key directly |

**Column settings** in the top right lets you show or hide columns (for example **Created at**, hidden by default). The filter bar at the top supports searching by API key name.

Each row's action menu offers: copy key, open in a chat client, **Test command**, **Connect OpenClaw**, jump to that API key's **Usage** and **Logs**, edit and delete.

## Creating and editing API keys

Click **New API key** to open the editor panel, which is split into several sections.

### Basic information

- **Name** (required): logs and usage analytics are aggregated by API key name. Renaming does not rewrite historical records, so it is best to settle on a name up front.
- **Never expires / Expiry**: never expires by default; turn the switch off to set a specific expiry time, after which the API key can no longer be used.

### Quota and period

There is a single quota input here, and its meaning depends on the **Reset period**:

- **No quota, no period**: unlimited. The API key itself has no cap; actual spend is still bound by your account quota.
- **Quota set, no period**: a total cap. The API key is disabled once cumulative spend reaches that quota.
- **Quota set with a period** (daily / weekly / monthly): the quota becomes a **per-period cap**. It resets automatically at midnight on each period boundary and does not draw extra from your account balance. When a period is selected the quota must be greater than 0.

The panel shows the time zone the site uses for resets and the first day of the week (Monday or Sunday). The **Limit** column in the list shows the next reset time.

::: warning Quota has two layers
An API key quota only caps that key's own usage; what is actually deducted is the account quota. If the account quota runs out, requests fail even for an unlimited API key; conversely, once the API key quota is exhausted, requests are blocked no matter how much account balance remains.
:::

### Channel group

- **Group**: the channel group this API key's requests use; leave it empty to follow your account group.
- **Fallback group**: when the primary group is unavailable, requests are routed to the fallback group instead.

If a group has been deleted or you are not allowed to use it, the group tag in the list is highlighted.

### Advanced settings

- **Model restrictions**: once enabled, you can select the models this API key may use; selecting none means all models are allowed. Useful for restricting an application to a handful of models.
- **IP allowlist**: once enabled, enter one IP or CIDR range per line (for example `192.168.1.1`, `10.0.0.0/8`); only requests from those addresses are allowed through.
- **Heartbeat (experimental)**: when a request returns no data for a long time, clients may time out and disconnect. Once enabled, if there is still no response after the wait time you configure, Modeltaps sends a heartbeat every 5 seconds (an empty line for non-streaming responses, `::PING` for streaming) to keep the connection alive. The wait time can be set between 30 and 90 seconds. Not recommended if your requests pass through a relay.

## Testing an API key

Choose **Test command** from the action menu and the dialog shows a runnable `curl` on each of the OpenAI / Claude / Gemini tabs: the command already contains your real key, site address and the selected model, so you can paste it into a terminal to confirm the API key works. Models can be searched and switched in the dropdown.

If a protocol reports **No models available under this protocol**, your current group has no channel of that type and the protocol is temporarily unusable — test with the OpenAI tab instead.

## Disabling and deleting

- The status switch in the list disables or restores an API key at any time; calls made with a disabled API key are rejected outright.
- **Delete** cannot be undone — make sure no production application still uses the key.

## Topping up quota

Account quota is topped up on the **Top up** page (reached from the balance row at the bottom of the sidebar), either by online payment or with a redemption code; top-up records are on the **Transactions** tab of the **Billing** page in the user menu. See [Quickstart](/guide/quickstart) for details.

Once you switch to an organization, this page manages organization API keys: quota comes from the organization quota pool, and each member can also have a periodic budget. See [Organizations](/guide/organization).
