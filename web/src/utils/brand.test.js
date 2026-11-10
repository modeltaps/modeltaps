import { describe, expect, it } from 'vitest';
import { brandName, brandWordmark } from './brand';

describe('brandWordmark', () => {
  it('lowercases the default brand', () => {
    expect(brandWordmark('Modeltaps')).toBe('modeltaps');
  });

  it('keeps a custom system name as typed', () => {
    expect(brandWordmark('Acme AI')).toBe('Acme AI');
    expect(brandWordmark('MODELTAPS')).toBe('MODELTAPS');
  });

  it('leaves the running-text name capitalised', () => {
    expect(brandName('Modeltaps')).toBe('Modeltaps');
  });
});
