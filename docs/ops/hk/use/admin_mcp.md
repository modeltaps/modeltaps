---
title: "讓 AI 助手查設定（管理 MCP）"
layout: doc
outline: deep
lastUpdated: true
---

# 讓 AI 助手查設定（管理 MCP）

AI 助手可以透過 MCP 幫你查站點的設定：有哪些設定、各自做甚麼、在哪裡改、現在的值，以及某個物件（渠道、模型、用戶分組、API 密鑰）實際生效的值。這個端點只讀，不會改動任何東西。

## 連接

- 端點：`https://<你的站點>/mcp/admin`（Streamable HTTP；SSE 在 `/mcp/admin/sse`）。
- 憑證：**root** 帳戶的**只讀**管理密鑰，放在 `Authorization: Bearer <密鑰>` 裡。用 root 登入後在 **API Keys** 頁底部的「管理密鑰」裡建立。其他用戶（包括管理員）的密鑰都會被拒絕。
- 不論用戶 MCP 端點（`/mcp`、`MCP_ENABLE`）有沒有開，這個端點都在。

用戶端設定示例：

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

| 工具 | 回答甚麼 |
|---|---|
| `settings_search` | 按關鍵詞（設定名、名稱、說明、近義詞）找設定，可按物件（`scope`）、分組、只看已修改篩選。平台設定附當前值；密鑰只說已設定或未設定。 |
| `settings_describe` | 一項或幾項設定的完整說明：類型、單位、範圍、取值、說明、示例、相關設定、指南頁和修改它的控制台連結，以及當前值和預設值。 |
| `settings_effective` | 設定實際生效的值和來源。`model.supply_mode`、`channel.fee_rate`、`channel.prompt_caching`、`user_group.ratio`、`token.group` 要給 `object_id`（模型名、渠道 id、分組標識、密鑰 id）。 |
| `settings_docs` | 英文運維文件的段落，含設定參考。 |

## 示例

- 「gpt-5.1 為甚麼走 Requesty？」→ `settings_effective` 查 `model.supply_mode`（`gpt-5.1`），再 `settings_describe` 看 `SupplyCostOrder`。
- 「我們改過哪些預設值？」→ `settings_search` 帶 `modified_only: true`。
- 「怎樣比官方價高 12% 賣？」→ `settings_docs` 搜 "margin group ratio"，再 `settings_describe` 看 `user_group.ratio` 和 `PaymentCostPercent`。

同樣的條目也整理成了頁面：[設定參考](../settings/index.md)。
