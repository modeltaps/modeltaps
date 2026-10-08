import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import { resources } from './resources';
import LanguageDetector from 'i18next-browser-languagedetector';

export const SUPPORTED_LANGUAGES = ['en_US', 'zh_CN', 'zh_HK', 'ja_JP'];
export const LANGUAGE_STORAGE_KEY = 'appLanguage';

// Maps a browser language tag (zh-CN, zh-TW, zh-Hant-HK, ja, en-GB, ...) onto the app's language
// codes. Codes already in app form pass through; anything unsupported is returned unchanged so
// i18next skips it and falls back to English.
export function toAppLanguage(tag) {
  if (!tag) return tag;
  if (SUPPORTED_LANGUAGES.includes(tag)) return tag;
  const lower = String(tag).toLowerCase().replace('_', '-');
  if (lower.startsWith('zh')) {
    return /hant|-tw|-hk|-mo/.test(lower) ? 'zh_HK' : 'zh_CN';
  }
  if (lower.startsWith('ja')) return 'ja_JP';
  if (lower.startsWith('en')) return 'en_US';
  return tag;
}

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    supportedLngs: SUPPORTED_LANGUAGES,
    fallbackLng: 'en_US',
    debug: false,
    // A language the user picked wins; otherwise follow the browser. Detection results are not
    // cached, so only an explicit choice (setAppLanguage) is remembered.
    detection: {
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: LANGUAGE_STORAGE_KEY,
      caches: [],
      convertDetectedLanguage: toAppLanguage
    },
    interpolation: {
      escapeValue: true,
      // Only angle brackets are escaped: interpolated values always render as text, and escaping
      // < and > is enough to stop tags from forming. i18next would also escape / & ' ", which
      // turns ordinary text such as Asia/Shanghai, URLs with & and provider/model ids into entities.
      escape: (value) => (typeof value === 'string' ? value.replace(/</g, '&lt;').replace(/>/g, '&gt;') : value)
    }
  });

const syncDocumentLanguage = (lng) => {
  if (typeof document !== 'undefined' && lng) {
    document.documentElement.lang = lng.replace('_', '-');
  }
};
syncDocumentLanguage(i18n.language);
i18n.on('languageChanged', syncDocumentLanguage);

export default i18n;
