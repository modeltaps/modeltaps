// The tools the "Connect a tool" dialog knows how to configure (AGENT-8, AGENT-9). Each entry
// names the one way to connect it that asks the least of the user (`primary`), and builds every
// way it supports for one site address, key and model:
//
// - `deeplink`: a button that opens a local app's own import dialog (CC Switch, Cherry Studio);
// - `fields`: the tool has its own settings screen: numbered steps plus the values to paste;
// - `command`: one command to run in a terminal;
// - `file`: the config file written by hand, shown folded as the last resort.
//
// The dialog only renders these; adding a tool is one entry here plus its icon and strings
// (connectTool.tool.<id>.*). The snippets follow docs/guide/api.md.

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

// Whether an address can only be reached from this machine or network. Tools that call the
// model from their own servers (Cursor) cannot use it.
export function isPrivateAddress(base) {
  let host;
  try {
    host = new URL(base).hostname.toLowerCase();
  } catch {
    return false;
  }
  return (
    host === 'localhost' ||
    host.endsWith('.localhost') ||
    host.endsWith('.local') ||
    /^127\./.test(host) ||
    /^10\./.test(host) ||
    /^192\.168\./.test(host) ||
    /^172\.(1[6-9]|2\d|3[01])\./.test(host) ||
    host === '[::1]'
  );
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
const GEMINI_PREFER = [/gemini.*pro/i, /gemini/i];

const json = (value) => JSON.stringify(value, null, 2);

// One code block: an optional file path it goes into, a highlight language and the text.
const block = (code, { path, language = 'bash', note } = {}) => ({ code, path, language, note });

const exportKey = (key) => `export ${KEY_ENV}="${key}"`;

// A value to paste into a tool's settings screen; `label` names the tool's own field.
const field = (label, value, { secret = false } = {}) => ({ label, value, secret });

// CC Switch's provider import link (ccswitch://v1/import, documented in its user manual,
// "Deep link" FAQ). Values are URL-encoded; the app shows a confirmation before importing.
export function ccSwitchLink({ app, name, endpoint, key, model, apiFormat, homepage }) {
  const params = new URLSearchParams({ resource: 'provider', app, name, endpoint, apiKey: key });
  if (model) params.set('model', model);
  if (apiFormat) params.set('apiFormat', apiFormat);
  if (homepage) params.set('homepage', homepage);
  return `ccswitch://v1/import?${params.toString()}`;
}

function base64Utf8(text) {
  const bytes = new TextEncoder().encode(text);
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}

// Cherry Studio's provider import link: `data` is standard Base64 of a JSON object with id,
// baseUrl, apiKey, name and type, left unescaped (Cherry Studio swaps + and / itself before
// parsing the query; see its protocol handler providersImport.ts).
export function cherryStudioLink({ id, name, baseUrl, key }) {
  const data = base64Utf8(JSON.stringify({ id, baseUrl, apiKey: key, name, type: 'openai' }));
  return `cherrystudio://providers/api-keys?v=1&data=${data}`;
}

// Caching status shown under each tool. `verified`: confirmed on real traffic through the
// gateway; `unverified`: the tool sends cache markers by its own docs, not yet confirmed here;
// `depends`: depends on the protocol or model the user picks.
export const CACHE = { verified: 'verified', unverified: 'unverified', depends: 'depends' };

// The categories the directory groups tools under, in display order.
export const CATEGORIES = ['terminal', 'editor', 'desktop', 'chat', 'other'];

// Tools shown before the user searches or opens the full list.
export const POPULAR = ['claude-code', 'codex', 'cursor', 'cline', 'goose', 'cherry-studio', 'opencode', 'openclaw'];

const claudeCode = {
  id: 'claude-code',
  name: 'Claude Code',
  icon: 'claudecode',
  category: 'terminal',
  keywords: 'anthropic claude cli',
  site: 'https://docs.anthropic.com/en/docs/claude-code',
  docs: '/guide/api#claude-code',
  needsAnthropic: true,
  needsModel: false,
  primary: 'deeplink',
  cache: () => CACHE.verified,
  methods: ({ base, key, siteName }) => {
    const env = {
      ANTHROPIC_BASE_URL: `${base}/claude-code`,
      ANTHROPIC_AUTH_TOKEN: key,
      ANTHROPIC_API_KEY: '',
      CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1'
    };
    return [
      {
        id: 'deeplink',
        links: [
          { app: 'ccswitch', url: ccSwitchLink({ app: 'claude', name: siteName, endpoint: `${base}/claude-code`, key, homepage: base }) }
        ]
      },
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
      },
      { id: 'file', blocks: [block(json({ env }), { path: '~/.claude/settings.json', language: 'json', note: 'mergeJson' })] }
    ];
  },
  verify: 'claudeCode'
};

