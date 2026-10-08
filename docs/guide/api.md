---
title: "API Access"
layout: doc
outline: deep
lastUpdated: true
---

# API Access

Modeltaps exposes OpenAI-, Claude- and Gemini-compatible APIs at the same time. Existing applications need no code changes — just point the base URL at your Modeltaps site address and use an API key created on the [API Keys](/guide/tokens) page.

In the examples below, replace `https://your-modeltaps-domain.com` with your actual site address and `sk-replace-with-your-key` with your API key. The same API key works across all three protocols.

::: tip Verify before integrating
Open **Test command** on any API key in the **API Keys** page and Modeltaps assembles your real key, site address and available models into a runnable `curl`. Copy it into a terminal to verify.
:::

## OpenAI-compatible API

Usage is identical to the [OpenAI API](https://platform.openai.com/docs/api-reference/introduction); the base URL is your site root.

::: warning How to write the API base
Clients expect different API base formats. If a connection fails, try these in order:

- `https://your-modeltaps-domain.com`
- `https://your-modeltaps-domain.com/v1`
- `https://your-modeltaps-domain.com/v1/chat/completions`
  :::

### Example

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/v1/chat/completions \
    --header 'Authorization: Bearer sk-replace-with-your-key' \
    -H "Content-Type: application/json" \
    --data '{
      "model": "gpt-5-mini",
      "messages": [
          {
              "role": "user",
              "content": "hi~"
          }
      ]
  }'
```

### Available endpoints

Besides chat, `/v1` also provides text completions, Responses, embeddings, rerank, moderations, image generation / edit / variation, speech synthesis / transcription / translation, and the realtime API. Request and response structures match the official OpenAI ones.

List the models available to the current API key:

```bash
curl https://your-modeltaps-domain.com/v1/models \
  --header 'Authorization: Bearer sk-replace-with-your-key'
```

## Claude-compatible API

Usage is identical to the [Claude API](https://docs.anthropic.com/en/api/messages). The base URL is `https://your-modeltaps-domain.com/claude` and authentication uses the `x-api-key` header.

### Example

```bash
curl --request POST \
    --url https://your-modeltaps-domain.com/claude/v1/messages \
    -H "Content-Type: application/json" \
    -H "x-api-key: sk-replace-with-your-key" \
    -H "anthropic-version: 2023-06-01" \
    --data '{
  "model": "claude-sonnet-4-5",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "hi~"
    }
  ]
}'
```

List the models available under this protocol:

```bash
curl https://your-modeltaps-domain.com/claude/v1/models \
  -H "x-api-key: sk-replace-with-your-key"
```

## Gemini-compatible API

The base URL is `https://your-modeltaps-domain.com/gemini` and authentication uses the `x-goog-api-key` header. The version segment in the path (such as `v1beta`) matches the official API.

### Example

```bash
curl --request POST \
  --url https://your-modeltaps-domain.com/gemini/v1beta/models/gemini-2.5-pro:generateContent \
  --header 'Content-Type: application/json' \
  --header 'x-goog-api-key: sk-replace-with-your-key' \
  --data '{
	"contents": [
		{
			"role": "user",
			"parts": [
				{
					"text": "hi~"
				}
			]
		}
	]
}'
```

List the models available under this protocol:

```bash
curl https://your-modeltaps-domain.com/gemini/v1beta/models \
  -H "x-goog-api-key: sk-replace-with-your-key"
```

## The API catalogue in the console

The **API** group in the sidebar lists one page per modality. `/panel/api` is the overview; each modality card opens either the **API docs** or the **console**. The docs pages cover the same ground as this page, but the addresses are already filled in with your actual site address:

| Modality | API docs | Console | Endpoints covered |
| --- | --- | --- | --- |
| Chat | `/panel/api/docs/chat` | `/panel/api/chat` | `/v1/chat/completions`, `/v1/responses`, `/claude/v1/messages`, `/gemini/v1beta/models/{model}:generateContent` |
| Image | `/panel/api/docs/image` | `/panel/api/image` | `/v1/images/generations`, `/v1/images/edits`, `/v1/images/variations`, plus the Recraft and Midjourney extensions |
| Audio | `/panel/api/docs/speech` | `/panel/api/speech` | `/v1/audio/speech`, `/v1/audio/transcriptions`, `/v1/audio/translations`, plus `/v1/realtime` |
| Video | `/panel/api/docs/video` | — (not yet available) | Kling text-to-video / image-to-video and task lookup, plus the Gemini Veo long-running endpoint |

Every docs page has **Endpoints**, **Examples**, **Available models** and a link to the full documentation. The key in the examples is the placeholder `sk-YOUR_TOKEN` — replace it with an API key created on the **API Keys** page and the command runs as is.

### Console

The **Chat**, **Image** and **Audio** pages are the console, calling this site's own `/v1`. Start from an example card or the composer at the bottom; the model and parameters are picked from chips on the composer. Chat streams its answer, can be stopped at any time, and shows time to first token, duration and usage for each turn; image results can be previewed and downloaded; audio is split into **Text to speech** and **Speech to text** segments — listen to and download synthesised speech, or upload or record audio for transcription. Each page can show the equivalent code for the request it just sent, so you can move the same call into your own code.

The model picker is a menu anchored to the model chip, with search, capability filters and grouping by provider; it only lists models available for that capability.

- The console uses a dedicated key the system creates for you — never shown, never copied. **Its usage and cost are billed to your personal account**, separately from the API keys you create by hand.
- It is only usable in a personal context; after switching to an organization the page explains why and links a way out (organization-quota usage is tracked separately).
- When a capability has no available model the page stays browsable, explains why and where to fix it, and sends no request.
- Administrators can turn `builtin_chat_enabled` off in system settings to disable the console site-wide.
- The old Playground addresses (`/playground`, `/panel/playground`) redirect to `/panel/api/chat`.

#### Compare

**Compare** in the sidebar's **API** group (`/panel/api/compare`) runs 2–4 chat models side by side as columns: **Add column** at the top right goes up to 4, and each column picks its model — and can override temperature, max tokens and reasoning effort — in its header. With **Sync input** on, one message from the shared composer goes to every column; with it off, only the selected column receives it, and each column can be stopped on its own. **View code** switches between columns and shows each column's equivalent request with its overrides merged in.

Midjourney and async task records are not on the catalogue pages — see the **Midjourney** and **Async tasks** tabs on the **Logs** page.

## About models and groups

- Which models you can call depends on the channel group selected for the API key and on whether the API key has model restrictions enabled. The available list is shown under **Available models** on the **Dashboard**, and can also be queried through the models endpoint of each protocol above.
- Claude / Gemini protocol requests can only be routed to channels of the matching type. If those two protocols return no models, your current group has no channel of that type — use the OpenAI-compatible API instead.
- When a request fails, use the [Usage and logs](/guide/usage) page to locate the exact record by API key and time; the detail view shows the finish reason and the billing breakdown.

## Related

- [Reasoning settings](/guide/reasoning): control the thinking behaviour of reasoning models
- [Advanced usage](/guide/special): call patterns for non-standard scenarios
- [API keys and quota](/guide/tokens): quota, expiry, model restrictions and IP allowlists
- [FAQ](/guide/faq): troubleshooting integration errors
