// Model allowlist matching for the console (ORG-15), the same rules as the backend's
// model.ModelAllowlistPermits / ModelAllowlistCovers without alias resolution (the server checks
// aliases on save and at request time):
//   - an empty list does not restrict;
//   - case-insensitive;
//   - an entry ending in "*" matches by prefix.

const norm = (s) =>
  String(s || '')
    .trim()
    .toLowerCase();

export const isPattern = (entry) => norm(entry).endsWith('*');

export function allowlistPermits(list, name) {
  const entries = (list || []).map(norm).filter(Boolean);
  if (entries.length === 0) return true;
  const n = norm(name);
  return entries.some((e) => (e.endsWith('*') ? n.startsWith(e.slice(0, -1)) : e === n));
}

// Whether `entry` of a lower layer stays inside `parent`: a model name is covered when the parent
// permits it, a prefix "x*" only by a parent prefix "y*" with x starting with y.
export function allowlistCovers(parent, entry) {
  const entries = (parent || []).map(norm).filter(Boolean);
  if (entries.length === 0) return true;
  const e = norm(entry);
  if (!e) return true;
  if (!e.endsWith('*')) return allowlistPermits(entries, e);
  const prefix = e.slice(0, -1);
  return entries.some((p) => p.endsWith('*') && prefix.startsWith(p.slice(0, -1)));
}

// Entries of `child` outside any of the `parents` lists.
export function uncoveredEntries(parents, child) {
  return (child || []).filter((entry) => !(parents || []).every((p) => allowlistCovers(p, entry)));
}

// Models of `names` that every list of `parents` permits.
export function filterPermitted(parents, names) {
  return (names || []).filter((name) => (parents || []).every((p) => allowlistPermits(p, name)));
}
