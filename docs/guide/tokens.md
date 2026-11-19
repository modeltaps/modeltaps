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
| API key | Name and the last characters of the key (`sk-…Rc6g`); the full key is never listed |
| Group | The channel group this API key uses; shows **Follow user** when unset |
| Expiry | **Never**, the remaining time, or **Expired** |
| Last used | Time of the last successful call, or **Never used** |
| Usage | Total quota consumed, with today's usage alongside |
| Limit | Remaining quota or **Unlimited**; when a period is set, shows the period cap and the next reset time |
| Status | A switch to enable / disable the API key directly |

**Column settings** in the top right lets you show or hide columns (for example **Created at**, hidden by default). The filter bar at the top supports searching by API key name.

Each row's action menu offers: edit, **Reset key**, **Test command**, **Connect OpenClaw**, jump to that API key's **Usage** and **Logs**, and delete.

## The key is shown only once

Modeltaps does not store your keys, only a fingerprint (hash) of each one, so a key cannot be shown again after it was created:

- When you create an API key, the dialog shows the full key once, with a copy button. Copy it then and keep it somewhere safe, for example in your password manager or your application's secret store.
- Lists and the API key's details only show it masked (`sk-A****Rc6g`), enough to tell keys apart.
- If you lost a key, or think it leaked, choose **Reset key** (in the action menu or on the key row of its details). A new key is shown once and the old one stops working at once; the name, settings, limits and usage stay as they are. In an organization, only the member who created a key can reset it.

Keys created before this change keep working unchanged.

## Handing a key to an agent

Agents that take third-party keys (OpenClaw, Hermes and the like) can connect themselves: tell the agent to read `https://<your site>/skill.md` and set Modeltaps up. That file, served by every Modeltaps site from its own settings, explains the base URLs, how to get a key, how to keep it, how to verify it and what to report back; `/llms.txt` is a short index pointing to it. Neither holds any user data or key.

Don't paste the key into the chat: conversations are stored and synced, and agents with memory may write it down. Give the agent the key one of these two ways instead.

### Pairing code

When you create an API key, the dialog also shows a **pairing code** (`pair-...`). Give the agent the code instead of the key: it redeems it at `POST /api/agent/pair` and receives the key itself.

- A pairing code is valid for 10 minutes, works once and only for that key. Five wrong tries void it.
- Redeeming gives the key a new secret: the key shown in the dialog stops working, and only the agent holds a working copy.
- The **Agent** section of the key's details shows when the key was claimed, from which IP address and with which User-Agent. **New pairing code** issues another code (a pending one stops working); in an organization only the member who created the key can do this.
- **Reset key** voids a pending pairing code, and the agent's copy stops working.

### Device authorization

An agent without a key can ask for one (the OAuth device flow, RFC 8628): it shows you a link to `https://<your site>/device` and a code such as `BCDF-GHJK`. Open the link, sign in, check that the request shows the agent's name, IP address and User-Agent, then choose who the key belongs to (you or an organization you are a member of), its spending limit, expiry and models, and **Approve**. The agent receives the new key on its next check; nothing is shown on the page. A request is open for 15 minutes and is answered once; **Deny** tells the agent no.

Only approve a request you started yourself on an agent you trust. Both ways only ever hand out API keys: management keys cannot be paired or requested, and issuing codes or answering requests needs the signed-in console, never a key.

## Key types: API keys and management keys

**New API key** first asks for the type:

| Type | What it can do | What it cannot do |
| --- | --- | --- |
| **API key** (default) | Call every model API (`/v1/...`, Claude, Gemini and the other relay endpoints); read its own balance and usage (`/v1/dashboard/billing/subscription`, `/v1/dashboard/billing/usage`) and its models (`/v1/models`) | Call the management API (`/api/...`): it is refused with `401` and the code `api_key_not_allowed` |
| **Management, read-only** | Read your balance, usage, logs (without request and response bodies) and the API key list (keys masked), including those of the organizations you belong to; use MCP | Call models; change anything |
| **Management, read-write** | Everything read-only can do, plus create, edit, disable, delete and reset **API keys** | Call models; create management keys; change your password, sign-in methods, passkeys / 2FA, sessions or payments; anything in the admin console |

