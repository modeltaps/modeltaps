import { describe, expect, it } from 'vitest';
import { toAppLanguage } from './i18n';

describe('toAppLanguage', () => {
  it('maps browser tags onto the supported app languages', () => {
    expect(toAppLanguage('zh-CN')).toBe('zh_CN');
    expect(toAppLanguage('zh')).toBe('zh_CN');
    expect(toAppLanguage('zh-Hans-SG')).toBe('zh_CN');
    expect(toAppLanguage('zh-TW')).toBe('zh_CN');
    expect(toAppLanguage('zh-HK')).toBe('zh_CN');
    expect(toAppLanguage('zh-Hant')).toBe('zh_CN');
    expect(toAppLanguage('ja')).toBe('ja_JP');
    expect(toAppLanguage('ja-JP')).toBe('ja_JP');
    expect(toAppLanguage('en-GB')).toBe('en_US');
    expect(toAppLanguage('en')).toBe('en_US');
  });

  it('maps a saved zh_HK to Chinese and leaves unsupported languages for the English fallback', () => {
    expect(toAppLanguage('zh_HK')).toBe('zh_CN');
    expect(toAppLanguage('xx-XX')).toBe('xx-XX');
    expect(toAppLanguage('')).toBe('');
  });
});
