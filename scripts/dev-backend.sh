#!/bin/sh
# Runs the dev backend binary built by scripts/dev.sh (or air) inside DEV_DATA_DIR. An env.sh in
# that directory is sourced first; the port always comes from DEV_BACKEND_PORT.
REPO=$(cd "$(dirname "$0")/.." && pwd)
cd "$DEV_DATA_DIR" || exit 1
[ -f env.sh ] && . ./env.sh
PORT=$DEV_BACKEND_PORT exec "$REPO/tmp/modeltaps-dev"
