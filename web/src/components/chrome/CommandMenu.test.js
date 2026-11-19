import { describe, expect, it } from 'vitest';

import { commandFilter } from './CommandMenu';
import { packTiers } from './commandScore';

// "On this page" entries carry a settings card's title, group and whole text; they match on every
// word of the query, ranked by where it is found (fuzzy matching over a whole card would match
// almost anything). Menu entries keep fuzzy matching for Latin queries, at a low score.

describe('commandFilter', () => {
  const card = 'page-card:notify:smtp';
  const text = packTiers({ primary: ['Mail sending'], secondary: ['Notifications'], tertiary: ['mail sending smtp host port 465 tls'] });

  it('matches a card when every word of the query is in its title, group or text', () => {
    expect(commandFilter(card, 'smtp', text)).toBeGreaterThan(0);
    expect(commandFilter(card, 'port 465', text)).toBeGreaterThan(0);
    expect(commandFilter(card, 'NOTIFICATIONS', text)).toBeGreaterThan(0);
  });

  it('ranks a card by its title above a card that only mentions the word', () => {
    const other = packTiers({ primary: ['Branding'], tertiary: ['icon cache and mail logo'] });
    expect(commandFilter(card, 'mail', text)).toBeGreaterThan(commandFilter('page-card:site:branding', 'mail', other));
  });

  it('does not match a card on scattered letters', () => {
    expect(commandFilter(card, 'stp', text)).toBe(0);
    expect(commandFilter(card, 'smtp webhook', text)).toBe(0);
  });

  it('keeps fuzzy matching for menu entries, below a word match', () => {
    expect(commandFilter('Channels channel', 'chnl')).toBeGreaterThan(0);
    expect(commandFilter('Channels channel', 'chnl')).toBeLessThan(commandFilter('Channels channel', 'chan'));
    expect(commandFilter('Channels channel', 'xyz')).toBe(0);
    expect(commandFilter('渠道 channels', '渠')).toBeGreaterThan(0);
    expect(commandFilter('渠道 channels', '道渠')).toBe(0);
  });
});
