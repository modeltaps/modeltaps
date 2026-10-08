#!/usr/bin/env bash
# Modeltaps one-click OpenClaw setup script (served statically, contains no secrets).
# Inputs are passed via environment variables so the API key never lands in argv:
#   MODELTAPS_BASE_URL     Base URL incl. /v1 (e.g. https://modeltaps.com/v1)
#   MODELTAPS_API_KEY      Token key incl. sk- prefix
#   MODELTAPS_MODEL        Bare model id (the <provider>/ prefix is applied internally)
#   MODELTAPS_PROVIDER_ID  Optional provider id (default: modeltaps; must match [a-z0-9-])
# Three-tier strategy: openclaw CLI onboard -> node merge openclaw.json -> manual guide.
set -euo pipefail

CONFIG_DIR="${HOME}/.openclaw"
CONFIG_FILE="${CONFIG_DIR}/openclaw.json"

usage() {
  cat >&2 <<'EOF'
Usage:
  MODELTAPS_BASE_URL="https://your-modeltaps-host/v1" \
  MODELTAPS_API_KEY="sk-xxxxxx" \
  MODELTAPS_MODEL="model-id" \
  bash -c "$(curl -fsSL https://your-modeltaps-host/openclaw-setup.sh)"

All three environment variables are required.
EOF
}

if [ -z "${MODELTAPS_BASE_URL:-}" ] || [ -z "${MODELTAPS_API_KEY:-}" ] || [ -z "${MODELTAPS_MODEL:-}" ]; then
  echo "Error: missing required environment variables." >&2
  usage
  exit 1
fi

# Provider id is optional; default to modeltaps and validate to keep JSON keys/refs safe.
PROVIDER_ID="${MODELTAPS_PROVIDER_ID:-modeltaps}"
if ! printf '%s' "${PROVIDER_ID}" | grep -Eq '^[a-z0-9-]+$'; then
  echo "Error: MODELTAPS_PROVIDER_ID must match [a-z0-9-]." >&2
  exit 1
fi

# Normalize model id: strip an accidental <provider>/ prefix so we control it consistently.
MODEL_ID="${MODELTAPS_MODEL#${PROVIDER_ID}/}"

finish_note() {
  echo ""
  echo "Done. Restart the OpenClaw desktop app (or its gateway) for changes to take effect."
  echo "Note: your shell history may now contain the API key; clear it if this is a shared machine."
}

