import { beforeEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { describeResult, describeResults, rowOutcome } from './channelCheckUtils';

const t = (...args) => i18n.t(...args);

describe('rowOutcome', () => {
  it('folds a row into passed / warning / failed', () => {
    expect(rowOutcome({ status: 'done', data: { status: 1 } })).toBe('passed');
    // A model whose checks are all not applicable is not a failure.
    expect(rowOutcome({ status: 'done', data: { status: 3 } })).toBe('passed');
    expect(rowOutcome({ status: 'done', data: { status: 2 } })).toBe('warning');
    expect(rowOutcome({ status: 'done', data: { status: 0 } })).toBe('failed');
    expect(rowOutcome({ status: 'error', message: 'x' })).toBe('failed');
  });
});

describe('describeResult', () => {
  beforeEach(() => i18n.changeLanguage('en_US'));

  it('translates the stable keys and fills in the params', () => {
    expect(
      describeResult(t, { name: 'tool_call', code: 'tool_partial', params: { matched: 1, total: 2, calls: ['add(1, 2)', 'multiply(?)'] } })
    ).toEqual({
      label: 'Tool calls',
      message: 'Only 1 of 2 calls correct (no parallel calls): add(1, 2), multiply(?)'
    });
  });

  it('keeps upstream text as it is', () => {
    const { message } = describeResult(t, { name: 'response', code: 'upstream_error', params: { message: 'a <b> & "c" / d' } });
    expect(message).toBe('a <b> & "c" / d');
  });

  it('falls back to the raw keys for codes it does not know', () => {
    expect(describeResult(t, { name: 'new_check', code: 'new_code' })).toEqual({ label: 'new_check', message: 'new_code' });
  });

  it('translates into the other languages', async () => {
    await i18n.changeLanguage('zh_CN');
    expect(describeResults(t, [{ name: 'challenge', code: 'challenge_mismatch', params: { expected: '7', answer: '8' } }])).toBe(
      '算术校验: 期望 7，实际回答：8'
    );
    await i18n.changeLanguage('zh_HK');
    expect(describeResult(t, { name: 'request_id', code: 'request_id_found', params: { count: 3 } }).message).toContain('3 個 request id');
  });
});
