import { describe, expect, it } from 'vitest';

import {
  NO_TAG,
  channelDot,
  channelIssues,
  channelMatches,
  filterChannels,
  groupChannelsByType,
  hasLowBalance,
  openIssues,
  selectionState,
  splitList,
  statusCounts,
  tagOptions
} from './channelGroups';

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

describe('search and filters', () => {
  const list = [
    ch(1, 1, { name: 'openai-main', models: 'gpt-4.1,o3', tag: 'prod', key_status: { masked: ['sk-p****x9Qk'] } }),
    ch(2, 1, { name: 'openai-backup', status: 2, base_url: 'https://proxy.example.com/v1', models: 'gpt-4.1' }),
    ch(3, 1, { name: 'relay', status: 3, tag: 'prod', models: 'o4-mini' })
  ];

  it('matches name, model, tag, key suffix and host', () => {
    const ids = (q) => list.filter((c) => channelMatches(c, q)).map((c) => c.id);
    expect(ids('')).toEqual([1, 2, 3]);
    expect(ids('BACKUP')).toEqual([2]);
    expect(ids('o4-mini')).toEqual([3]);
    expect(ids('prod')).toEqual([1, 3]);
    expect(ids('x9qk')).toEqual([1]);
    expect(ids('proxy.example')).toEqual([2]);
    expect(ids('sk-p')).toEqual([]);
  });

  it('filters by status and tag', () => {
    expect(filterChannels(list, { status: 'manual' }).map((c) => c.id)).toEqual([2]);
    expect(filterChannels(list, { tag: 'prod' }).map((c) => c.id)).toEqual([1, 3]);
    expect(filterChannels(list, { tag: NO_TAG }).map((c) => c.id)).toEqual([2]);
    expect(filterChannels(list, { status: 'auto', tag: 'prod' }).map((c) => c.id)).toEqual([3]);
  });

  it('counts statuses within the tag filter and lists tags', () => {
    expect(statusCounts(list)).toEqual({ all: 3, enabled: 1, manual: 1, auto: 1 });
    expect(statusCounts(list, 'prod')).toEqual({ all: 2, enabled: 1, manual: 0, auto: 1 });
    expect(tagOptions(list)).toEqual({ tags: ['prod'], hasUntagged: true });
  });
});

describe('selectionState', () => {
  it('reports none, some or all of the visible rows', () => {
    expect(selectionState([1, 2], new Set())).toBe('none');
    expect(selectionState([1, 2], new Set([2, 9]))).toBe('some');
    expect(selectionState([1, 2], new Set([1, 2]))).toBe('all');
    expect(selectionState([], new Set([1]))).toBe('none');
  });
});
