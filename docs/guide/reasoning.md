---
title: "Reasoning Settings"
layout: doc
outline: deep
lastUpdated: true
---

# Reasoning Settings

Everything on this page applies to the `OpenAI SDK` only; it has no effect on the native `Claude` / `Gemini` APIs.

## Claude

### Enabling thinking via the model name

Append `#thinking` to the model name in the request to enable thinking mode.

For example:

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "claude-sonnet-4-5#thinking",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

### Customizing reasoning parameters for Claude models

Use the following parameters to control the `reasoning` field in a request:

```json
{
  "model": "claude-sonnet-4-5",
  "messages": [],
  "reasoning": {
    "effort": "high", // effort level (optional)
    "max_tokens": 2000 // maximum reasoning tokens; currently maps to budget_tokens (optional)
  }
}
```

#### Enabling reasoning

As long as the request carries a `reasoning` field (even an empty object `"reasoning": {}`), thinking is enabled. The effect is the same as `claude-sonnet-4-5#thinking`, so there is no need to change the model name.

Once thinking is enabled, `top_p` in the request is dropped.

#### Effort level

The `effort` parameter accepts `high`, `medium` or `low`, and carves `budget_tokens` out of `max_tokens` proportionally:

- `low`: `20%`
- `medium`: `50%`
- `high` (and any other unrecognized value): `80%`

If the request does not set `max_tokens` explicitly, the admin setting "Claude default MaxTokens" is used (`8192` by default).

When neither `effort` nor `reasoning.max_tokens` is provided, the admin setting "Claude BudgetTokens percentage" is used, which also defaults to `80%`.

#### Maximum reasoning tokens

`reasoning.max_tokens` accepts an explicit number representing the maximum reasoning tokens, which currently maps to `budget_tokens`.

:::warning

`reasoning.max_tokens` cannot be lower than `1024`, nor greater than the `max_tokens` of the request; otherwise a `budget_tokens_too_small` / `budget_tokens_too_large` error is returned.

:::

If `budget_tokens` falls below `1024`, it is raised to `1024`. Whether `budget_tokens` comes from the percentage calculation or from an explicit `reasoning.max_tokens`, as long as `max_tokens` is less than or equal to `budget_tokens`, `max_tokens` is automatically adjusted to `1280`.

#### Priority

When both `effort` and `max_tokens` are provided inside `reasoning`, the priority is:

- `max_tokens` > `effort`

## OpenAI

### Switching ReasoningEffort quickly via the model name

For the `o` series (`o1`, `o3`, `o4`, etc.) and the `gpt-5` series, append `#minimal`/`#low`/`#medium`/`#high` to the model name to set `reasoning_effort` quickly.

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gpt-5#low",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

Two more things happen automatically on these models: `max_tokens` in the request is converted to `max_completion_tokens`, and `temperature` is dropped for every model except `gpt-5-chat-latest`.

### Priority

When the model name suffix and `reasoning.effort` are used together, `reasoning.effort` wins; the model name suffix only takes effect when the request has no `reasoning` field.

## Gemini

### Customizing reasoning parameters for Gemini models

`Gemini` reasoning parameters differ from `Claude`. `gemini 2.5` and newer models enable reasoning by default; pass the `reasoning` parameter to turn it off or adjust it.

```json
{
  "model": "gemini-2.5-flash",
  "messages": [],
  "reasoning": {
    "effort": "medium", // thinking level (optional)
    "max_tokens": 2000 // maximum reasoning tokens; currently maps to thinkingBudget (optional)
  }
}
```

`effort` accepts `minimal`, `low`, `medium` and `high`, which map to Gemini's `MINIMAL`/`LOW`/`MEDIUM`/`HIGH` thinking levels. Passing `reasoning` also turns on the return of the thinking process (`includeThoughts`).

:::warning

Setting `reasoning.max_tokens` to 0 disables reasoning.
Passing `"reasoning": {}` alone is equivalent to `max_tokens` being 0, which disables reasoning.

Disabling thinking only works on models that allow `thinkingBudget=0` (such as `gemini-2.5-flash`). `gemini-2.5-pro` does not support disabling thinking; passing `"reasoning": {}` or `"max_tokens": 0` to it is rejected upstream.

:::

:::warning Passing effort alone disables thinking

`thinkingBudget` is taken from `reasoning.max_tokens`; when it is absent it is treated as `0` and sent as usual. So passing `effort` without `max_tokens` sets the thinking level while zeroing the budget, which disables thinking instead. To make `effort` take effect, pass `reasoning.max_tokens` as well, or choose parameters according to the official documentation of the model.

:::

If `reasoning.max_tokens` is greater than or equal to the requested `max_tokens`, it is automatically lowered to `max_tokens - 1` (Gemini requires thinkingBudget to be strictly less than maxOutputTokens).

:::tip

In a multi-turn conversation, if the previous assistant message does not start with thinking content, the thinking configuration is not sent for this request, to avoid a 400 from upstream.

:::

Administrators can enable forced return of the thinking process per model in the `GeminiOpenThink` setting in the admin panel: matched models get `includeThoughts` even when the request has no `reasoning` field.
