// Pure helpers for the channel panel's upstream model list (GET /api/channel/:id/upstream_models).

// States in the order the filter shows them. "off" rows (a supply that is switched off) are
// counted with "available": both mean the channel could serve the model and does not.
export const UPSTREAM_FILTERS = ['all', 'in_use', 'available', 'not_in_catalog', 'unknown'];

const FILTER_OF = { in_use: 'in_use', off: 'available', available: 'available', not_in_catalog: 'not_in_catalog', unknown: 'unknown' };

export const upstreamFilterOf = (state) => FILTER_OF[state] || 'unknown';

export function upstreamCounts(models = []) {
  const counts = { all: models.length, in_use: 0, available: 0, not_in_catalog: 0, unknown: 0 };
  models.forEach((m) => {
    counts[upstreamFilterOf(m.state)] += 1;
  });
  return counts;
}

// Rows of one filter whose upstream or public name contains the query (case-insensitive).
export function filterUpstreamModels(models = [], filter = 'all', query = '') {
  const q = query.trim().toLowerCase();
  return models.filter((m) => {
    if (filter !== 'all' && upstreamFilterOf(m.state) !== filter) return false;
    if (!q) return true;
    return m.name.toLowerCase().includes(q) || (m.public_model || '').toLowerCase().includes(q);
  });
}
