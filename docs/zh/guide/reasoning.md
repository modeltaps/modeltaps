---
title: "推理设置"
layout: doc
outline: deep
lastUpdated: true
---

# 推理设置

本页面所有说明均适用于 `OpenAI SDK`，在 `Claude` / `Gemini` 原生 API 中无效。

## Claude

### 通过模型名开启 thinking

在请求时，将模型名称后面添加 `#thinking` 可以开启 thinking 模式。

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

### 自定义 Claude 模型推理参数

可使用以下参数控制请求中的推理标记 reasoning：

```json
{
  "model": "claude-sonnet-4-5",
  "messages": [],
  "reasoning": {
    "effort": "high", // 强度参数 (可选)
    "max_tokens": 2000 // 推理最大tokens，目前特指budget_tokens (可选)
  }
}
```

#### 开启推理

只要请求中带了 `reasoning` 字段（哪怕是空对象 `"reasoning": {}`），就会开启 thinking，效果与 `claude-sonnet-4-5#thinking` 一致，无需再更改模型名称。

开启 thinking 后，请求中的 `top_p` 会被丢弃。

#### 强度参数

`effort` 参数可以设置为 `high`、`medium` 或 `low`，按比例从 `max_tokens` 中划出 `budget_tokens`：

- `low`：`20%`
- `medium`：`50%`
- `high`（以及其他未识别的取值）：`80%`

若请求未显式指定 `max_tokens`，则使用后台「Claude 默认 MaxTokens」设置（默认 `8192`）。

未传 `effort` 也未传 `reasoning.max_tokens` 时，按后台「Claude BudgetTokens 百分比」设置计算，默认同样是 `80%`。

#### 推理最大 tokens

`reasoning.max_tokens` 参数可以设置为具体的数值，表示推理最大 tokens，目前特指 budget_tokens。

:::warning

`reasoning.max_tokens` 不能低于 `1024`，也不能大于本次请求的 `max_tokens`，否则会返回 `budget_tokens_too_small` / `budget_tokens_too_large` 错误。

:::

`budget_tokens` 若低于 `1024`，会被抬到 `1024`。无论 `budget_tokens` 来自比例计算还是显式传入的 `reasoning.max_tokens`，只要 `max_tokens` 小于或等于 `budget_tokens`，`max_tokens` 就会被自动调整为 `1280`。

#### 优先级

若同时在 `reasoning` 中传入 `effort` 和 `max_tokens`，优先级如下：

- `max_tokens` > `effort`

## OpenAI

### 通过模型名快速切换 ReasoningEffort

对于 `o` 系列（`o1`、`o3`、`o4` 等）与 `gpt-5` 系列模型，在模型名称后面添加 `#minimal`/`#low`/`#medium`/`#high` 可以快速设置 `reasoning_effort`。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gpt-5#low",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

这些模型上还有两个自动处理：请求里的 `max_tokens` 会转成 `max_completion_tokens`；除 `gpt-5-chat-latest` 外，`temperature` 会被丢弃。

### 优先级

若同时使用模型名后缀和 `reasoning.effort`，以 `reasoning.effort` 为准；仅当请求中没有 `reasoning` 字段时，模型名后缀才会生效。

## Gemini

### 自定义 Gemini 模型推理参数

`Gemini` 的推理参数与 `Claude` 不同。`gemini 2.5` 及更新的模型默认开启推理，如需关闭或调整则需传入 `reasoning` 参数。

```json
{
  "model": "gemini-2.5-flash",
  "messages": [],
  "reasoning": {
    "effort": "medium", // 思考档位 (可选)
    "max_tokens": 2000 // 推理最大tokens，目前特指thinkingBudget (可选)
  }
}
```

`effort` 可取 `minimal`、`low`、`medium`、`high`，分别对应 Gemini 的 `MINIMAL`/`LOW`/`MEDIUM`/`HIGH` 思考档位。传入 `reasoning` 后会同时开启思考过程返回（`includeThoughts`）。

:::warning

`reasoning.max_tokens` 为 0 时关闭推理。
仅传入 `"reasoning": {}` 时，等价于 `max_tokens` 为 0，即关闭推理。

关闭思考只适用于允许 `thinkingBudget=0` 的模型（如 `gemini-2.5-flash`）。`gemini-2.5-pro` 不支持关闭思考，对它传 `"reasoning": {}` 或 `"max_tokens": 0` 会被上游拒绝。

:::

:::warning 只传 effort 会关掉思考

`thinkingBudget` 取自 `reasoning.max_tokens`，未传时按 `0` 处理并照常下发。因此只给 `effort` 而不给 `max_tokens`，会在设置思考档位的同时把预算置 0，反而关闭思考。需要 `effort` 生效时，请同时传入 `reasoning.max_tokens`，或按对应模型的官方文档选择参数。

:::

若 `reasoning.max_tokens` 大于等于请求的 `max_tokens`，会被自动下调为 `max_tokens - 1`（Gemini 要求 thinkingBudget 严格小于 maxOutputTokens）。

:::tip

多轮对话中，如果上一条 assistant 消息不是以思考内容开头，本次请求不会下发 thinking 配置，以避免上游返回 400。

:::

管理员可在后台的 `GeminiOpenThink` 设置中按模型开启强制返回思考过程：命中的模型即便请求里没有 `reasoning` 字段，也会下发 `includeThoughts`。
