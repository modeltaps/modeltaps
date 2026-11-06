// Pure helpers behind ModelMultiSelect: vendor inference, grouping, newest-first ordering and
// search. No React and no data fetching, so callers and tests feed plain option arrays.
//
// An option is { id, vendor?: { key, name, icon? }, releasedAt?: number (unix seconds), meta?: string }.
// Callers that know the vendor (catalog data) pass it; otherwise it is inferred from the id.

import { BRAND_LABELS, resolveBrandFromText } from '@/components/brand/brandIcons';

// Model-id tokens the channel-name rules in brandIcons do not cover. Checked after them.
const EXTRA_VENDOR_RULES = [
  { key: 'zhipu', name: 'Zhipu AI', tokens: ['glm', 'chatglm', 'cogview'] },
  { key: 'tencent', name: 'Tencent', tokens: ['hunyuan'] },
  { key: 'bytedance', name: 'ByteDance', tokens: ['doubao', 'seed-'] },
  { key: 'cohere', name: 'Cohere', tokens: ['command-', 'embed-english', 'embed-multilingual', 'rerank-'] },
  { key: 'microsoft', name: 'Microsoft', tokens: ['phi-'] },
  { key: 'yi', name: '01.AI', tokens: ['yi-'] }
];

export const CUSTOM_VENDOR = 'custom';

const titleCase = (s) =>
  s.replace(/(^|[-_\s])([a-z])/g, (_, sep, c) => `${sep === '-' || sep === '_' ? ' ' : sep}${c.toUpperCase()}`).trim();

// Vendor of a model id: brand tokens first (claude -> Anthropic, also for "anthropic/claude-x"),
// then the "vendor/" prefix as written, then the custom bucket.
export function inferVendor(id) {
  const value = String(id || '').toLowerCase();
  const brand = resolveBrandFromText(value);
  if (brand) return { key: brand, name: BRAND_LABELS[brand] || titleCase(brand) };
  const extra = EXTRA_VENDOR_RULES.find((r) => r.tokens.some((t) => value.includes(t)));
  if (extra) return { key: extra.key, name: extra.name };
  const slash = value.replace(/^~/, '').indexOf('/');
  if (slash > 0) {
    const prefix = value.replace(/^~/, '').slice(0, slash);
    return { key: prefix, name: titleCase(prefix) };
  }
  return { key: CUSTOM_VENDOR, name: '' };
}

const vendorOf = (option) => (option.vendor?.key ? option.vendor : inferVendor(option.id));

// Newest first: a known release date beats an unknown one, then the later date wins; ties and
// undated ids fall back to a version-aware descending compare (claude-opus-5-5 before 4-5,
// gpt-6.1 before gpt-5.6).
export function compareNewestFirst(a, b) {
  const ra = a.releasedAt || 0;
  const rb = b.releasedAt || 0;
  if (ra !== rb) {
    if (!ra) return 1;
    if (!rb) return -1;
    return rb - ra;
  }
  return b.id.localeCompare(a.id, undefined, { numeric: true, sensitivity: 'base' });
}

// Case-insensitive dedupe by id; the first occurrence wins, so callers list their richest
// source (upstream detail) before plain suggestions.
export function mergeOptions(...lists) {
  const seen = new Set();
  const out = [];
  lists.flat().forEach((o) => {
    const option = typeof o === 'string' ? { id: o } : o;
    const id = (option?.id || '').trim();
    const k = id.toLowerCase();
    if (!id || seen.has(k)) return;
    seen.add(k);
    out.push({ ...option, id });
  });
  return out;
}

// Groups the options that match `query` (substring of the id or vendor name) by vendor.
// Groups are ordered by their newest model, undated groups by name after them, and the custom
// bucket last. `onlySelected` keeps the selected ids only. Returns
// [{ key, name, icon, newest, items: option[] }].
export function groupOptions(options, { query = '', selected = new Set(), onlySelected = false } = {}) {
  const q = query.trim().toLowerCase();
  const groups = new Map();
  options.forEach((option) => {
    if (onlySelected && !selected.has(option.id)) return;
    const vendor = vendorOf(option);
    if (q && !option.id.toLowerCase().includes(q) && !(vendor.name || '').toLowerCase().includes(q)) return;
    if (!groups.has(vendor.key)) groups.set(vendor.key, { key: vendor.key, name: vendor.name, icon: vendor.icon, newest: 0, items: [] });
    const group = groups.get(vendor.key);
    group.items.push(option);
    group.newest = Math.max(group.newest, option.releasedAt || 0);
  });
  const list = [...groups.values()];
  list.forEach((g) => g.items.sort(compareNewestFirst));
  return list.sort((a, b) => {
    if (a.key === CUSTOM_VENDOR) return 1;
    if (b.key === CUSTOM_VENDOR) return -1;
    if (a.newest !== b.newest) {
      if (!a.newest) return 1;
      if (!b.newest) return -1;
      return b.newest - a.newest;
    }
    return (a.name || a.key).localeCompare(b.name || b.key);
  });
}

// Selected ids grouped the same way, for the summary under the trigger. Ids missing from
// `options` (typed by hand, or removed upstream) still show, under their inferred vendor.
export function groupSelected(options, value) {
  const byId = new Map(options.map((o) => [o.id.toLowerCase(), o]));
  const picked = value.map((id) => byId.get(id.toLowerCase()) || { id });
  return groupOptions(picked);
}

// Toggles a whole group: selects all of `ids` unless every one is already selected, in which
// case it clears them. Keeps the order of `value` and appends new ids at the end.
export function toggleGroup(value, ids) {
  const current = new Set(value);
  const allOn = ids.every((id) => current.has(id));
  if (allOn) return value.filter((id) => !ids.includes(id));
  return [...value, ...ids.filter((id) => !current.has(id))];
}

export function toggleOne(value, id) {
  return value.includes(id) ? value.filter((x) => x !== id) : [...value, id];
}

// Month of a release date as YYYY-MM, '' when unknown.
export function releaseMonth(unix) {
  return unix ? new Date(unix * 1000).toISOString().slice(0, 7) : '';
}