# --- Tier 1: openclaw CLI ---------------------------------------------------
# Prefer PATH, then probe common global bin dirs (npm global / nvm / homebrew).
find_openclaw() {
  if command -v openclaw >/dev/null 2>&1; then
    command -v openclaw
    return 0
  fi
  local candidates=(
    "/usr/local/bin/openclaw"
    "/opt/homebrew/bin/openclaw"
    "${HOME}/.npm-global/bin/openclaw"
    "${HOME}/.local/bin/openclaw"
    "${HOME}/.nvm/versions/node"/*/bin/openclaw
  )
  local c
  for c in "${candidates[@]}"; do
    if [ -x "$c" ]; then
      echo "$c"
      return 0
    fi
  done
  return 1
}

if OPENCLAW_BIN="$(find_openclaw)"; then
  echo "Found OpenClaw CLI at: ${OPENCLAW_BIN}"
  echo "Configuring Modeltaps provider via 'openclaw onboard'..."
  # Key is passed through CUSTOM_API_KEY (env), never as a command-line argument.
  CUSTOM_API_KEY="${MODELTAPS_API_KEY}" "${OPENCLAW_BIN}" onboard --non-interactive \
    --auth-choice custom-api-key \
    --custom-base-url "${MODELTAPS_BASE_URL}" \
    --custom-model-id "${MODEL_ID}" \
    --custom-provider-id "${PROVIDER_ID}" \
    --custom-compatibility openai
  finish_note
  exit 0
fi

# --- Tier 2: node merge of ~/.openclaw/openclaw.json ------------------------
if command -v node >/dev/null 2>&1; then
  echo "OpenClaw CLI not found; merging ${CONFIG_FILE} with Node.js..."
  mkdir -p "${CONFIG_DIR}"
  MERGE_JS="$(mktemp -t modeltaps-openclaw-merge.XXXXXX.js)"
  trap 'rm -f "${MERGE_JS}"' EXIT
  cat > "${MERGE_JS}" <<'NODE_EOF'
'use strict';
const fs = require('fs');
const path = require('path');

const dir = path.join(process.env.HOME, '.openclaw');
const file = path.join(dir, 'openclaw.json');
const baseUrl = process.env.MODELTAPS_BASE_URL;
const apiKey = process.env.MODELTAPS_API_KEY;
const providerId = process.env.MODELTAPS_PROVIDER_ID || 'modeltaps';
const modelId = (process.env.MODELTAPS_MODEL || '').replace(new RegExp('^' + providerId + '/'), '');
const modelRef = providerId + '/' + modelId;

let config = {};
if (fs.existsSync(file)) {
  const raw = fs.readFileSync(file, 'utf8');
  try {
    config = raw.trim() ? JSON.parse(raw) : {};
  } catch (err) {
    console.error('Existing openclaw.json is not valid JSON:', err.message);
    process.exit(1);
  }
  const backup = file + '.modeltaps-bak-' + new Date().toISOString().replace(/[:.]/g, '-');
  fs.copyFileSync(file, backup);
  console.log('Backed up existing config to: ' + backup);
}

// Only add/overwrite Modeltaps-related keys; never delete other providers or settings.
config.models = config.models || {};
config.models.providers = config.models.providers || {};
config.models.providers[providerId] = {
  baseUrl: baseUrl,
  apiKey: apiKey,
  api: 'openai-completions',
  models: [{ id: modelId, name: modelId }]
};

config.agents = config.agents || {};
config.agents.defaults = config.agents.defaults || {};
config.agents.defaults.model = config.agents.defaults.model || {};
config.agents.defaults.model.primary = modelRef;

// Whitelist the model ref; keep an existing entry's value untouched (idempotent add-only).
config.agents.defaults.models = config.agents.defaults.models || {};
if (!Object.prototype.hasOwnProperty.call(config.agents.defaults.models, modelRef)) {
  config.agents.defaults.models[modelRef] = {};
}

// If an explicit allow-list is in effect (non-empty array), append the ref (deduped).
const modelPolicy = config.agents.defaults.modelPolicy;
if (modelPolicy && Array.isArray(modelPolicy.allow) && modelPolicy.allow.length > 0 && !modelPolicy.allow.includes(modelRef)) {
  modelPolicy.allow.push(modelRef);
}

fs.writeFileSync(file, JSON.stringify(config, null, 2) + '\n');
console.log('Wrote Modeltaps provider to: ' + file);
NODE_EOF
  MODELTAPS_BASE_URL="${MODELTAPS_BASE_URL}" \
  MODELTAPS_API_KEY="${MODELTAPS_API_KEY}" \
  MODELTAPS_MODEL="${MODEL_ID}" \
  MODELTAPS_PROVIDER_ID="${PROVIDER_ID}" \
  HOME="${HOME}" \
    node "${MERGE_JS}"
  finish_note
  exit 0
fi

# --- Tier 3: manual guide ---------------------------------------------------
cat >&2 <<EOF
Neither the OpenClaw CLI nor Node.js was found on this machine.
Please configure Modeltaps manually by editing:
  ${CONFIG_FILE}

Add (or merge) the following provider, then restart the OpenClaw app:
  models.providers.${PROVIDER_ID} = {
    "baseUrl": "${MODELTAPS_BASE_URL}",
    "apiKey": "<your sk- key>",
    "api": "openai-completions",
    "models": [{ "id": "${MODEL_ID}", "name": "${MODEL_ID}" }]
  }
  agents.defaults.model.primary = "${PROVIDER_ID}/${MODEL_ID}"
  agents.defaults.models["${PROVIDER_ID}/${MODEL_ID}"] = {}
EOF
exit 2
