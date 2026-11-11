import { describe, expect, it } from 'vitest';

import {
  dirtyGroupIds,
  hasDirtyCards,
  matchesQuery,
  normalizeText,
  reportDirty,
  cardIndex,
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

describe('cardIndex', () => {
  // A stand-in for a rendered card slot: its text, its title element and its anchor.
  const slot = (text, { title = null, anchor = null } = {}) => ({
    textContent: text,
    querySelector: (selector) => {
      if (selector === 'h3') return title ? { textContent: ` ${title} ` } : null;
      if (selector === '[data-highlight]') return anchor ? { getAttribute: () => anchor } : null;
      return null;
    }
  });
  const groups = [
    { id: 'notify', label: 'Notifications', to: '/s/notify', cards: [{ id: 'smtp' }, { id: 'telegram' }, { id: 'empty' }] },
    { id: 'general', label: 'General', sectionLabel: 'Organization settings', to: '/o/1/general', cards: [{ id: 'budget' }] }
  ];
  const slots = {
    smtp: slot('Mail sending  SMTP host  Port', { title: 'Mail sending', anchor: 'smtp' }),
    telegram: slot('Telegram bot /start /balance'),
    empty: slot('   '),
    budget: slot('Team budget Period', { title: 'Team budget', anchor: 'team-budget' })
  };

  it('lists every card that renders text, with its title, group, link and text', () => {
    expect(cardIndex(groups, slots)).toEqual([
      {
        id: 'notify:smtp',
        title: 'Mail sending',
        group: 'Notifications',
        to: '/s/notify?highlight=smtp',
        text: 'mail sending smtp host port'
      },
      { id: 'notify:telegram', title: 'Notifications', group: 'Notifications', to: '/s/notify', text: 'telegram bot /start /balance' },
      {
        id: 'general:budget',
        title: 'Team budget',
        group: 'Organization settings · General',
        to: '/o/1/general?highlight=team-budget',
        text: 'team budget period'
      }
    ]);
  });

  it('copes with cards that have not mounted', () => {
    expect(cardIndex(groups, {})).toEqual([]);
    expect(cardIndex(undefined, slots)).toEqual([]);
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