const codex = {
  id: 'codex',
  name: 'Codex',
  icon: 'codex',
  category: 'terminal',
  keywords: 'openai cli',
  site: 'https://github.com/openai/codex',
  docs: '/guide/api#codex',
  needsModel: true,
  prefer: OPENAI_PREFER,
  primary: 'deeplink',
  cache: () => CACHE.verified,
  methods: ({ base, key, model, providerId, siteName }) => [
    {
      id: 'deeplink',
      links: [
        { app: 'ccswitch', url: ccSwitchLink({ app: 'codex', name: siteName, endpoint: `${base}/codex/v1`, key, model, homepage: base }) }
      ]
    },
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
  category: 'terminal',
  site: 'https://opencode.ai',
  docs: '/guide/api#opencode',
  needsModel: true,
  primary: 'deeplink',
  cache: ({ model, anthropic }) => (anthropic && isClaudeModel(model) ? CACHE.unverified : CACHE.depends),
  // Claude models go through the Anthropic protocol so OpenCode can send cache_control;
  // everything else through the OpenAI-compatible one.
  methods: ({ base, key, model, providerId, anthropic, siteName }) => {
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
        id: 'deeplink',
        links: [
          {
            app: 'ccswitch',
            url: ccSwitchLink({
              app: 'opencode',
              name: siteName,
              endpoint: `${base}/coding-agent/v1`,
              key,
              model,
              apiFormat: viaAnthropic ? 'anthropic' : 'openai_chat',
              homepage: base
            })
          }
        ]
      },
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
  category: 'desktop',
  keywords: 'block',
  site: 'https://goose-docs.ai',
  docs: '/guide/api#goose',
  needsModel: true,
  primary: 'fields',
  cache: ({ model, anthropic }) => (anthropic && isClaudeModel(model) ? CACHE.verified : CACHE.depends),
  // Claude models use Goose's built-in Anthropic provider, which adds cache_control markers on
  // its own; other models a custom OpenAI-compatible provider with model discovery.
  methods: ({ base, key, model, providerId, anthropic }) => {
    if (anthropic && isClaudeModel(model)) {
      const settings = { GOOSE_PROVIDER: 'anthropic', GOOSE_MODEL: model, ANTHROPIC_HOST: `${base}/coding-agent` };
      return [
        {
          id: 'fields',
          steps: 'gooseAnthropic',
          fields: [
            field('ANTHROPIC_HOST', `${base}/coding-agent`),
            field('ANTHROPIC_API_KEY', key, { secret: true }),
            field('Model', model)
          ]
        },
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
        id: 'fields',
        steps: 'gooseCustom',
        fields: [field('API URL', `${base}/coding-agent/v1`), field('API Key', key, { secret: true }), field('Model', model)]
      },
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
  category: 'desktop',
  site: 'https://openclaw.ai',
  needsModel: true,
  primary: 'command',
  cache: () => CACHE.depends,
  // The hosted openclaw-setup.sh configures the provider (CLI or desktop app); the config file
  // is the same result written by hand.
  methods: ({ base, key, model, providerId, siteName }) => {
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
      {
        id: 'deeplink',
        links: [
          {
            app: 'ccswitch',
            url: ccSwitchLink({
              app: 'openclaw',
              name: siteName,
              endpoint: `${base}/v1`,
              key,
              model,
              apiFormat: 'openai_chat',
              homepage: base
            })
          }
        ]
      },
      { id: 'file', blocks: [block(json(config), { path: '~/.openclaw/openclaw.json', language: 'json', note: 'openclawConfig' })] }
    ];
  },
  verify: 'openclaw'
};

const hermes = {
  id: 'hermes',
  name: 'Hermes Agent',
  icon: 'nousresearch',
  category: 'terminal',
  keywords: 'nous research',
  site: 'https://hermes-agent.nousresearch.com',
  needsModel: true,
  primary: 'deeplink',
  cache: () => CACHE.depends,
  methods: ({ base, key, model, siteName }) => [
    {
      id: 'deeplink',
      links: [
        {
          app: 'ccswitch',
          url: ccSwitchLink({ app: 'hermes', name: siteName, endpoint: `${base}/v1`, key, model, apiFormat: 'openai_chat', homepage: base })
        }
      ]
    },
    {
      id: 'fields',
      steps: 'hermes',
      fields: [field('Base URL', `${base}/v1`), field('API Key', key, { secret: true }), field('Model', model)]
    }
  ],
  verify: 'other'
};

const geminiCli = {
  id: 'gemini-cli',
  name: 'Gemini CLI',
  icon: 'geminicli',
  category: 'terminal',
  keywords: 'google',
  site: 'https://github.com/google-gemini/gemini-cli',
  needsGemini: true,
  needsModel: true,
  prefer: GEMINI_PREFER,
  primary: 'deeplink',
  cache: () => CACHE.depends,
  methods: ({ base, key, model, siteName }) => [
    {
      id: 'deeplink',
      links: [
        { app: 'ccswitch', url: ccSwitchLink({ app: 'gemini', name: siteName, endpoint: `${base}/gemini`, key, model, homepage: base }) }
      ]
    },
    {
      id: 'command',
      blocks: [
        block(
          [`export GOOGLE_GEMINI_BASE_URL="${base}/gemini"`, `export GEMINI_API_KEY="${key}"`, `export GEMINI_MODEL="${model}"`].join('\n'),
          {
            note: 'shellSession'
          }
        )
      ]
    }
  ],
  verify: 'other'
};

// Editors and extensions with a provider screen of their own: steps plus three values.
const openAIFieldsTool = ({ id, name, icon, category, site, steps, keywords, note }) => ({
  id,
  name,
  icon,
  category,
  keywords,
  site,
  needsModel: true,
  primary: 'fields',
  note,
  cache: () => CACHE.depends,
  methods: ({ base, key, model }) => [
    { id: 'fields', steps, fields: [field('Base URL', `${base}/v1`), field('API Key', key, { secret: true }), field('Model ID', model)] }
  ],
  verify: 'other'
});

const cursor = openAIFieldsTool({
  id: 'cursor',
  name: 'Cursor',
  icon: 'cursor',
  category: 'editor',
  site: 'https://cursor.com',
  steps: 'cursor',
  note: 'cursorPublic'
});
const cline = openAIFieldsTool({
  id: 'cline',
  name: 'Cline',
  icon: 'cline',
  category: 'editor',
  keywords: 'vscode',
  site: 'https://cline.bot',
  steps: 'cline'
});
const rooCode = openAIFieldsTool({
  id: 'roo-code',
  name: 'Roo Code',
  icon: 'roocode',
  category: 'editor',
  keywords: 'vscode roo',
  site: 'https://roocode.com',
  steps: 'cline'
});
const kiloCode = openAIFieldsTool({
  id: 'kilo-code',
  name: 'Kilo Code',
  icon: 'kilocode',
  category: 'editor',
  keywords: 'vscode kilo',
  site: 'https://kilo.ai',
  steps: 'cline'
});
const openWebUI = openAIFieldsTool({
  id: 'open-webui',
  name: 'Open WebUI',
  icon: 'openwebui',
  category: 'chat',
  site: 'https://openwebui.com',
  steps: 'openWebUI'
});
const lobeChat = openAIFieldsTool({
  id: 'lobechat',
  name: 'LobeChat',
  icon: 'lobehub',
  category: 'chat',
  keywords: 'lobehub',
  site: 'https://lobehub.com',
  steps: 'lobeChat'
});

// Cherry Studio adds an OpenAI-type provider from its own import link; the API address is the
// site root, Cherry Studio appends /v1 itself.
const cherryStudio = {
  id: 'cherry-studio',
  name: 'Cherry Studio',
  icon: 'cherrystudio',
  category: 'chat',
  keywords: 'cherry',
  site: 'https://cherry-ai.com',
  needsModel: false,
  primary: 'deeplink',
  cache: () => CACHE.depends,
  methods: ({ base, key, providerId, siteName }) => [
    { id: 'deeplink', links: [{ app: 'cherry', url: cherryStudioLink({ id: providerId, name: siteName, baseUrl: base, key }) }] },
    { id: 'fields', steps: 'cherryStudio', fields: [field('API Address', base), field('API Key', key, { secret: true })] }
  ],
  verify: 'other'
};

// Any other tool: the two generic coding-agent endpoints and the key.
const other = {
  id: 'other',
  name: '',
  icon: '',
  category: 'other',
  docs: '/guide/api#other-coding-agents',
  needsModel: false,
  primary: 'fields',
  cache: () => CACHE.depends,
  methods: ({ base, key, anthropic }) => [
    {
      id: 'fields',
      steps: 'other',
      fields: [
        field('OpenAI', `${base}/coding-agent/v1`),
        ...(anthropic ? [field('Anthropic', `${base}/coding-agent`)] : []),
        field('API Key', key, { secret: true })
      ]
    }
  ],
  verify: 'other'
};

export const TOOLS = [
  claudeCode,
  codex,
  opencode,
  geminiCli,
  hermes,
  cursor,
  cline,
  rooCode,
  kiloCode,
  goose,
  openclaw,
  cherryStudio,
  openWebUI,
  lobeChat,
  other
];

// The tools this site can serve: Claude Code needs the Anthropic endpoint, Gemini CLI the
// Gemini one.
export function availableTools({ anthropic, gemini }) {
  return TOOLS.filter((tool) => (!tool.needsAnthropic || anthropic) && (!tool.needsGemini || gemini));
}

export const findTool = (id) => TOOLS.find((tool) => tool.id === id);

// Tools whose name, id or keywords contain every word of `query` (case-insensitive).
export function searchTools(tools, query) {
  const words = (query || '').toLowerCase().split(/\s+/).filter(Boolean);
  if (!words.length) return tools;
  return tools.filter((tool) => {
    const text = `${tool.name} ${tool.id} ${tool.keywords || ''}`.toLowerCase();
    return words.every((w) => text.includes(w));
  });
}

// Everything the dialog shows for one tool: the primary method first, then the others with the
// hand-written config file last, and the caching status.
export function buildToolGuide(tool, { base, key, model, anthropic, siteName }) {
  const ctx = {
    base,
    key: key || KEY_PLACEHOLDER,
    model: model || '',
    anthropic: Boolean(anthropic),
    providerId: providerIdFor(base),
    siteName: siteName || 'Modeltaps'
  };
  const all = tool.methods(ctx);
  const primary = all.find((m) => m.id === tool.primary) || all[0];
  const rest = all.filter((m) => m !== primary);
  const others = rest.filter((m) => m.id !== 'file');
  const manual = rest.find((m) => m.id === 'file') || null;
  return { primary, others, manual, cache: tool.cache(ctx) };
}
