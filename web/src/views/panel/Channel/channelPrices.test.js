import { describe, expect, it } from 'vitest';
import { amount, priceDetails, priceIndex, priceSummary, upstreamNameOf, usd } from './channelPrices';

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
    expect(priceSummary({ input: 3, output: 15 })).toEqual({ kind: 'tokens', text: '3 / 15' });
    expect(priceSummary({ image: 0.04 })).toEqual({ kind: 'image', value: '$0.04' });
    expect(priceSummary({ request: 0.01 })).toEqual({ kind: 'request', value: '$0.01' });
    expect(priceSummary({ video_second: 0.05 })).toEqual({ kind: 'video', value: '$0.05' });
    expect(priceSummary({ input: 0, output: 0 })).toEqual({ kind: 'free' });
    expect(priceSummary({ input: 0, output: 0, other: true })).toEqual({ kind: 'other' });
    expect(priceSummary(undefined)).toBeNull();
  });
});

describe('priceDetails', () => {
  it('lists only the prices the upstream states', () => {
    expect(priceDetails({ input: 3, output: 15, cache_read: 0.3 })).toEqual([
      ['input', '$3'],
      ['output', '$15'],
      ['cache_read', '$0.3']
    ]);
  });
});

describe('priceIndex', () => {
  it('looks prices up by upstream model name', () => {
    expect(priceIndex([{ model: 'a', input: 1 }]).get('a').input).toBe(1);
  });
});
