---
title: "Auto Routing"
layout: doc
outline: deep
lastUpdated: true
---

# Auto Routing

An auto routing policy is a model name you can call like any other model, such as `auto/coder`. Behind it, the gateway reads the **first user message** of each conversation, asks a small decision model which kind of request it is, and forwards the conversation to the model the administrator mapped to that kind. Simple work goes to a cheap, fast model; hard work goes to the strongest one. The choice is then kept for the whole session.

Coding agents such as Claude Code, Codex or Cursor cannot pass routing parameters in a request; they only know a model name. A policy packs the routing into that name.

## Calling a policy

Use the policy name in the `model` field. It works on the Chat Completions, Claude Messages and Responses APIs, with streaming and tools, and it appears in `GET /v1/models` like any other model.

```bash
curl -X POST https://your-modeltaps-domain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-proj-1234567890" \
  -d '{
    "model": "auto/coder",
    "messages": [{"role": "user", "content": "Add a --verbose flag to the CLI and log the parsed arguments"}]
  }'
```

The response `model` field is the model that actually answered (unless the site is configured to echo the requested name). Every routed response also carries a header:

```
X-Modeltaps-Route: policy=auto/coder; label=additive; model=claude-haiku-5-5; confidence=0.90; reason=classified
```

| Reason | Meaning |
| --- | --- |
| `classified` | The decision model picked a label with confidence at or above the policy threshold. |
| `pinned` | A later turn of a session that was already routed; no new decision was made. |
| `below_threshold` | A label was picked but with too little confidence; the fallback model answered. |
| `decision_error` | The decision call failed, timed out or answered with an unknown label; the fallback model answered. |
| `empty_input` | The first user message had no text (for example only an image); the fallback model answered. |
| `auxiliary` | A Claude Code side request (such as generating the session title) arrived before the session was routed; the fallback model answered and no decision was made. |

A request to a policy is never rejected because of routing: whenever the decision cannot be made, the fallback model answers.

## Sessions

The decision is made once per session and reused for later turns, so a long conversation never switches models halfway and the upstream prompt cache keeps working. A session is recognised, in this order, by the Claude Code session in `metadata.user_id`, a session header (`session_id`, `session-id`, `x-session-id`, `x-claude-code-session-id`), the OpenAI `prompt_cache_key`, or the prompt prefix (system prompt plus first user message). Sending a stable session id is the most reliable way to keep a conversation on one model; a new id starts a new decision. Only a real classification is pinned: after a `decision_error` or `empty_input` fallback the next turn is classified again, so a brief outage of the decision model does not hold new sessions on the fallback model.

How long a session stays pinned is set per policy by the administrator (one hour by default).

## Using a policy in coding agents

**Claude Code**

```bash
export ANTHROPIC_BASE_URL=https://your-modeltaps-domain.com/claude
export ANTHROPIC_AUTH_TOKEN=sk-proj-1234567890
export ANTHROPIC_MODEL=auto/coder
claude
```

**Codex CLI** — set the model in `~/.codex/config.toml` for the provider that points at the gateway:

```toml
model = "auto/coder"
```

**Cursor and other OpenAI-compatible clients** — add the gateway as an OpenAI-compatible endpoint and enter `auto/coder` as a custom model name.

Coding agents add their own context to the conversation: Claude Code puts `<system-reminder>` blocks into the first message, and Codex sends `<environment_context>` and your `AGENTS.md` instructions as a separate message ahead of your prompt. The gateway strips these blocks and skips messages that contain nothing else, so the decision is based on what you actually typed. Claude Code also sends side requests with the same model, such as generating the session title; they carry no tools and are not classified. Before the session is routed they go to the fallback model; after that they follow the session.

## Billing and logs

- The routed request is billed at the price of the model that answered, not at a price of the policy itself. On the model catalog, a policy shows "billed by target model" instead of its own prices.
- The decision call is billed to you once per session at the decision model's price, as its own log entry marked as a decision call. The site administrator can turn this off.
- In your logs, the routed request shows the policy, the chosen label, the confidence and the reason; you can filter logs by routing policy.

## Setting up a policy (administrators)

Policies are created in the console under **Gateway → Auto routing**. A policy consists of:

- **Model name** — what clients call, for example `auto/coder`. Letters, digits and `. _ / -` only; it must not collide with an existing model. Once created it shows up in `/v1/models` and can be added to token and organization allow lists like any model. Allow lists are checked against the policy name; the target models do not have to be in the caller's allow list, so you can keep expensive or hidden models reachable only through a policy.
- **Decision model** — a small, fast chat model that classifies the first message. Any chat model the gateway serves will do; pick one that follows instructions well. It must not itself be a policy.
- **Question and routes** — a one-sentence question and two to sixteen labels, each with a short description and a target model. Describe the request, not the model, and keep labels distinct: overlapping labels split the probability and push requests to the fallback. Three templates (coding change size, support triage, reasoning depth) give a starting point.
- **Fallback model and threshold** — the fallback answers whenever the decision cannot be trusted. Put your safest model there. The threshold (0 to 1, 0.5 is a good start) is compared with the confidence the decision model reports; raising it sends more borderline requests to the fallback. The confidence is self-reported, so treat it as a ranking rather than a probability.
- **Advanced** — how long a session stays pinned (0 classifies every request), how many characters of the first message are sent to the decision model, and the decision timeout.

The **Try it** tab runs only the decision step on a pasted first message, optionally comparing several decision models side by side, without forwarding, billing or pinning anything. Test with real first messages from your logs before enabling a policy.

The decision billing switch is under **Settings → Gateway → Auto routing**.
