import { describe, expect, it } from 'vitest';
import { amount, priceIndex, priceSummary, sortByTokenPrice, tokenPrice, upstreamNameOf, usd } from './channelPrices';

describe('upstreamNameOf', () => {
  it('uses the mapping target without the billing "+"', () => {
    expect(upstreamNameOf('opus', { opus: '+anthropic/claude-opus-4.8' })).toBe('anthropic/claude-opus-4.8');
    expect(upstreamNameOf('gpt-4o', { opus: 'x' })).toBe('gpt-4o');
    expect(upstreamNameOf('gpt-4o', { 'gpt-4o': '  ' })).toBe('gpt-4o');
  });
});

describe('usd', () => {
  it('keeps four significant digits and drops trailing zeros', () => {
    expect(usd(3)).toBe('$3');
    expect(usd(0.075)).toBe('$0.075');
    expect(usd(2.999999999)).toBe('$3');
    expect(usd(0)).toBe('$0');
    expect(amount(0.12)).toBe('0.12');
  });
});

describe('priceSummary', () => {
  it('reads token, per-image, per-request and free prices', () => {
    expect(priceSummary({ input: 3, output: 15 })).toEqual({ kind: 'tokens', input: '3', output: '15' });
    expect(priceSummary({ image: 0.04 })).toEqual({ kind: 'image', value: '$0.04' });
    expect(priceSummary({ request: 0.01 })).toEqual({ kind: 'request', value: '$0.01' });
    expect(priceSummary({ video_second: 0.05 })).toEqual({ kind: 'video', value: '$0.05' });
    expect(priceSummary({ input: 0, output: 0 })).toEqual({ kind: 'free' });
    expect(priceSummary({ input: 0, output: 0, other: true })).toEqual({ kind: 'other' });
    expect(priceSummary(undefined)).toBeNull();
  });
});

describe('priceIndex', () => {
  it('looks prices up by upstream model name', () => {
    expect(priceIndex([{ model: 'a', input: 1 }]).get('a').input).toBe(1);
  });
});

describe('tokenPrice', () => {
  it('reads input or output of a model priced per token', () => {
    expect(tokenPrice({ input: 3, output: 15 }, 'input')).toBe(3);
    expect(tokenPrice({ input: 0.1, output: 0 }, 'output')).toBe(0);
  });
  it('is null for other pricing or no price', () => {
    expect(tokenPrice({ image: 0.04 }, 'input')).toBeNull();
    expect(tokenPrice(undefined, 'output')).toBeNull();
  });
});

describe('sortByTokenPrice', () => {
  const prices = { a: { input: 3, output: 15 }, b: { image: 0.04 }, c: { input: 1, output: 20 }, d: { input: 3, output: 1 }, e: null };
  const priceOf = (r) => prices[r];
  const rows = ['a', 'b', 'c', 'd', 'e'];
  it('sorts ascending with unpriced rows last in their order', () => {
    expect(sortByTokenPrice(rows, 'input', 'asc', priceOf)).toEqual(['c', 'a', 'd', 'b', 'e']);
  });
  it('sorts descending, ties keeping their order', () => {
    expect(sortByTokenPrice(rows, 'input', 'desc', priceOf)).toEqual(['a', 'd', 'c', 'b', 'e']);
    expect(sortByTokenPrice(rows, 'output', 'desc', priceOf)).toEqual(['c', 'a', 'd', 'b', 'e']);
  });
});
