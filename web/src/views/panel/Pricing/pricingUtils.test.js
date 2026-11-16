import { describe, expect, it } from 'vitest';

import { convertUnit, formatPrice, getPriceTypeOptions, isFlatPriceType, rateToValue, valueToRate } from './pricingUtils';

describe('price type helpers', () => {
  it('treats per-call and per-minute prices as flat, per-token prices as scaled', () => {
    expect(isFlatPriceType('times')).toBe(true);
    expect(isFlatPriceType('minutes')).toBe(true);
    expect(isFlatPriceType('tokens')).toBe(false);
  });

  it('offers tokens, times and minutes', () => {
    const t = (key) => key;
    expect(getPriceTypeOptions(t)).toEqual([
      { value: 'tokens', label: 'modelpricePage.tokens' },
      { value: 'times', label: 'modelpricePage.times' },
      { value: 'minutes', label: 'modelpricePage.minutes' }
    ]);
  });
});

describe('rate conversion', () => {
  it('scales per-token prices by K / M', () => {
    expect(valueToRate(2, 'USD', 'M', 'tokens')).toBe(1);
    expect(valueToRate(0.002, 'USD', 'K', 'tokens')).toBe(1);
    expect(rateToValue(1, 'USD', 'M', 'tokens')).toBe(2);
    expect(rateToValue(1, 'RMB', 'K', 'tokens')).toBe(0.014);
  });

  it('keeps per-call prices unscaled', () => {
    expect(valueToRate(0.002, 'USD', 'M', 'times')).toBe(1);
    expect(rateToValue(1, 'USD', 'M', 'times')).toBe(0.002);
  });

  it('converts per-minute prices like per-call ones, ignoring K / M', () => {
    // $0.006 per minute = 3 rate; ￥0.042 per minute = 3 rate.
    expect(valueToRate(0.006, 'USD', 'M', 'minutes')).toBe(3);
    expect(valueToRate(0.006, 'USD', 'K', 'minutes')).toBe(3);
    expect(valueToRate(0.042, 'RMB', 'M', 'minutes')).toBe(3);
    expect(rateToValue(3, 'USD', 'M', 'minutes')).toBe(0.006);
    expect(rateToValue(3, 'RMB', 'K', 'minutes')).toBe(0.042);
    expect(valueToRate(3, 'rate', 'M', 'minutes')).toBe(3);
    expect(convertUnit(0.006, 'USD', 'K', 'RMB', 'M', 'minutes')).toBe(0.042);
    expect(convertUnit(0.006, 'USD', 'K', 'USD', 'M', 'minutes')).toBe(0.006);
  });
});

describe('formatPrice', () => {
  it('suffixes per-token prices with the K / M unit', () => {
    expect(formatPrice(1, 'tokens', 'M')).toBe('$2 / 1M');
    expect(formatPrice(1, 'tokens', 'K')).toBe('$0.002 / 1K');
  });

  it('shows per-call prices without a unit or scaling', () => {
    expect(formatPrice(1, 'times', 'M')).toBe('$0.002');
  });

  it('shows per-minute prices per minute without scaling', () => {
    expect(formatPrice(3, 'minutes', 'M')).toBe('$0.006 / min');
    expect(formatPrice(0, 'minutes', 'M')).toBe('Free');
  });
});
