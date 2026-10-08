---
title: "特殊用法"
layout: doc
outline: deep
lastUpdated: true
---

# 特殊用法

## o 系列 / gpt-5 系列快速切换 ReasoningEffort

在请求时，将模型名称后面添加 `#minimal`/`#low`/`#medium`/`#high` 可以快速切换 `reasoning_effort` 参数。适用于 `o1`、`o3`、`o4` 等 `o` 系列模型与 `gpt-5` 系列模型。
例如：

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gpt-5#low",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

请求体中带 `reasoning.effort` 时以它为准；带 `reasoning` 但没有 `effort` 时，`#` 后缀不会生效。详见 [推理设置](/zh/guide/reasoning)。

## Claude 模型开启 thinking

在模型名称后添加 `#thinking` 后缀，即可为任意 Claude 模型开启 thinking 模式，用法与参数详见 [推理设置](/zh/guide/reasoning)。

## Gemini 模型 开启联网搜索

在请求时，增加 `tools` 参数，并设置 `name` 为 `googleSearch` 即可开启联网搜索。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "今天有什么新闻"}],
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

## Gemini 模型 开启代码执行

在请求时，增加 `tools` 参数，并设置 `name` 为 `codeExecution` 即可开启代码执行。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "计算2的7次方"}],
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

## Gemini 模型 开启网页上下文提取

在请求时，增加 `tools` 参数，并设置 `name` 为 `urlContext` 即可开启网页上下文提取。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "解析这个url的内容，https://example.com"}],
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

::: warning 内置工具不能混用

`codeExecution` 与 `urlContext` 互斥，同时传入时只有 `codeExecution` 生效。使用任意一个内置工具（含 `googleSearch`）时，同一请求里的自定义 function 声明都不会下发。`googleSearch` 可以与 `codeExecution` 或 `urlContext` 一起使用。

:::
