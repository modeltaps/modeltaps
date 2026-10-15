import { describe, expect, it } from 'vitest';

import { CHANNEL_OPTIONS, COMMON_CHANNEL_TYPES, channelTypeGroups } from './ChannelConstants';

const values = (group) => group.options.map((o) => o.value);

describe('channelTypeGroups', () => {
  it('puts the common types first, in their configured order', () => {
    const [common] = channelTypeGroups();
    expect(values(common)).toEqual(COMMON_CHANNEL_TYPES.map(String));
    expect(values(common).at(-1)).toBe('8');
  });

  it('sorts every other type by its displayed name', () => {
    const [, others] = channelTypeGroups();
    const labels = others.options.map((o) => o.label);
    expect(labels).toEqual([...labels].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' })));
  });

  it('lists each channel type exactly once', () => {
    const all = channelTypeGroups().flatMap(values);
    expect(new Set(all).size).toBe(all.length);
    expect(all.sort()).toEqual(Object.keys(CHANNEL_OPTIONS).sort());
  });

  it('sorts by the label function and accepts app language codes', () => {
    // Reversing the names flips the order, so the sort must follow labelOf rather than o.text.
    const reversed = (o) => o.text.split('').reverse().join('');
    const [, others] = channelTypeGroups(reversed, 'zh_CN');
    const labels = others.options.map((o) => o.label);
    expect(labels).toEqual([...labels].sort((a, b) => a.localeCompare(b, 'zh-CN', { sensitivity: 'base' })));
    expect(labels[0]).toBe(reversed(CHANNEL_OPTIONS[Number(others.options[0].value)]));
  });
});
