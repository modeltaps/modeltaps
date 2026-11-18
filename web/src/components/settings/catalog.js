import data from '@/generated/settings-catalog.json';
import { admin, workbench } from 'routes/paths';

// ==============================|| SETTINGS — CATALOG ||============================== //
// The settings catalog (common/settings, generated into src/generated/settings-catalog.json): every
// platform setting and the object fields worth explaining, with the English text. Translations live
// in i18n/locales/catalog/<language>/<group>.json under `settingsCatalog.<key>`: label, description,
// examples.<n>, enum.<value>.label / .description and keywords (an array of extra search words).

const ENTRIES = data.settings || [];
const BY_KEY = new Map(ENTRIES.map((entry) => [entry.key, entry]));

export const catalogEntries = () => ENTRIES;

export const catalogEntry = (key) => (key ? BY_KEY.get(key) || null : null);

const tk = (key, part) => `settingsCatalog.${key}.${part}`;

export function settingLabel(t, key) {
  const entry = catalogEntry(key);
  return entry ? t(tk(key, 'label'), { defaultValue: entry.label }) : undefined;
}

export function settingDescription(t, key) {
  const entry = catalogEntry(key);
  return entry ? t(tk(key, 'description'), { defaultValue: entry.description }) : undefined;
}

export function settingExamples(t, key) {
  const entry = catalogEntry(key);
  return (entry?.examples || []).map((example, i) => ({
    value: example.value,
    effect: t(tk(key, `examples.${i}`), { defaultValue: example.effect })
  }));
}

export function settingEnumLabel(t, key, value) {
  const option = catalogEntry(key)?.enum?.find((e) => e.value === value);
  return option ? t(tk(key, `enum.${value}.label`), { defaultValue: option.label }) : value;
}

export function settingEnumDescription(t, key, value) {
  const option = catalogEntry(key)?.enum?.find((e) => e.value === value);
  return option?.description ? t(tk(key, `enum.${value}.description`), { defaultValue: option.description }) : undefined;
}

// Search words: the key, the English and translated label, description and keywords.
export function settingSearchText(t, key) {
  const entry = catalogEntry(key);
  if (!entry) return '';
  const translated = t(tk(key, 'keywords'), { returnObjects: true, defaultValue: [] });
  return [
    entry.key,
    entry.label,
    settingLabel(t, key),
    entry.description,
    settingDescription(t, key),
    ...(entry.keywords || []),
    ...(Array.isArray(translated) ? translated : [])
  ].join(' ');
}

// The first sentence of a description: what shows under a field; the rest goes into its help.
// A sentence ends at 。！？ or at . ! ? followed by a space (so 1.12 and e.g. do not end one).
const ABBREVIATIONS = /(?:^|\s)(?:e\.g|i\.e|vs|etc)\.$/i;

export function splitFirstSentence(text) {
  const value = String(text || '').trim();
  for (let i = 0; i < value.length; i += 1) {
    const ch = value[i];
    const cjkEnd = ch === '。' || ch === '！' || ch === '？';
    const latinEnd = (ch === '.' || ch === '!' || ch === '?') && /\s/.test(value[i + 1] || '');
    if (!cjkEnd && !latinEnd) continue;
    if (latinEnd && ABBREVIATIONS.test(value.slice(0, i + 1))) continue;
    const rest = value.slice(i + 1).trim();
    if (rest) return { first: value.slice(0, i + 1), rest };
  }
  return { first: value, rest: '' };
}

// Where an object field is set, for links from search and help.
export const OBJECT_PAGES = {
  channels: admin.providers(),
  models: admin.models(),
  allocation: `${admin.settings('gateway')}?tab=allocation`,
  'user-groups': admin.userGroups(),
  'routing-policies': admin.routing(),
  organizations: admin.orgs(),
  tokens: workbench.keys()
};
