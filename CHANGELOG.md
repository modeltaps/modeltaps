# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- A user's own usage total on the logs page is counted by user ID, like the log list, instead of
  by username: logs from before a rename are no longer left out, and the query uses the
  per-user index instead of scanning every log in the time range.
- Gemini Long Decode can be continued through the native non-streaming `generateContent` relay:
  the response keeps `candidates[].continuationToken` instead of dropping it. A
  `finishReason` of `CONTINUATION` now maps to `length` on the OpenAI-compatible endpoints and in
  the logs, instead of `content_filter` / `error`.
- A channel custom parameter `store` (such as `{"store": false}` to stop OpenAI from retaining
  chats) is now only sent to Chat Completions and Responses. Image, embedding, speech, moderation
  and legacy Completions requests on the same channel failed with `Unknown parameter: 'store'`.
- Sticky sessions for Claude Code, Gemini CLI and Codex account channels never matched: the
  session lookup asserted an interface the router's request context no longer satisfies, so every
  request was spread across accounts. They now stick as intended, also without Redis.
- Request fields that move the upstream to a pricier processing tier are no longer forwarded by
  default: OpenAI `service_tier` `priority` / `fast` / `ultrafast` on Chat Completions and Responses,
  and Claude `speed: "fast"` and `inference_geo: "us"` on the native Messages pass-through. The
  upstream charged the higher rate while the request was billed at the standard price. A new
  per-channel switch, "Allow premium processing tiers", forwards them again.
- Requests are billed at the processing tier the upstream reports: OpenAI `service_tier` (Fast /
  Priority 2x, Ultrafast 6x, Flex 0.5x), Claude `usage.speed` fast (2x) and `usage.inference_geo`
  `us` (1.1x). The tier and its ratio are recorded in the log details; the upstream cost follows
  the same factor.
- Model lists (`/v1/models`, `/claude/v1/models`, Gemini `models`) for organization API keys only
  show the models the member's allowlist permits, so clients and coding agents no longer offer
  models that are then rejected. An API key with the model limit on and no model selected now lists
  no models, matching the requests it rejects, and the key form no longer saves that combination.
- Requests for a model the API key may not use return `404 model_not_found` instead of a `503`
  "upstream at capacity" error that SDKs keep retrying.
- Organization member limits: an organization's own limit can now be unlimited even when the
  site default is not. `max_members` is null to follow the site default, 0 for unlimited, or a
  positive limit; existing organizations keep following the site default. Site admins set it from
  the organization list, which also shows each organization's members against its limit.
- Claude-protocol requests (`/v1/messages`, as sent by Claude Code) to OpenRouter and Vercel AI
  Gateway channels are forwarded to the gateways' Anthropic-compatible endpoints instead of being
  converted to Chat Completions and back. The conversion dropped the client's `cache_control`
  breakpoints on system prompts, tool definitions and recent turns, so prompt caching was lost
  through these channels; thinking blocks and beta headers now also pass through unchanged, and
  cache read / write tokens are billed from the upstream usage.

### Added

- Subscription channels show their usage windows instead of a money balance: refreshing a Codex
  channel's balance (or the periodic refresh) queries its 5-hour and weekly windows, and Claude
  Code channels keep the windows Anthropic reports on each response. The channel list shows the
  used percentage per window with the reset times in a tooltip.
- Hourly channel metrics: attempts, successes, latency, time to first token and output speed per
  channel and model, counted in memory and written every minute, kept 30 days by default.
  `GET /api/channel/metrics` returns them; the log settings card turns them off or sets the
  retention.
- Channel options for the upstream model check: "Ignored upstream models" (exact names, `*`
  patterns or `re:` regular expressions) are never reported as new, and "Add new upstream models
  automatically" adds newly offered models to the channel with their prices and metadata.
- Account channel credentials (Codex, Claude Code, Gemini CLI, Antigravity) are refreshed in the
  background every 30 minutes when they expire soon, including idle and auto-disabled channels;
  failures are sent as a credential notice. The interval is set in the channel auto-disable
  settings (0 turns it off).
- Admin notifications can be turned on or off per kind (channel disabled / re-enabled, channel
  test, low balance, price changes, model list changes, model metadata, credential problems) in the
  new "Admin notifications" settings card, and each kind is limited to 20 notices per hour by
  default; the first notice after a limited hour says how many were dropped.
