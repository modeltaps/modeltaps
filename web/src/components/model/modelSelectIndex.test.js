import { describe, expect, it } from 'vitest';

import { CUSTOM_VENDOR, compareNewestFirst, groupOptions, groupSelected, inferVendor, mergeOptions, toggleGroup } from './modelSelectIndex';

const DAY = 86400;

describe('inferVendor', () => {
  it('reads brand tokens before the vendor prefix', () => {
    expect(inferVendor('anthropic/claude-opus-5-5').name).toBe('Anthropic');
    expect(inferVendor('claude-haiku-4-5-20251001').name).toBe('Anthropic');
    expect(inferVendor('z-ai/glm-5.1').name).toBe('Zhipu AI');
  });

  it('falls back to the prefix as written, then to the custom bucket', () => {
    expect(inferVendor('nousresearch/hermes-5')).toEqual({ key: 'nousresearch', name: 'Nousresearch' });
    expect(inferVendor('my-model').key).toBe(CUSTOM_VENDOR);
  });
});

describe('compareNewestFirst', () => {
  it('puts the later release first and undated ids after dated ones', () => {
    const list = [{ id: 'a', releasedAt: 10 * DAY }, { id: 'b' }, { id: 'c', releasedAt: 20 * DAY }];
    expect(list.sort(compareNewestFirst).map((o) => o.id)).toEqual(['c', 'a', 'b']);
  });

  it('orders undated ids by version, highest first', () => {
    const list = ['claude-opus-4-5', 'claude-opus-5-5', 'gpt-5.6-sol', 'gpt-6.1-sol'].map((id) => ({ id }));
    expect(list.sort(compareNewestFirst).map((o) => o.id)).toEqual(['gpt-6.1-sol', 'gpt-5.6-sol', 'claude-opus-5-5', 'claude-opus-4-5']);
  });
});

describe('groupOptions', () => {
  const options = [
    { id: 'openai/gpt-5.5', releasedAt: 5 * DAY },
    { id: 'anthropic/claude-opus-5-5', releasedAt: 9 * DAY },
    { id: 'openai/gpt-6.1-sol', releasedAt: 8 * DAY },
    { id: 'my-local-model' }
  ];

  it('groups by vendor, orders groups by their newest model and keeps custom last', () => {
    const groups = groupOptions(options);
    expect(groups.map((g) => g.name)).toEqual(['Anthropic', 'OpenAI', '']);
    expect(groups[1].items.map((o) => o.id)).toEqual(['openai/gpt-6.1-sol', 'openai/gpt-5.5']);
    expect(groups.at(-1).key).toBe(CUSTOM_VENDOR);
  });

  it('matches the query against the id and the vendor name', () => {
    expect(groupOptions(options, { query: 'opus' }).flatMap((g) => g.items.map((o) => o.id))).toEqual(['anthropic/claude-opus-5-5']);
    expect(groupOptions(options, { query: 'openai' })[0].items).toHaveLength(2);
  });

  it('keeps only selected ids when asked', () => {
    const groups = groupOptions(options, { selected: new Set(['openai/gpt-5.5']), onlySelected: true });
    expect(groups.flatMap((g) => g.items.map((o) => o.id))).toEqual(['openai/gpt-5.5']);
  });

  it('uses the vendor a caller passes instead of guessing', () => {
    const [group] = groupOptions([{ id: 'x-1', vendor: { key: 'acme', name: 'Acme', icon: 'brand:acme' } }]);
    expect(group).toMatchObject({ key: 'acme', name: 'Acme', icon: 'brand:acme' });
  });
});

describe('selection helpers', () => {
  it('dedupes options case-insensitively, first one wins', () => {
    expect(mergeOptions([{ id: 'GPT-5', releasedAt: 1 }], ['gpt-5', 'o9'])).toEqual([{ id: 'GPT-5', releasedAt: 1 }, { id: 'o9' }]);
  });

  it('summarises ids that are not in the options', () => {
    expect(groupSelected([], ['claude-x']).map((g) => g.name)).toEqual(['Anthropic']);
  });

  it('selects a whole group unless it is already fully selected', () => {
    expect(toggleGroup(['a'], ['a', 'b'])).toEqual(['a', 'b']);
    expect(toggleGroup(['a', 'b', 'c'], ['a', 'b'])).toEqual(['c']);
  });
});
