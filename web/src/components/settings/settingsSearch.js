// ==============================|| SETTINGS — SEARCH AND DIRTY HELPERS ||============================== //
// Pure helpers behind SettingsPage: matching a query against the text of a card, listing a page's
// cards for the search palette, and deriving which groups hold unsaved cards. Kept free of React
// so they are testable.

// Pages with this many groups get a search button at the top of their subnav.
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

// The cards of a settings page as the search palette lists them: title, group (with its part on
// a page of two parts), the link that opens it (highlighted when the card has an anchor) and all
// of its text to match against (title, description, field labels and hints, collection rows).
// `slots` maps card ids to their rendered elements; cards that render nothing are left out.
export function cardIndex(groups, slots) {
  return (groups || []).flatMap((group) =>
    group.cards
      .map((card) => {
        const el = slots?.[card.id];
        const text = normalizeText(el?.textContent);
        if (!text) return null;
        const title = (el.querySelector('h3') || el.querySelector('p.font-medium'))?.textContent?.trim() || String(group.label);
        const anchor = el.querySelector('[data-highlight]')?.getAttribute('data-highlight');
        return {
          id: `${group.id}:${card.id}`,
          title,
          group: group.sectionLabel ? `${group.sectionLabel} · ${group.label}` : String(group.label),
          to: anchor ? `${group.to}?highlight=${encodeURIComponent(anchor)}` : group.to,
          text
        };
      })
      .filter(Boolean)
  );
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
