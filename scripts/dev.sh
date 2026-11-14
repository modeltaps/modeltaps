#!/bin/sh
# Development mode: backend and frontend together, both reloading on change. Run from the repo
# root with `make dev`; Ctrl-C stops both.
#
#   backend   air (hot reload) when installed, otherwise one `go build` + run; port DEV_BACKEND_PORT
#   frontend  Vite dev server on DEV_PORT, proxying API and relay paths to the backend
#
# Settings (environment, or a git-ignored .dev.env file in the repo root):
#   DEV_DATA_DIR      working directory of the backend: database, logs, optional env.sh (default .dev)
#   DEV_PORT          frontend port (default 3010)
#   DEV_BACKEND_PORT  backend port (default 3050)
set -e
REPO=$(cd "$(dirname "$0")/.." && pwd)
cd "$REPO"
[ -f .dev.env ] && . ./.dev.env

DEV_DATA_DIR=${DEV_DATA_DIR:-$REPO/.dev}
DEV_PORT=${DEV_PORT:-3010}
DEV_BACKEND_PORT=${DEV_BACKEND_PORT:-3050}
mkdir -p "$DEV_DATA_DIR"
DEV_DATA_DIR=$(cd "$DEV_DATA_DIR" && pwd)
export DEV_DATA_DIR DEV_BACKEND_PORT

# go:embed needs web/build; a placeholder is enough because Vite serves the UI.
[ -f web/build/index.html ] || { mkdir -p web/build && echo '<!doctype html>' > web/build/index.html; }
[ -d web/node_modules ] || (cd web && pnpm install --frozen-lockfile)

if command -v air >/dev/null 2>&1; then
  air -c .air.toml &
else
  echo "air not found: the backend will not reload on Go changes (go install github.com/air-verse/air@latest)"
  (go build -o tmp/modeltaps-dev . && exec scripts/dev-backend.sh) &
fi
BACKEND_PID=$!
trap 'kill $BACKEND_PID 2>/dev/null; wait $BACKEND_PID 2>/dev/null' EXIT INT TERM

echo "Modeltaps dev: http://localhost:$DEV_PORT (backend :$DEV_BACKEND_PORT, data $DEV_DATA_DIR)"
cd web
VITE_DEV_PORT=$DEV_PORT VITE_PROXY_TARGET=http://127.0.0.1:$DEV_BACKEND_PORT pnpm exec vite --no-open
