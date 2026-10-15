import { describe, expect, it } from 'vitest';

import { buildModelSuggestions } from './modelSuggestions';
import { typeConfig } from './typeConfig';

const prices = [
  { model: 'gpt-6.1-sol', channel_type: 1 },
  { model: 'claude-opus-5-5', channel_type: 14 },
  { model: 'claude-legacy-x', channel_type: 14 },
  { model: 'gemini-3.8-flash', channel_type: 25 },
  { model: 'gemini-old', channel_type: 25 }
];

describe('buildModelSuggestions', () => {
  it('lists the default models first, then priced models of the type, without duplicates', () => {
    const got = buildModelSuggestions(14, prices);
    const defaults = typeConfig[14].input.models;
    expect(got.slice(0, defaults.length)).toEqual(defaults);
    expect(got.slice(defaults.length)).toEqual(['claude-legacy-x']);
    expect(new Set(got).size).toBe(got.length);
  });

  it('reads the price entries of the type a channel forwards to', () => {
    expect(buildModelSuggestions(3, prices)).toEqual(['gpt-6.1-sol']);
    expect(buildModelSuggestions(55, prices)).toEqual(['gpt-6.1-sol']);
    expect(buildModelSuggestions(61, prices)).toContain('gemini-old');
  });

  it('does not mix another vendor catalog into fixed-model channels', () => {
    // Bedrock serves only what bedrockMap maps; Anthropic API prices are not all reachable there.
    expect(buildModelSuggestions(32, prices)).toEqual(typeConfig[32].input.models);
  });

  it('works without a price list', () => {
    expect(buildModelSuggestions(28)).toEqual(typeConfig[28].input.models);
    expect(buildModelSuggestions(9999, prices)).toEqual([]);
  });
});
