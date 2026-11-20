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
  it('recommends the last model used first, then a fast one of each family', () => {
    expect(recommendedModels(names, 'Llama-3.3-70B')).toEqual([
      'Llama-3.3-70B',
      'claude-haiku-4.5',
      'gpt-5-mini',
      'gemini-2.5-flash',
      'claude-sonnet-4-6'
    ]);
    expect(recommendedModels(names, 'gone-model')[0]).toBe('claude-haiku-4.5');
    // The last model's family adds no second model.
    expect(recommendedModels([...names, 'claude-haiku-5-5'], 'claude-haiku-5-5')).toEqual([
      'claude-haiku-5-5',
      'gpt-5-mini',
      'gemini-2.5-flash',
      'claude-sonnet-4-6',
      'deepseek-chat'
    ]);
    expect(recommendedModels(['a', 'b'], null)).toEqual([]);
  });

  it('defaults to a preferred model, else the first', () => {
    expect(defaultAssistantModel(names)).toBe('claude-haiku-4.5');
    expect(defaultAssistantModel(['a', 'b'])).toBe('a');
    expect(defaultAssistantModel([])).toBe('');
  });
});
