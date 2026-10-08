import { describe, expect, it } from 'vitest';

import {
  buildCapabilitySections,
  capabilityEmptyKey,
  compareCatalogModels,
  filterModels,
  isAliasOrRouterModel,
  matchesRule,
  pickPreferredModel,
  pickSectionRecommended,
  resolveVendorSlug
} from './availability';

// 可用性求值回归：规则只看后端目录字段，缺字段的模型不冒充「支持」；
// 概览页的模态卡与模态页页头共用 buildCapabilitySections / pickSectionRecommended。

const model = (id, { endpoints = [], input = [], output = [], capabilities = [], vendor = null, ownedBy = '' } = {}) => ({
  id,
  endpoints,
  vendor,
  ownedBy,
  info: { inputModalities: input, outputModalities: output, capabilities }
});

describe('matchesRule', () => {
  it('空规则放行，空模型不放行', () => {
    expect(matchesRule(model('a'), null)).toBe(true);
    expect(matchesRule(null, { endpoints: ['chat'] })).toBe(false);
  });

  it('endpoints 任一命中即可', () => {
    const m = model('a', { endpoints: ['chat', 'responses'] });
    expect(matchesRule(m, { endpoints: ['responses'] })).toBe(true);
    expect(matchesRule(m, { endpoints: ['images', 'chat'] })).toBe(true);
    expect(matchesRule(m, { endpoints: ['images'] })).toBe(false);
  });

  it('模态与能力必须全部命中', () => {
    const m = model('a', { endpoints: ['chat'], input: ['text', 'image'], capabilities: ['tool_call'] });
    expect(matchesRule(m, { inputModalities: ['image'] })).toBe(true);
    expect(matchesRule(m, { inputModalities: ['image', 'video'] })).toBe(false);
    expect(matchesRule(m, { capabilities: ['tool_call'] })).toBe(true);
    expect(matchesRule(m, { capabilities: ['tool_call', 'reasoning'] })).toBe(false);
  });

  it('大小写不敏感', () => {
    const m = model('a', { endpoints: ['Chat'], output: ['Image'] });
    expect(matchesRule(m, { endpoints: ['CHAT'], outputModalities: ['image'] })).toBe(true);
  });

  it('模型缺字段时，对该维度有要求的规则判为不满足', () => {
    expect(matchesRule({ id: 'a' }, { endpoints: ['chat'] })).toBe(false);
    expect(matchesRule({ id: 'a' }, { inputModalities: ['image'] })).toBe(false);
    expect(matchesRule({ id: 'a' }, {})).toBe(true);
  });

  it('vendors 任一命中即可，vendor 缺失时回退 owned_by', () => {
    expect(matchesRule(model('a', { vendor: 'anthropic' }), { vendors: ['anthropic'] })).toBe(true);
    expect(matchesRule(model('a', { vendor: 'openai' }), { vendors: ['anthropic', 'google'] })).toBe(false);
    expect(matchesRule(model('a', { ownedBy: 'google' }), { vendors: ['google'] })).toBe(true);
  });

  it('vendors 大小写不敏感，厂商未知时判不满足', () => {
    expect(matchesRule(model('a', { vendor: 'Anthropic' }), { vendors: ['ANTHROPIC'] })).toBe(true);
    expect(matchesRule(model('a'), { vendors: ['anthropic'] })).toBe(false);
    expect(matchesRule({ id: 'a' }, { vendors: ['anthropic'] })).toBe(false);
  });

  it('规则 slug 是模型 slug 的前缀时也算命中', () => {
    expect(matchesRule(model('a', { vendor: 'google-gemini' }), { vendors: ['google'] })).toBe(true);
    expect(matchesRule(model('a', { vendor: 'googlegemini' }), { vendors: ['google'] })).toBe(true);
    // 前缀是单向的：规则要的比模型更细时不能放行。
    expect(matchesRule(model('a', { vendor: 'google' }), { vendors: ['google-gemini'] })).toBe(false);
  });
});

