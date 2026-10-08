// ==============================|| PLAYGROUND TOKEN — name predicate ||============================== //

import { describe, it, expect } from 'vitest';
import { PLAYGROUND_TOKEN_NAME, isPlaygroundToken } from './playgroundToken.js';

describe('isPlaygroundToken', () => {
  it('matches the backend-created playground token name', () => {
    expect(isPlaygroundToken({ name: PLAYGROUND_TOKEN_NAME })).toBe(true);
  });

  it('rejects other names, including near misses', () => {
    expect(isPlaygroundToken({ name: 'playground' })).toBe(false);
    expect(isPlaygroundToken({ name: 'sys_playground_2' })).toBe(false);
    expect(isPlaygroundToken({ name: 'Sys_Playground' })).toBe(false);
    expect(isPlaygroundToken({ name: '' })).toBe(false);
  });

  it('tolerates missing tokens and missing names', () => {
    expect(isPlaygroundToken(null)).toBe(false);
    expect(isPlaygroundToken(undefined)).toBe(false);
    expect(isPlaygroundToken({})).toBe(false);
  });
});
