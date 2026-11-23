import LANGUAGES from './languages.json';

// ==============================|| I18N — THE CONSOLE'S LANGUAGES ||============================== //
// languages.json is the one list of languages, read by the server too (common/lang), in the order
// the pickers show it: by ISO 639-1 code (en, ja, zh), which also keeps English first. Each entry
// has its code (locale file names), own name, short label, BCP47 tag for identity providers, the
// browser tags that mean it, and the language a missing text falls back to.
// Adding a language: an entry here, locales/<code>.json and locales/modeltaps/<code>.json, and the
// server's email texts (common/stmp/locales/<code>.json). See CONTRIBUTING.md.

export { LANGUAGES };

export const SUPPORTED_LANGUAGES = LANGUAGES.map((l) => l.code);

export const ENGLISH = LANGUAGES[0].code;

// Maps a language tag (zh-CN, zh-Hant-HK, ja, en-GB, zh_HK ...) to a supported code, or '' when
// it is none of them. The longest matching entry wins, as on the server.
export function normalizeLanguage(tag) {
  if (!tag) return '';
  if (SUPPORTED_LANGUAGES.includes(tag)) return tag;
  const lower = String(tag).trim().toLowerCase().replace(/_/g, '-');
  let best = '';
  let bestLen = 0;
  for (const l of LANGUAGES) {
    for (const m of l.match || []) {
      if ((lower === m || lower.startsWith(`${m}-`)) && m.length > bestLen) {
        best = l.code;
        bestLen = m.length;
      }
    }
  }
  return best;
}

// The order a missing text is looked up in for each language (i18next's fallbackLng): its
// fallbacks, then English.
export function fallbackChains() {
  const chains = { default: [ENGLISH] };
  for (const l of LANGUAGES) {
    const chain = [];
    for (let c = l.fallback; c && !chain.includes(c); c = LANGUAGES.find((x) => x.code === c)?.fallback) chain.push(c);
    if (!chain.includes(ENGLISH) && l.code !== ENGLISH) chain.push(ENGLISH);
    chains[l.code] = chain;
  }
  return chains;
}
