---
title: "推理設定"
layout: doc
outline: deep
lastUpdated: true
---

# 推理設定

本頁面所有說明均適用於 `OpenAI SDK`，在 `Claude` / `Gemini` 原生 API 中無效。

## Claude

### 通過模型名開啟 thinking

在請求時，將模型名稱後面添加 `#thinking` 可以開啟 thinking 模式。

例如：

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "claude-sonnet-4-5#thinking",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

### 自定義 Claude 模型推理參數

可使用以下參數控制請求中的推理標記 reasoning：

```json
{
  "model": "claude-sonnet-4-5",
  "messages": [],
  "reasoning": {
    "effort": "high", // 強度參數（可選）
    "max_tokens": 2000 // 推理最大 tokens，目前特指 budget_tokens（可選）
  }
}
```

#### 開啟推理

只要請求中帶了 `reasoning` 字段（哪怕是空對象 `"reasoning": {}`），就會開啟 thinking，效果與 `claude-sonnet-4-5#thinking` 一致，無需再更改模型名稱。

開啟 thinking 後，請求中的 `top_p` 會被丟棄。

#### 強度參數

`effort` 參數可以設定為 `high`、`medium` 或 `low`，按比例從 `max_tokens` 中劃出 `budget_tokens`：

- `low`：`20%`
- `medium`：`50%`
- `high`（以及其他未識別的取值）：`80%`

若請求未顯式指定 `max_tokens`，則使用後台「Claude 預設 MaxTokens」設定（預設 `8192`）。

未傳 `effort` 也未傳 `reasoning.max_tokens` 時，按後台「Claude BudgetTokens 百分比」設定計算，預設同樣是 `80%`。

#### 推理最大 tokens

`reasoning.max_tokens` 參數可以設定為具體的數值，表示推理最大 tokens，目前特指 budget_tokens。

:::warning

`reasoning.max_tokens` 不能低於 `1024`，也不能大於本次請求的 `max_tokens`，否則會返回 `budget_tokens_too_small` / `budget_tokens_too_large` 錯誤。

:::

`budget_tokens` 若低於 `1024`，會被抬到 `1024`。無論 `budget_tokens` 來自比例計算還是顯式傳入的 `reasoning.max_tokens`，只要 `max_tokens` 小於或等於 `budget_tokens`，`max_tokens` 就會被自動調整為 `1280`。

#### 優先級

若同時在 `reasoning` 中傳入 `effort` 和 `max_tokens`，優先級如下：

- `max_tokens` > `effort`

## OpenAI

### 通過模型名快速切換 ReasoningEffort

對於 `o` 系列（`o1`、`o3`、`o4` 等）與 `gpt-5` 系列模型，在模型名稱後面添加 `#minimal`/`#low`/`#medium`/`#high` 可以快速設定 `reasoning_effort`。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gpt-5#low",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

這些模型上還有兩個自動處理：請求裏的 `max_tokens` 會轉成 `max_completion_tokens`；除 `gpt-5-chat-latest` 外，`temperature` 會被丟棄。

### 優先級

若同時使用模型名後綴和 `reasoning.effort`，以 `reasoning.effort` 為準；僅當請求中沒有 `reasoning` 字段時，模型名後綴才會生效。

## Gemini

### 自定義 Gemini 模型推理參數

`Gemini` 的推理參數與 `Claude` 不同。`gemini 2.5` 及更新的模型預設開啟推理，如需關閉或調整則需傳入 `reasoning` 參數。

```json
{
  "model": "gemini-2.5-flash",
  "messages": [],
  "reasoning": {
    "effort": "medium", // 思考檔位（可選）
    "max_tokens": 2000 // 推理最大 tokens，目前特指 thinkingBudget（可選）
  }
}
```

`effort` 可取 `minimal`、`low`、`medium`、`high`，分別對應 Gemini 的 `MINIMAL`/`LOW`/`MEDIUM`/`HIGH` 思考檔位。傳入 `reasoning` 後會同時開啟思考過程返回（`includeThoughts`）。

:::warning

`reasoning.max_tokens` 為 0 時關閉推理。
僅傳入 `"reasoning": {}` 時，等價於 `max_tokens` 為 0，即關閉推理。

關閉思考只適用於允許 `thinkingBudget=0` 的模型（如 `gemini-2.5-flash`）。`gemini-2.5-pro` 不支持關閉思考，對它傳 `"reasoning": {}` 或 `"max_tokens": 0` 會被上游拒絕。

:::

:::warning 只傳 effort 會關掉思考

`thinkingBudget` 取自 `reasoning.max_tokens`，未傳時按 `0` 處理並照常下發。因此只給 `effort` 而不給 `max_tokens`，會在設定思考檔位的同時把預算置 0，反而關閉思考。需要 `effort` 生效時，請同時傳入 `reasoning.max_tokens`，或按對應模型的官方文檔選擇參數。

:::

若 `reasoning.max_tokens` 大於等於請求的 `max_tokens`，會被自動下調為 `max_tokens - 1`（Gemini 要求 thinkingBudget 嚴格小於 maxOutputTokens）。

:::tip

多輪對話中，如果上一條 assistant 消息不是以思考內容開頭，本次請求不會下發 thinking 配置，以避免上游返回 400。

:::

管理員可在後台的 `GeminiOpenThink` 設定中按模型開啟強制返回思考過程：命中的模型即便請求裏沒有 `reasoning` 字段，也會下發 `includeThoughts`。
