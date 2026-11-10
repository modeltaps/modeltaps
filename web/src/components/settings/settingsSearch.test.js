import { describe, expect, it } from 'vitest';

import {
  dirtyGroupIds,
  hasDirtyCards,
  matchesQuery,
  normalizeText,
  reportDirty,
  searchCards,
  shouldShowSearch,
  staysOnPage
} from './settingsSearch';

describe('matchesQuery', () => {
  it('ignores case and extra whitespace', () => {
    expect(normalizeText('  SMTP   Server ')).toBe('smtp server');
    expect(matchesQuery('SMTP 服务器地址', 'smtp')).toBe(true);
    expect(matchesQuery('Retry times', '  RETRY  ')).toBe(true);
  });

  it('needs every token, in any order', () => {
    expect(matchesQuery('Channel auto disable and recovery', 'recovery channel')).toBe(true);
    expect(matchesQuery('Channel auto disable and recovery', 'channel webhook')).toBe(false);
  });

  it('an empty query matches; empty text never does', () => {
    expect(matchesQuery('anything', '   ')).toBe(true);
    expect(matchesQuery('', 'x')).toBe(false);
  });
});

describe('searchCards', () => {
  const entries = [
    { cardId: 'smtp', groupId: 'notify', text: '邮件发送 SMTP 服务器 端口' },
    { cardId: 'telegram', groupId: 'notify', text: 'Telegram 机器人 /start 余额查询' },
    { cardId: 'retry', groupId: 'gateway', text: '重试 次数 冷却' },
    // A card that renders nothing (e.g. not applicable to the account system) never matches.
    { cardId: 'hidden', groupId: 'gateway', text: '' }
  ];

  it('returns matches across groups and counts per group', () => {
    const { matches, counts } = searchCards(entries, '/start');
    expect([...matches]).toEqual(['telegram']);
    expect(counts).toEqual({ notify: 1, gateway: 0 });
  });

  it('matches collection rows like any other text', () => {
    expect([...searchCards(entries, '余额').matches]).toEqual(['telegram']);
    expect(searchCards(entries, 'smtp 端口').counts.notify).toBe(1);
  });

  it('an empty query matches every non-empty card', () => {
    expect([...searchCards(entries, '').matches]).toEqual(['smtp', 'telegram', 'retry']);
  });
});

describe('dirty tracking', () => {
  it('adds and removes cards, returning the same object when nothing changed', () => {
    const a = reportDirty({}, 'smtp', 'notify', true);
    expect(a).toEqual({ smtp: 'notify' });
    expect(reportDirty(a, 'smtp', 'notify', true)).toBe(a);
    const b = reportDirty(a, 'retry', 'gateway', true);
    expect(dirtyGroupIds(b)).toEqual(new Set(['notify', 'gateway']));
    const c = reportDirty(b, 'smtp', 'notify', false);
    expect(c).toEqual({ retry: 'gateway' });
    expect(reportDirty(c, 'smtp', 'notify', false)).toBe(c);
    expect(hasDirtyCards(c)).toBe(true);
    expect(hasDirtyCards(reportDirty(c, 'retry', 'gateway', false))).toBe(false);
  });

  it('two dirty cards in one group give one dot', () => {
    expect(dirtyGroupIds({ smtp: 'notify', 'mj-notify': 'notify' })).toEqual(new Set(['notify']));
  });
});

describe('page helpers', () => {
  it('shows the search box from four groups on', () => {
    expect(shouldShowSearch([{}, {}, {}])).toBe(false);
    expect(shouldShowSearch([{}, {}, {}, {}])).toBe(true);
  });

  it('group switches stay on the page; other routes leave it', () => {
    const paths = ['/panel/setting/site', '/panel/setting/notify'];
    expect(staysOnPage('/panel/setting/notify', paths)).toBe(true);
    expect(staysOnPage('/panel/setting/notify/', paths)).toBe(true);
    expect(staysOnPage('/panel/channel', paths)).toBe(false);
    expect(staysOnPage('/panel/settings/org/3/members', ['/panel/settings/org/3/general'])).toBe(false);
  });
});
