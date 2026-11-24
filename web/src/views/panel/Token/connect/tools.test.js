import { describe, expect, it } from 'vitest';

import { KEY_ENV, KEY_PLACEHOLDER, TOOLS, availableTools, buildToolGuide, findTool, pickModel, providerIdFor } from './tools';

const base = 'https://gw.example.com';
const key = 'sk-test123';
const guide = (id, over = {}) => buildToolGuide(findTool(id), { base, key, model: 'claude-sonnet-5-5', anthropic: true, ...over });
const blocks = (g, method) => g.methods.find((m) => m.id === method).blocks;

describe('tool guides', () => {
  it('every tool builds valid JSON blocks for every input', () => {
    for (const tool of TOOLS) {
      for (const model of ['claude-sonnet-5-5', 'gpt-5.5']) {
        for (const anthropic of [true, false]) {
          const g = buildToolGuide(tool, { base, key, model, anthropic });
          expect(g.methods.length, tool.id).toBeGreaterThan(0);
          for (const m of g.methods)
            for (const b of m.blocks) if (b.language === 'json') expect(() => JSON.parse(b.code), tool.id).not.toThrow();
        }
      }
    }
  });

  it('Claude Code points at /claude-code without /v1 and leaves ANTHROPIC_API_KEY empty', () => {
    const env = JSON.parse(blocks(guide('claude-code'), 'file')[0].code).env;
    expect(env).toEqual({
      ANTHROPIC_BASE_URL: `${base}/claude-code`,
      ANTHROPIC_AUTH_TOKEN: key,
      ANTHROPIC_API_KEY: '',
      CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1'
    });
  });

  it('Codex uses the responses wire API, the model catalog and the key variable', () => {
    const [toml, env] = blocks(guide('codex', { model: 'gpt-5.5' }), 'file');
    expect(toml.code).toContain(`base_url = "${base}/codex/v1"`);
    expect(toml.code).toContain('wire_api = "responses"');
    expect(toml.code).toContain(`model_catalog_url = "${base}/codex/v1/models"`);
    expect(toml.code).toContain(`env_key = "${KEY_ENV}"`);
    expect(toml.code).toContain('model = "gpt-5.5"');
    expect(env.code).toBe(`export ${KEY_ENV}="${key}"`);
  });

  it('OpenCode sends Claude models over the Anthropic protocol and others over OpenAI', () => {
    const claude = JSON.parse(blocks(guide('opencode'), 'file')[0].code);
    const provider = claude.provider['modeltaps-local'];
    expect(provider.npm).toBe('@ai-sdk/anthropic');
    expect(provider.options).toEqual({ baseURL: `${base}/coding-agent/v1`, apiKey: `{env:${KEY_ENV}}` });
    expect(claude.model).toBe('modeltaps-local/claude-sonnet-5-5');
    const gpt = JSON.parse(blocks(guide('opencode', { model: 'gpt-5.5' }), 'file')[0].code);
    expect(gpt.provider['modeltaps-local'].npm).toBe('@ai-sdk/openai-compatible');
    const noAnthropic = JSON.parse(blocks(guide('opencode', { anthropic: false }), 'file')[0].code);
    expect(noAnthropic.provider['modeltaps-local'].npm).toBe('@ai-sdk/openai-compatible');
  });

  it('Goose uses the built-in Anthropic provider for Claude models, without /v1 in the host', () => {
    const cmd = blocks(guide('goose'), 'command')[0].code;
    expect(cmd).toContain('export GOOSE_PROVIDER="anthropic"');
    expect(cmd).toContain(`export ANTHROPIC_HOST="${base}/coding-agent"`);
    expect(cmd).toContain(`export ANTHROPIC_API_KEY="${key}"`);
    expect(cmd).toContain('export GOOSE_MODEL="claude-sonnet-5-5"');
  });

  it('Goose uses a custom OpenAI-compatible provider for other models', () => {
    const [provider, config] = blocks(guide('goose', { model: 'gpt-5.5' }), 'file');
    const p = JSON.parse(provider.code);
    expect(provider.path).toBe('~/.config/goose/custom_providers/modeltaps-local.json');
    expect(p).toMatchObject({
      engine: 'openai',
      api_key_env: KEY_ENV,
      base_url: `${base}/coding-agent/v1/chat/completions`,
      dynamic_models: true
    });
    expect(config.code).toBe('GOOSE_PROVIDER: modeltaps-local\nGOOSE_MODEL: gpt-5.5');
  });

  it('OpenClaw keeps the hosted setup script and the openclaw.json shape', () => {
    const g = guide('openclaw', { base: 'https://modeltaps.com', model: 'claude-opus-5-5' });
    expect(blocks(g, 'command')[0].code).toContain('bash -c "$(curl -fsSL https://modeltaps.com/openclaw-setup.sh)"');
    const cfg = JSON.parse(blocks(g, 'file')[0].code);
    expect(cfg.models.providers.modeltaps.baseUrl).toBe('https://modeltaps.com/v1');
    expect(cfg.agents.defaults.model.primary).toBe('modeltaps/claude-opus-5-5');
  });

  it('other tools get both generic endpoints, the Anthropic one only when enabled', () => {
    expect(blocks(guide('other'), 'values').map((b) => b.code)).toEqual([`${base}/coding-agent/v1`, `${base}/coding-agent`, key]);
    expect(blocks(guide('other', { anthropic: false }), 'values').map((b) => b.code)).toEqual([`${base}/coding-agent/v1`, key]);
  });

  it('falls back to the placeholder while the key is unknown', () => {
    expect(blocks(guide('codex', { key: '' }), 'file')[1].code).toBe(`export ${KEY_ENV}="${KEY_PLACEHOLDER}"`);
  });

  it('hides Claude Code when the Anthropic endpoint is off', () => {
    expect(availableTools({ anthropic: false }).map((t) => t.id)).not.toContain('claude-code');
    expect(availableTools({ anthropic: true }).map((t) => t.id)).toContain('claude-code');
  });

  it('marks caching by model and protocol', () => {
    expect(guide('claude-code').cache).toBe('verified');
    expect(guide('goose').cache).toBe('unverified');
    expect(guide('goose', { model: 'gpt-5.5' }).cache).toBe('depends');
  });
});

describe('helpers', () => {
  it('pickModel follows the preference order, then the first model', () => {
    expect(pickModel(['gpt-5.5', 'claude-sonnet-5-5', 'claude-opus-5-5'])).toBe('claude-opus-5-5');
    expect(pickModel(['claude-opus-5-5', 'gpt-5.5'], findTool('codex').prefer)).toBe('gpt-5.5');
    expect(pickModel(['x', 'y'])).toBe('x');
    expect(pickModel([])).toBe('');
  });

  it('providerIdFor separates the public site from other hosts', () => {
    expect(providerIdFor('https://modeltaps.com')).toBe('modeltaps');
    expect(providerIdFor('http://localhost:3010')).toBe('modeltaps-local');
    expect(providerIdFor('not a url')).toBe('modeltaps-local');
  });
});
