import { describe, expect, it } from 'vitest';

import { lossOf, orderByRank, sortByName } from './channelModelDetails';

describe('orderByRank', () => {
  const ranks = { a: 3, b: 1, c: 2 };
  it('follows the rank and leaves rows without details last in their order', () => {
    expect(orderByRank(['x', 'a', 'b', 'y', 'c'], (r) => (ranks[r] ? { rank: ranks[r] } : undefined))).toEqual(['b', 'c', 'a', 'x', 'y']);
  });
});

describe('sortByName', () => {
  it('sorts both ways', () => {
    const rows = [{ name: 'b' }, { name: 'a' }, { name: 'c' }];
    expect(sortByName(rows, 'asc').map((r) => r.name)).toEqual(['a', 'b', 'c']);
    expect(sortByName(rows, 'desc').map((r) => r.name)).toEqual(['c', 'b', 'a']);
  });
});

describe('lossOf', () => {
  const price = { input: 3, output: 15 };
  it('finds the cheapest public model sold below the upstream price', () => {
    const publics = [
      { model: 'm1', enabled: true, input_price: 2.5, output_price: 20 },
      { model: 'm2', enabled: true, input_price: 1, output_price: 20 }
    ];
    expect(lossOf(price, publics, 'input')).toEqual({ model: 'm2', sell: 1, cost: 3 });
    expect(lossOf(price, publics, 'output')).toBeNull();
  });
  it('treats prices equal up to float noise as no loss', () => {
    expect(
      lossOf({ input: 1, output: 3.9600000000000004 }, [{ model: 'm', enabled: true, input_price: 1, output_price: 3.96 }], 'output')
    ).toBeNull();
  });
  it('skips disabled supplies and models not sold per token', () => {
    const publics = [
      { model: 'off', enabled: false, input_price: 1, output_price: 1 },
      { model: 'flat', enabled: true, input_price: null, output_price: null }
    ];
    expect(lossOf(price, publics, 'input')).toBeNull();
  });
  it('has nothing to compare without a token price upstream', () => {
    expect(lossOf({ image: 0.04 }, [{ model: 'm', enabled: true, input_price: 0, output_price: 0 }], 'input')).toBeNull();
  });
});
