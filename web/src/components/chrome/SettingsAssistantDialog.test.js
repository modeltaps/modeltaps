import { describe, expect, it } from 'vitest';

import { defaultAssistantModel, recommendedModels, stepText } from './SettingsAssistantDialog';

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
    // Of one version the plain name, not a fast variant or a dated snapshot; no version comes last.
    expect(defaultAssistantModel(['claude-sonnet-5-5-fast', 'claude-sonnet-5-5-20261001', 'claude-sonnet-5-5'])).toBe('claude-sonnet-5-5');
    expect(recommendedModels(['gpt-5.6-terra-fast', 'gpt-5.6', 'gemini-pro-latest', 'gemini-3-pro'], null)).toEqual([
      'gpt-5.6',
      'gemini-3-pro'
    ]);
    expect(defaultAssistantModel(['a', 'b'])).toBe('a');
    expect(defaultAssistantModel([])).toBe('');
  });
});

describe('stepText', () => {
  const t = (key, values) => `${key}${values ? ` ${JSON.stringify(values)}` : ''}`;
  const describeKey = (key) => ({ RetryTimes: { label: '重试次数' }, RetryCooldownSeconds: { label: '限流冷却' } })[key] || null;
  it('says each tool call in words, with the console names of the settings', () => {
    expect(stepText(t, { name: 'settings_search', args: '{"query":"retry 429"}' }, describeKey, 'zh_CN')).toBe(
      'settingsAssistant.step.search {"query":"retry 429"}'
    );
    expect(stepText(t, { name: 'settings_search', args: '{"modified_only":true}' }, describeKey, 'zh_CN')).toBe(
      'settingsAssistant.step.modified'
    );
    expect(
      stepText(t, { name: 'settings_describe', args: '{"keys":["RetryTimes","RetryCooldownSeconds","Other"]}' }, describeKey, 'en_US')
    ).toBe('settingsAssistant.step.describe {"names":"重试次数, 限流冷却, and Other"}');
    expect(stepText(t, { name: 'settings_effective', args: '{"key":"RetryTimes","object_id":"7"}' }, describeKey, 'en_US')).toBe(
      'settingsAssistant.step.effectiveOf {"names":"重试次数","object":"7"}'
    );
    expect(stepText(t, { name: 'propose_setting_changes', args: '{"changes":[{},{}]}' }, describeKey, 'en_US')).toBe(
      'settingsAssistant.step.propose {"count":2}'
    );
    expect(stepText(t, { name: 'settings_search', args: 'not json' }, describeKey, 'en_US')).toBe('settingsAssistant.step.list');
    expect(stepText(t, { name: 'unknown_tool', args: '{}' }, describeKey, 'en_US')).toBe('unknown_tool');
  });
});
