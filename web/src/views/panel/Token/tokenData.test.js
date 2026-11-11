import { describe, expect, it } from 'vitest';

import { EXPIRY_PRESETS, expiryFromPreset } from './tokenData';

const at = (iso) => new Date(iso);
const unix = (iso) => Math.floor(new Date(iso).getTime() / 1000);

describe('expiryFromPreset', () => {
  it('offers never, a day, a week and a month', () => {
    expect(EXPIRY_PRESETS).toEqual(['never', 'day', 'week', 'month']);
  });

  it('counts from now', () => {
    const now = at('2026-11-11T10:00:00');
    expect(expiryFromPreset('never', now)).toBe(-1);
    expect(expiryFromPreset('day', now)).toBe(unix('2026-11-12T10:00:00'));
    expect(expiryFromPreset('week', now)).toBe(unix('2026-11-18T10:00:00'));
    expect(expiryFromPreset('month', now)).toBe(unix('2026-12-11T10:00:00'));
  });

  it('keeps a month on the last day of a shorter next month', () => {
    expect(expiryFromPreset('month', at('2027-01-31T09:00:00'))).toBe(unix('2027-02-28T09:00:00'));
  });
});
