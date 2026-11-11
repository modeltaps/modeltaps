import { describe, expect, it } from 'vitest';

import { commandFilter } from './CommandMenu';

// "On this page" entries carry a settings card's whole text; they match on every word of the query
// (fuzzy matching over a whole card would match almost anything). Menu entries stay fuzzy.

describe('commandFilter', () => {
  const card = 'page-card:notify:smtp Mail sending Notifications';
  const text = ['mail sending smtp host port 465 tls'];

  it('matches a card when every word of the query is in its title, group or text', () => {
    expect(commandFilter(card, 'smtp', text)).toBe(1);
    expect(commandFilter(card, 'port 465', text)).toBe(1);
    expect(commandFilter(card, 'NOTIFICATIONS', text)).toBe(1);
  });

  it('does not match a card on scattered letters', () => {
    expect(commandFilter(card, 'stp', text)).toBe(0);
    expect(commandFilter(card, 'smtp webhook', text)).toBe(0);
  });

  it('keeps fuzzy matching for menu entries', () => {
    expect(commandFilter('Channels channel', 'chnl')).toBeGreaterThan(0);
    expect(commandFilter('Channels channel', 'xyz')).toBe(0);
  });
});
