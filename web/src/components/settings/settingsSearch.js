// ==============================|| SETTINGS — SEARCH AND DIRTY HELPERS ||============================== //
// Pure helpers behind SettingsPage: matching a query against the text of a card, counting matches
// per group, and deriving which groups hold unsaved cards. Kept free of React so they are testable.

// Groups beyond this count get a search box at the top of the subnav.
export const SEARCH_MIN_GROUPS = 4;

export const shouldShowSearch = (groups) => (groups?.length || 0) >= SEARCH_MIN_GROUPS;

// Lower-cased, whitespace collapsed; null-safe.
export const normalizeText = (value) =>
  String(value ?? '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();

// Every whitespace-separated token of the query must appear in the text (any order).
// An empty query matches everything.
export function matchesQuery(text, query) {
  const q = normalizeText(query);
  if (!q) return true;
  const hay = normalizeText(text);
  if (!hay) return false;
  return q.split(' ').every((token) => hay.includes(token));
}

// entries: [{ cardId, groupId, text }] (text is everything a user can read on the card: title,
// description, field labels and hints, collection rows). Returns the matching card ids and the
// number of matches per group.
export function searchCards(entries, query) {
  const matches = new Set();
  const counts = {};
  for (const { cardId, groupId, text } of entries || []) {
    if (!counts[groupId]) counts[groupId] = 0;
    if (!normalizeText(text)) continue;
    if (matchesQuery(text, query)) {
      matches.add(cardId);
      counts[groupId] += 1;
    }
  }
  return { matches, counts };
}

// dirty: { [cardId]: groupId } for every card with unsaved changes.
export const dirtyGroupIds = (dirty) => new Set(Object.values(dirty || {}));

export const hasDirtyCards = (dirty) => Object.keys(dirty || {}).length > 0;

// Next dirty map after a card reports its state; returns the same object when nothing changed so
// React can skip the update.
export function reportDirty(prev, cardId, groupId, dirty) {
  const has = Object.prototype.hasOwnProperty.call(prev, cardId);
  if (dirty ? has && prev[cardId] === groupId : !has) return prev;
  const next = { ...prev };
  if (dirty) next[cardId] = groupId;
  else delete next[cardId];
  return next;
}

// Whether a navigation target stays on the settings page: group switches keep the edits, since
// every group stays mounted. `paths` are the page's own paths (its group links).
export const staysOnPage = (pathname, paths) => (paths || []).includes(String(pathname || '').replace(/\/+$/, ''));
