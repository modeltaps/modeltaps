import { describe, expect, it } from 'vitest';

import { visibleFields } from '../shared/fieldSchema';
import { CHAT_FIELDS, REASONING_EFFORTS, paramsSummary } from './chatFields';

// 参数 chip 的字段：键名必须与会话 state 对得上（否则改了 chip 参数发不出去），
// 能力型字段只对声明了对应 capability 的模型出现。

const model = (capabilities = []) => ({ id: 'm', info: { capabilities } });
const keysOf = (capabilities, values = {}) => visibleFields(CHAT_FIELDS, model(capabilities), values).map((field) => field.key);

describe('CHAT_FIELDS', () => {
  it('键名对齐会话 state（settings + system）', () => {
    expect(CHAT_FIELDS.map((field) => field.key)).toEqual([
      'system',
      'temperature',
      'topP',
      'maxTokens',
      'seed',
      'thinking',
      'reasoningEffort',
      'thinkingBudget',
      'stream',
      'jsonMode'
    ]);
  });

  it('数值字段都是 number：留空即不发送，滑块没有「空」这个位置', () => {
    for (const key of ['temperature', 'topP', 'maxTokens', 'seed', 'thinkingBudget']) {
      expect(CHAT_FIELDS.find((field) => field.key === key).type, key).toBe('number');
    }
  });

  it('不声明能力的模型看不到思考与 JSON 模式', () => {
    const keys = keysOf([]);
    expect(keys).not.toContain('thinking');
    expect(keys).not.toContain('jsonMode');
    expect(keys).toContain('temperature');
  });

  it('思考预算只在开了思考之后出现', () => {
    expect(keysOf(['reasoning'])).not.toContain('thinkingBudget');
    expect(keysOf(['reasoning'], { thinking: true })).toContain('thinkingBudget');
  });

  it('思考深度与思考预算同门控，且排在思考预算之前', () => {
    expect(keysOf([], { thinking: true })).not.toContain('reasoningEffort');
    expect(keysOf(['reasoning'])).not.toContain('reasoningEffort');

    const keys = keysOf(['reasoning'], { thinking: true });
    expect(keys.indexOf('reasoningEffort')).toBeLessThan(keys.indexOf('thinkingBudget'));
  });

  it('思考深度默认为空（不发送），取值集合是 low / medium / high', () => {
    const field = CHAT_FIELDS.find((item) => item.key === 'reasoningEffort');
    expect(field.type).toBe('select');
    expect(field.default).toBe('');
    expect(field.options).toEqual(['', ...REASONING_EFFORTS]);
    expect(REASONING_EFFORTS).toEqual(['low', 'medium', 'high']);
  });

  it('声明 structured_output 才有 JSON 模式', () => {
    expect(keysOf(['structured_output'])).toContain('jsonMode');
  });
});

describe('paramsSummary', () => {
  it('只报改过的关键值', () => {
    expect(paramsSummary({ temperature: '0.7', maxTokens: '4096' })).toBe('0.7 · 4096');
    expect(paramsSummary({ temperature: '0.7' })).toBe('0.7');
  });

  it('一个都没改就是空串，由调用方回落成「参数」', () => {
    expect(paramsSummary({ temperature: '', maxTokens: '', stream: true })).toBe('');
    expect(paramsSummary()).toBe('');
  });

  it('0 是改过的值，不当成空', () => {
    expect(paramsSummary({ temperature: 0 })).toBe('0');
  });
});
