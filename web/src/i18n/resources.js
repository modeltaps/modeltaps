import { SUPPORTED_LANGUAGES } from './languages';

// locales/*.json follow the one-way upstream sync; strings for pages of our own live in
// locales/modeltaps/*.json and are deep-merged in at startup, so they never conflict with a sync.
// Both are picked up by file name for every language in languages.json.
const upstream = import.meta.glob('./locales/*.json', { eager: true, import: 'default' });
const own = import.meta.glob('./locales/modeltaps/*.json', { eager: true, import: 'default' });

const isPlainObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

const deepMerge = (base, extra) => {
  const merged = { ...base };
  for (const [key, value] of Object.entries(extra || {})) {
    merged[key] = isPlainObject(value) && isPlainObject(merged[key]) ? deepMerge(merged[key], value) : value;
  }
  return merged;
};

export const resources = Object.fromEntries(
  SUPPORTED_LANGUAGES.map((code) => [
    code,
    { translation: deepMerge(upstream[`./locales/${code}.json`] || {}, own[`./locales/modeltaps/${code}.json`] || {}) }
  ])
);
