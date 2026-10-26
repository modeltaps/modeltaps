import { describe, expect, it } from 'vitest';

import {
  EMPTY_FILTERS,
  expiresSoon,
  filterOptions,
  formatContext,
  formatPrice,
  groupFamilies,
  matchesFilters,
  toggleMany,
  vendorOf,
  visibleModels
} from './modelPicker';

const row = (model, extra = {}) => ({
  model,
  family: 'f',
  version: '',
  latest: false,
  input_modalities: ['text'],
  output_modalities: ['text'],
  capabilities: [],
  open_weights: null,
  input_price: null,
  output_price: null,
  context_length: 0,
  other_channels: 0,
  ...extra
});

const ITEMS = [
  row('anthropic/claude-opus-5.5', {
    family: 'claude-opus',
    version: '5.5',
    latest: true,
    capabilities: ['tool_call', 'reasoning'],
    input_modalities: ['text', 'image'],
    open_weights: false,
    input_price: 5,
    output_price: 25,
    context_length: 1000000,
    other_channels: 1
  }),
  row('anthropic/claude-opus-5', {
    family: 'claude-opus',
    version: '5',
    capabilities: ['tool_call'],
    open_weights: false,
    input_price: 5,
    output_price: 25,
    context_length: 200000
  }),
  row('anthropic/claude-opus-4.8', {
    family: 'claude-opus',
    version: '4.8',
    open_weights: false,
    input_price: 15,
    output_price: 75,
    context_length: 200000
  }),
  row('anthropic/claude-opus-5.5:batch', { family: 'claude-opus', version: '5.5', variant: 'batch', open_weights: false }),
  row('qwen/qwen3.8-27b', {
    family: 'qwen-27b',
    version: '3.8',
    latest: true,
    capabilities: ['tool_call'],
    open_weights: true,
    input_price: 0.2,
    output_price: 2.5,
    context_length: 1000000
  }),
  row('qwen/qwen3.6-27b', {
    family: 'qwen-27b',
    version: '3.6',
    open_weights: true,
    input_price: 0.3,
    output_price: 3,
    context_length: 128000
  }),
  row('openai/gpt-image-2', { family: 'gpt-image', version: '2', latest: true, output_modalities: ['image'], open_weights: false }),
  row('custom-model', { family: 'custom-model', latest: true, input_modalities: [], output_modalities: [] })
];
const FAMILIES = [
  { family: 'qwen-27b', rank: 1, requests: 40, count: 2 },
  { family: 'claude-opus', rank: 2, requests: 0, count: 4 },
  { family: 'gpt-image', rank: 3, requests: 0, count: 1 },
  { family: 'custom-model', rank: 4, requests: 0, count: 1 }
];
const none = new Set();
const filters = (extra) => ({ ...EMPTY_FILTERS, ...extra });

describe('vendorOf', () => {
  it('uses the vendor prefix, else the first family word', () => {
    expect(vendorOf(row('anthropic/claude-opus-5.5'))).toBe('anthropic');
    expect(vendorOf(row('~anthropic/claude-opus-latest'))).toBe('anthropic');
    expect(vendorOf(row('claude-opus-5-5', { family: 'claude-opus' }))).toBe('claude');
  });
});

describe('groupFamilies', () => {
  it('orders families by rank and shows only the latest version by default', () => {
    const groups = groupFamilies(ITEMS, FAMILIES, filters(), none);
    expect(groups.map((g) => g.family)).toEqual(['qwen-27b', 'claude-opus', 'custom-model']);
    expect(visibleModels(groups)).toEqual(['qwen/qwen3.8-27b', 'anthropic/claude-opus-5.5', 'custom-model']);
    expect(groups[1].hidden).toBe(3);
  });

  it('keeps rows without modality metadata under the default text filter', () => {
    const groups = groupFamilies(ITEMS, FAMILIES, filters(), none);
    expect(groups.some((g) => g.family === 'custom-model')).toBe(true);
    expect(groups.some((g) => g.family === 'gpt-image')).toBe(false);
  });

  it('expands a family newest first, latest on top', () => {
    const groups = groupFamilies(ITEMS, FAMILIES, filters(), none, { expanded: new Set(['claude-opus']) });
    expect(groups[1].rows.map((r) => r.model)).toEqual([
      'anthropic/claude-opus-5.5',
      'anthropic/claude-opus-5.5:batch',
      'anthropic/claude-opus-5',
      'anthropic/claude-opus-4.8'
    ]);
    expect(groups[1].hidden).toBe(0);
  });

  it('falls back to the best remaining row when the latest is filtered out', () => {
    const selected = new Set(['anthropic/claude-opus-5', 'anthropic/claude-opus-4.8']);
    const groups = groupFamilies(ITEMS, FAMILIES, filters({ status: 'selected' }), selected);
    expect(groups).toHaveLength(1);
    expect(groups[0].rows.map((r) => r.model)).toEqual(['anthropic/claude-opus-5']);
    expect(groups[0].hidden).toBe(1);
    const cheap = groupFamilies(ITEMS, FAMILIES, filters({ maxInput: '0.25' }), none);
    expect(visibleModels(cheap)).toEqual(['qwen/qwen3.8-27b']);
  });

  it('shows every match while searching, even with latest only on', () => {
    const groups = groupFamilies(ITEMS, FAMILIES, filters({ search: 'claude-opus' }), none);
    expect(groups).toHaveLength(1);
    expect(groups[0].rows).toHaveLength(4);
  });

  it('can turn the folding off', () => {
    const groups = groupFamilies(ITEMS, FAMILIES, filters(), none, { latestOnly: false });
    expect(visibleModels(groups)).toHaveLength(7);
  });
});

