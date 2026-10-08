# Contributing

## Requirements

- Go (the version in `go.mod`; the `toolchain` line is fetched automatically)
- Node.js 24 and [pnpm](https://pnpm.io) (the version in the root `package.json` `packageManager`
  field); `web/`, `docs/` and `ittest/authgear/` form one pnpm workspace with a single lockfile
- Docker, only for container builds

Run `pnpm install` once in the repository root to install the dependencies and the commitlint git
hooks.

## Build

```bash
make all          # builds web/build, then the binary in dist/
./dist/modeltaps -h
```

`main.go` embeds `web/build`, so the frontend has to be built before `go build` or `go vet`.
The Taskfile offers the same steps (`task build`, `task run`, `task lint`).

```text
Usage of ./dist/modeltaps:
  -config string    specify the config.yaml path (default "config.yaml")
  -export           exports prices to a JSON file
  -help             print help and exit
  -log-dir string   specify the log directory
  -port int         the listening port
  -version          print version and exit
```

See the [documentation site](docs/guide/quickstart.md) for configuration.

## Development servers

The backend listens on `port` from `config.yaml` (default `3000`, or `--port`):

```bash
go run .
cd web && pnpm dev
```

Override the frontend port and the API proxy target in `web/.env.local` (git-ignored):

```bash
VITE_DEV_PORT=5678                        # default 3010
VITE_PROXY_TARGET=http://127.0.0.1:5679   # default http://127.0.0.1:3000
VITE_ALLOWED_HOSTS=devbox                 # extra host names Vite should accept
```

Vite runs with `strictPort`: it exits when the port is taken instead of picking another one.

## Checks before pushing

These are the steps `.github/workflows/ci.yml` runs:

```bash
go vet ./... && go build ./... && go test -race ./...
pnpm install --frozen-lockfile
(cd web && pnpm lint && pnpm test && pnpm build)
(cd docs && pnpm docs:lint:hk && pnpm docs:build)
```

## Docker

```bash
docker build -t modeltaps:dev .
```

To run it with `docker-compose.yml`, replace `image: ghcr.io/modeltaps/modeltaps:latest` with
`image: modeltaps:dev`, or with a build section:

```yaml
build:
  dockerfile: Dockerfile
  context: .
```

and run `docker compose build`.

## Commits and pull requests

Commit messages are English only (no CJK or full-width characters in the header, body or
footer) and follow [Conventional Commits](https://www.conventionalcommits.org):

```text
<type>(<scope>): <summary>

<body: what changed and why, wrapped at 100 columns>

Refs: SEC-23
```

- **type**: one of `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `build`, `perf`,
  `style`, `revert`, `deps`.
- **scope**: optional and lowercase, for example `web`, `relay`, `model`, `oidc`, `org`, `docs`,
  `i18n`, `release`, `ci`, `deps`.
- **summary**: imperative mood, lowercase first letter, no trailing period; the whole header is
  at most 72 characters.
- **No task or issue codes in the header** (such as `SEC-23`, `Wave 4`, `Task C`, `P0-1`); put
  them in a `Refs:` footer instead.
- **body**: required for non-trivial changes; explain what changed and why, not how. Body lines
  are at most 100 characters.
- **No tool-generated trailers**: `Co-authored-by:` lines and "Generated with" notices are
  rejected.
- Release commits: `chore(release): bump version to 0.7.0` (CHANGELOG + VERSION).
- Commit with `TZ=UTC` so commit timestamps are recorded in UTC.

Examples:

- `feat(auth): allow root password login while password login is off`
- `fix(auth): refuse to unbind the last remaining login method` with a `Refs: SEC-23` footer

The rules live in `commitlint.config.mjs`. The `commit-msg` hook checks each `git commit`, the
`pre-push` hook checks every commit being pushed, and the `commitlint` workflow checks pull
requests and pushes to `main`. When the pre-push hook rejects a push, reword the offending
commits with `git rebase -i <base>` and push again.

PR titles and bodies are English as well; the PR title uses the commit header format above.
Changelog entries in `CHANGELOG.md` are English, and the PR body can reuse the
`## [Unreleased]` changelog text.
