---
title: "Settings for AI assistants (admin MCP)"
layout: doc
outline: deep
lastUpdated: true
---

# Settings for AI assistants (admin MCP)

An AI assistant can look up your site's settings for you over MCP: which settings exist, what each does, where to change it, its current value, and the value an object (a channel, a model, a user group, an API key) actually works with. The endpoint only reads; nothing is changed through it.

## Connect

- Endpoint: `https://<your site>/mcp/admin` (streamable HTTP; SSE at `/mcp/admin/sse`).
- Credential: a **read** management key of the **root** account, sent as `Authorization: Bearer <key>`. Create it at the bottom of the **API Keys** page (Management keys) while signed in as root. Keys of other users are refused, even admins.
- The endpoint is served whether or not the user MCP endpoint (`/mcp`, `MCP_ENABLE`) is on.

Example client configuration:

```json
{
  "mcpServers": {
    "modeltaps-admin": {
      "type": "http",
      "url": "https://gateway.example.com/mcp/admin",
      "headers": { "Authorization": "Bearer mk-..." }
    }
  }
}
```

## Tools

| Tool | What it answers |
|---|---|
| `settings_search` | Settings matching some words (key, name, description, keywords), filtered by object (`scope`), group, modified only. Platform settings come with their current value; secrets only say set or unset. |
| `settings_describe` | One or more settings in full: type, unit, range, values, description, examples, related settings, guide page and the console URL to change it, with current and default values. |
| `settings_effective` | The value a setting takes effect with and where it comes from. For `channel.fee_rate`, `channel.prompt_caching`, `user_group.ratio` and `token.group`, give `object_id` (model name, channel id, group symbol, key id). |
| `settings_docs` | Sections of the English operator docs, including the settings reference. |

## Examples

- "Why does gpt-5.1 go to Requesty?" → `settings_describe` `SupplyCostOrder`, then `settings_effective` `channel.fee_rate` for the Requesty channel.
- "What did we change from the defaults?" → `settings_search` with `modified_only: true`.
- "How do I sell at 12% over the official price?" → `settings_docs` "margin group ratio", then `settings_describe` `user_group.ratio` and `PaymentCostPercent`.

The [settings reference](../settings/index.md) holds the same entries as pages.