describe('matchesFilters', () => {
  const opus = ITEMS[0];
  const qwen = ITEMS[4];
  it('filters by status', () => {
    const selected = new Set([opus.model]);
    expect(matchesFilters(opus, filters({ status: 'selected' }), selected)).toBe(true);
    expect(matchesFilters(qwen, filters({ status: 'selected' }), selected)).toBe(false);
    expect(matchesFilters(qwen, filters({ status: 'unselected' }), selected)).toBe(true);
    expect(matchesFilters(opus, filters({ status: 'elsewhere' }), none)).toBe(true);
    expect(matchesFilters(qwen, filters({ status: 'elsewhere' }), none)).toBe(false);
  });
  it('filters by weights, vendor, capability and vision', () => {
    expect(matchesFilters(qwen, filters({ weights: 'open' }), none)).toBe(true);
    expect(matchesFilters(opus, filters({ weights: 'open' }), none)).toBe(false);
    expect(matchesFilters(ITEMS[7], filters({ weights: 'closed' }), none)).toBe(false);
    expect(matchesFilters(qwen, filters({ vendors: ['anthropic'] }), none)).toBe(false);
    expect(matchesFilters(opus, filters({ capabilities: ['reasoning', 'vision'] }), none)).toBe(true);
    expect(matchesFilters(qwen, filters({ capabilities: ['vision'] }), none)).toBe(false);
  });
  it('treats an unknown price as not matching a price limit', () => {
    expect(matchesFilters(ITEMS[3], filters({ maxInput: '100' }), none)).toBe(false);
    expect(matchesFilters(opus, filters({ maxOutput: '25' }), none)).toBe(true);
    expect(matchesFilters(opus, filters({ maxOutput: '24' }), none)).toBe(false);
    expect(matchesFilters(opus, filters({ maxInput: 'abc' }), none)).toBe(true);
  });
});

describe('helpers', () => {
  it('toggleMany adds all, or removes all when every id is already in', () => {
    expect([...toggleMany(new Set(['a']), ['a', 'b'])]).toEqual(['a', 'b']);
    expect([...toggleMany(new Set(['a', 'b', 'c']), ['a', 'b'])]).toEqual(['c']);
    expect([...toggleMany(new Set(['a']), [])]).toEqual(['a']);
  });
  it('lists vendors by count and the output modalities present', () => {
    const options = filterOptions(ITEMS);
    expect(options.vendors[0]).toEqual({ id: 'anthropic', count: 4 });
    expect(options.outputs).toEqual(['image', 'text']);
  });
  it('formats context, price and retirement', () => {
    expect(formatContext(1000000)).toBe('1M');
    expect(formatContext(1048576)).toBe('1M');
    expect(formatContext(128000)).toBe('128K');
    expect(formatContext(0)).toBe('');
    expect(formatPrice(25)).toBe('25');
    expect(formatPrice(0.2)).toBe('0.2');
    expect(formatPrice(0.0015)).toBe('0.0015');
    expect(formatPrice(null)).toBe('');
    expect(expiresSoon({ expires_at: 1000 + 86400 * 10 }, 1000)).toBe(true);
    expect(expiresSoon({ expires_at: 1000 + 86400 * 60 }, 1000)).toBe(false);
    expect(expiresSoon({ expires_at: 0 }, 1000)).toBe(false);
  });
});
