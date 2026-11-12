import { describe, expect, it } from 'vitest';

import { relayHost } from './providerDefaults';

describe('relayHost', () => {
  it('shows the host only when it is not the type’s own address', () => {
    expect(relayHost({ type: 1, base_url: 'https://api.venice.ai/api/v1' })).toBe('api.venice.ai');
    expect(relayHost({ type: 1, base_url: 'https://api.openai.com' })).toBe('');
    expect(relayHost({ type: 1, base_url: '' })).toBe('');
    expect(relayHost({ type: 62 })).toBe('');
  });
});