- Upstream model check: the consume log records the model sent upstream when a mapping renamed
  it, and flags a response whose declared model is neither the requested nor the mapped one
  (dated, regional and provider-path variants still match; Azure deployments are skipped). The
  request log shows a warning on those rows, and `GET /api/channel/cache_stats` counts them per
  channel.
- Channel price sync shows a review first: each channel model with its upstream price change
  (new, increase, decrease) and the user groups priced below the channel's cost ratio. Only the
  selected models are written.
- Failed requests are logged: when a request still fails after retries, a log entry records the
  status and message the client received, the channels tried and their status codes. The request
  log page switches between succeeded and failed requests, and the log details show the retry chain
  (the provider responses list and the fallback count) for both. The "Record failed requests"
  switch in the log settings turns it off.
- The scheduled channel test is set in the gateway settings (test mode and interval). The new
  "recovery" mode tests only auto-disabled channels and re-enables those that pass, so healthy
  channels spend no quota on tests.
- Session affinity on every channel type: requests of one client session stay on the channel that
  served the session, so the upstream prompt cache keeps hitting when several channels serve a
  model. Sessions are identified by the Claude Code session in `metadata.user_id`, a session
  header (`session_id`, as sent by Codex), the OpenAI `prompt_cache_key`, or the same system prompt
  and first message, scoped to the API key and model. It is on by default and works without Redis
  (an in-process map is used). The new "Session affinity" settings card turns it off, sets the
  binding lifetime (default one hour after the last request) and offers a strict mode that returns
  the error instead of moving a session whose channel fails. Each consume log records whether the
  request stayed on its bound channel (`session_affinity`), and `GET /api/channel/cache_stats`
  reports per-channel requests, prompt and cache read tokens and affinity counts.
- Vercel AI Gateway channel type: OpenAI-compatible relay, model list with prices and metadata
  synced like OpenRouter, and balance lookup through `/v1/credits`.
- OpenRouter channels report the key's remaining amount as the channel balance and store a quota
  snapshot (limit, remaining, reset interval, daily / weekly / monthly usage).
- Channels have a low-balance alert threshold; the periodic balance refresh
  (`CHANNEL_UPDATE_FREQUENCY`) notifies once when the balance drops below it.

### Changed

- The daily upstream model check (`MODEL_DRIFT_AUTO_CHECK`) is on by default.
- Automatic price syncs (startup, daily, channel save) write new models and price increases, and
  hold decreases for review; held changes and locked prices below the upstream price are sent as
  notifications. The "Automatic price sync" setting in the billing settings can write every change
  (the previous behavior) or new models only.
- Failover defaults: retries default to 2 (never more than the channels available), automatic
  re-enabling of auto-disabled channels defaults to on, and the scheduled channel test defaults to
  recovery mode every 10 minutes. Sites that saved their own values keep them;
  `CHANNEL_TEST_FREQUENCY` still tests every channel at its interval when set.
- "Members can view everyone's usage" is a plain organization switch: the site-wide default for it
  is removed and new organizations start with it off, so members only see their own usage until an
  owner or admin turns it on. Existing organizations keep their current behavior.
- Periodic and bulk balance refresh covers every channel type with a balance API instead of
  OpenAI only, and skips types without one; the single-channel refresh reports "not supported"
  for those types.
- Saving a channel no longer overwrites its balance, quota snapshot or model drift result with the
  form's stale values.

### Fixed

- Azure v1 channels accept `/v1/images/edits` and `/v1/images/variations` instead of answering
  501 `unsupported_api`.

### Security

- Upstream errors that reached clients without the usual masking are now masked: errors arriving
  after a stream started (both stream formats), Midjourney submit errors and image fetch errors,
  and Suno / Kling task errors could carry the channel's upstream address, proxy or key.
- Links to providers' public documentation sites (OpenAI, Anthropic, Google AI, AWS, Azure,
  OpenRouter, Mistral, DeepSeek, xAI, Groq) stay readable in error messages; channel addresses,
  proxies, IPs and keys are still masked.
- Removed a debug print that wrote every relay error body to standard output.
- The admin payment gateway endpoints no longer return gateway credentials (Epay key, Alipay
  private key, WeChat Pay APIv3 key and private key, Stripe secret key and webhook secret) in
  clear text. They are returned masked (first four and last four characters, shorter values show
  less), the edit form displays the mask next to an empty field, and leaving the field blank keeps
  the stored credential.

## [0.2.0] - 2026-10-18

### Added

- Channels have a remark field for admin notes, shown under the channel name. It is kept per
  channel and not shared across a tag.
