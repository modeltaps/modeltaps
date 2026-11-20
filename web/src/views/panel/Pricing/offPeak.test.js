import { describe, expect, it } from 'vitest';
import { hasOffPeak, peakHoursText } from './offPeak';

const t = (key) => key;
const deepseek = {
  ratio: 0.5,
  peak: [
    { days: [1, 2, 3, 4, 5], from: '01:00', to: '04:00' },
    { days: [1, 2, 3, 4, 5], from: '06:00', to: '10:00' }
  ]
};

describe('off-peak pricing', () => {
  it('groups windows on the same weekdays', () => {
    expect(peakHoursText(deepseek, t)).toBe('pricing_edit.offPeak.weekdays 01:00–04:00, 06:00–10:00');
  });

  it('names other days', () => {
    expect(peakHoursText({ ratio: 0.5, peak: [{ days: [6, 0], from: '00:00', to: '24:00' }] }, t, 'en')).toBe('Sun, Sat 00:00–24:00');
  });

  it('applies only a complete schedule', () => {
    expect(hasOffPeak(deepseek)).toBe(true);
    expect(hasOffPeak({ ratio: 1, peak: deepseek.peak })).toBe(false);
    expect(hasOffPeak({ ratio: 0.5, peak: [] })).toBe(false);
    expect(hasOffPeak(null)).toBe(false);
  });
});
