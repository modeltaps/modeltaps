// Pure helpers for the allocation policy page. A policy is a list of tiers tried first to last;
// a tier is a list of entries sharing its traffic by weight. An entry is a provider
// ({ provider: key, weight }) or one channel ({ channel_id, weight }). Every function returns new
// arrays and never changes its input.

const clone = (tiers) => tiers.map((tier) => tier.map((entry) => ({ ...entry })));
const dropEmpty = (tiers) => tiers.filter((tier) => tier.length > 0);

export const entryKey = (entry) => (entry.channel_id ? `c${entry.channel_id}` : `p${entry.provider}`);

// Providers with a channel the policy does not place: no entry for the provider and not every
// channel placed on its own. Those channels are tried after every tier.
export function unlistedProviders(providers = [], tiers = []) {
  const placedProviders = new Set();
  const placedChannels = new Set();
  for (const tier of tiers)
    for (const entry of tier) {
      if (entry.channel_id) placedChannels.add(entry.channel_id);
      else placedProviders.add(entry.provider);
    }
  return providers.filter((p) => !placedProviders.has(p.key) && p.channels.some((c) => !placedChannels.has(c.id)));
}

// Providers the policy lists by key but no channel belongs to any more.
export function missingEntry(entry, providers = []) {
  if (entry.channel_id) return !providers.some((p) => p.channels.some((c) => c.id === entry.channel_id));
  return !providers.some((p) => p.key === entry.provider);
}

// Moves an entry one tier up or down. At the top or bottom it gets a tier of its own when it
// shares one; a lone entry at an end stays.
export function moveEntry(tiers, t, i, dir) {
  const next = clone(tiers);
  const [entry] = next[t].splice(i, 1);
  if (dir === 'up') {
    if (t > 0) next[t - 1].push(entry);
    else next.unshift([entry]);
  } else if (t < next.length - 1) next[t + 1].push(entry);
  else next.push([entry]);
  return dropEmpty(next);
}

export function canMoveEntry(tiers, t, dir) {
  const alone = tiers[t].length === 1;
  return dir === 'up' ? t > 0 || !alone : t < tiers.length - 1 || !alone;
}

// Takes an entry out of a shared tier into its own tier right below.
export function splitOut(tiers, t, i) {
  const next = clone(tiers);
  const [entry] = next[t].splice(i, 1);
  next.splice(t + 1, 0, [entry]);
  return dropEmpty(next);
}

export function moveTier(tiers, t, dir) {
  const to = dir === 'up' ? t - 1 : t + 1;
  if (to < 0 || to >= tiers.length) return tiers;
  const next = clone(tiers);
  [next[t], next[to]] = [next[to], next[t]];
  return next;
}

export function removeEntry(tiers, t, i) {
  const next = clone(tiers);
  next[t].splice(i, 1);
  return dropEmpty(next);
}

// Places a provider in a new last tier.
export function addProvider(tiers, key) {
  return [...clone(tiers), [{ provider: key, weight: 1 }]];
}

export function setWeight(tiers, t, i, weight) {
  const next = clone(tiers);
  next[t][i].weight = Math.max(1, Math.round(Number(weight)) || 1);
  return next;
}

// Replaces a provider entry with one entry per channel of the provider that is not placed on
// its own yet, each with the provider's weight.
export function splitProvider(tiers, t, i, provider) {
  const next = clone(tiers);
  const entry = next[t][i];
  const placed = new Set(
    next
      .flat()
      .filter((e) => e.channel_id)
      .map((e) => e.channel_id)
  );
  const channels = (provider?.channels || []).filter((c) => !placed.has(c.id));
  next[t].splice(i, 1, ...channels.map((c) => ({ channel_id: c.id, weight: entry.weight || 1 })));
  return dropEmpty(next);
}

// Turns the channel entries of one provider in a tier back into a single provider entry, with
// their weights added up. Does nothing when the provider already has an entry elsewhere.
export function mergeChannels(tiers, t, provider) {
  if (!provider || tiers.some((tier) => tier.some((e) => e.provider === provider.key))) return tiers;
  const ids = new Set(provider.channels.map((c) => c.id));
  const next = clone(tiers);
  const own = next[t].filter((e) => e.channel_id && ids.has(e.channel_id));
  if (own.length === 0) return tiers;
  const weight = own.reduce((sum, e) => sum + (e.weight || 1), 0);
  const at = next[t].findIndex((e) => e.channel_id && ids.has(e.channel_id));
  next[t] = next[t].filter((e) => !(e.channel_id && ids.has(e.channel_id)));
  next[t].splice(at, 0, { provider: provider.key, weight });
  return next;
}

// Whole-number percentages of a tier's weights that add up to 100.
export function tierShares(tier = []) {
  const weights = tier.map((e) => Math.max(1, e.weight || 1));
  const total = weights.reduce((a, b) => a + b, 0);
  if (!total) return weights.map(() => 0);
  const raw = weights.map((w) => (w * 100) / total);
  const shares = raw.map(Math.floor);
  let rest = 100 - shares.reduce((a, b) => a + b, 0);
  const order = raw.map((r, i) => [r - Math.floor(r), i]).sort((a, b) => b[0] - a[0]);
  for (const [, i] of order) {
    if (rest <= 0) break;
    shares[i] += 1;
    rest -= 1;
  }
  return shares;
}

// Which provider a channel entry belongs to.
export function providerOfChannel(providers = [], channelId) {
  return providers.find((p) => p.channels.some((c) => c.id === channelId)) || null;
}

export const sameTiers = (a, b) => JSON.stringify(a) === JSON.stringify(b);

// Public models an entry serves: its channel's, or for a provider entry those of its channels
// that have no entry of their own.
export function entryModels(entry, providers = [], tiers = []) {
  if (entry.channel_id) {
    const owner = providerOfChannel(providers, entry.channel_id);
    return new Set(owner?.channels.find((c) => c.id === entry.channel_id)?.models || []);
  }
  const placed = new Set(
    tiers
      .flat()
      .filter((e) => e.channel_id)
      .map((e) => e.channel_id)
  );
  const provider = providers.find((p) => p.key === entry.provider);
  return new Set((provider?.channels || []).filter((c) => !placed.has(c.id)).flatMap((c) => c.models || []));
}

// How a tier really splits traffic. Entries only share for models they have in common, so the
// tier falls apart into groups of entries linked by a common model. Each entry gets its group
// (index of its first member) and its share in percent inside the group; an entry that shares
// no model with another one in the tier gets share null: its weight changes nothing.
export function tierSplit(tier = [], providers = [], tiers = []) {
  const models = tier.map((entry) => entryModels(entry, providers, tiers));
  const parent = tier.map((_, i) => i);
  const find = (i) => (parent[i] === i ? i : (parent[i] = find(parent[i])));
  for (let i = 0; i < tier.length; i++)
    for (let j = i + 1; j < tier.length; j++) {
      if ([...models[i]].some((m) => models[j].has(m))) parent[find(j)] = find(i);
    }
  const members = {};
  tier.forEach((_, i) => (members[find(i)] ||= []).push(i));
  const out = tier.map(() => ({ group: 0, share: null, peers: [] }));
  for (const list of Object.values(members)) {
    const shares = list.length > 1 ? tierShares(list.map((i) => tier[i])) : null;
    list.forEach((i, k) => {
      out[i] = { group: list[0], share: shares ? shares[k] : null, peers: list.filter((x) => x !== i) };
    });
  }
  return out;
}