describe('resolveVendorSlug', () => {
  // 后端 /api/available_model 的 vendor 字段形态不定，归一化错了会让协议兼容节数成 0。
  it('对象 vendor 优先取 slug', () => {
    expect(resolveVendorSlug({ vendor: { slug: 'google-gemini', name: 'Google Gemini' } })).toBe('google-gemini');
    expect(resolveVendorSlug({ vendor_slug: 'anthropic', vendor: { slug: 'ignored' } })).toBe('anthropic');
  });

  it('对象 vendor 没有 slug 时退化为展示名的 slug 形态', () => {
    expect(resolveVendorSlug({ vendor: { name: 'Google Gemini' } })).toBe('googlegemini');
  });

  it('只有字符串 vendor / owned_by 时同样归一化', () => {
    expect(resolveVendorSlug({ vendor: 'OpenAI' })).toBe('openai');
    expect(resolveVendorSlug({ owned_by: 'Anthropic' })).toBe('anthropic');
    expect(resolveVendorSlug({})).toBe('');
    expect(resolveVendorSlug(undefined)).toBe('');
  });

  it('归一化结果直接喂给 matchesRule 能命中规则里的粗粒度厂商', () => {
    const vendor = resolveVendorSlug({ vendor: { name: 'Google Gemini' } });
    expect(matchesRule(model('a', { vendor }), { vendors: ['google'] })).toBe(true);
    expect(matchesRule(model('a', { vendor: resolveVendorSlug({ owned_by: 'Anthropic' }) }), { vendors: ['google'] })).toBe(false);
  });
});

describe('capabilityEmptyKey', () => {
  it('只有后端没有路由的能力才说「即将上线」', () => {
    expect(capabilityEmptyKey({ id: 'generation', status: 'planned' })).toBe('apiCatalogPage.comingSoon');
    expect(capabilityEmptyKey({ id: 'realtime' })).toBe('apiCatalogPage.noModels');
    expect(capabilityEmptyKey(undefined)).toBe('apiCatalogPage.noModels');
  });

  it('取数中说「加载中」，取数失败说「可用性未知」', () => {
    expect(capabilityEmptyKey({ id: 'realtime' }, { loading: true })).toBe('common.loading');
    expect(capabilityEmptyKey({ id: 'realtime' }, { error: true })).toBe('apiCatalogPage.availabilityUnknown');
    // 模态卡不对应具体能力，同一个 helper 求值。
    expect(capabilityEmptyKey(null, { error: true })).toBe('apiCatalogPage.availabilityUnknown');
    expect(capabilityEmptyKey(null, {})).toBe('apiCatalogPage.noModels');
  });

  it('planned 能力在取数中 / 取数失败时仍说「即将上线」', () => {
    // 后端没有这条路由是静态事实，取数成败不改变它；说「可用性未知」等于把已知说成未知。
    const planned = { id: 'generation', status: 'planned' };
    expect(capabilityEmptyKey(planned, { error: true })).toBe('apiCatalogPage.comingSoon');
    expect(capabilityEmptyKey(planned, { loading: true })).toBe('apiCatalogPage.comingSoon');
    expect(capabilityEmptyKey(planned, { loading: true, error: true })).toBe('apiCatalogPage.comingSoon');
  });
});

describe('filterModels', () => {
  const models = [model('a', { endpoints: ['chat'] }), model('b', { endpoints: ['images'] }), model('c', { endpoints: ['chat'] })];

  it('按规则筛选并保持入参顺序', () => {
    expect(filterModels(models, { endpoints: ['chat'] }).map((m) => m.id)).toEqual(['a', 'c']);
  });

  it('非数组入参返回空列表', () => {
    expect(filterModels(undefined, { endpoints: ['chat'] })).toEqual([]);
  });
});

