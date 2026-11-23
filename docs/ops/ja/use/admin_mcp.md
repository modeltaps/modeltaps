---
title: "AI アシスタントで設定を調べる（管理 MCP）"
layout: doc
outline: deep
lastUpdated: true
---

# AI アシスタントで設定を調べる（管理 MCP）

AI アシスタントは MCP を通じてサイトの設定を調べられます。どんな設定があるか、それぞれの働き、変更する場所、現在の値、そしてオブジェクト（チャネル、モデル、ユーザーグループ、API キー）で実際に有効な値です。このエンドポイントは読み取り専用で、何も変更しません。

## 接続

- エンドポイント：`https://<あなたのサイト>/mcp/admin`（Streamable HTTP。SSE は `/mcp/admin/sse`）。
- 認証情報：**root** アカウントの**読み取り**管理キーを `Authorization: Bearer <キー>` で送ります。root でサインインし、**API キー** ページ下部の「管理キー」で作成します。他のユーザー（管理者を含む）のキーは拒否されます。
- ユーザー向け MCP エンドポイント（`/mcp`、`MCP_ENABLE`）のオン・オフに関係なく提供されます。

クライアント設定の例：

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

## ツール

| ツール | 答える内容 |
|---|---|
| `settings_search` | 語句（キー、名前、説明、キーワード）に合う設定。オブジェクト（`scope`）、グループ、変更済みのみで絞り込めます。プラットフォーム設定には現在の値が付き、シークレットは設定済みか未設定かだけを返します。 |
| `settings_describe` | 1 つ以上の設定の詳細：型、単位、範囲、値、説明、例、関連する設定、ガイドのページ、変更するコンソールの URL、現在の値と既定値。 |
| `settings_effective` | 設定が実際に有効になる値とその出所。`channel.fee_rate`、`channel.prompt_caching`、`user_group.ratio`、`token.group` には `object_id`（モデル名、チャネル ID、グループ識別子、キー ID）を指定します。 |
| `settings_docs` | 英語の運用ドキュメントのセクション（設定リファレンスを含む）。 |

## 例

- 「gpt-5.1 はなぜ Requesty に行くのか？」→ `settings_describe` で `SupplyCostOrder`、次に `settings_effective` で Requesty チャネルの `channel.fee_rate`。
- 「既定値から何を変えたか？」→ `settings_search` に `modified_only: true`。
- 「公式価格より 12% 高く売るには？」→ `settings_docs` で "margin group ratio"、次に `settings_describe` で `user_group.ratio` と `PaymentCostPercent`。

同じ項目はページとしても [設定リファレンス](../settings/index.md) にまとめています。
