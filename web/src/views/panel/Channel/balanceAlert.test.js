import { describe, expect, it } from 'vitest';
import { balanceCurrency, balanceSupport, noticeProblems } from './balanceAlert';

const caps = { types: { balance: [1, 20, 28, 62], usage_windows: [58, 59] } };

describe('balanceSupport', () => {
  it('tells balance, usage, official OpenAI and unsupported channels apart', () => {
    expect(balanceSupport({ type: 62 }, caps)).toBe('balance');
    expect(balanceSupport({ type: 58 }, caps)).toBe('usage');
    expect(balanceSupport({ type: 14 }, caps)).toBe('none');
    expect(balanceSupport({ type: 1, base_url: '' }, caps)).toBe('official');
    expect(balanceSupport({ type: 1, base_url: 'https://api.openai.com/' }, caps)).toBe('official');
    expect(balanceSupport({ type: 1, base_url: 'https://relay.example.com' }, caps)).toBe('balance');
    expect(balanceSupport({ type: 62 }, null)).toBeNull();
  });
});

describe('balanceCurrency', () => {
  it('reads DeepSeek and SiliconFlow balances in CNY', () => {
    expect(balanceCurrency(28)).toBe('CNY');
    expect(balanceCurrency(45)).toBe('CNY');
    expect(balanceCurrency(20)).toBe('USD');
  });
});

describe('noticeProblems', () => {
  it('lists what keeps a notice from arriving', () => {
    const ok = { refresh_minutes: 60, kind_on: true, targets: [{ name: 'Email', ready: true }] };
    expect(noticeProblems(ok)).toEqual([]);
    expect(noticeProblems({ ...ok, refresh_minutes: 0 })).toEqual(['noRefresh']);
    expect(noticeProblems({ ...ok, kind_on: false, targets: [{ name: 'Email', ready: false }] })).toEqual(['kindOff', 'noTarget']);
    expect(noticeProblems(null)).toEqual([]);
  });
});
