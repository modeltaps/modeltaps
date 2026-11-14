import { describe, expect, it } from 'vitest';

import { allowlistCovers, allowlistPermits, filterPermitted, uncoveredEntries } from './modelAllowlist';

describe('modelAllowlist', () => {
  it('matches like the backend', () => {
    expect(allowlistPermits([], 'anything')).toBe(true);
    expect(allowlistPermits(['GPT-4o'], 'gpt-4O')).toBe(true);
    expect(allowlistPermits(['claude-fable-*'], 'claude-fable-5-2')).toBe(true);
    expect(allowlistPermits(['claude-fable-*'], 'gpt-4o')).toBe(false);
  });

  it('checks that a lower layer stays inside the one above', () => {
    expect(allowlistCovers(['claude-*'], 'claude-fable-*')).toBe(true);
    expect(allowlistCovers(['claude-fable-*'], 'claude-*')).toBe(false);
    expect(allowlistCovers(['claude-fable-5-1'], 'claude-fable-*')).toBe(false);
    expect(allowlistCovers([], 'x-*')).toBe(true);
    expect(uncoveredEntries([['gpt-*'], ['gpt-4o', 'o3']], ['gpt-4o', 'gpt-5', 'o3'])).toEqual(['gpt-5', 'o3']);
    expect(filterPermitted([['gpt-*'], []], ['gpt-4o', 'claude-x'])).toEqual(['gpt-4o']);
  });
});
