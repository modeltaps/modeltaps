---
title: "让 AI 助手查设置（管理 MCP）"
layout: doc
outline: deep
lastUpdated: true
---

# 让 AI 助手查设置（管理 MCP）

AI 助手可以通过 MCP 帮你查站点的设置：有哪些设置、各自做什么、在哪里改、现在的值，以及某个对象（渠道、模型、用户分组、API 密钥）实际生效的值。这个端点只读，不会改动任何东西。

## 连接

- 端点：`https://<你的站点>/mcp/admin`（Streamable HTTP；SSE 在 `/mcp/admin/sse`）。
- 凭据：**root** 账号的**只读**管理密钥，放在 `Authorization: Bearer <密钥>` 里。用 root 登录后在 **API Keys** 页底部的「管理密钥」里创建。其他用户（包括管理员）的密钥都会被拒绝。
- 不管用户 MCP 端点（`/mcp`、`MCP_ENABLE`）开没开，这个端点都在。

客户端配置示例：

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

## 工具

| 工具 | 回答什么 |
|---|---|
| `settings_search` | 按关键词（设置名、名称、说明、近义词）找设置，可按对象（`scope`）、分组、只看已修改筛选。平台设置附当前值；密钥只说已设置或未设置。 |
| `settings_describe` | 一项或几项设置的完整说明：类型、单位、范围、取值、说明、示例、相关设置、指南页和修改它的控制台链接，以及当前值和默认值。 |
| `settings_effective` | 设置实际生效的值和来源。`model.supply_mode`、`channel.fee_rate`、`channel.prompt_caching`、`user_group.ratio`、`token.group` 要给 `object_id`（模型名、渠道 id、分组标识、密钥 id）。 |
| `settings_docs` | 英文运维文档的段落，含设置参考。 |

## 示例

- 「gpt-5.1 为什么走 Requesty？」→ `settings_effective` 查 `model.supply_mode`（`gpt-5.1`），再 `settings_describe` 看 `SupplyCostOrder`。
- 「我们改过哪些默认值？」→ `settings_search` 带 `modified_only: true`。
- 「怎样比官方价高 12% 卖？」→ `settings_docs` 搜 "margin group ratio"，再 `settings_describe` 看 `user_group.ratio` 和 `PaymentCostPercent`。

同样的条目也整理成了页面：[设置参考](../settings/index.md)。
