import { describe, expect, it } from 'vitest';

import { pageOf } from './LongList';

describe('pageOf', () => {
  it('slices one page of rows', () => {
    expect(pageOf(511, 0)).toEqual({ page: 0, pages: 11, start: 0, end: 50 });
    expect(pageOf(511, 10)).toEqual({ page: 10, pages: 11, start: 500, end: 511 });
  });
  it('keeps the page inside the pages there are', () => {
    expect(pageOf(60, 5)).toEqual({ page: 1, pages: 2, start: 50, end: 60 });
    expect(pageOf(0, 3)).toEqual({ page: 0, pages: 1, start: 0, end: 0 });
    expect(pageOf(10, -1).page).toBe(0);
  });
});
