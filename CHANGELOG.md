# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

### Changed

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

### Fixed

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
