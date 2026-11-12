import { describe, expect, it } from 'vitest';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { OAUTH_TYPES, keyPageUrl } from './providerDefaults';

// Types without a key page of their own: a custom endpoint, or services reached through a
// proxy or a workspace whose address the user brings.
const NO_KEY_PAGE = new Set([8, 18, 19, 34, 41, 54]);

describe('keyPageUrl', () => {
  it('links every key-based type to an https page', () => {
    for (const type of Object.keys(CHANNEL_OPTIONS).map(Number)) {
      if (OAUTH_TYPES.has(type) || NO_KEY_PAGE.has(type)) continue;
      expect(keyPageUrl(type), `type ${type}`).toMatch(/^https:\/\/\S+$/);
    }
  });

  it('opens the key page, not the product page', () => {
    expect(keyPageUrl(62)).toContain('ai-gateway%2Fapi-keys');
    expect(keyPageUrl(1)).toBe('https://platform.openai.com/api-keys');
  });

  it('is empty for login-based and custom types', () => {
    expect(keyPageUrl(58)).toBe('');
    expect(keyPageUrl(8)).toBe('');
  });
});