Keep the key an application or agent uses day to day an API key: if it leaks, the damage stops at its spending limit. Give a management key only to the script or agent that manages your keys, and keep it on the server side.

Management keys:

- start with `mk-` and are sent as `Authorization: Bearer mk-...` to the management API (`/api/...`) and to MCP (`/mcp`). Anything outside what their type allows is refused with `403` and the code `management_key_forbidden`; using one on a model API returns `401`.
- must expire: 90 days by default, at most one year. An expired key is refused with the code `management_key_expired`.
- are shown once, like API keys: only their fingerprint is stored. Lost keys cannot be reset; delete them and create new ones.
- act as you, with your current role: inside an organization a management key never does more than your role there allows, and the API keys it creates are yours (your member budget and model list apply).
- are listed under **Management keys** below the API key list, each with its type tag, and deleted from there; deleting one stops it at once. They can only be created and deleted from the signed-in console, never with another key.

Management keys are personal: in an organization the page creates organization API keys only.

### Access token (deprecated)

The old per-account **access token** (Settings, Account security) is replaced by management keys. It keeps working for one more release with its old permissions, and each response to it carries a `Deprecation: true` header; the server log notes its use. It is now stored as a fingerprint too, so regenerating it shows the new token once. It appears in the management key list as **Access token (deprecated)**, where it can be deleted. Move your scripts to a management key before it stops working.

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

- **Model restrictions**: once enabled, you can select the models this API key may use; selecting none means all models are allowed. Useful for restricting an application to a handful of models. Editing a saved key shows its available models, which also take the key's group and, for organization keys, your member allowlist into account.
- **IP allowlist**: once enabled, enter one IP or CIDR range per line (for example `192.168.1.1`, `10.0.0.0/8`); only requests from those addresses are allowed through.
- **Prompt caching**: how this key's requests use the upstream prompt cache. **Auto** (the default) follows the channel's setting. **On** adds cache breakpoints to Claude requests that carry none, whatever the channel says: a long prompt sent again and again then costs far less, while a prompt sent only once costs 25% more for the cache write. **Off** removes every `cache_control` marker from the request and adds none; providers that cache prefixes on their own, such as OpenAI and DeepSeek, still do. One request can choose differently with the `X-Modeltaps-Prompt-Cache` header (`auto`, `on` or `off`), which wins over the key's setting.
- **Heartbeat (experimental)**: when a request returns no data for a long time, clients may time out and disconnect. Once enabled, if there is still no response after the wait time you configure, Modeltaps sends a heartbeat every 5 seconds (an empty line for non-streaming responses, `::PING` for streaming) to keep the connection alive. The wait time can be set between 30 and 90 seconds. Not recommended if your requests pass through a relay.

## Testing an API key

Choose **Test command** from the action menu and the dialog shows a runnable `curl` on each of the OpenAI / Claude / Gemini tabs, with your site address and the selected model. Models can be searched and switched in the dropdown.

Right after you create or reset a key on the page, the commands already contain it. Otherwise paste the key into the **API key** field at the top of the dialog: it is only used on that page and is not saved. The same applies to **Connect OpenClaw**.

If a protocol reports **No models available under this protocol**, your current group has no channel of that type and the protocol is temporarily unusable — test with the OpenAI tab instead.

## Disabling and deleting

- The status switch in the list disables or restores an API key at any time; calls made with a disabled API key are rejected outright.
- **Delete** cannot be undone — make sure no production application still uses the key.

## Topping up quota

Account quota is topped up on the **Top up** page (reached from the balance row at the bottom of the sidebar), either by online payment or with a redemption code; top-up records are on the **Transactions** tab of the **Billing** page in the user menu. See [Quickstart](/guide/quickstart) for details.

Once you switch to an organization, this page manages organization API keys: quota comes from the organization quota pool, and each member can also have a periodic budget. See [Organizations](/guide/organization).
