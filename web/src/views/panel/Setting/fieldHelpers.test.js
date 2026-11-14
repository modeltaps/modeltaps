import { describe, expect, it } from 'vitest';

import { splitLabelUnit } from './parts';
import { parseStatusCooldown, serializeStatusCooldown } from './cards/GatewayCards';

describe('splitLabelUnit', () => {
  it('moves a unit in brackets out of the label', () => {
    expect(splitLabelUnit('重试间隔(秒)')).toEqual({ name: '重试间隔', unit: '秒' });
    expect(splitLabelUnit('测试间隔（分钟）')).toEqual({ name: '测试间隔', unit: '分钟' });
    expect(splitLabelUnit('Cooldown (s)')).toEqual({ name: 'Cooldown', unit: 's' });
    expect(splitLabelUnit('Test interval (minutes)')).toEqual({ name: 'Test interval', unit: 'minutes' });
  });

  it('leaves plurals and other brackets alone', () => {
    expect(splitLabelUnit('model(s)')).toEqual({ name: 'model(s)', unit: null });
    expect(splitLabelUnit('标识 (slug)')).toEqual({ name: '标识 (slug)', unit: null });
    expect(splitLabelUnit('服务器地址')).toEqual({ name: '服务器地址', unit: null });
  });
});

describe('status cooldown rows', () => {
  it('round-trips JSON and rows', () => {
    const rows = parseStatusCooldown('{"503": 120, "429": 30}');
    expect(rows).toEqual([
      { code: '429', seconds: '30' },
      { code: '503', seconds: '120' }
    ]);
    expect(JSON.parse(serializeStatusCooldown(rows))).toEqual({ 429: 30, 503: 120 });
  });

  it('treats empty as no rows and drops rows without a code', () => {
    expect(parseStatusCooldown('')).toEqual([]);
    expect(serializeStatusCooldown([{ code: ' ', seconds: '5' }])).toBe('');
  });

  it('returns null for text that is not a JSON object', () => {
    expect(parseStatusCooldown('{oops')).toBeNull();
    expect(parseStatusCooldown('[1,2]')).toBeNull();
  });
});
