import { describe, expect, it } from 'vitest';

import { channelDot, channelIssues, groupChannelsByType, hasLowBalance, openIssues, splitList } from './channelGroups';

const ch = (id, type, extra = {}) => ({ id, type, status: 1, priority: 0, weight: 1, ...extra });

describe('groupChannelsByType', () => {
  it('groups by type, larger providers first, rows by priority then weight', () => {
    const groups = groupChannelsByType([
      ch(1, 14),
      ch(2, 1, { priority: 1 }),
      ch(3, 1, { priority: 5 }),
      ch(4, 1, { priority: 5, weight: 3 }),
      ch(5, 25)
    ]);
    expect(groups.map((g) => g.type)).toEqual([1, 14, 25]);
    expect(groups[0].channels.map((c) => c.id)).toEqual([4, 3, 2]);
  });

  it('returns nothing for no channels', () => {
    expect(groupChannelsByType([])).toEqual([]);
  });
});

describe('channelIssues', () => {
  it('lists auto-disable, missing key, low balance and drift', () => {
    const c = ch(7, 1, {
      status: 3,
      key_status: { configured: false },
      balance: 10,
      balance_threshold: 50,
      balance_updated_time: 100,
      model_drift: { new_models: ['a', 'b'], missing_models: ['c'] }
    });
    const issues = channelIssues(c);
    expect(issues.map((i) => i.kind)).toEqual(['autoDisabled', 'keyMissing', 'lowBalance', 'driftNew', 'driftMissing']);
    expect(issues[3].count).toBe(2);
  });

  it('has none for a healthy channel', () => {
    expect(channelIssues(ch(1, 1, { key_status: { configured: true } }))).toEqual([]);
  });

  it('ignores the balance of subscription channels and unread balances', () => {
    expect(hasLowBalance(ch(1, 1, { balance_threshold: 5, balance_updated_time: 0 }))).toBe(false);
    expect(hasLowBalance(ch(1, 1, { balance_threshold: 5, balance_updated_time: 1, usage_windows: { windows: [{}] } }))).toBe(false);
  });

  it('drops dismissed items and brings them back when the data changes', () => {
    const c = ch(2, 1, { model_drift: { new_models: ['a'] } });
    const [issue] = channelIssues(c);
    expect(openIssues([c], new Set([issue.key]))).toEqual([]);
    const changed = { ...c, model_drift: { new_models: ['a', 'b'] } };
    expect(openIssues([changed], new Set([issue.key]))).toHaveLength(1);
  });
});

describe('channelDot', () => {
  it('maps status and issues to a dot', () => {
    expect(channelDot(ch(1, 1))).toBe('live');
    expect(channelDot(ch(1, 1, { status: 2 }))).toBe('off');
    expect(channelDot(ch(1, 1, { status: 3 }))).toBe('warn');
    expect(channelDot(ch(1, 1, { key_status: { configured: false } }))).toBe('warn');
  });
});

describe('splitList', () => {
  it('trims and de-duplicates', () => {
    expect(splitList(' a, b,,a\nc ')).toEqual(['a', 'b', 'c']);
    expect(splitList(null)).toEqual([]);
  });
});
