# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
