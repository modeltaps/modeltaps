// Pure helpers for model supplies: which channel serves a public model, in which priority
// tier, and whether it can take requests right now. No React and no API calls, so they are
// easy to test.

export const MODEL_STATUSES = ['draft', 'published', 'retired'];
export const SUPPLY_MODES = ['balance', 'failover'];

// Manual pause lengths offered on a supply row, in seconds.
export const PAUSE_DURATIONS = [1800, 3600, 6 * 3600, 24 * 3600];

const CHANNEL_STATUS_ENABLED = 1;

// A public model is a catalog row that is neither an alias nor a routing policy: only these
// rows can have supplies, a status and a supply mode.
export function isPublicModelRow(row) {
  return Boolean(row && row.catalogId > 0 && !row.alias_of && !row.kind);
}

export function normalizeModelStatus(status) {
  return MODEL_STATUSES.includes(status) ? status : 'draft';
}

export function normalizeSupplyMode(mode) {
  return SUPPLY_MODES.includes(mode) ? mode : 'balance';
}

function parseMapping(raw) {
  if (!raw) return {};
  if (typeof raw === 'object') return raw;
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

const channelModels = (channel) =>
  String(channel?.models || '')
    .split(',')
    .map((name) => name.trim())
    .filter(Boolean);

// The model name a channel should receive for a public model: the channel's model mapping
// when it has an entry (without the leading "+" that only changes billing), else the name itself.
export function upstreamFor(channel, model) {
  const mapped = parseMapping(channel?.model_mapping)[model];
  if (typeof mapped === 'string' && mapped.trim()) return mapped.trim().replace(/^\+/, '');
  return model;
}

// Whether the channel already lists the model (directly or as a mapping key).
export function channelOffersModel(channel, model) {
  return channelModels(channel).includes(model) || Object.hasOwn(parseMapping(channel?.model_mapping), model);
}

// Channels that can be added as a supply: not used by the model yet, the ones that already
// list the model first.
export function supplyCandidates(channels = [], supplies = [], model = '') {
  const used = new Set(supplies.map((s) => s.channel_id));
  const free = channels.filter((c) => !used.has(c.id));
  return {
    offering: free.filter((c) => channelOffersModel(c, model)),
    others: free.filter((c) => !channelOffersModel(c, model))
  };
}

// Supplies grouped into priority tiers, highest priority first; within a tier by weight
// (highest first) and then id, the order failover mode tries them in.
export function groupTiers(supplies = []) {
  const byPriority = new Map();
  supplies.forEach((s) => {
    const p = Number(s.priority) || 0;
    if (!byPriority.has(p)) byPriority.set(p, []);
    byPriority.get(p).push(s);
  });
  return [...byPriority.entries()]
    .sort((a, b) => b[0] - a[0])
    .map(([priority, items]) => ({
      priority,
      supplies: [...items].sort((a, b) => effectiveWeight(b) - effectiveWeight(a) || a.id - b.id)
    }));
}

export const effectiveWeight = (supply) => Number(supply?.weight) || 1;

// Moves a supply one tier up or down. Returns the priority changes to save as
// [{ id, priority }]; an empty list means it cannot move further.
// - into the neighbouring tier when there is one;
// - past the top / bottom, into a new tier of its own when it shares its tier with others;
// - a new bottom tier below priority 0 lifts every other supply by one instead, so priorities
//   stay at 0 or above.
export function moveSupply(supplies = [], id, direction) {
  const tiers = groupTiers(supplies);
  const index = tiers.findIndex((tier) => tier.supplies.some((s) => s.id === id));
  if (index < 0) return [];
  const tier = tiers[index];
  const alone = tier.supplies.length === 1;
  if (direction === 'up') {
    if (index > 0) return [{ id, priority: tiers[index - 1].priority }];
    return alone ? [] : [{ id, priority: tier.priority + 1 }];
  }
  if (index < tiers.length - 1) return [{ id, priority: tiers[index + 1].priority }];
  if (alone) return [];
  if (tier.priority > 0) return [{ id, priority: tier.priority - 1 }];
  return supplies.filter((s) => s.id !== id).map((s) => ({ id: s.id, priority: (Number(s.priority) || 0) + 1 }));
}

// Default priority for a new supply: a backup below the current lowest tier, or 0 for the first.
export function defaultNewPriority(supplies = []) {
  if (supplies.length === 0) return 0;
  const lowest = Math.min(...supplies.map((s) => Number(s.priority) || 0));
  return Math.max(0, lowest - 1);
}

// Share of requests each supply of a tier gets in balance mode, in percent (0–100).
export function tierShares(tier, mode) {
  const items = tier.supplies.filter((s) => s.enabled !== false);
  const shares = {};
  if (normalizeSupplyMode(mode) === 'failover') {
    if (items[0]) shares[items[0].id] = 100;
    return shares;
  }
  const total = items.reduce((sum, s) => sum + effectiveWeight(s), 0);
  items.forEach((s) => {
    shares[s.id] = total ? Math.round((effectiveWeight(s) / total) * 100) : 0;
  });
  return shares;
}

// Pauses that block a supply: one on the whole channel, or one on this model on the channel
// (the model is the public name while supplies route, the channel's model name before).
export function pausesFor(supply, pauses = []) {
  return pauses.filter(
    (p) => p.channel_id === supply.channel_id && (!p.model || p.model === supply.model || p.model === supply.upstream_model)
  );
}

// State of a supply row, first match wins:
// channel_off (channel disabled or deleted) → disabled (supply switched off) → paused (until a
// time) → probing (time is up, waiting for the recovery test) → ok.
export function supplyState(supply, pauses = [], now = Math.floor(Date.now() / 1000)) {
  if ((supply.channel_status ?? CHANNEL_STATUS_ENABLED) !== CHANNEL_STATUS_ENABLED) return { kind: 'channel_off' };
  if (supply.enabled === false) return { kind: 'disabled' };
  const matching = pausesFor(supply, pauses);
  const active = matching.filter((p) => p.until > now).sort((a, b) => b.until - a.until)[0];
  if (active) return { kind: 'paused', pause: active, pauses: matching };
  const probing = matching.find((p) => p.probe);
  if (probing) return { kind: 'probing', pause: probing, pauses: matching };
  return { kind: 'ok', pauses: matching };
}

// Supplies per public model, for the catalog table.
export function countSuppliesByModel(supplies = []) {
  const counts = {};
  supplies.forEach((s) => {
    const entry = (counts[s.model] ||= { total: 0, usable: 0 });
    entry.total += 1;
    if (s.enabled !== false && (s.channel_status ?? CHANNEL_STATUS_ENABLED) === CHANNEL_STATUS_ENABLED) entry.usable += 1;
  });
  return counts;
}

// The PUT body for a supply with some fields changed (the API wants every editable field).
export function supplyUpdateBody(supply, changes = {}) {
  return {
    id: supply.id,
    upstream_model: supply.upstream_model,
    priority: Number(supply.priority) || 0,
    weight: Number(supply.weight) || 0,
    enabled: supply.enabled !== false,
    note: supply.note || '',
    ...changes
  };
}

// Validates the inline edit / add form. Returns { error } (an i18n key) or the parsed values.
export function parseSupplyForm({ upstream_model, priority, weight }) {
  const upstream = String(upstream_model ?? '').trim();
  if (!upstream) return { error: 'modelSupply.upstreamRequired' };
  const p = Number(priority);
  if (String(priority ?? '').trim() === '' || !Number.isInteger(p)) return { error: 'modelSupply.priorityInvalid' };
  const w = Number(weight);
  if (String(weight ?? '').trim() === '' || !Number.isInteger(w) || w < 0) return { error: 'modelSupply.weightInvalid' };
  return { upstream_model: upstream, priority: p, weight: w };
}

// Deep link to a model's supplies on the model page.
export function modelSupplyPath(model) {
  return `/panel/pricing?tab=models&model=${encodeURIComponent(model)}&panel=supplies`;
}