- Channels can adjust which chat models are sent to the Responses API ("Chat-to-Responses
  models"): listed models are added to the built-in list, and a `-` prefix removes one.
- A multi-model test dialog tests several models of a channel at once, with an optional
  endpoint override. Image, video and deep research models are not selected by default.
- The multi-model test dialog can retry only the failed models and test a single row; a batch
  cannot start while single-row tests are running.
- The channel list API accepts `exact_model`, which matches a whole entry of a channel's model
  list. Batch model removal uses it, so searching `gemini-2.5-flash` no longer lists every
  `gemini-2.5-flash-*` channel and pushes the wanted ones past the first 100 results.
- Admin API responses can carry a stable `code` (snake_case) and optional `params` next to the
  unchanged English `message`; the 429 rate-limit body adds `params.retry_after` under `error`.
  The web console shows coded messages in the interface language (English, Simplified Chinese,
  Traditional Chinese, Japanese) and falls back to `message` for anything else. The first codes
  cover channel tests (the upstream reason stays as is), rate limiting and common channel and
  channel tag validation errors.
- The built-in default price table covers current models at their official list prices,
  including cache read and write prices and long-context tiers: OpenAI GPT-6 (Astra, 6.1 Sol, Sol,
  Luna), GPT-5.x and the Codex models, GPT-4.1, o-series; Anthropic Claude Fable 5 / 5.1, Opus,
  Sonnet and Haiku 4.x / 5.x; Gemini 4 Argon, 3.x and 2.5 (with audio input and image output
  prices); and DeepSeek Flash / V4 Pro. Model names that Antigravity maps to Claude or Gemini
  models use the target model's price. Codex, Claude Code, Gemini CLI, Antigravity, Bedrock and
  Vertex AI channels bill these models at the same prices, since prices are keyed by model name.
  New deployments no longer reject these models under the default `unpriced_model_policy=block`;
  existing deployments get the new entries on restart with `auto_price_updates` on, or through
  `Update Prices` with `/api/prices?type=default` as the price source.
- The model search box of the channel form suggests models as you type: the default models of
  the channel type and the price-table models of that type (Azure OpenAI channels use the OpenAI
  entries, Vertex AI Express the Gemini ones). Models already in the list are not suggested.
  Enter still adds the typed text, and "Edit as text" is unchanged.
- Codex channels can sign in with a device code, the default in the authorization dialog: the
  console shows a one-time code to enter at `auth.openai.com/codex/device` from any browser,
  so nothing has to reach `localhost:1455`. The first use needs device code login turned on in
  ChatGPT Settings → Security (a workspace admin turns it on for workspace accounts). New admin
  endpoints `POST /api/codex/oauth/device/start` and `POST /api/codex/oauth/device/poll`; the
  credentials are returned once, in the poll response, to the admin who started the login.

### Changed

- The provider dropdown of the channel form lists common providers first (OpenAI, Anthropic,
  Gemini, Azure OpenAI, Amazon Bedrock, Vertex AI, OpenRouter, DeepSeek, Codex, Claude Code and
  the custom channel), then a separator and every other provider sorted by name. Search covers
  both parts.
- Default models of new channels are the current ones: Anthropic, Bedrock and Claude Code list
  Claude 4.5 to 5.5 (retired Claude 1 to 3 models are gone); Gemini, Vertex AI and Vertex AI
  Express list Gemini 4 Argon and 3.x; DeepSeek, Moonshot (Kimi), Mistral, Zhipu, MiniMax and
  Cohere follow their current model lists; Codex, xAI and Azure Databricks get default models.
  Vertex AI defaults are Gemini only. Groq no longer prefills its retired models; use **Fetch
  models**. Test models are cheap models of each list.
- The **Fetch models** button is shown only for channel types whose backend can list models;
  xAI and Ollama channels get it.
- A manual channel test no longer disables the channel when it fails; the scheduled test of
  all channels still does. Only tests of the channel's test model update its response time.
- **The channel check is rewritten.** It now checks one model per request at
  `GET /api/channel/check/:id?model=` (admin only); the dialog runs two models at a time, warns
  that each model sends up to five real requests to the upstream, and asks before starting. The
  checks are a random arithmetic challenge instead of "hi", reading digits from a locally
  rendered image, a JSON schema answer, two parallel tool calls and a relay hint. Each result is
  passed, warning, failed or not applicable, and only a failed basic check fails a model. Checks
  that a request conversion makes meaningless (response_format on channels converted to Claude,
  the invalid-role probe on Claude and Gemini conversions) are marked not applicable instead of
  failed, and upstream errors are grouped into categories such as authentication, quota or rate
  limit. Results are returned as stable codes and translated in the web console. The SSE endpoint
  `POST /api/sse/channel/check` is removed; scripts calling it must switch to the new endpoint.
- Codex channels now report Codex CLI version 0.159.0 (was 0.144.1) in the `version` header and
  the default User-Agent, since the upstream rejects new models for clients below a minimum
  version. A channel can set its own version with the new **Client version** plugin setting
  (`codex.client_version`); a value that is not a version such as `0.159.0` or
  `0.158.0-alpha.2` is logged and the default is used.
- **Codex channels no longer add the Codex CLI system prompt to requests without
  instructions.** Chat requests and `/v1/responses` requests that bring no `instructions` now
  get the one-line "You are a helpful assistant." instead of the 2025 coding-agent prompt of
  about 24,000 characters; instructions sent by the client are passed through unchanged. On a
  real Codex channel this bills about 5,000 fewer input tokens per request (a one-word chat
  prompt went from 5,017 to 22 input tokens). The output style changes with it: answers no
  longer follow the coding-agent conventions of that prompt, and tool calls start without a
  preamble sentence. Clients that relied on that behavior should send their own instructions.
- The default price of `gpt-4o` is the current official price ($2.50 / $10 per 1M tokens, cached
  input $1.25) instead of the 2024-05-13 snapshot price. Built-in defaults never overwrite
  existing price entries; update `gpt-4o` from `Update Prices` if needed.
- The ClaudeCode and Codex paste dialogs explain what to copy for each provider (the Codex
  error page's address, the Claude authentication code), check the pasted text as it is typed
  and submit a valid paste by itself. Pasting a callback from another login attempt, the
  authorization link itself, a provider error or an already used code now gives a specific,
  translated message.

### Fixed

- Sign-in, sign-up, password reset, email code and account security errors (wrong password,
  validation, invite code, verification code, expired session, unlinking the last sign-in
  method and similar) were shown in English whatever the interface language. These responses
  now carry a `code` and the web console shows them in the interface language; `message` is
  unchanged. Email labels on the sign-up and forgot-password pages are translated as well.
- Organization errors (members, invitations, roles, ownership transfer, budgets, quota transfer,
  top-ups and organization API keys) and admin user management errors (role limits, root user
  protection, shadow accounts, quota changes) are shown in the interface language as well. An
  unknown or malformed user ID in the admin user API now answers "User not found" or "invalid
  user ID" instead of the database or parser text.
- System settings, invite code, redemption code, API key, model catalog, order, payment method,
  invoice and third-party sign-in (GitHub, LINUX DO, Lark, WeChat, OIDC) messages are shown in
  the interface language as well. Redeeming an invalid, used or expired code keeps its reason
  instead of a generic failure, and the invoice month and log context prompts in the console are
  translated.
- Password length errors said "at most 20 characters" while the limit is 64; length messages
  now take their limits from the validation rules.
- When Turnstile was enabled and the session could not be saved after a successful check, the
  request still went on to the handler after the error response.
- Chat requests on Codex channels returned no tool calls, streaming or not: function call
  events were dropped and only the text was forwarded. They now return `tool_calls` with
  `finish_reason: "tool_calls"`, and stream reasoning text as well.
- Streamed chat answers converted from the Responses API end with `finish_reason:
  "tool_calls"` instead of `"stop"` when the model called a function, as non-streamed answers
  already did.
- `/v1/responses` on Codex channels accepts a string `input`, as the public Responses API does;
  the upstream rejected it with 400 "Input must be a list".
- The ClaudeCode and Codex code exchange checks that a pasted callback belongs to the login
  session, and no longer ends the session when the paste is wrong or the code is rejected, so
  the admin can paste again or reopen the same authorization link within its 10 minutes. Only
  the admin who started a login can finish it.
- Moderation requests without a `model` now default to `omni-moderation-latest`, since the
  `text-moderation` models were retired upstream. `omni-moderation` models are priced at 0 by
  default.
- Chat requests routed to the Responses API (o1-pro, o3-pro, deep-research and
  `codex-mini-latest`) now honor the channel model mapping, always return chat completions
  (also after a retry on a passthrough channel), and stream chat chunks on Codex channels.
- Channel tests for those models use the Responses API, matching real traffic.
- Adding keys while editing a channel tag no longer silently drops the new channels.
- The channel check applies the channel's model mapping and `#` suffix, uses the Responses API
  for Responses-only models like real traffic does, and masks upstream URLs and keys in errors.
  A provider panic during a check no longer crashes the server, and a closed dialog no longer
  leaves check goroutines blocked.
- The image check no longer needs a server address reachable from the internet, and the public
  `GET /api/image/:id` endpoint it relied on is removed: anyone could add access records to a
  running check through it without signing in.
- Chat requests sent to Claude channels (Anthropic, Claude Code, Bedrock and Vertex AI) now treat
  `developer` messages as system instructions instead of sending them as assistant messages.
  Several system messages are joined with line breaks, and messages with an unknown role are sent
  as user messages.
- Codex channels with a custom `user_agent` in their settings no longer drop the `version` and
  `originator` headers; the custom value now replaces only the User-Agent.
- Channel tag endpoints answered missing tags and invalid status or setting types with an error
  shape the web console could not read, so no message was shown; they now answer
  `{success: false, message, code}` like the other admin endpoints.
- Malformed or unknown channel ids now answer "invalid channel id" / "channel not found"
  instead of raw parser and database text.
- Binding LINUX DO while signed in with an access token no longer crashes the request, and an
  account already linked to LINUX DO must be unlinked before it can be linked to another one.
- AWS Bedrock sent `claude-haiku-5-5` by its bare model id, which AWS rejects because the model
  is only available through inference profiles. It now uses the `us.` profile in US and Canadian
  regions, `eu.` in EU regions and `global.` elsewhere.

### Security

- The Gemini CLI and Antigravity OAuth callback pages, which need no sign-in, put the
  `error_description` query parameter into the page unescaped, so a crafted link could run
  script on the console's origin. All text on these pages is now escaped, the pages send a
  nonce-based Content-Security-Policy, and they no longer use inline event handlers.
- These callback pages sent the OAuth credentials with `postMessage` to any opener window.
  They now send them only to the console origin that started the authorization (taken from
  the start request), and fall back to polling when that origin is unknown; the console only
  accepts messages from the popup it opened.
- The admin user list and user detail endpoints returned each user's `access_token`, so an
  admin could read the root user's token and act as root. Only `/api/user/self` returns it now.

## [0.1.1] - 2026-10-10

### Changed

- The home page hero figure is redrawn as symmetric line art: lines from members, apps and API
  keys gather into the Modeltaps mark and fan out to model providers, with dots on both ring
  crossings. The documentation site home page uses the same figure, and the figure's
  screen-reader description is updated in every language.
- The Hong Kong Chinese environment variable guide now uses Hong Kong wording for "account".

## [0.1.0] - 2026-10-09

### Added

- **First Modeltaps release line**: the repository starts fresh from the code of its private
  predecessor, without the earlier history. Upstream attribution is in NOTICE. It ships with the
  organization and multi-tenant layer, the shadcn web console, the model catalog and the native
  API console already in place. Deploy it as a new installation; no upgrade path from earlier
  builds is provided.
- Deployment guides now cover setting the server address after install and tightening data
  directory permissions.
- Channels have a new switch for the unified request/response model, available in the channel
  settings of the web console.
- Rate-limited requests (HTTP 429) now include `Retry-After` and `X-RateLimit-*` headers and a
  JSON error body with the wait time and request id; the web console shows this message.
- Global rate limits now count per signed-in user as well as per source IP, so users behind a
  shared NAT no longer block each other. Trusted IP/CIDR sources can be exempted with
  `GLOBAL_RATE_LIMIT_WHITELIST`, and local loopback requests are not counted.
- Channel auto-disable logs and notifications now name the model that triggered the disable.
- AWS Bedrock supports `claude-haiku-5-5`, with a default price.
- New AWS Bedrock channels are prefilled with the current Claude models (Sonnet 4.5 to Fable
  5.1) instead of only the Claude 2 and 3 models.

### Fixed

- Static assets no longer count toward the web rate limit, so pages no longer go blank after a
  few reloads.
- Requests sent through a channel proxy now reuse connections per proxy address instead of
  opening new ones each time.
- Gemini image models are detected more reliably; streaming image output is billed at the image
  output ratio, and Imagen requests are billed only for images actually returned, with a content
  policy error when every image is blocked.

### Security

- Changing your own password now requires the current password.
- Epay payment callbacks are verified more strictly: unexpected parameters are rejected,
  signatures are compared in constant time, the trade status is checked only after signature
  verification, and concurrent callbacks for the same order are serialized.
