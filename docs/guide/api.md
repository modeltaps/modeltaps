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
Open **Test command** on any API key in the **API Keys** page and Modeltaps assembles your key, site address and available models into a runnable `curl`. Copy it into a terminal to verify. Keys are shown only once, so for a key you did not just create or reset, paste it into the dialog first.
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

Besides chat, `/v1` also provides text completions, Responses, embeddings, rerank, moderations, image generation / edit / variation, speech synthesis / transcription / translation, and the realtime API. Request and response structures match the official OpenAI ones. Decision models answer on `/v1/decisions`; see [Decision Models](/guide/decisions).

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

### Live API (realtime voice and video)

Gemini Live sessions (voice, camera frames and screen sharing over one WebSocket) go through the same base URL. The Google SDKs build the address from it, so set the base URL and connect as usual:

```text
wss://your-modeltaps-domain.com/gemini/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent
```

```python
from google import genai
from google.genai import types

client = genai.Client(
    api_key="sk-replace-with-your-key",
    http_options=types.HttpOptions(base_url="https://your-modeltaps-domain.com/gemini"),
)

async with client.aio.live.connect(model="gemini-3.8-live", config={"response_modalities": ["AUDIO"]}) as session:
    ...
```

- The key goes in the `x-goog-api-key` header or the `key` query parameter. `v1`, `v1alpha` and `v1beta` are accepted (the SDKs use `v1` with a custom base URL; it goes upstream as `v1beta`); ephemeral tokens (`BidiGenerateContentConstrained`) are not.
- The model comes from `setup.model` in the first message, and the session is routed to a Gemini channel (Google AI Studio key or service account) that carries it. Messages pass through unchanged.
- Each turn is billed from the `usageMetadata` the model reports: input tokens by modality (text, audio, image and video frames), output tokens and thinking. A session stops when the key, balance or budget runs out.
- Errors arrive as WebSocket close frames with the reason, as from Google: 1007 for an invalid setup, 1008 for a refused request (key, quota, model, content moderation).

## Coding agents

Coding agents have endpoints of their own. The path says which agent is calling; the protocol still comes from the rest of the path, and authentication, routing, billing and errors are the same as on the endpoints above. What differs is the model list, which each endpoint returns in the format its agent reads, so the agent's model picker shows the models your API key can call.

| Base URL | Protocol | For |
| --- | --- | --- |
| `https://your-modeltaps-domain.com/claude-code` | Anthropic Messages: `/v1/messages`, `/v1/messages/count_tokens`, `/v1/models` | Claude Code and the Claude Agent SDK |
| `https://your-modeltaps-domain.com/codex/v1` | OpenAI: `/responses`, `/chat/completions` and the rest of `/v1` | Codex CLI |
| `https://your-modeltaps-domain.com/coding-agent/v1` | OpenAI, same as `/v1` | Other OpenAI-compatible coding agents |
| `https://your-modeltaps-domain.com/coding-agent` | Anthropic Messages, same as `/claude` | Other Anthropic-compatible coding agents |

Usage logs record which agent sent each request: the endpoint decides, and on the other endpoints the agent is recognised from its User-Agent (Claude Code, Codex, OpenCode, Gemini CLI). Organizations can break their usage down by tool; see [Organizations](/guide/organization#usage).

### Claude Code

Add the gateway to `~/.claude/settings.json` (or export the same variables in your shell):

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://your-modeltaps-domain.com/claude-code",
    "ANTHROPIC_AUTH_TOKEN": "sk-replace-with-your-key",
    "ANTHROPIC_API_KEY": "",
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
  }
}
```

- The base URL has no `/v1`: Claude Code appends `/v1/messages` itself.
- Leave `ANTHROPIC_API_KEY` empty. Claude Code uses it before `ANTHROPIC_AUTH_TOKEN` when it is set.
- With `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY`, `/model` lists every chat model your key can call, as `claude-code/<model>`. Models that accept a prompt of one million tokens or more end in `[1m]`, which makes Claude Code use that context window. The gateway removes the prefix and the suffix, so usage and logs show the real model name. Image, audio and embedding models are not listed.
- `cache_control` breakpoints reach the upstream unchanged, and the requests of one Claude Code session stay on one channel, so the upstream prompt cache keeps working. `/context` gets token counts from `/v1/messages/count_tokens`, which is an estimate and is not billed.

### Codex

Add a provider to `~/.codex/config.toml` and put your API key in the environment variable it names:

```toml
model_provider = "modeltaps"
model = "gpt-5.5"

