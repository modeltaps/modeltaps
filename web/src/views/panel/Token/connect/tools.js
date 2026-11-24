// The tools the "Connect a tool" dialog knows how to configure (AGENT-8). Each entry says which
// endpoint the tool talks to, whether it needs a default model, how far prompt caching is
// confirmed, and builds the snippets for one site address, key and model. The dialog only
// renders these; adding a tool is one entry here plus its icon and strings. The snippets follow
// docs/guide/api.md, which shows the same configuration with placeholder addresses.

// Placeholder used while the real key is unknown (keys are stored as hashes only, SEC-47).
export const KEY_PLACEHOLDER = 'sk-xxxxxx';

// The environment variable the snippets keep the key in, where the tool reads one.
export const KEY_ENV = 'MODELTAPS_API_KEY';

// Provider id written into tool configs: the public site uses `modeltaps`, any other host
// (local, intranet, staging) `modeltaps-local`, so both can sit side by side in one config.
export function providerIdFor(base) {
  let host;
  try {
    host = new URL(base).hostname.toLowerCase();
  } catch {
    host = '';
  }
  return host === 'modeltaps.com' || host === 'www.modeltaps.com' ? 'modeltaps' : 'modeltaps-local';
}

export const isClaudeModel = (id) => /claude/i.test(id || '');

// Picks the default model from a key's models: the first match of `prefer` in order, else the
// first model.
export function pickModel(ids, prefer = DEFAULT_PREFER) {
  for (const re of prefer) {
    const hit = ids.find((id) => re.test(id));
    if (hit) return hit;
  }
  return ids[0] || '';
}

const DEFAULT_PREFER = [/claude-opus/i, /claude-sonnet/i, /gpt-5/i, /gemini.*pro/i];
const OPENAI_PREFER = [/gpt-5/i, /codex/i, /gpt/i];

const json = (value) => JSON.stringify(value, null, 2);

// One code block: an optional file path it goes into, a highlight language and the text.
const block = (code, { path, language = 'bash', note } = {}) => ({ code, path, language, note });

const exportKey = (key) => `export ${KEY_ENV}="${key}"`;

// Caching status shown under each tool. `verified`: confirmed on real traffic through the
// gateway; `unverified`: the tool sends cache markers by its own docs, not yet confirmed here;
// `depends`: depends on the protocol or model the user picks.
export const CACHE = { verified: 'verified', unverified: 'unverified', depends: 'depends' };

const claudeCode = {
  id: 'claude-code',
  name: 'Claude Code',
  icon: 'claudecode',
  kind: 'terminal',
  site: 'https://docs.anthropic.com/en/docs/claude-code',
  docs: '/guide/api#claude-code',
  needsAnthropic: true,
  needsModel: false,
  cache: () => CACHE.verified,
  methods: ({ base, key }) => {
    const env = {
      ANTHROPIC_BASE_URL: `${base}/claude-code`,
      ANTHROPIC_AUTH_TOKEN: key,
      ANTHROPIC_API_KEY: '',
      CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1'
    };
    return [
      { id: 'file', blocks: [block(json({ env }), { path: '~/.claude/settings.json', language: 'json', note: 'mergeJson' })] },
      {
        id: 'command',
        blocks: [
          block(
            Object.entries(env)
              .map(([k, v]) => `export ${k}="${v}"`)
              .join('\n'),
            { note: 'shellSession' }
          )
        ]
      }
    ];
  },
  verify: 'claudeCode'
};

const codex = {
  id: 'codex',
  name: 'Codex',
  icon: 'codex',
  kind: 'terminal',
  site: 'https://github.com/openai/codex',
  docs: '/guide/api#codex',
  needsModel: true,
  prefer: OPENAI_PREFER,
  cache: () => CACHE.verified,
  methods: ({ base, key, model, providerId }) => [
    {
      id: 'file',
      blocks: [
        block(
          [
            `model_provider = "${providerId}"`,
            `model = "${model}"`,
            '',
            `[model_providers.${providerId}]`,
            'name = "Modeltaps"',
            `base_url = "${base}/codex/v1"`,
            `env_key = "${KEY_ENV}"`,
            'wire_api = "responses"',
            `model_catalog_url = "${base}/codex/v1/models"`
          ].join('\n'),
          { path: '~/.codex/config.toml', language: 'toml', note: 'mergeToml' }
        ),
        block(exportKey(key), { note: 'keyEnv' })
      ]
    }
  ],
  verify: 'codex'
};

const opencode = {
  id: 'opencode',
  name: 'OpenCode',
  icon: 'opencode',
  kind: 'terminal',
  site: 'https://opencode.ai',
  docs: '/guide/api#opencode',
  needsModel: true,
  cache: ({ model, anthropic }) => (anthropic && isClaudeModel(model) ? CACHE.unverified : CACHE.depends),
  // Claude models go through the Anthropic protocol so OpenCode can send cache_control;
  // everything else through the OpenAI-compatible one.
  methods: ({ base, key, model, providerId, anthropic }) => {
    const viaAnthropic = anthropic && isClaudeModel(model);
    const config = {
      $schema: 'https://opencode.ai/config.json',
      provider: {
        [providerId]: {
          npm: viaAnthropic ? '@ai-sdk/anthropic' : '@ai-sdk/openai-compatible',
          name: 'Modeltaps',
          options: { baseURL: `${base}/coding-agent/v1`, apiKey: `{env:${KEY_ENV}}` },
          models: { [model]: {} }
        }
      },
      model: `${providerId}/${model}`
    };
    return [
      {
        id: 'file',
        blocks: [
          block(json(config), { path: '~/.config/opencode/opencode.json', language: 'json', note: 'mergeJson' }),
          block(exportKey(key), { note: 'keyEnv' })
        ]
      }
    ];
  },
  verify: 'opencode'
};

