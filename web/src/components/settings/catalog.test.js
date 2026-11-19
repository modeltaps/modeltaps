import { describe, expect, it } from 'vitest';

import { PLATFORM_GROUPS } from 'views/panel/Setting/registry';
import { OBJECT_PAGES, catalogEntries, catalogEntry, splitFirstSentence } from './catalog';
import { commandFilter, makeCommandFilter, parseSettingFilters, settingFieldValue } from '@/components/chrome/CommandMenu';
import { normalizeSynonyms, packTiers } from '@/components/chrome/commandScore';

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
  const plainRetry = settingFieldValue({ key: 'RetryTimes', scope: 'site' });

  it('parses @ filters and keeps the words', () => {
    expect(parseSettingFilters('@object:channel fee')).toEqual({
      filters: { key: null, object: 'channel', advanced: false, any: true },
      text: 'fee'
    });
  });

  const tiers = (label, keywords = [], description = '') => packTiers({ primary: [label], secondary: keywords, tertiary: [description] });

  it('needs every word somewhere in the entry', () => {
    expect(commandFilter(field, 'top-up fee', tiers('Fee rate', ['top-up'], 'processing'))).toBeGreaterThan(0);
    expect(commandFilter(field, 'fee banana', tiers('Fee rate'))).toBe(0);
  });

  it('ranks a match in the name above keywords, and keywords above the description', () => {
    const name = commandFilter(field, 'fee', tiers('Fee rate', [], 'what the channel charges'));
    const keyword = commandFilter(field, 'surcharge', tiers('Fee rate', ['surcharge'], ''));
    const description = commandFilter(field, 'charges', tiers('Fee rate', [], 'what the channel charges'));
    expect(name).toBeGreaterThan(keyword);
    expect(keyword).toBeGreaterThan(description);
    expect(description).toBeGreaterThan(0);
    expect(commandFilter(field, 'fee rate', tiers('Fee rate'))).toBe(1);
  });

  it('applies @key, @object and @advanced, and ranks advanced settings a little lower', () => {
    expect(commandFilter(field, '@object:channel', [''])).toBe(1);
    expect(commandFilter(field, '@object:model', [''])).toBe(0);
    expect(commandFilter(field, '@key:fee', [''])).toBe(1);
    const plain = settingFieldValue({ key: 'RetryTimes', scope: 'site' });
    expect(commandFilter(advanced, 'retry', tiers('Retry time limit'))).toBeGreaterThan(0);
    expect(commandFilter(advanced, 'retry', tiers('Retry time limit'))).toBeLessThan(commandFilter(plain, 'retry', tiers('Retry time limit')));
    expect(commandFilter(plain, '@advanced retry', tiers('Retries'))).toBe(0);
    expect(commandFilter(advanced, '@advanced retry', tiers('Retry time limit'))).toBeGreaterThan(0);
    // Filters leave out everything that is not a setting.
    expect(commandFilter('Channels providers', '@object:channel', [])).toBe(0);
  });

  it('finds a setting by a synonym from the locale', () => {
    const filter = makeCommandFilter(normalizeSynonyms({ 故障转移: '重试|failover', 'load balancing': 'weight' }));
    expect(filter(plainRetry, '故障转移', tiers('重试次数', ['retry']))).toBeGreaterThan(0);
    expect(filter(plainRetry, '负载', tiers('重试次数'))).toBe(0);
    expect(filter(field, 'load balancing', tiers('Weight'))).toBeGreaterThan(0);
  });
});
