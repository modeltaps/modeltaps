import { describe, expect, it } from 'vitest';
import { matchesSearch, normalizeSearchText, searchTerms } from './textSearch';

describe('textSearch', () => {
  it('ignores case, spaces and separators', () => {
    ['gpt mini', 'GPT-5-Mini', 'gpt5mini', 'gpt  mini', 'ＧＰＴ mini', 'gpt-5 mini'].forEach((q) =>
      expect(matchesSearch(q, 'gpt-5-mini')).toBe(true)
    );
    expect(matchesSearch('gpt mini', 'gpt-4o-mini-2024-07-18')).toBe(true);
    expect(matchesSearch('gpt mini', 'gpt-4o')).toBe(false);
  });

  it('lets each word match a different field', () => {
    expect(matchesSearch('openai mini', ['o4-mini', 'OpenAI'])).toBe(true);
    expect(matchesSearch('anthropic mini', ['o4-mini', 'OpenAI'])).toBe(false);
  });

  it('matches everything on a blank query and nothing on empty fields', () => {
    expect(matchesSearch('  ', 'x')).toBe(true);
    expect(matchesSearch('x', [null, undefined, ''])).toBe(false);
  });

  it('accepts pre-split terms', () => {
    expect(matchesSearch(searchTerms('claude 4.5'), 'claude-sonnet-4-5')).toBe(true);
    expect(normalizeSearchText('Claude Sonnet 4.5')).toBe('claudesonnet45');
  });
});