[model_providers.modeltaps]
name = "Modeltaps"
base_url = "https://your-modeltaps-domain.com/codex/v1"
env_key = "MODELTAPS_API_KEY"
wire_api = "responses"
model_catalog_url = "https://your-modeltaps-domain.com/codex/v1/models"
```

```bash
export MODELTAPS_API_KEY=sk-replace-with-your-key
```

- `wire_api` must be `responses`: Codex only speaks the Responses API.
- `model_catalog_url` makes Codex read its model list from Modeltaps, so `/model` lists the chat models your key can call and Codex does not warn about unknown model metadata. Each entry carries Codex's own system prompt, so Codex behaves as it does with its built-in models.
- The `prompt_cache_key` Codex sends reaches the upstream unchanged, and the requests of one Codex session stay on one channel.

### Other coding agents

Point an OpenAI-compatible agent at `https://your-modeltaps-domain.com/coding-agent/v1` and an Anthropic-compatible one at `https://your-modeltaps-domain.com/coding-agent`. They behave exactly like `/v1` and `/claude`; the requests are counted as coding-agent traffic.

## The API catalogue in the console

The **API** group in the sidebar lists one page per modality. `/panel/docs` is the overview; each modality card opens either the **API docs** or the **console**. The docs pages cover the same ground as this page, but the addresses are already filled in with your actual site address:

| Modality | API docs | Console | Endpoints covered |
| --- | --- | --- | --- |
| Chat | `/panel/docs/chat` | `/panel/playground/chat` | `/v1/chat/completions`, `/v1/responses`, `/claude/v1/messages`, `/gemini/v1beta/models/{model}:generateContent` |
| Image | `/panel/docs/image` | `/panel/playground/image` | `/v1/images/generations`, `/v1/images/edits`, `/v1/images/variations`, plus the Recraft and Midjourney extensions |
| Audio | `/panel/docs/speech` | `/panel/playground/speech` | `/v1/audio/speech`, `/v1/audio/transcriptions`, `/v1/audio/translations`, plus `/v1/realtime` |
| Video | `/panel/docs/video` | — (not yet available) | Kling text-to-video / image-to-video and task lookup, plus the Gemini Veo long-running endpoint |

Every docs page has **Endpoints**, **Examples**, **Available models** and a link to the full documentation. The key in the examples is the placeholder `sk-YOUR_TOKEN` — replace it with an API key created on the **API Keys** page and the command runs as is.

### Console

The **Chat**, **Image** and **Audio** pages are the console, calling this site's own `/v1`. Start from an example card or the composer at the bottom; the model and parameters are picked from chips on the composer. Chat streams its answer, can be stopped at any time, and shows time to first token, duration and usage for each turn; image results can be previewed and downloaded; audio is split into **Text to speech** and **Speech to text** segments — listen to and download synthesised speech, or upload or record audio for transcription. Each page can show the equivalent code for the request it just sent, so you can move the same call into your own code.

The model picker is a menu anchored to the model chip, with search, capability filters and grouping by provider; it only lists models available for that capability.

- The console uses a dedicated key the system creates for you — never shown, never copied; the page signs its requests with a short-lived credential (valid for an hour and renewed automatically). **Its usage and cost are billed to your personal account**, separately from the API keys you create by hand.
- It is only usable in a personal context; after switching to an organization the page explains why and links a way out (organization-quota usage is tracked separately).
- When a capability has no available model the page stays browsable, explains why and where to fix it, and sends no request.
- Administrators can turn `builtin_chat_enabled` off in system settings to disable the console site-wide.
- The old Playground addresses (`/playground`, `/panel/playground`) redirect to `/panel/playground/chat`.

#### Compare

**Compare** in the sidebar's **API** group (`/panel/playground/compare`) runs 2–4 chat models side by side as columns: **Add column** at the top right goes up to 4, and each column picks its model — and can override temperature, max tokens and reasoning effort — in its header. With **Sync input** on, one message from the shared composer goes to every column; with it off, only the selected column receives it, and each column can be stopped on its own. **View code** switches between columns and shows each column's equivalent request with its overrides merged in.

Midjourney and async task records are not on the catalogue pages — see the **Midjourney** and **Async tasks** tabs on the **Logs** page.

## About models and groups

- Which models you can call depends on the channel group selected for the API key and on whether the API key has model restrictions enabled. The available list is shown under **Available models** on the **Dashboard**, and can also be queried through the models endpoint of each protocol above.
- Requests on `/claude/v1/messages` can reach any chat model: when a channel does not speak the Anthropic protocol, the gateway converts the request. Claude models are sent upstream over the Anthropic protocol whenever a channel supports it, whichever endpoint you call, so prompt caching keeps working.
- Gemini protocol requests can only be routed to Gemini-type channels. If the Gemini protocol returns no models, your current group has no such channel — use the OpenAI-compatible API instead.
- When a request fails, use the [Usage and logs](/guide/usage) page to locate the exact record by API key and time; the detail view shows the finish reason and the billing breakdown.

## Related

- [Reasoning settings](/guide/reasoning): control the thinking behaviour of reasoning models
- [Advanced usage](/guide/special): call patterns for non-standard scenarios
- [API keys and quota](/guide/tokens): quota, expiry, model restrictions and IP allowlists
- [FAQ](/guide/faq): troubleshooting integration errors
