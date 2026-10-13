import { beforeEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { resources } from 'i18n/resources';
import { apiMessage, apiMessageKey, localizeApiMessage } from './apiError';

describe('apiMessage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('zh_CN');
  });

  it('translates a known code with its params and keeps the raw reason untranslated', () => {
    const data = {
      success: false,
      message: 'speed test failed, reason: model <x> does not exist',
      code: 'channel_test_failed',
      params: { reason: 'model <x> does not exist' }
    };
    expect(apiMessage(data)).toBe('测试失败：model <x> does not exist');
  });

  it('follows the UI language', async () => {
    const data = { success: false, message: 'tag is required', code: 'channel_tag_required' };
    await i18n.changeLanguage('ja_JP');
    expect(apiMessage(data)).toBe('タグが必要です');
    await i18n.changeLanguage('en_US');
    expect(apiMessage(data)).toBe('A tag is required');
  });

  it('reads code and params nested under error (rate limiter 429)', () => {
    const data = {
      error: {
        message: 'Too many requests, please retry after 7 seconds.',
        type: 'system_error',
        code: 'rate_limit_exceeded',
        params: { retry_after: 7 }
      }
    };
    expect(apiMessage(data)).toBe('请求过于频繁，请 7 秒后重试。');
  });

  it('falls back to the server message for unknown, missing or malformed codes', () => {
    expect(apiMessage({ message: 'record not found' })).toBe('record not found');
    expect(apiMessage({ message: 'boom', code: 'not_a_known_code' })).toBe('boom');
    expect(apiMessage({ message: 'boom', code: 'common.errorPrefix' })).toBe('boom');
    expect(apiMessage({ message: 'boom', code: 42 })).toBe('boom');
  });

  it('falls back to the given text when there is no message', () => {
    expect(apiMessage(undefined, 'fallback')).toBe('fallback');
    expect(apiMessage({ error: { type: 'x' } }, 'fallback')).toBe('fallback');
    expect(apiMessage({ message: '' }, 'fallback')).toBe('fallback');
  });

  it('does not let params override i18next options', () => {
    const data = { message: 'tag is required', code: 'channel_tag_required', params: { lng: 'ja_JP', ns: 'other' } };
    expect(apiMessage(data)).toBe('缺少标签');
  });
});

describe('localizeApiMessage', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('zh_HK');
  });

  it('rewrites message in place for a known code', () => {
    const data = { success: true, message: 'speed test succeeded', code: 'channel_test_succeeded', time: 0.4 };
    localizeApiMessage(data);
    expect(data).toEqual({ success: true, message: '測試通過', code: 'channel_test_succeeded', time: 0.4 });
  });

  it('leaves bodies without a known code untouched', () => {
    const plain = { success: false, message: 'record not found' };
    localizeApiMessage(plain);
    expect(plain.message).toBe('record not found');
    expect(localizeApiMessage('text')).toBe('text');
    expect(localizeApiMessage(null)).toBeNull();
  });
});

describe('apiError locale keys', () => {
  it('every locale defines the same codes', () => {
    const codes = (lng) => Object.keys(resources[lng].translation.apiError || {}).sort();
    const base = codes('en_US');
    expect(base.length).toBeGreaterThan(0);
    for (const lng of ['zh_CN', 'zh_HK', 'ja_JP']) {
      expect(codes(lng)).toEqual(base);
    }
    expect(apiMessageKey({ code: base[0] })).toBe(`apiError.${base[0]}`);
  });
});
