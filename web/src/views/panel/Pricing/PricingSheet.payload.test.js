import { describe, expect, it, vi } from 'vitest';

vi.mock('utils/api', () => ({ API: {} }));

const { buildPricePayload, priceFormValues } = await import('./PricingSheet');

const offPeak = { ratio: 0.5, peak: [{ days: [1, 2, 3, 4, 5], from: '01:00', to: '04:00' }] };
const stored = { model: 'deepseek-v4-pro', type: 'tokens', channel_type: 36, input: 0.66, output: 1.98, off_peak: offPeak };

describe('price payload', () => {
  it('keeps the off-peak schedule of the price being edited', () => {
    const { price } = buildPricePayload(priceFormValues(stored), 'ratio', 'M');
    expect(price.off_peak).toEqual(offPeak);
  });

  it('drops it once removed', () => {
    const { price } = buildPricePayload({ ...priceFormValues(stored), off_peak: null }, 'ratio', 'M');
    expect(price.off_peak).toBeUndefined();
  });
});
