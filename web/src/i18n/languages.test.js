import { describe, expect, it } from 'vitest';

import { LANGUAGES, fallbackChains, normalizeLanguage } from './languages';
import { resources } from './resources';

const upstreamFiles = import.meta.glob('./locales/*.json');
const ownFiles = import.meta.glob('./locales/modeltaps/*.json');

describe('languages.json', () => {
  it('starts with English and has unique codes and BCP47 tags', () => {
    expect(LANGUAGES[0].code).toBe('en_US');
    expect(new Set(LANGUAGES.map((l) => l.code)).size).toBe(LANGUAGES.length);
    expect(new Set(LANGUAGES.map((l) => l.bcp47)).size).toBe(LANGUAGES.length);
    for (const l of LANGUAGES) {
      expect(l.name, l.code).toBeTruthy();
      if (l.fallback)
        expect(
          LANGUAGES.some((x) => x.code === l.fallback),
          l.code
        ).toBe(true);
    }
  });

  it('every language has its translation files', () => {
    for (const { code } of LANGUAGES) {
      expect(upstreamFiles[`./locales/${code}.json`], `locales/${code}.json`).toBeTruthy();
      expect(ownFiles[`./locales/modeltaps/${code}.json`], `locales/modeltaps/${code}.json`).toBeTruthy();
      expect(Object.keys(resources[code].translation).length, code).toBeGreaterThan(0);
    }
  });

  it('maps browser tags like the server and falls back to English', () => {
    expect(normalizeLanguage('zh-Hant-TW')).toBe('zh_CN');
    expect(normalizeLanguage('zh_HK')).toBe('zh_CN');
    expect(normalizeLanguage('zh')).toBe('zh_CN');
    expect(normalizeLanguage('xx')).toBe('');
    expect(fallbackChains()).toMatchObject({ zh_CN: ['en_US'], ja_JP: ['en_US'], en_US: [], default: ['en_US'] });
  });
});
