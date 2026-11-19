import { describe, expect, it } from 'vitest';

import { lossOf, orderByRank, rowSellCmp, sellDeltaOf, sellSideOf, sortByName, sortBySellDelta } from './channelModelDetails';

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

describe('sellDeltaOf', () => {
  const price = { input: 2.27, output: 6.8 };
  it('compares the upstream price with the sell price, the dearer side', () => {
    const cmp = sellDeltaOf({ price, sellInput: 2, sellOutput: 6 });
    expect(cmp.delta).toBeCloseTo(0.135, 3);
    expect(cmp.above).toBe(true);
    expect(sellSideOf(cmp)).toBe('above');
  });
  it('prefers the known channel cost and its verdict', () => {
    const cost = { known: true, input: 2.27, output: 6.8, above_quote: false };
    const cmp = sellDeltaOf({ price, cost, sellInput: 2, sellOutput: 6 });
    expect(cmp.above).toBe(false);
    expect(sellSideOf(cmp)).toBe('above');
  });
  it('tells cheaper and equal prices, and nothing without a sell price', () => {
    expect(sellSideOf(sellDeltaOf({ price: { input: 1.6, output: 4.9 }, sellInput: 2, sellOutput: 6 }))).toBe('below');
    expect(sellSideOf(sellDeltaOf({ price: { input: 2, output: 6 }, sellInput: 2, sellOutput: 6 }))).toBe('equal');
    expect(sellDeltaOf({ price, sellInput: null, sellOutput: null })).toBeNull();
    expect(sellSideOf(null)).toBeNull();
  });
});

describe('rowSellCmp', () => {
  const price = { input: 2.27, output: 6.8 };
  it('compares with the enabled public models, else all of them', () => {
    const publics = [
      { model: 'a', enabled: false, input_price: 1, output_price: 3 },
      { model: 'b', enabled: true, input_price: 2, output_price: 6 }
    ];
    expect(rowSellCmp(price, publics).public.model).toBe('b');
    expect(rowSellCmp(price, [{ ...publics[0] }]).public.model).toBe('a');
    expect(rowSellCmp(price, [])).toBeNull();
  });
  it('sorts by the comparison, rows without one last', () => {
    const cmps = { a: { delta: 0.2 }, b: { delta: -0.1 }, c: null };
    expect(sortBySellDelta(['a', 'c', 'b'], 'asc', (r) => cmps[r])).toEqual(['b', 'a', 'c']);
    expect(sortBySellDelta(['a', 'c', 'b'], 'desc', (r) => cmps[r])).toEqual(['a', 'b', 'c']);
  });
});
