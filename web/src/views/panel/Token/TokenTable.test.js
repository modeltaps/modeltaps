import { describe, expect, it } from 'vitest';

import { estimateTableWidth, keySuffix, keyState } from './TokenTable';
import { listLayout } from './index';

describe('estimateTableWidth', () => {
  it('adds up the visible columns only', () => {
    const ids = ['select', 'name', 'used_quota', 'status', 'actions'];
    expect(estimateTableWidth(ids, {})).toBe(32 + 180 + 220 + 70 + 56);
    expect(estimateTableWidth(ids, { status: false })).toBe(32 + 180 + 220 + 56);
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
  it('shows the table whenever its natural width fits, the phone list otherwise', () => {
    expect(listLayout(0, 750)).toBe('table');
    expect(listLayout(752, 750)).toBe('table');
    expect(listLayout(751, 750)).toBe('list');
    expect(listLayout(343, 750)).toBe('list');
  });
});
