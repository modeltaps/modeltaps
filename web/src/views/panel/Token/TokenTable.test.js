import { describe, expect, it } from 'vitest';

import { fitColumns, keySuffix, keyState } from './TokenTable';
import { listLayout } from './index';

const ids = ['select', 'name', 'group', 'expired_time', 'accessed_time', 'used_quota', 'created_time', 'status', 'actions'];
const all = Object.fromEntries(ids.map((id) => [id, true]));

describe('fitColumns', () => {
  it('keeps every picked column when the table is wide enough', () => {
    expect(fitColumns(all, ids, 2000)).toEqual(all);
  });

  it('hides columns in priority order as the table narrows', () => {
    const hidden = (w) => ids.filter((id) => fitColumns(all, ids, w)[id] === false);
    // All nine columns add up to 954px.
    expect(hidden(954)).toEqual([]);
    expect(hidden(940)).toEqual(['group']);
    expect(hidden(840)).toEqual(['group', 'created_time']);
    expect(hidden(700)).toEqual(['group', 'expired_time', 'created_time']);
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

describe('keyState', () => {
  const now = 1_000_000;
  const key = (over) => ({ status: 1, expired_time: -1, ...over });
  it('merges status and expiry into one state', () => {
    expect(keyState(key({}), now).key).toBe('enabled');
    expect(keyState(key({ status: 2 }), now).key).toBe('disabled');
    expect(keyState(key({ status: 4 }), now).key).toBe('exhausted');
    expect(keyState(key({ status: 3 }), now).key).toBe('expired');
    expect(keyState(key({ expired_time: now - 1 }), now).key).toBe('expired');
    expect(keyState(key({ expired_time: now + 3 * 86400 }), now).key).toBe('expiring');
    expect(keyState(key({ expired_time: now + 30 * 86400 }), now).key).toBe('enabled');
  });
});

describe('listLayout', () => {
  it('picks the layout from the list width', () => {
    expect(listLayout(375)).toBe('narrow');
    expect(listLayout(599)).toBe('narrow');
    expect(listLayout(600)).toBe('medium');
    expect(listLayout(899)).toBe('medium');
    expect(listLayout(900)).toBe('wide');
  });
});
