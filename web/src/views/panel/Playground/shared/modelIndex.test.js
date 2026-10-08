import { describe, expect, it } from 'vitest';

import { AVAILABILITY, buildProviders, collectCapabilities, filterRows, nextActiveIndex, normalizeModel } from './modelIndex';

// 归一层是选择器的口径来源：目录没给的字段必须是 undefined（UI 据此隐藏整列），
// 不能退化成 0 / 空串，否则「未知」会被画成「免费」「0 上下文」。

const catalogModel = (overrides = {}) => ({
  id: 'claude-sonnet-5',
  ownedBy: 'Anthropic',
  vendor: 'anthropic',
  endpoints: ['chat'],
  aliases: [],
  groups: ['default'],
  price: { type: 'tokens', input: 1.5, output: 7.5 },
  info: { capabilities: ['reasoning', 'tool_call'], contextLength: 200000, inputModalities: [], outputModalities: [] },
  ...overrides
});

describe('normalizeModel', () => {
  it('把目录记录整理成选择器行', () => {
    expect(normalizeModel(catalogModel())).toEqual({
      id: 'claude-sonnet-5',
      provider: 'anthropic',
      contextLength: 200000,
      pricing: { type: 'tokens', input: 1.5, output: 7.5 },
      capabilities: ['reasoning', 'tool_call'],
      availability: AVAILABILITY.available,
      channels: undefined,
      aliases: []
    });
  });

  it('上下文为 0 / 缺失时留空，不显示成 0', () => {
    expect(normalizeModel(catalogModel({ info: { contextLength: 0 } })).contextLength).toBeUndefined();
    expect(normalizeModel(catalogModel({ info: undefined })).contextLength).toBeUndefined();
  });

  it('未配置价格不是免费：pricing 留空', () => {
    expect(normalizeModel(catalogModel({ price: { type: 'tokens', input: 0, output: 0, unconfigured: true } })).pricing).toBeUndefined();
    expect(normalizeModel(catalogModel({ price: null })).pricing).toBeUndefined();
  });

  it('0 价是真的免费，保留下来', () => {
    expect(normalizeModel(catalogModel({ price: { type: 'tokens', input: 0, output: 0 } })).pricing).toEqual({
      type: 'tokens',
      input: 0,
      output: 0
    });
  });

  it('没有 vendor 时退化到 owned_by 的小写形态', () => {
    expect(normalizeModel(catalogModel({ vendor: '', ownedBy: 'OpenAI' })).provider).toBe('openai');
    expect(normalizeModel(catalogModel({ vendor: '', ownedBy: '' })).provider).toBe('');
  });

  it('一个可见分组都没有的模型判为不可用', () => {
    expect(normalizeModel(catalogModel({ groups: [] })).availability).toBe(AVAILABILITY.unavailable);
  });

  it('逐渠道报价缺失时为 undefined，后端补上同名字段即透传', () => {
    expect(normalizeModel(catalogModel()).channels).toBeUndefined();
    const channels = [{ id: 1, name: 'main', pricing: { type: 'tokens', input: 1.5, output: 7.5 } }];
    expect(normalizeModel(catalogModel({ channels })).channels).toEqual(channels);
  });
});

const rows = [
  normalizeModel(catalogModel()),
  normalizeModel(catalogModel({ id: 'claude-haiku-5', info: { capabilities: ['tool_call'], contextLength: 200000 } })),
  normalizeModel(catalogModel({ id: 'gpt-5', vendor: 'openai', ownedBy: 'OpenAI', info: { capabilities: ['reasoning'] } })),
  normalizeModel(catalogModel({ id: 'legacy-model', vendor: '', ownedBy: '', groups: [], info: { capabilities: [] } }))
];

describe('buildProviders / collectCapabilities', () => {
  it('按模型数降序列出供应商，未知供应商排在末尾', () => {
    expect(buildProviders(rows)).toEqual([
      { id: 'anthropic', count: 2 },
      { id: 'openai', count: 1 },
      { id: '', count: 1 }
    ]);
  });

  it('能力过滤项取目录里实际出现过的能力', () => {
    expect(collectCapabilities(rows)).toEqual(['reasoning', 'tool_call']);
  });
});

describe('filterRows', () => {
  const ids = (list) => list.map((row) => row.id);

  it('搜索同时命中模型名与供应商', () => {
    expect(ids(filterRows(rows, { search: 'haiku' }))).toEqual(['claude-haiku-5']);
    expect(ids(filterRows(rows, { search: 'openai' }))).toEqual(['gpt-5']);
    expect(ids(filterRows(rows, { search: 'CLAUDE' }))).toEqual(['claude-sonnet-5', 'claude-haiku-5']);
  });

  it('搜索命中别名', () => {
    const withAlias = [normalizeModel(catalogModel({ id: 'gpt-5', aliases: ['gpt-5-latest'] }))];
    expect(ids(filterRows(withAlias, { search: 'latest' }))).toEqual(['gpt-5']);
  });

  it('供应商与能力叠加，多个能力取交集', () => {
    expect(ids(filterRows(rows, { provider: 'anthropic' }))).toEqual(['claude-sonnet-5', 'claude-haiku-5']);
    expect(ids(filterRows(rows, { capabilities: ['reasoning', 'tool_call'] }))).toEqual(['claude-sonnet-5']);
    expect(ids(filterRows(rows, { provider: 'anthropic', capabilities: ['reasoning'] }))).toEqual(['claude-sonnet-5']);
  });

  it('隐藏不可用只影响不可用行', () => {
    expect(ids(filterRows(rows, { hideUnavailable: true }))).toEqual(['claude-sonnet-5', 'claude-haiku-5', 'gpt-5']);
    expect(ids(filterRows(rows, { hideUnavailable: false }))).toHaveLength(4);
  });
});

describe('nextActiveIndex', () => {
  it('上下键在列表内循环', () => {
    expect(nextActiveIndex(-1, 1, 3)).toBe(0);
    expect(nextActiveIndex(2, 1, 3)).toBe(0);
    expect(nextActiveIndex(0, -1, 3)).toBe(2);
    expect(nextActiveIndex(-1, -1, 3)).toBe(2);
  });

  it('空列表没有可高亮的行', () => {
    expect(nextActiveIndex(1, 1, 0)).toBe(-1);
  });

  // 回车选中的是过滤后的 visible[active]，而不是原始列表的第 N 行。
  it('Enter 选中 visible[active]：过滤 + 上下键联动', () => {
    const visible = filterRows(rows, { search: 'claude', hideUnavailable: true });
    let active = -1;
    active = nextActiveIndex(active, 1, visible.length);
    expect(visible[active].id).toBe('claude-sonnet-5');
    active = nextActiveIndex(active, 1, visible.length);
    expect(visible[active].id).toBe('claude-haiku-5');
    active = nextActiveIndex(active, 1, visible.length);
    expect(visible[active].id).toBe('claude-sonnet-5');
    expect(visible[nextActiveIndex(active, -1, visible.length)].id).toBe('claude-haiku-5');
  });
});