const goose = {
  id: 'goose',
  name: 'Goose',
  icon: 'goose',
  kind: 'desktop',
  site: 'https://goose-docs.ai',
  docs: '/guide/api#goose',
  needsModel: true,
  cache: ({ model, anthropic }) => (anthropic && isClaudeModel(model) ? CACHE.unverified : CACHE.depends),
  // Claude models use Goose's built-in Anthropic provider, which adds cache_control markers on
  // its own; other models a custom OpenAI-compatible provider with model discovery.
  methods: ({ base, key, model, providerId, anthropic }) => {
    if (anthropic && isClaudeModel(model)) {
      const settings = { GOOSE_PROVIDER: 'anthropic', GOOSE_MODEL: model, ANTHROPIC_HOST: `${base}/coding-agent` };
      return [
        {
          id: 'command',
          blocks: [
            block([...Object.entries(settings).map(([k, v]) => `export ${k}="${v}"`), `export ANTHROPIC_API_KEY="${key}"`].join('\n'), {
              note: 'shellSession'
            })
          ]
        },
        {
          id: 'file',
          blocks: [
            block(
              Object.entries(settings)
                .map(([k, v]) => `${k}: ${v}`)
                .join('\n'),
              { path: '~/.config/goose/config.yaml', language: 'yaml', note: 'gooseConfig' }
            ),
            block('goose configure', { note: 'gooseKey' })
          ]
        }
      ];
    }
    const provider = {
      name: providerId,
      engine: 'openai',
      display_name: 'Modeltaps',
      api_key_env: KEY_ENV,
      base_url: `${base}/coding-agent/v1/chat/completions`,
      dynamic_models: true,
      models: [{ name: model, context_limit: 128000 }]
    };
    return [
      {
        id: 'file',
        blocks: [
          block(json(provider), { path: `~/.config/goose/custom_providers/${providerId}.json`, language: 'json' }),
          block(`GOOSE_PROVIDER: ${providerId}\nGOOSE_MODEL: ${model}`, {
            path: '~/.config/goose/config.yaml',
            language: 'yaml',
            note: 'gooseConfig'
          }),
          block(exportKey(key), { note: 'keyEnv' })
        ]
      }
    ];
  },
  verify: 'goose'
};

const openclaw = {
  id: 'openclaw',
  name: 'OpenClaw',
  icon: 'openclaw',
  kind: 'desktop',
  site: 'https://openclaw.ai',
  needsModel: true,
  cache: () => CACHE.depends,
  // The hosted openclaw-setup.sh configures the provider (CLI or desktop app); the config file
  // tab is the same result written by hand.
  methods: ({ base, key, model, providerId }) => {
    const command = `MODELTAPS_BASE_URL="${base}/v1" \\
MODELTAPS_API_KEY="${key}" \\
MODELTAPS_MODEL="${model}" \\
MODELTAPS_PROVIDER_ID="${providerId}" \\
bash -c "$(curl -fsSL ${base}/openclaw-setup.sh)"`;
    const config = {
      models: {
        providers: {
          [providerId]: { baseUrl: `${base}/v1`, apiKey: key, api: 'openai-completions', models: [{ id: model, name: model }] }
        }
      },
      agents: { defaults: { model: { primary: `${providerId}/${model}` }, models: { [`${providerId}/${model}`]: {} } } }
    };
    return [
      { id: 'command', blocks: [block(command, { note: 'openclawCommand' })] },
      { id: 'file', blocks: [block(json(config), { path: '~/.openclaw/openclaw.json', language: 'json', note: 'openclawConfig' })] }
    ];
  },
  verify: 'openclaw'
};

// Any other tool: the two generic coding-agent endpoints and the key.
const other = {
  id: 'other',
  name: '',
  icon: '',
  kind: 'other',
  docs: '/guide/api#other-coding-agents',
  needsModel: false,
  cache: () => CACHE.depends,
  methods: ({ base, key, anthropic }) => [
    {
      id: 'values',
      blocks: [
        block(`${base}/coding-agent/v1`, { language: 'text', note: 'openaiBase' }),
        ...(anthropic ? [block(`${base}/coding-agent`, { language: 'text', note: 'anthropicBase' })] : []),
        block(key, { language: 'text', note: 'apiKey' })
      ]
    }
  ],
  verify: 'other'
};

export const TOOLS = [claudeCode, codex, opencode, goose, openclaw, other];

// The tools this site can serve: Claude Code needs the Anthropic endpoint.
export function availableTools({ anthropic }) {
  return TOOLS.filter((tool) => !tool.needsAnthropic || anthropic);
}

export const findTool = (id) => TOOLS.find((tool) => tool.id === id);

// Everything the dialog shows for one tool: the configuration methods (tabs) with their code
// blocks, and the caching status.
export function buildToolGuide(tool, { base, key, model, anthropic }) {
  const ctx = { base, key: key || KEY_PLACEHOLDER, model: model || '', anthropic: Boolean(anthropic), providerId: providerIdFor(base) };
  return { methods: tool.methods(ctx), cache: tool.cache(ctx) };
}
