// Pure helpers for the state filters of the channel panel's model table: whether each listed
// model is offered as a public model, whether the upstream still lists it, and which models the
// upstream lists that the channel does not.

import { upstreamNameOf } from './channelPrices';

// Filters in the order the table shows them.
export const STATE_FILTERS = ['offered', 'unoffered', 'gone', 'unlisted'];

// Whether a listed model serves an enabled public model, and why not when it does not:
// not_in_catalog / no_supply / duplicate / wildcard (from the model details) or off (its supplies
// are all switched off). null while the details have not loaded or supplies do not route requests.
export function offerOf(detail) {
  if (!detail || !Array.isArray(detail.public)) return null;
  if (detail.public.some((p) => p.enabled)) return { offered: true };
  if (detail.public.length) return { offered: false, reason: 'off' };
  const offer = { offered: false, reason: detail.unoffered || 'not_in_catalog', suggested: detail.suggested || '' };
  if (offer.reason === 'duplicate') Object.assign(offer, { sameAs: detail.same_as || '', suppliedBy: detail.supplied_by || '' });
  return offer;
}

// The upstream's names at the last model check, or null when it never listed any (never checked,
// or a provider that cannot list): then nothing is marked as gone or unlisted.
export function snapshotOf(upstream) {
  const models = upstream?.models || [];
  if (!upstream?.checked_at || !models.length) return null;
  return new Set(models.map((m) => m.name));
}

// A listed model the upstream no longer lists. Wildcards are never gone.
export const isGone = (name, mapping, snapshot) => Boolean(snapshot) && !name.endsWith('*') && !snapshot.has(upstreamNameOf(name, mapping));

// Rows of the upstream list the channel does not list, by name or as a mapping target.
export function unlistedOf(upstream, models, mapping) {
  if (!snapshotOf(upstream)) return [];
  const listed = new Set(models);
  models.forEach((name) => listed.add(upstreamNameOf(name, mapping)));
  return upstream.models.filter((m) => !listed.has(m.name));
}

// Whether a listed row matches a state filter.
export function matchesState(row, key) {
  if (key === 'offered') return row.offer?.offered === true;
  if (key === 'unoffered') return row.offer?.offered === false;
  if (key === 'gone') return row.gone;
  return false;
}

export function stateCounts(rows, unlisted) {
  const out = { offered: 0, unoffered: 0, gone: 0, unlisted: unlisted.length };
  for (const r of rows) {
    if (r.offer?.offered === true) out.offered += 1;
    if (r.offer?.offered === false) out.unoffered += 1;
    if (r.gone) out.gone += 1;
  }
  return out;
}
