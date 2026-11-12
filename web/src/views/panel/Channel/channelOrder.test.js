import { describe, expect, it } from 'vitest';

import {
  canMoveStep,
  moveBefore,
  moveStep,
  orderFromPolicy,
  orderInfo,
  orderToTiers,
  setWeight,
  shareWithAbove,
  stopSharing
} from './channelOrder';

const channels = [{ id: 1 }, { id: 2 }, { id: 3 }, { id: 4 }, { id: 5 }, { id: 9 }];
const providers = [
  {
    key: '62:',
    channels: [
      { id: 1, models: ['gpt', 'claude'] },
      { id: 2, models: ['gpt', 'claude'] }
    ]
  },
  { key: '1:api.venice.ai', channels: [{ id: 3, models: ['llama'] }] },
  { key: '1:router.requesty.ai', channels: [{ id: 4, models: ['gpt', 'claude', 'llama'] }] },
  {
    key: '1:',
    channels: [
      { id: 5, models: ['gpt'] },
      { id: 9, models: [] }
    ]
  }
];

describe('orderFromPolicy', () => {
  it('expands provider entries, keeps their split and puts unplaced channels last', () => {
    const tiers = [
      [
        { provider: '62:', weight: 3 },
        { channel_id: 3, weight: 1 }
      ],
      [{ provider: '1:router.requesty.ai', weight: 1 }],
      [{ channel_id: 99, weight: 1 }]
    ];
    const groups = orderFromPolicy(tiers, providers, channels);
    // Vercel's 3 between two keys: scaled by 2 so it stays whole, Venice scaled too.
    expect(groups[0]).toEqual({ ids: [1, 2, 3], weights: { 1: 3, 2: 3, 3: 2 } });
    expect(groups[1]).toEqual({ ids: [4], weights: { 4: 1 } });
    // The deleted channel 99 is gone; 5 and 9 follow one position each.
    expect(groups.slice(2).map((g) => g.ids)).toEqual([[5], [9]]);
    expect(orderToTiers(groups)[0]).toEqual([
      { channel_id: 1, weight: 3 },
      { channel_id: 2, weight: 3 },
      { channel_id: 3, weight: 2 }
    ]);
  });
});

describe('order edits', () => {
  const base = orderFromPolicy([], providers, channels); // 1, 2, 3, 4, 5, 9 one each

  it('moves, shares and splits positions', () => {
    expect(moveBefore(base, 4, 1).map((g) => g.ids)).toEqual([[4], [1], [2], [3], [5], [9]]);
    expect(moveBefore(base, 1, null).map((g) => g.ids)).toEqual([[2], [3], [4], [5], [9], [1]]);
    const shared = shareWithAbove(base, 2);
    expect(shared.map((g) => g.ids)).toEqual([[1, 2], [3], [4], [5], [9]]);
    expect(stopSharing(shared, 1).map((g) => g.ids)).toEqual([[2], [1], [3], [4], [5], [9]]);
    expect(moveStep(shared, 3, 'up').map((g) => g.ids)).toEqual([[3], [1, 2], [4], [5], [9]]);
    expect(canMoveStep(shared, 1, 'up')).toBe(false);
    expect(setWeight(shared, 2, '3')[0].weights[2]).toBe(3);
    expect(base[0].ids).toEqual([1]); // input untouched
  });

  it('works out shares only between channels with a model in common', () => {
    let groups = shareWithAbove(shareWithAbove(base, 2), 3); // 1, 2 (Vercel) and 3 (Venice) together
    groups = setWeight(groups, 1, 3);
    const info = orderInfo(groups, providers);
    expect(info.get(1)).toMatchObject({ position: 1, first: true, share: 75 });
    expect(info.get(2)).toMatchObject({ position: 1, share: 25 });
    expect(info.get(3)).toMatchObject({ position: 1, last: true, share: null });
    expect(info.get(4).position).toBe(2);
  });
});
