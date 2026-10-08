import ja_JP from './locales/ja_JP.json';
import en_US from './locales/en_US.json';
import zh_CN from './locales/zh_CN.json';
import zh_HK from './locales/zh_HK.json';
import modeltaps_ja_JP from './locales/modeltaps/ja_JP.json';
import modeltaps_en_US from './locales/modeltaps/en_US.json';
import modeltaps_zh_CN from './locales/modeltaps/zh_CN.json';
import modeltaps_zh_HK from './locales/modeltaps/zh_HK.json';

// locales/*.json follow the one-way upstream sync; strings for pages of our own live in
// locales/modeltaps/*.json and are deep-merged in at startup, so they never conflict with a sync.
const isPlainObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

const deepMerge = (base, extra) => {
  const merged = { ...base };
  for (const [key, value] of Object.entries(extra || {})) {
    merged[key] = isPlainObject(value) && isPlainObject(merged[key]) ? deepMerge(merged[key], value) : value;
  }
  return merged;
};

export const resources = {
  ja_JP: {
    translation: deepMerge(ja_JP, modeltaps_ja_JP)
  },
  en_US: {
    translation: deepMerge(en_US, modeltaps_en_US)
  },
  zh_CN: {
    translation: deepMerge(zh_CN, modeltaps_zh_CN)
  },
  zh_HK: {
    translation: deepMerge(zh_HK, modeltaps_zh_HK)
  }
};
