# Modeltaps

> One endpoint. Every model. Built to flow.

The unified gateway for every AI model — multi-provider routing, usage analytics, channel control, and battle-tested reliability behind a single OpenAI-compatible API.

## Features

- **Multi-provider routing** — OpenAI, Anthropic, Google, and 40+ providers reachable through one OpenAI-compatible endpoint.
- **Usage analytics** — per-user quotas, channel weights, and token-level accounting for every request.
- **Reliability** — automatic failover, rate-limiting, retries, and model aliasing keep traffic flowing under load.
- **Authentication** — OAuth 2.0 / OIDC sign-in (Google, GitHub, and more).
- **Storage** — runs on SQLite, MySQL, or PostgreSQL.

## Quick start

The fastest way to run Modeltaps is with Docker Compose. The example below starts the gateway with SQLite (no external database required):

```yaml
services:
  modeltaps:
    image: ghcr.io/modeltaps/modeltaps:latest
    container_name: modeltaps
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - ./data:/data
    environment:
      - SESSION_SECRET=change_me_to_a_random_string
      - USER_TOKEN_SECRET=change_me_to_a_32_char_random_string
      - TZ=UTC
```

> The image is published to a private registry (`ghcr.io/modeltaps/modeltaps`). Log in to GHCR before pulling — see the [deployment guide](docs/ops/en/deployment/index.md#signing-in-to-the-private-registry).

Bring it up and open <[http://localhost:3000](http://localhost:3000)>:

```bash
docker compose up -d
```

Minimal environment variables:

| Variable | Required | Description |
| --- | --- | --- |
| USER_TOKEN_SECRET | yes | Random string (32+ chars) used to sign user tokens. The gateway will not start without it. |
| SESSION_SECRET | recommended | Random string used to sign sessions. If omitted, a random secret is generated at startup, so logged-in users are signed out whenever the gateway restarts. |
| SQL_DSN | no | MySQL/PostgreSQL DSN. Omit to use the bundled SQLite. |
| REDIS_CONN_STRING | no | Redis connection string for caching and rate-limiting. |

To use MySQL or PostgreSQL instead of SQLite, set `SQL_DSN`. A full multi-service example (gateway + MySQL + Redis) is provided in `docker-compose.yml`.

> Fresh installs default to the database name `modeltaps` (MySQL/PostgreSQL via `SQL_DSN` and `MYSQL_DATABASE` in `docker-compose.yml`) and the SQLite file `modeltaps.db`.

## Image

The container image is published to GitHub Container Registry at `ghcr.io/modeltaps/modeltaps`. Tags follow a simple scheme:

- `dev` — rebuilt on every push to `main`.
- `latest` — the most recent tagged release.
- version numbers (e.g. `v1.2.3`) — one tag per release.

Images are built for both `linux/amd64` and `linux/arm64`, and are built and pushed automatically by GitHub Actions (`.github/workflows/docker-image.yml`).

To build the image yourself, run `docker build .` from the repository root using the top-level `Dockerfile`.

## Development

Requirements:

- Go ≥ 1.26
- Node ≥ 22.22 with pnpm 10.x

This repository uses [Task](https://taskfile.dev) for common workflows:

```bash
task build   # build the frontend and compile the Go binary
task run     # build, then run the gateway locally
task lint    # format and lint the Go sources
```

You can also work on components individually — the Go backend lives at the repository root and the frontend lives in `web/` (managed with pnpm; the lockfile is at the repository root).

After cloning, run the setup script once to register the local merge driver declared in `.gitattributes`:

```bash
./scripts/setup-dev.sh
```

## License

GNU Affero General Public License v3.0 only (AGPL-3.0-only) — see `LICENSE`.

**Additional terms under GNU AGPL version 3 section 7:** modified versions of this software that provide a user interface must retain the attribution notice "Powered by Modeltaps" and a visible hyperlink to the original project repository in the interface's about, legal, or footer area.

**Commercial licensing.** If you cannot accept the obligations of the AGPLv3 (for example, the requirement to release the source of a modified version offered over a network), a separate commercial license is available. To enquire, open an issue via GitHub Issues on this repository.

Third-party copyright and license attribution is recorded in `NOTICE`.