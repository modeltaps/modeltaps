import { describe, expect, it } from 'vitest';

import { usageNumbers, usageOf, usageOptions, usagePeers } from './channelUsage';

const all = [
  { id: 1, type: 1, status: 1, group: 'default', priority: 10, weight: 3 },
  { id: 2, type: 1, status: 1, group: 'default,vip', priority: 10, weight: 1 },
  { id: 3, type: 1, status: 2, group: 'default', priority: 20, weight: 1 },
  { id: 4, type: 14, status: 1, group: 'default', priority: 30, weight: 1 },
  { id: 5, type: 1, status: 1, group: 'internal', priority: 40, weight: 1 }
];

describe('usagePeers', () => {
  it('keeps enabled channels of the same type sharing a group, top first', () => {
    expect(usagePeers({ id: 9, type: 1, groups: ['default'] }, all).map((c) => c.id)).toEqual([1, 2]);
    expect(usagePeers({ id: 2, type: 1, group: 'vip' }, all)).toEqual([]);
  });
});

describe('usageOptions', () => {
  const peers = usagePeers({ id: 9, type: 1, groups: ['default'] }, all);

  it('offers backup / share / first relative to the top peer', () => {
    const { top, options } = usageOptions({ priority: 0, weight: 1 }, peers);
    expect(top.id).toBe(1);
    expect(options.map((o) => [o.key, o.priority, o.weight])).toEqual([
      ['backup', 9, 1],
      ['share', 10, 1],
      ['first', 11, 1],
      ['custom', 0, 1]
    ]);
    // weight 1 next to a tier of weight 3 + 1
    expect(options[1].share).toBe(20);
  });

  it('is the only provider without peers', () => {
    expect(usageOptions({ priority: 4, weight: 2 }, []).options.map((o) => o.key)).toEqual(['only', 'custom']);
  });
});

describe('usageOf / usageNumbers', () => {
  const peers = usagePeers({ id: 9, type: 1, groups: ['default'] }, all);

  it('recognizes the current relation, custom otherwise', () => {
    expect(usageOf({ priority: 9, weight: 1 }, peers)).toBe('backup');
    expect(usageOf({ priority: 10, weight: 1 }, peers)).toBe('share');
    expect(usageOf({ priority: 11, weight: 1 }, peers)).toBe('first');
    expect(usageOf({ priority: 3, weight: 2 }, peers)).toBe('custom');
    expect(usageOf({ priority: 3, weight: 2 }, [])).toBe('only');
  });

  it('turns a choice into priority and weight', () => {
    expect(usageNumbers('backup', { priority: 0, weight: 1 }, peers)).toEqual({ priority: 9, weight: 1 });
    expect(usageNumbers('custom', {}, peers, { priority: '7', weight: '0' })).toEqual({ priority: 7, weight: 1 });
    expect(usageNumbers('only', { priority: 2, weight: 4 }, [])).toEqual({ priority: 2, weight: 4 });
  });
});
