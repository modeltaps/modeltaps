import { describe, expect, it } from 'vitest';

import { CHANNEL_OPTIONS, CHANNEL_TYPE_CATEGORIES, COMMON_CHANNEL_TYPES, channelTypeAuth, channelTypeSections } from './ChannelConstants';

const values = (section) => section.options.map((o) => o.value);

describe('channelTypeSections', () => {
  it('puts the common types first, in their configured order', () => {
    const [common] = channelTypeSections();
    expect(common.key).toBe('common');
    expect(values(common)).toEqual(COMMON_CHANNEL_TYPES.map(String));
  });

  it('files every channel type under exactly one category', () => {
    const categorized = channelTypeSections()
      .filter((s) => s.key !== 'common')
      .flatMap(values);
    expect(new Set(categorized).size).toBe(categorized.length);
    expect([...categorized].sort()).toEqual(Object.keys(CHANNEL_OPTIONS).sort());
  });

  it('keeps every category type known to CHANNEL_OPTIONS, so no section shows a dead entry', () => {
    CHANNEL_TYPE_CATEGORIES.flatMap((c) => c.types).forEach((id) => expect(CHANNEL_OPTIONS[id]).toBeDefined());
  });

  it('keeps the curated order inside a category and labels through labelOf', () => {
    const sections = channelTypeSections((o) => `#${o.text}`);
    const subscription = sections.find((s) => s.key === 'subscription');
    expect(values(subscription)).toEqual(['58', '59', '57', '60']);
    expect(subscription.options[0].label).toBe(`#${CHANNEL_OPTIONS[58].text}`);
  });

  it('ends with the legacy types', () => {
    expect(channelTypeSections().at(-1).key).toBe('legacy');
  });
});

describe('channelTypeAuth', () => {
  it('tells OAuth, endpoint, multi-part credential and plain key providers apart', () => {
    expect(channelTypeAuth(58)).toBe('oauth');
    expect(channelTypeAuth('3')).toBe('endpoint');
    expect(channelTypeAuth(32)).toBe('credentials');
    expect(channelTypeAuth(1)).toBe('apiKey');
  });
});