describe('pickPreferredModel', () => {
  const models = [model('a'), model('b')];

  it('优先取 preferredModels 里第一个真实可用的', () => {
    expect(pickPreferredModel(models, ['zz', 'b', 'a'])?.id).toBe('b');
  });

  it('preferredModels 都不可用（或未配置）时取列表首项', () => {
    expect(pickPreferredModel(models, ['zz'])?.id).toBe('a');
    expect(pickPreferredModel(models)?.id).toBe('a');
  });

  it('名单落空时优先取规则点名厂商的首项，再退到列表首项', () => {
    const mixed = [model('gpt', { vendor: 'openai' }), model('claude', { vendor: 'anthropic' })];
    expect(pickPreferredModel(mixed, ['zz'], ['anthropic'])?.id).toBe('claude');
    expect(pickPreferredModel(mixed, ['zz'], ['cohere'])?.id).toBe('gpt');
  });

  it('回退时跳过别名 / 路由模型，只剩它们时才取', () => {
    const list = [
      model('~anthropic/claude-latest', { vendor: 'anthropic' }),
      model('openrouter/auto'),
      model('anthropic/claude', { vendor: 'anthropic' }),
      model('gpt', { vendor: 'openai' })
    ];
    expect(pickPreferredModel(list)?.id).toBe('anthropic/claude');
    expect(pickPreferredModel(list, ['zz'], ['anthropic'])?.id).toBe('anthropic/claude');
    expect(pickPreferredModel(list, ['~anthropic/claude-latest'])?.id).toBe('~anthropic/claude-latest');
    expect(pickPreferredModel([model('~a/x'), model('openrouter/auto')])?.id).toBe('~a/x');
  });

  it('无可用模型时返回 null', () => {
    expect(pickPreferredModel([], ['a'])).toBeNull();
  });
});

describe('compareCatalogModels', () => {
  it('具体模型在前，~ 别名与 openrouter/ 路由模型排到末尾，组内按名称', () => {
    const ids = ['~anthropic/claude-latest', 'openrouter/auto', 'gpt-4o', 'anthropic/claude', '~google/gemini-latest'];
    expect(
      ids
        .map((id) => model(id))
        .sort(compareCatalogModels)
        .map((m) => m.id)
    ).toEqual(['anthropic/claude', 'gpt-4o', '~anthropic/claude-latest', '~google/gemini-latest', 'openrouter/auto']);
  });

  it('isAliasOrRouterModel 只认 ~ 前缀与 openrouter/ 前缀', () => {
    expect(isAliasOrRouterModel('~anthropic/claude-latest')).toBe(true);
    expect(isAliasOrRouterModel('openrouter/auto')).toBe(true);
    expect(isAliasOrRouterModel('anthropic/claude')).toBe(false);
    expect(isAliasOrRouterModel(undefined)).toBe(false);
  });
});

describe('buildCapabilitySections', () => {
  it('逐能力求值，能力顺序与数量不变', () => {
    const capabilities = [
      { id: 'text', availability: { endpoints: ['chat'] } },
      { id: 'vision', availability: { endpoints: ['chat'], inputModalities: ['image'] } }
    ];
    const models = [model('a', { endpoints: ['chat'] }), model('b', { endpoints: ['chat'], input: ['image'] })];

    expect(buildCapabilitySections(capabilities, models).map((s) => [s.capability.id, s.models.map((m) => m.id)])).toEqual([
      ['text', ['a', 'b']],
      ['vision', ['b']]
    ]);
  });

  it('无能力定义时返回空列表', () => {
    expect(buildCapabilitySections(undefined, [])).toEqual([]);
  });

  it('enabledFlag 被站点显式关掉时整节不参与渲染', () => {
    const capabilities = [
      { id: 'text', availability: {} },
      { id: 'claude', enabledFlag: 'ClaudeAPIEnabled', availability: {} }
    ];
    const ids = (siteInfo) => buildCapabilitySections(capabilities, [], siteInfo).map((s) => s.capability.id);

    expect(ids({ ClaudeAPIEnabled: false })).toEqual(['text']);
    expect(ids({ ClaudeAPIEnabled: true })).toEqual(['text', 'claude']);
    // siteInfo 尚未加载时按开着处理，首屏不闪掉默认开启的能力。
    expect(ids(undefined)).toEqual(['text', 'claude']);
  });
});

describe('pickSectionRecommended', () => {
  it('顺延到首个有可用模型的能力', () => {
    const sections = [
      { capability: { id: 'generations', preferredModels: ['x'] }, models: [] },
      { capability: { id: 'chatGenerations', preferredModels: ['b'] }, models: [model('a'), model('b')] }
    ];
    expect(pickSectionRecommended(sections)?.id).toBe('b');
  });

  it('全部能力都没有模型时返回 null', () => {
    expect(pickSectionRecommended([{ capability: { id: 'generation' }, models: [] }])).toBeNull();
    expect(pickSectionRecommended()).toBeNull();
  });
});
