// Pure helpers for the channel model picker (ModelSelectorModal). The backend
// (`provider_models_list?detail=1`, see model/model_picker.go) already splits
// every upstream id into family / version, marks the newest version of each
// family and ranks the families by usage; this module only filters, groups and
// selects on the client.

export const CONTEXT_STEPS = [0, 32000, 128000, 200000, 1000000];

export const CAPABILITY_FILTERS = ['tool_call', 'reasoning', 'structured_output', 'vision'];

export const EMPTY_FILTERS = {
  search: '',
  status: 'all', // all | selected | unselected | elsewhere
  weights: 'all', // all | open | closed
  vendors: [],
  capabilities: [],
  output: 'text', // '' = any output modality
  maxInput: '',
  maxOutput: '',
  minContext: 0
};

// Vendor of an upstream id: the `vendor/` prefix when present ("~" aliases
// included), otherwise the first word of the family ("claude-opus" -> "claude").
export function vendorOf(item) {
  const id = item.model.replace(/^~/, '');
  const slash = id.indexOf('/');
  if (slash > 0) return id.slice(0, slash);
  return (item.family || id).split('-')[0];
}

export function hasCapability(item, capability) {
  if (capability === 'vision') return (item.input_modalities || []).includes('image');
  return (item.capabilities || []).includes(capability);
}

// Distinct vendors and output modalities present in the list, for the filter panel.
export function filterOptions(items) {
  const vendors = new Map();
  const outputs = new Set();
  items.forEach((item) => {
    const vendor = vendorOf(item);
    vendors.set(vendor, (vendors.get(vendor) || 0) + 1);
    (item.output_modalities || []).forEach((m) => outputs.add(m));
  });
  return {
    vendors: [...vendors.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([id, count]) => ({ id, count })),
    outputs: [...outputs].sort()
  };
}

const parseLimit = (value) => {
  if (value === '' || value === null || value === undefined) return null;
  const n = Number(value);
  return Number.isFinite(n) && n >= 0 ? n : null;
};

// Whether one row passes every filter except the "latest only" folding, which
// groupFamilies applies per family. `selected` is a Set of model ids.
export function matchesFilters(item, filters, selected) {
  const search = filters.search.trim().toLowerCase();
  if (search) {
    const id = item.model.toLowerCase();
    const family = (item.family || '').toLowerCase();
    if (!id.includes(search) && !family.includes(search)) return false;
  }
  if (filters.status === 'selected' && !selected.has(item.model)) return false;
  if (filters.status === 'unselected' && selected.has(item.model)) return false;
  if (filters.status === 'elsewhere' && !(item.other_channels > 0)) return false;
  if (filters.weights === 'open' && item.open_weights !== true) return false;
  if (filters.weights === 'closed' && item.open_weights !== false) return false;
  if (filters.vendors.length > 0 && !filters.vendors.includes(vendorOf(item))) return false;
  if (!filters.capabilities.every((c) => hasCapability(item, c))) return false;
  // Rows without modality metadata stay visible: hiding them would drop every
  // model of channels whose ids are not in the catalog.
  const outputs = item.output_modalities || [];
  if (filters.output && outputs.length > 0 && !outputs.includes(filters.output)) return false;
  const maxInput = parseLimit(filters.maxInput);
  if (maxInput !== null && (item.input_price === null || item.input_price === undefined || item.input_price > maxInput)) return false;
  const maxOutput = parseLimit(filters.maxOutput);
  if (maxOutput !== null && (item.output_price === null || item.output_price === undefined || item.output_price > maxOutput)) return false;
  if (filters.minContext > 0 && !(item.context_length >= filters.minContext)) return false;
  return true;
}

// Rows of one family in display order: the latest version first, then the
// rest newest-first by version, snapshot and release time.
function compareVersions(a = '', b = '') {
  if (a === b) return 0;
  if (!a) return -1;
  if (!b) return 1;
  const pa = a.split('.');
  const pb = b.split('.');
  const major = Number(pa[0]) - Number(pb[0]);
  if (major !== 0) return major > 0 ? 1 : -1;
  for (let i = 1; i < Math.max(pa.length, pb.length); i += 1) {
    let sa = pa[i] || '';
    let sb = pb[i] || '';
    const width = Math.max(sa.length, sb.length);
    sa = sa.padEnd(width, '0');
    sb = sb.padEnd(width, '0');
    if (sa !== sb) return sa > sb ? 1 : -1;
  }
  return 0;
}

function compareRows(a, b) {
  if (a.latest !== b.latest) return a.latest ? -1 : 1;
  const version = compareVersions(b.version, a.version);
  if (version !== 0) return version;
  if ((a.variant || '') !== (b.variant || '')) return (a.variant || '') < (b.variant || '') ? -1 : 1;
  if ((a.snapshot || '') !== (b.snapshot || '')) return (b.snapshot || '') < (a.snapshot || '') ? -1 : 1;
  return (b.released_at || 0) - (a.released_at || 0) || a.model.localeCompare(b.model);
}

// Groups the filtered rows by family in rank order. With `latestOnly`, a family
// shows only its latest row (or, when the latest row is filtered out, its best
// remaining row) unless it is in `expanded`; `hidden` counts the folded rows.
// A search always shows every match, since the user asked for those ids.
export function groupFamilies(items, families, filters, selected, { latestOnly = true, expanded = new Set() } = {}) {
  const byFamily = new Map();
  items.forEach((item) => {
    if (!matchesFilters(item, filters, selected)) return;
    if (!byFamily.has(item.family)) byFamily.set(item.family, []);
    byFamily.get(item.family).push(item);
  });
  const ranked = families.map((f) => f.family);
  // Families the backend did not rank (should not happen) go last, by name.
  [...byFamily.keys()]
    .filter((f) => !ranked.includes(f))
    .sort()
    .forEach((f) => ranked.push(f));
  const meta = new Map(families.map((f) => [f.family, f]));
  const searching = filters.search.trim() !== '';

  const groups = [];
  ranked.forEach((family) => {
    const rows = byFamily.get(family);
    if (!rows) return;
    rows.sort(compareRows);
    const fold = latestOnly && !searching && !expanded.has(family);
    const visible = fold ? rows.slice(0, 1) : rows;
    groups.push({
      family,
      rank: meta.get(family)?.rank ?? 0,
      requests: meta.get(family)?.requests ?? 0,
      total: meta.get(family)?.count ?? rows.length,
      matched: rows.length,
      rows: visible,
      hidden: rows.length - visible.length
    });
  });
  return groups;
}

export function visibleModels(groups) {
  return groups.flatMap((group) => group.rows.map((row) => row.model));
}

// Adds every id to the selection, or removes them all when all are already selected.
export function toggleMany(selected, models) {
  const next = new Set(selected);
  const allIn = models.length > 0 && models.every((m) => next.has(m));
  models.forEach((m) => (allIn ? next.delete(m) : next.add(m)));
  return next;
}

export function formatContext(tokens) {
  if (!tokens) return '';
  if (tokens >= 1000000) return `${Number((tokens / 1000000).toFixed(1))}M`;
  return `${Math.round(tokens / 1000)}K`;
}

export function formatPrice(usd) {
  if (usd === null || usd === undefined) return '';
  if (usd === 0) return '0';
  if (usd < 0.01) return usd.toPrecision(2);
  return String(Number(usd.toFixed(usd < 1 ? 3 : 2)));
}

// A retirement date within `days` (default 30) of `now` is shown as a warning.
export function expiresSoon(item, now = Date.now() / 1000, days = 30) {
  return item.expires_at > 0 && item.expires_at - now <= days * 86400;
}
