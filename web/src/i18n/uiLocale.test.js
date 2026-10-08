import { beforeEach, describe, expect, it, vi } from 'vitest';

const i18n = { language: 'zh_CN' };
vi.mock('./i18n', () => ({ default: i18n }));

const { toBcp47, uiLocalesQuery } = await import('./uiLocale');

// 后端只放行 zh-CN / zh-HK / en / ja 四个值，非法值不会拼进跳转 URL；
// 未知语言这里就返回空串，请求与不带该参数时逐字节一致。

describe('toBcp47', () => {
  it('四种界面语言映射到 BCP47 标签', () => {
    expect(toBcp47('zh_CN')).toBe('zh-CN');
    expect(toBcp47('zh_HK')).toBe('zh-HK');
    expect(toBcp47('en_US')).toBe('en');
    expect(toBcp47('ja_JP')).toBe('ja');
  });

  it('未知语言返回空串', () => {
    expect(toBcp47('fr_FR')).toBe('');
    expect(toBcp47('zh-CN')).toBe('');
    expect(toBcp47(undefined)).toBe('');
  });
});

describe('uiLocalesQuery', () => {
  beforeEach(() => {
    i18n.language = 'zh_CN';
  });

  it('按当前界面语言给出查询串', () => {
    expect(uiLocalesQuery()).toBe('?ui_locales=zh-CN');
    i18n.language = 'zh_HK';
    expect(uiLocalesQuery()).toBe('?ui_locales=zh-HK');
    i18n.language = 'ja_JP';
    expect(uiLocalesQuery()).toBe('?ui_locales=ja');
  });

  it('语言未知时不产生查询串', () => {
    i18n.language = 'fr_FR';
    expect(uiLocalesQuery()).toBe('');
  });
});
