---
title: "特殊用法"
layout: doc
outline: deep
lastUpdated: true
---

# 特殊用法

## o 系列 / gpt-5 系列快速切換 ReasoningEffort

在請求時，將模型名稱後面添加 `#minimal`/`#low`/`#medium`/`#high` 可以快速切換 `reasoning_effort` 參數。適用於 `o1`、`o3`、`o4` 等 `o` 系列模型與 `gpt-5` 系列模型。
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

請求體中帶 `reasoning.effort` 時以它為準；帶 `reasoning` 但沒有 `effort` 時，`#` 後綴不會生效。詳見 [推理設定](/hk/guide/reasoning)。

## Claude 模型開啟 thinking

在模型名稱後添加 `#thinking` 後綴，即可為任意 Claude 模型開啟 thinking 模式，用法與參數詳見 [推理設定](/hk/guide/reasoning)。

## Gemini 模型 開啟聯網搜索

在請求時，增加 `tools` 參數，並設定 `name` 為 `googleSearch` 即可開啟聯網搜索。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "今天有甚麼新聞"}],
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

## Gemini 模型 開啟代碼執行

在請求時，增加 `tools` 參數，並設定 `name` 為 `codeExecution` 即可開啟代碼執行。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "計算 2 的 7 次方"}],
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

## Gemini 模型 開啟網頁上下文提取

在請求時，增加 `tools` 參數，並設定 `name` 為 `urlContext` 即可開啟網頁上下文提取。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "解析這個 url 的內容，https://example.com"}],
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

::: warning 內置工具不能混用

`codeExecution` 與 `urlContext` 互斥，同時傳入時只有 `codeExecution` 生效。使用任意一個內置工具（含 `googleSearch`）時，同一請求裏的自定義 function 聲明都不會下發。`googleSearch` 可以與 `codeExecution` 或 `urlContext` 一起使用。

:::
