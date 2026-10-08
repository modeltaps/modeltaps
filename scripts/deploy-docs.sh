#!/usr/bin/env bash
# Build the VitePress docs site and upload it to Cloudflare Pages (Direct Upload).
# Credentials come from .env.deploy in the repo root (gitignored, never echoed).
# Usage: scripts/deploy-docs.sh [--skip-build]
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/.env.deploy"
SKIP_BUILD=0

for arg in "$@"; do
  case "$arg" in
    --skip-build) SKIP_BUILD=1 ;;
    *) echo "Unknown option: $arg" >&2; exit 2 ;;
  esac
done

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing .env.deploy. Copy .env.deploy.example and fill in the values." >&2
  exit 1
fi

# Never print the file contents: source it with tracing off.
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

missing=()
[[ -n "${CLOUDFLARE_API_TOKEN:-}" ]] || missing+=("CLOUDFLARE_API_TOKEN")
[[ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ]] || missing+=("CLOUDFLARE_ACCOUNT_ID")
if (( ${#missing[@]} > 0 )); then
  echo "Missing required variables in .env.deploy: ${missing[*]}" >&2
  exit 1
fi

CF_PAGES_PROJECT="${CF_PAGES_PROJECT:-modeltaps-docs}"
CF_PAGES_BRANCH="${CF_PAGES_BRANCH:-main}"
DOCS_DOMAIN="${DOCS_DOMAIN:-docs.modeltaps.com}"

cd "$REPO_ROOT/docs"

# Both the build and the deploy step run through pnpm (version pinned by packageManager).
if (( SKIP_BUILD == 0 )); then
  echo "==> Building docs (pnpm)"
  pnpm install --frozen-lockfile --filter docs
  pnpm docs:build
else
  echo "==> Skipping build (--skip-build)"
fi

if [[ ! -d .vitepress/dist ]]; then
  echo "Build output docs/.vitepress/dist not found." >&2
  exit 1
fi

echo "==> Deploying to Cloudflare Pages project '$CF_PAGES_PROJECT' (branch '$CF_PAGES_BRANCH')"
# wrangler is a pinned docs devDependency (lockfile), not a floating download.
# Token/account id are passed via the environment, never as CLI arguments.
pnpm exec wrangler pages deploy .vitepress/dist \
  --project-name "$CF_PAGES_PROJECT" \
  --branch "$CF_PAGES_BRANCH"

echo "==> Done. Custom domain: https://$DOCS_DOMAIN"
