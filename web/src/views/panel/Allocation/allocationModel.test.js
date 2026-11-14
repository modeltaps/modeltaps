import { describe, expect, it } from 'vitest';

import {
  addProvider,
  canMoveEntry,
  entryModels,
  mergeChannels,
  missingEntry,
  moveEntry,
  moveTier,
  removeEntry,
  setWeight,
  splitOut,
  splitProvider,
  tierShares,
  tierSplit,
  unlistedProviders
} from './allocationModel';

const vercel = { key: '62:', type: 62, host: '', channels: [{ id: 1 }, { id: 2 }] };
const requesty = { key: '1:router.requesty.ai', type: 1, host: 'router.requesty.ai', channels: [{ id: 3 }] };
const venice = { key: '1:api.venice.ai', type: 1, host: 'api.venice.ai', channels: [{ id: 4 }] };
const providers = [vercel, requesty, venice];
const tiers = [[{ provider: '62:', weight: 1 }], [{ provider: '1:router.requesty.ai', weight: 1 }]];

describe('allocation tiers', () => {
  it('lists providers the policy does not place', () => {
    expect(unlistedProviders(providers, tiers).map((p) => p.key)).toEqual(['1:api.venice.ai']);
    // Every channel placed on its own counts as placed.
    expect(unlistedProviders([vercel], [[{ channel_id: 1 }, { channel_id: 2 }]])).toEqual([]);
    expect(unlistedProviders([vercel], [[{ channel_id: 1 }]]).map((p) => p.key)).toEqual(['62:']);
  });

  it('moves an entry into the next tier, or into a tier of its own at an end', () => {
    expect(moveEntry(tiers, 1, 0, 'up')).toEqual([
      [
        { provider: '62:', weight: 1 },
        { provider: '1:router.requesty.ai', weight: 1 }
      ]
    ]);
    const shared = [[{ provider: 'a' }, { provider: 'b' }]];
    expect(moveEntry(shared, 0, 1, 'up')).toEqual([[{ provider: 'b' }], [{ provider: 'a' }]]);
    expect(moveEntry(shared, 0, 0, 'down')).toEqual([[{ provider: 'b' }], [{ provider: 'a' }]]);
    expect(canMoveEntry(tiers, 0, 'up')).toBe(false);
    expect(canMoveEntry(shared, 0, 'up')).toBe(true);
    expect(tiers[1]).toHaveLength(1); // input untouched
  });

  it('splits, reorders, removes and adds', () => {
    const shared = [[{ provider: 'a' }, { provider: 'b' }], [{ provider: 'c' }]];
    expect(splitOut(shared, 0, 0)).toEqual([[{ provider: 'b' }], [{ provider: 'a' }], [{ provider: 'c' }]]);
    expect(moveTier(shared, 1, 'up')).toEqual([[{ provider: 'c' }], [{ provider: 'a' }, { provider: 'b' }]]);
    expect(removeEntry(shared, 1, 0)).toEqual([[{ provider: 'a' }, { provider: 'b' }]]);
    expect(addProvider(tiers, '1:api.venice.ai')[2]).toEqual([{ provider: '1:api.venice.ai', weight: 1 }]);
    expect(setWeight(tiers, 0, 0, '0')[0][0].weight).toBe(1);
  });

  it('splits a provider into its channels and merges them back', () => {
    const split = splitProvider([[{ provider: '62:', weight: 3 }]], 0, 0, vercel);
    expect(split).toEqual([
      [
        { channel_id: 1, weight: 3 },
        { channel_id: 2, weight: 3 }
      ]
    ]);
    expect(mergeChannels(split, 0, vercel)).toEqual([[{ provider: '62:', weight: 6 }]]);
  });

  it('gives whole percentages adding up to 100', () => {
    expect(tierShares([{ weight: 1 }, { weight: 1 }, { weight: 1 }])).toEqual([34, 33, 33]);
    expect(tierShares([{ weight: 70 }, { weight: 30 }])).toEqual([70, 30]);
  });

  it('spots entries whose provider or channel is gone', () => {
    expect(missingEntry({ provider: 'gone' }, providers)).toBe(true);
    expect(missingEntry({ channel_id: 4 }, providers)).toBe(false);
  });
});

describe('tierSplit', () => {
  const anthropic = {
    key: '14:',
    type: 14,
    channels: [
      { id: 3, models: ['claude'] },
      { id: 4, models: ['claude'] }
    ]
  };
  const openai = {
    key: '1:',
    type: 1,
    channels: [
      { id: 1, models: ['gpt', 'gpt-mini'] },
      { id: 2, models: ['gpt'] }
    ]
  };
  const deepseek = { key: '28:', type: 28, channels: [{ id: 5, models: ['deepseek'] }] };
  const list = [anthropic, openai, deepseek];

  it('splits only between entries that serve a model in common', () => {
    const tier = [
      { channel_id: 3, weight: 2 },
      { channel_id: 4, weight: 1 },
      { channel_id: 1, weight: 3 },
      { channel_id: 2, weight: 1 },
      { provider: '28:', weight: 5 }
    ];
    const split = tierSplit(tier, list, [tier]);
    expect(split.map((s) => s.share)).toEqual([67, 33, 75, 25, null]);
    expect(split[0].peers).toEqual([1]);
    expect(split[2].peers).toEqual([3]);
  });

  it('counts a provider entry only for its channels without an entry of their own', () => {
    const tiers = [
      [
        { provider: '1:', weight: 1 },
        { provider: '14:', weight: 1 }
      ],
      [{ channel_id: 1, weight: 1 }]
    ];
    // Channel 1 is placed in the second tier, so the OpenAI entry only serves "gpt" through channel 2.
    expect(entryModels(tiers[0][0], list, tiers)).toEqual(new Set(['gpt']));
    expect(tierSplit(tiers[0], list, tiers).map((s) => s.share)).toEqual([null, null]);
  });
});
