import { describe, expect, it } from 'vitest';

import { effectiveUsageTiers, formatTiers, parseUsageTiers, usageSource } from './usageAlert';

describe('usage alert tiers', () => {
  it('reads tiers the way the server stores them', () => {
    expect(parseUsageTiers('')).toEqual({ value: '' });
    expect(parseUsageTiers(' 0 ')).toEqual({ value: '0' });
    expect(parseUsageTiers('off')).toEqual({ value: '0' });
    expect(parseUsageTiers('100, 50,75')).toEqual({ value: '50,75,100' });
    expect(parseUsageTiers('50%，80% 80')).toEqual({ value: '50,80' });
    for (const bad of ['abc', '0,50', '150', '-5', ',']) expect(parseUsageTiers(bad).error).toBe(true);
  });

  it('prefers the channel tiers over the site tiers', () => {
    expect(effectiveUsageTiers('30,60', [50])).toEqual({ tiers: [30, 60], source: 'channel' });
    expect(effectiveUsageTiers('0', [50])).toEqual({ tiers: [], source: 'channel' });
    expect(effectiveUsageTiers('', [90])).toEqual({ tiers: [90], source: 'site' });
    expect(effectiveUsageTiers('', [])).toEqual({ tiers: [], source: 'site' });
    expect(effectiveUsageTiers('', null)).toEqual({ tiers: [50, 75, 100], source: 'site' });
    expect(formatTiers([50, 75])).toBe('50%, 75%');
  });

  it('knows that Claude Code reports usage on its responses', () => {
    expect(usageSource(58)).toBe('headers');
    expect(usageSource('59')).toBe('api');
  });
});
