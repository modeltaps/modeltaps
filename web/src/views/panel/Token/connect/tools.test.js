import { describe, expect, it } from 'vitest';

import {
  KEY_ENV,
  KEY_PLACEHOLDER,
  POPULAR,
  TOOLS,
  availableTools,
  buildToolGuide,
  ccSwitchLink,
  cherryStudioLink,
  findTool,
  isPrivateAddress,
  pickModel,
  providerIdFor,
  searchTools
} from './tools';

const base = 'https://gw.example.com';
const key = 'sk-test123';
const guide = (id, over = {}) => buildToolGuide(findTool(id), { base, key, model: 'claude-sonnet-5-5', anthropic: true, ...over });
const method = (g, id) => [g.primary, ...g.others, g.manual].find((m) => m && m.id === id);
const params = (url) => new URLSearchParams(url.slice(url.indexOf('?') + 1));

describe('tool directory', () => {
  it('every tool has a known primary way, valid JSON blocks and a category', () => {
    for (const tool of TOOLS) {
      expect(['terminal', 'editor', 'desktop', 'chat', 'other'], tool.id).toContain(tool.category);
      for (const model of ['claude-sonnet-5-5', 'gpt-5.5']) {
        for (const anthropic of [true, false]) {
          const g = buildToolGuide(tool, { base, key, model, anthropic });
          expect(g.primary.id, tool.id).toBe(tool.primary);
          for (const m of [g.primary, ...g.others, g.manual].filter(Boolean)) {
            for (const b of m.blocks || []) if (b.language === 'json') expect(() => JSON.parse(b.code), tool.id).not.toThrow();
          }
          // The config file always comes last, folded, never among the other ways.
          expect(
            g.others.map((m) => m.id),
            tool.id
          ).not.toContain('file');
        }
      }
    }
  });

  it('popular tools all exist', () => {
    for (const id of POPULAR) expect(findTool(id), id).toBeTruthy();
  });

  it('hides tools whose endpoint is off', () => {
    const ids = (o) => availableTools(o).map((t) => t.id);
    expect(ids({ anthropic: false, gemini: false })).not.toContain('claude-code');
    expect(ids({ anthropic: false, gemini: false })).not.toContain('gemini-cli');
    expect(ids({ anthropic: true, gemini: true })).toEqual(expect.arrayContaining(['claude-code', 'gemini-cli']));
  });

  it('searches names, ids and keywords', () => {
    expect(searchTools(TOOLS, 'vscode').map((t) => t.id)).toEqual(expect.arrayContaining(['cline', 'roo-code']));
    expect(searchTools(TOOLS, 'CHERRY').map((t) => t.id)).toEqual(['cherry-studio']);
    expect(searchTools(TOOLS, 'nothing-like-this')).toEqual([]);
    expect(searchTools(TOOLS, '  ')).toBe(TOOLS);
  });
});

describe('import links', () => {
  it('CC Switch links name the app, endpoint, key and model', () => {
    const p = params(method(guide('codex', { model: 'gpt-5.5' }), 'deeplink').links[0].url);
    expect(Object.fromEntries(p)).toMatchObject({
      resource: 'provider',
      app: 'codex',
      endpoint: `${base}/codex/v1`,
      apiKey: key,
      model: 'gpt-5.5'
    });
    const claude = params(method(guide('claude-code'), 'deeplink').links[0].url);
    expect(claude.get('app')).toBe('claude');
    expect(claude.get('endpoint')).toBe(`${base}/claude-code`);
    expect(ccSwitchLink({ app: 'claude', name: 'A B', endpoint: base, key })).toMatch(/^ccswitch:\/\/v1\/import\?/);
  });

  it('CC Switch links carry the config the plain fields cannot (tested with CC Switch 4.0.40)', () => {
    const decode = (url) => JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(params(url).get('config')), (c) => c.charCodeAt(0))));
    const claude = decode(method(guide('claude-code'), 'deeplink').links[0].url);
    expect(claude).toEqual({
      env: { ANTHROPIC_BASE_URL: `${base}/claude-code`, ANTHROPIC_AUTH_TOKEN: key, CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1' }
    });
    const codex = decode(method(guide('codex', { model: 'gpt-5.5' }), 'deeplink').links[0].url);
    expect(codex.auth).toEqual({ OPENAI_API_KEY: key });
    expect(codex.config).toContain(`model_catalog_url = "${base}/codex/v1/models"`);
    expect(codex.config).toContain('wire_api = "responses"');
    expect(codex.config).toContain('model = "gpt-5.5"');
  });

  it('OpenCode imports Claude models over the Anthropic format', () => {
    expect(params(method(guide('opencode'), 'deeplink').links[0].url).get('apiFormat')).toBe('anthropic');
    expect(params(method(guide('opencode', { model: 'gpt-5.5' }), 'deeplink').links[0].url).get('apiFormat')).toBe('openai_chat');
  });

  it('Cherry Studio data is standard Base64 JSON, unescaped', () => {
    const url = cherryStudioLink({ id: 'modeltaps', name: 'Modeltaps 网关', baseUrl: base, key });
    const data = url.split('data=')[1];
    const decoded = JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(data), (c) => c.charCodeAt(0))));
    expect(decoded).toEqual({ id: 'modeltaps', baseUrl: base, apiKey: key, name: 'Modeltaps 网关', type: 'openai' });
    expect(url).toMatch(/^cherrystudio:\/\/providers\/api-keys\?v=1&data=/);
  });
});

