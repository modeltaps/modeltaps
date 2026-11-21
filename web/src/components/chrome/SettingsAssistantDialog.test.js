import { describe, expect, it } from 'vitest';

import { defaultAssistantModel, recommendedModels } from './SettingsAssistantDialog';

const names = [
  'Llama-3.3-70B',
  'claude-haiku-4.5',
  'claude-sonnet-4-6',
  'deepseek-chat',
  'gemini-2.5-flash',
  'gemini-2.5-flash-lite',
  'gpt-5-mini'
];

describe('settings assistant models', () => {
  it('recommends the picked model first, then the newest of each family, strongest first', () => {
    expect(recommendedModels(names, 'Llama-3.3-70B')).toEqual([
      'Llama-3.3-70B',
      'claude-sonnet-4-6',
      'claude-haiku-4.5',
      'gpt-5-mini',
      'gemini-2.5-flash'
    ]);
    expect(recommendedModels(names, 'gone-model')[0]).toBe('claude-sonnet-4-6');
    // The picked model's family adds no second model.
    expect(recommendedModels([...names, 'claude-haiku-5-5'], 'claude-haiku-5-5')).toEqual([
      'claude-haiku-5-5',
      'claude-sonnet-4-6',
      'gpt-5-mini',
      'gemini-2.5-flash',
      'deepseek-chat'
    ]);
    // The newest version of a family, and the full model before its fast variants.
    expect(recommendedModels(['gpt-5-mini', 'gpt-5', 'gpt-5.1', 'gpt-5-codex', 'gemini-2.5-pro', 'gemini-3-pro'], null)).toEqual([
      'gpt-5.1',
      'gemini-3-pro',
      'gpt-5-mini'
    ]);
    expect(recommendedModels(['a', 'b'], null)).toEqual([]);
  });

  it('defaults to a preferred model, else the first', () => {
    expect(defaultAssistantModel(names)).toBe('claude-sonnet-4-6');
    expect(defaultAssistantModel(['claude-sonnet-4-5', 'claude-sonnet-5-5', 'gpt-5'])).toBe('claude-sonnet-5-5');
    expect(defaultAssistantModel(['a', 'b'])).toBe('a');
    expect(defaultAssistantModel([])).toBe('');
  });
});
