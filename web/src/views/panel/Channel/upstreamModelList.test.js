import { describe, expect, it } from 'vitest';

import { filterUpstreamModels, upstreamCounts, upstreamFilterOf } from './upstreamModelList';

const models = [
  { name: 'gpt-9', state: 'in_use', public_model: 'gpt-9' },
  { name: 'gpt-9-mini', state: 'off', public_model: 'gpt-9-mini' },
  { name: 'openai/o9', state: 'available', public_model: 'o9' },
  { name: 'gpt-8', state: 'not_in_catalog', public_model: 'gpt-8' },
  { name: 'internal-eval', state: 'unknown' }
];

describe('upstream model helpers', () => {
  it('counts a switched-off supply as available', () => {
    expect(upstreamFilterOf('off')).toBe('available');
    expect(upstreamCounts(models)).toEqual({ all: 5, in_use: 1, available: 2, not_in_catalog: 1, unknown: 1 });
  });

  it('treats a state it does not know as unknown', () => {
    expect(upstreamFilterOf('something-new')).toBe('unknown');
  });

  it('filters by state and by either name', () => {
    expect(filterUpstreamModels(models, 'available').map((m) => m.name)).toEqual(['gpt-9-mini', 'openai/o9']);
    expect(filterUpstreamModels(models, 'all', ' O9 ').map((m) => m.name)).toEqual(['openai/o9']);
    expect(filterUpstreamModels(models, 'in_use', 'mini')).toEqual([]);
    expect(filterUpstreamModels(models, 'all', '')).toHaveLength(5);
  });
});