describe('tool guides', () => {
  it('Claude Code keeps the settings.json env, folded as the manual way', () => {
    const env = JSON.parse(guide('claude-code').manual.blocks[0].code).env;
    expect(env).toEqual({
      ANTHROPIC_BASE_URL: `${base}/claude-code`,
      ANTHROPIC_AUTH_TOKEN: key,
      ANTHROPIC_API_KEY: '',
      CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1'
    });
  });

  it('Codex config uses the responses wire API and the model catalog', () => {
    const [toml, env] = guide('codex', { model: 'gpt-5.5' }).manual.blocks;
    expect(toml.code).toContain(`base_url = "${base}/codex/v1"`);
    expect(toml.code).toContain('wire_api = "responses"');
    expect(toml.code).toContain(`model_catalog_url = "${base}/codex/v1/models"`);
    expect(env.code).toBe(`export ${KEY_ENV}="${key}"`);
  });

  it('Goose fills the Anthropic host for Claude models and the OpenAI URL for others', () => {
    const claude = guide('goose');
    expect(claude.primary.fields.map((f) => [f.label, f.value])).toEqual([
      ['ANTHROPIC_HOST', `${base}/coding-agent`],
      ['ANTHROPIC_API_KEY', key],
      ['Model', 'claude-sonnet-5-5']
    ]);
    expect(method(claude, 'command').blocks[0].code).toContain('export GOOSE_PROVIDER="anthropic"');
    expect(claude.cache).toBe('verified');
    const gpt = guide('goose', { model: 'gpt-5.5' });
    expect(gpt.primary.fields[0].value).toBe(`${base}/coding-agent/v1`);
    expect(JSON.parse(gpt.manual.blocks[0].code)).toMatchObject({ engine: 'openai', api_key_env: KEY_ENV, dynamic_models: true });
    expect(gpt.cache).toBe('depends');
  });

  it('editor extensions get the /v1 base URL, the key and the model', () => {
    for (const id of ['cursor', 'cline', 'roo-code', 'kilo-code', 'open-webui', 'lobechat']) {
      const g = guide(id, { model: 'gpt-5.5' });
      expect(g.primary.id, id).toBe('fields');
      expect(
        g.primary.fields.map((f) => f.value),
        id
      ).toEqual([`${base}/v1`, key, 'gpt-5.5']);
      expect(g.primary.fields[1].secret, id).toBe(true);
    }
  });

  it('OpenClaw keeps the hosted setup script as the primary way', () => {
    const g = guide('openclaw', { base: 'https://modeltaps.com', model: 'claude-opus-5-5' });
    expect(g.primary.blocks[0].code).toContain('bash -c "$(curl -fsSL https://modeltaps.com/openclaw-setup.sh)"');
    expect(JSON.parse(g.manual.blocks[0].code).models.providers.modeltaps.baseUrl).toBe('https://modeltaps.com/v1');
  });

  it('other tools get both generic endpoints, the Anthropic one only when enabled', () => {
    expect(guide('other').primary.fields.map((f) => f.value)).toEqual([`${base}/coding-agent/v1`, `${base}/coding-agent`, key]);
    expect(guide('other', { anthropic: false }).primary.fields.map((f) => f.value)).toEqual([`${base}/coding-agent/v1`, key]);
  });

  it('falls back to the placeholder while the key is unknown', () => {
    expect(guide('codex', { key: '' }).manual.blocks[1].code).toBe(`export ${KEY_ENV}="${KEY_PLACEHOLDER}"`);
  });
});

describe('helpers', () => {
  it('pickModel follows the preference order, then the first model', () => {
    expect(pickModel(['gpt-5.5', 'claude-sonnet-5-5', 'claude-opus-5-5'])).toBe('claude-opus-5-5');
    expect(pickModel(['claude-opus-5-5', 'gpt-5.5'], findTool('codex').prefer)).toBe('gpt-5.5');
    expect(pickModel([])).toBe('');
  });

  it('providerIdFor separates the public site from other hosts', () => {
    expect(providerIdFor('https://modeltaps.com')).toBe('modeltaps');
    expect(providerIdFor('http://localhost:3010')).toBe('modeltaps-local');
  });

  it('isPrivateAddress spots addresses other servers cannot reach', () => {
    for (const a of [
      'http://localhost:3000',
      'http://127.0.0.1',
      'http://192.168.1.5',
      'http://10.0.0.2',
      'http://172.20.0.1',
      'http://gw.local'
    ])
      expect(isPrivateAddress(a), a).toBe(true);
    for (const a of ['https://gw.example.com', 'http://172.32.0.1', 'bad']) expect(isPrivateAddress(a), a).toBe(false);
  });
});
