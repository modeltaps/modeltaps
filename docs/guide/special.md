---
title: "Advanced Usage"
layout: doc
outline: deep
lastUpdated: true
---

# Advanced Usage

## Switching ReasoningEffort quickly on o-series / gpt-5-series models

Append `#minimal`/`#low`/`#medium`/`#high` to the model name in the request to switch the `reasoning_effort` parameter quickly. This applies to the `o` series (`o1`, `o3`, `o4`, etc.) and the `gpt-5` series.
For example:

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gpt-5#low",
    "messages": [{"role": "user", "content": "hello"}]
  }'
```

When the request body carries `reasoning.effort`, that value wins; when it carries `reasoning` without `effort`, the `#` suffix has no effect. See [Reasoning Settings](/guide/reasoning) for details.

## Enabling thinking on Claude models

Append the `#thinking` suffix to the model name to enable thinking mode on any Claude model; see [Reasoning Settings](/guide/reasoning) for usage and parameters.

## Enabling web search on Gemini models

Add a `tools` parameter with `name` set to `googleSearch` to enable web search.

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "What is in the news today?"}],
    "tools": [
		{
			"function": {
				"name": "googleSearch",
				"parameters": {}
			},
			"type": "function"
		}
	]
  }'
```

## Enabling code execution on Gemini models

Add a `tools` parameter with `name` set to `codeExecution` to enable code execution.

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "Compute 2 to the power of 7"}],
    "tools": [
		{
			"function": {
				"name": "codeExecution",
				"parameters": {}
			},
			"type": "function"
		}
	]
  }'
```

## Enabling URL context extraction on Gemini models

Add a `tools` parameter with `name` set to `urlContext` to enable URL context extraction.

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "Summarize the content of this url: https://example.com"}],
    "tools": [
		{
			"function": {
				"name": "urlContext",
				"parameters": {}
			},
			"type": "function"
		}
	]
  }'
```

::: warning Built-in tools cannot be mixed freely

`codeExecution` and `urlContext` are mutually exclusive; when both are passed only `codeExecution` takes effect. Whenever any built-in tool is used (including `googleSearch`), custom function declarations in the same request are not forwarded. `googleSearch` can be combined with either `codeExecution` or `urlContext`.

:::
