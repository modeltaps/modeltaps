---
title: "特殊な呼び出し"
layout: doc
outline: deep
lastUpdated: true
---

# 特殊な呼び出し

## o シリーズ / gpt-5 シリーズで ReasoningEffort をすばやく切り替える

リクエスト時にモデル名の後ろへ `#minimal`/`#low`/`#medium`/`#high` を付けると、`reasoning_effort` パラメータをすばやく切り替えられます。`o1`、`o3`、`o4` などの `o` シリーズモデルと `gpt-5` シリーズモデルに適用されます。
例：

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gpt-5#low",
    "messages": [{"role": "user", "content": "こんにちは"}]
  }'
```

リクエストボディに `reasoning.effort` がある場合はそちらが優先されます。`reasoning` はあるが `effort` がない場合、`#` サフィックスは有効になりません。詳しくは [推論設定](/ja/guide/reasoning) をご覧ください。

## Claude モデルで thinking を有効にする

モデル名の後ろに `#thinking` サフィックスを付けると、任意の Claude モデルで thinking モードを有効にできます。使い方とパラメータは [推論設定](/ja/guide/reasoning) をご覧ください。

## Gemini モデルで Web 検索を有効にする

リクエストに `tools` パラメータを追加し、`name` に `googleSearch` を指定すると Web 検索を有効にできます。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "今日はどんなニュースがありますか"}],
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

## Gemini モデルでコード実行を有効にする

リクエストに `tools` パラメータを追加し、`name` に `codeExecution` を指定するとコード実行を有効にできます。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "2 の 7 乗を計算してください"}],
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

## Gemini モデルで Web ページのコンテキスト抽出を有効にする

リクエストに `tools` パラメータを追加し、`name` に `urlContext` を指定すると Web ページのコンテキスト抽出を有効にできます。

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "gemini-2.5-flash",
    "messages": [{"role": "user", "content": "この URL の内容を解析してください。https://example.com"}],
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

::: warning 組み込みツールは併用できません

`codeExecution` と `urlContext` は排他的で、同時に渡した場合は `codeExecution` のみが有効になります。いずれかの組み込みツール（`googleSearch` を含む）を使用する場合、同じリクエスト内のカスタム function 宣言は送信されません。`googleSearch` は `codeExecution` または `urlContext` と併用できます。

:::
