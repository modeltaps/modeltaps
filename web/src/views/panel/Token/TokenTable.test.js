// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';

import { keySuffix, keyState } from './TokenTable';

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

