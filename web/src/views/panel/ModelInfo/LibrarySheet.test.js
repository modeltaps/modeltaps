import { describe, expect, it } from 'vitest';

import { DEFAULT_LIBRARY_FILTER, defaultPicks, filterLibrary, offerKey } from './LibrarySheet';

const NOW = 1_800_000_000;
const items = [
  {
    canonical: 'anthropic/claude-fable-5-2',
    model: 'claude-fable-5-2',
    name: 'Claude Fable 5.2',
    lab: 'anthropic',
    endpoints: '["chat","responses"]',
    released_at: NOW - 10 * 86400,
    channels: [{ id: 1, match: 'platform', default: true }],
    in_catalog: false
  },
  {
    canonical: 'openai/gpt-5',
    model: 'gpt-5',
    name: 'GPT-5',
    lab: 'openai',
    endpoints: '["chat"]',
    released_at: NOW - 400 * 86400,
    channels: [{ id: 2, match: 'variant' }],
    in_catalog: true
  },
  {
    canonical: 'meta/llama-4',
    model: 'llama-4',
    name: 'Llama 4',
    lab: 'meta',
    endpoints: '["chat"]',
    released_at: 0,
    channels: [],
    open_weights: true,
    deprecated: true
  }
];
const run = (patch) => filterLibrary(items, { ...DEFAULT_LIBRARY_FILTER, ...patch }, NOW).map((it) => it.model);

describe('filterLibrary', () => {
  it('hides added and deprecated models by default', () => {
    expect(run({})).toEqual(['claude-fable-5-2']);
  });
  it('filters by vendor, endpoint, release window, supply and open weights', () => {
    const all = { hideAdded: false, hideDeprecated: false };
    expect(run({ ...all, lab: 'openai' })).toEqual(['gpt-5']);
    expect(run({ ...all, endpoint: 'responses' })).toEqual(['claude-fable-5-2']);
    expect(run({ ...all, days: '30' })).toEqual(['claude-fable-5-2']);
    expect(run({ ...all, suppliedOnly: true })).toEqual(['claude-fable-5-2']);
    expect(run({ ...all, openOnly: true })).toEqual(['llama-4']);
  });
  it('matches the keyword against id, name and lab', () => {
    expect(run({ hideAdded: false, keyword: 'fable 5.2' })).toEqual(['claude-fable-5-2']);
    expect(run({ hideAdded: false, keyword: 'OPENAI' })).toEqual(['gpt-5']);
  });
});

describe('channel offers', () => {
  const offers = [
    { id: 1, upstream_model: 'anthropic/claude-fable-5.2', match: 'platform', default: true },
    { id: 1, upstream_model: 'anthropic/claude-fable-5.2:free', match: 'platform', default: false },
    { id: 2, upstream_model: 'aws-claude-fable-5-2', match: 'pending', default: false },
    { id: 3, upstream_model: 'claude-fable-5-2', match: 'exact', default: true, excluded: true }
  ];
  it('starts with the sure, not rejected offers', () => {
    expect([...defaultPicks(offers)]).toEqual([offerKey(offers[0])]);
  });
  it('tells offers on one channel apart by upstream name', () => {
    expect(offerKey(offers[0])).not.toEqual(offerKey(offers[1]));
  });
});
