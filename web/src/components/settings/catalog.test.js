import { describe, expect, it } from 'vitest';

import { PLATFORM_GROUPS } from 'views/panel/Setting/registry';
import { OBJECT_PAGES, catalogEntries, catalogEntry, splitFirstSentence } from './catalog';
import { commandFilter, parseSettingFilters, settingFieldValue } from '@/components/chrome/CommandMenu';

describe('settings catalog', () => {
  it('places every site setting on the card the settings page shows it on', () => {
    const cardOf = {};
    for (const group of PLATFORM_GROUPS) for (const card of group.cards) for (const key of card.keys) cardOf[key] = [group.id, card.id];
    const wrong = catalogEntries()
      .filter((e) => e.scope === 'site' && cardOf[e.key] && (cardOf[e.key][0] !== e.group || cardOf[e.key][1] !== e.card))
      .map((e) => `${e.key}: catalog ${e.group}/${e.card}, page ${cardOf[e.key].join('/')}`);
    expect(wrong).toEqual([]);
  });

  it('every card key of the settings page is in the catalog', () => {
    const keys = PLATFORM_GROUPS.flatMap((g) => g.cards.flatMap((c) => c.keys));
    expect(keys.filter((key) => !catalogEntry(key))).toEqual([]);
  });

  it('object fields link to a page', () => {
    const missing = catalogEntries()
      .filter((e) => e.scope !== 'site' && !OBJECT_PAGES[e.group])
      .map((e) => e.key);
    expect(missing).toEqual([]);
  });

  it('splits the first sentence off a description', () => {
    expect(splitFirstSentence('Does one thing. Then more.')).toEqual({ first: 'Does one thing.', rest: 'Then more.' });
    expect(splitFirstSentence('倍率 1.12 表示加价。其余说明。')).toEqual({ first: '倍率 1.12 表示加价。', rest: '其余说明。' });
    expect(splitFirstSentence('Use e.g. 1.12 here. Rest.')).toEqual({ first: 'Use e.g. 1.12 here.', rest: 'Rest.' });
    expect(splitFirstSentence('One sentence only')).toEqual({ first: 'One sentence only', rest: '' });
  });
});

describe('palette filters for settings', () => {
  const field = settingFieldValue({ key: 'channel.fee_rate', scope: 'channel' });
  const advanced = settingFieldValue({ key: 'RetryTimeOut', scope: 'site', advanced: true });

  it('parses @ filters and keeps the words', () => {
    expect(parseSettingFilters('@object:channel fee')).toEqual({
      filters: { key: null, object: 'channel', advanced: false, any: true },
      text: 'fee'
    });
  });

  it('matches every word against the search text', () => {
    expect(commandFilter(field, 'top-up fee', ['fee rate top-up processing'])).toBe(1);
    expect(commandFilter(field, 'fee banana', ['fee rate'])).toBe(0);
  });

  it('applies @key, @object and @advanced', () => {
    expect(commandFilter(field, '@object:channel', [''])).toBe(1);
    expect(commandFilter(field, '@object:model', [''])).toBe(0);
    expect(commandFilter(field, '@key:fee', [''])).toBe(1);
    expect(commandFilter(advanced, 'retry', ['retry timeout'])).toBe(0);
    expect(commandFilter(advanced, '@advanced retry', ['retry timeout'])).toBe(1);
    // Filters leave out everything that is not a setting.
    expect(commandFilter('Channels providers', '@object:channel', [])).toBe(0);
  });
});
