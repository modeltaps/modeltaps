#!/usr/bin/env bash
# Per-clone setup for Modeltaps developers.
# Registers the `ours` merge driver used by .gitattributes to keep
# Modeltaps-specific overrides (web/package.json, web/index.html, web/README.md)
# intact across upstream syncs.
set -euo pipefail

git config merge.ours.driver true
echo "Configured git merge.ours.driver=true"
