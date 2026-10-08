---
title: "推論設定"
layout: doc
outline: deep
lastUpdated: true
---

# 推論設定

本ページの説明はすべて `OpenAI SDK` に適用されるもので、`Claude` / `Gemini` のネイティブ API では無効です。

## Claude

### モデル名で thinking を有効にする

リクエスト時にモデル名の後ろへ `#thinking` を付けると thinking モードを有効にできます。

例：

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "claude-sonnet-4-5#thinking",
    "messages": [{"role": "user", "content": "こんにちは"}]
  }'
```

### Claude モデルの推論パラメータをカスタマイズする

以下のパラメータでリクエスト内の推論指定 reasoning を制御できます。

```json
{
  "model": "claude-sonnet-4-5",
  "messages": [],
  "reasoning": {
    "effort": "high", // 強度パラメータ（任意）
    "max_tokens": 2000 // 推論の最大 tokens。現時点では budget_tokens を指します（任意）
  }
}
```

#### 推論を有効にする

リクエストに `reasoning` フィールドが含まれていれば（空オブジェクト `"reasoning": {}` でも）thinking が有効になります。効果は `claude-sonnet-4-5#thinking` と同じで、モデル名を変更する必要はありません。

thinking を有効にすると、リクエスト内の `top_p` は破棄されます。

#### 強度パラメータ

`effort` パラメータには `high`、`medium`、`low` を設定でき、`max_tokens` から比率に応じて `budget_tokens` が切り出されます。

- `low`：`20%`
- `medium`：`50%`
- `high`（およびそれ以外の認識されない値）：`80%`

リクエストで `max_tokens` を明示していない場合は、管理画面の「Claude デフォルト MaxTokens」設定（既定値 `8192`）が使われます。

`effort` も `reasoning.max_tokens` も渡していない場合は、管理画面の「Claude BudgetTokens 割合」設定に従って計算され、既定値は同じく `80%` です。

#### 推論の最大 tokens

`reasoning.max_tokens` パラメータには具体的な数値を設定でき、推論の最大 tokens を表します。現時点では budget_tokens を指します。

:::warning

`reasoning.max_tokens` は `1024` を下回ることも、当該リクエストの `max_tokens` を上回ることもできません。超えた場合は `budget_tokens_too_small` / `budget_tokens_too_large` エラーが返ります。

:::

`budget_tokens` が `1024` を下回る場合は `1024` に引き上げられます。`budget_tokens` が比率計算によるものか、明示的に指定した `reasoning.max_tokens` によるものかを問わず、`max_tokens` が `budget_tokens` 以下であれば、`max_tokens` は自動的に `1280` に調整されます。

#### 優先順位

`reasoning` に `effort` と `max_tokens` を同時に渡した場合、優先順位は次のとおりです。

- `max_tokens` > `effort`

## OpenAI

### モデル名で ReasoningEffort をすばやく切り替える

`o` シリーズ（`o1`、`o3`、`o4` など）と `gpt-5` シリーズのモデルでは、モデル名の後ろへ `#minimal`/`#low`/`#medium`/`#high` を付けると `reasoning_effort` をすばやく設定できます。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gpt-5#low",
    "messages": [{"role": "user", "content": "こんにちは"}]
  }'
```

これらのモデルには自動処理が 2 つあります。リクエスト内の `max_tokens` は `max_completion_tokens` に変換され、`gpt-5-chat-latest` を除いて `temperature` は破棄されます。

### 優先順位

モデル名のサフィックスと `reasoning.effort` を同時に使った場合は `reasoning.effort` が優先されます。モデル名のサフィックスが有効になるのは、リクエストに `reasoning` フィールドがない場合だけです。

## Gemini

### Gemini モデルの推論パラメータをカスタマイズする

`Gemini` の推論パラメータは `Claude` とは異なります。`gemini 2.5` 以降のモデルは既定で推論が有効になっており、無効化または調整するには `reasoning` パラメータを渡す必要があります。

```json
{
  "model": "gemini-2.5-flash",
  "messages": [],
  "reasoning": {
    "effort": "medium", // 思考レベル（任意）
    "max_tokens": 2000 // 推論の最大 tokens。現時点では thinkingBudget を指します（任意）
  }
}
```

`effort` には `minimal`、`low`、`medium`、`high` を指定でき、それぞれ Gemini の `MINIMAL`/`LOW`/`MEDIUM`/`HIGH` の思考レベルに対応します。`reasoning` を渡すと、思考過程の返却（`includeThoughts`）も同時に有効になります。

:::warning

`reasoning.max_tokens` が 0 の場合、推論は無効になります。
`"reasoning": {}` のみを渡した場合は `max_tokens` が 0 の場合と同等で、推論は無効になります。

思考の無効化は `thinkingBudget=0` を許可するモデル（`gemini-2.5-flash` など）でのみ利用できます。`gemini-2.5-pro` は思考の無効化に対応しておらず、`"reasoning": {}` や `"max_tokens": 0` を渡すと上流で拒否されます。

:::

:::warning effort だけを渡すと思考が無効になります

`thinkingBudget` は `reasoning.max_tokens` から取得され、渡されていない場合は `0` として扱われそのまま送信されます。そのため `max_tokens` を渡さずに `effort` だけを指定すると、思考レベルを設定すると同時に予算が 0 になり、かえって思考が無効になります。`effort` を有効にしたい場合は `reasoning.max_tokens` も併せて渡すか、対象モデルの公式ドキュメントに従ってパラメータを選択してください。

:::

`reasoning.max_tokens` がリクエストの `max_tokens` 以上の場合、自動的に `max_tokens - 1` へ引き下げられます（Gemini は thinkingBudget が maxOutputTokens より厳密に小さいことを要求するためです）。

:::tip

マルチターンの対話では、直前の assistant メッセージが思考内容で始まっていない場合、上流から 400 が返るのを避けるため、今回のリクエストでは thinking の設定を送信しません。

:::

管理者は管理画面の `GeminiOpenThink` 設定で、モデルごとに思考過程の強制返却を有効にできます。対象となったモデルは、リクエストに `reasoning` フィールドがなくても `includeThoughts` が送信されます。
