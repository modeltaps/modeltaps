import { describe, expect, it } from 'vitest';

import { attachDripInteraction } from './dripInteraction';

// The static home page inlines the function's source text, so it must work without its module.
describe('attachDripInteraction', () => {
  it('is self-contained when rebuilt from its source', () => {
    const rebuilt = new Function(`return (${attachDripInteraction.toString()});`)();
    expect(typeof rebuilt).toBe('function');
    expect(typeof rebuilt(null)).toBe('function');
  });

  it('returns a no-op cleanup without a root', () => {
    const cleanup = attachDripInteraction(null);
    expect(() => cleanup()).not.toThrow();
  });

  it('has no closing script tag in its source', () => {
    expect(attachDripInteraction.toString()).not.toMatch(/<\/script/i);
  });
});
