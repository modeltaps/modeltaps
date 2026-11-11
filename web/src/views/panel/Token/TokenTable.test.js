import { describe, expect, it } from 'vitest';

import { fitColumns, keySuffix } from './TokenTable';

const ids = ['select', 'name', 'group', 'expired_time', 'accessed_time', 'used_quota', 'created_time', 'status', 'actions'];
const all = Object.fromEntries(ids.map((id) => [id, true]));

describe('fitColumns', () => {
  it('keeps every picked column when the table is wide enough', () => {
    expect(fitColumns(all, ids, 2000)).toEqual(all);
  });

  it('hides columns in priority order as the table narrows', () => {
    const hidden = (w) => ids.filter((id) => fitColumns(all, ids, w)[id] === false);
    // All nine columns add up to 1126px.
    expect(hidden(1126)).toEqual([]);
    expect(hidden(1100)).toEqual(['group']);
    expect(hidden(950)).toEqual(['group', 'expired_time']);
    expect(hidden(850)).toEqual(['group', 'expired_time', 'created_time']);
    expect(hidden(500)).toEqual(['group', 'expired_time', 'accessed_time', 'created_time']);
  });

  it('never hides the name, usage, status or actions, and leaves columns the user hid alone', () => {
    const picked = { ...all, accessed_time: false };
    const fitted = fitColumns(picked, ids, 100);
    for (const id of ['select', 'name', 'used_quota', 'status', 'actions']) expect(fitted[id]).toBe(true);
    expect(fitted.accessed_time).toBe(false);
  });

  it('changes nothing before the width is known', () => {
    expect(fitColumns(all, ids, 0)).toEqual(all);
  });
});

describe('keySuffix', () => {
  it('shows the prefix and the last four characters', () => {
    expect(keySuffix('abcdefghRc6g')).toBe('sk-…Rc6g');
    expect(keySuffix('ab')).toBe('sk-ab');
  });
});
