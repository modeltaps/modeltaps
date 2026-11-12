// Call order of the channels on the providers page, kept as the platform allocation policy.
// The page shows one list: positions tried first to last, a position holding one channel or a
// group of channels that share it by weight. Here the order is `groups`: [{ ids: [channelId],
// weights: { [id]: weight } }], every channel exactly once. It converts from and to the policy's
// tiers ([[{ provider | channel_id, weight }]]); provider entries are expanded into their channels.

import { tierSplit } from '../Allocation/allocationModel';

const gcd = (a, b) => (b ? gcd(b, a % b) : a);
const lcm = (a, b) => (a / gcd(a, b)) * b;

// Builds the order from the policy. A provider entry becomes its channels that have no entry of
// their own, sharing its weight equally; the tier's other weights are scaled so the split stays
// whole. Channels the policy does not place come last, one position each, oldest first. Entries
// for channels that no longer exist are dropped.
export function orderFromPolicy(tiers = [], providers = [], channels = []) {
  const exists = new Set(channels.map((c) => c.id));
  const ownEntry = new Set(
    tiers
      .flat()
      .filter((e) => e.channel_id)
      .map((e) => e.channel_id)
  );
  const placed = new Set();
  const groups = [];
  for (const tier of tiers) {
    const expanded = tier.map((entry) => {
      if (entry.channel_id) return exists.has(entry.channel_id) ? [entry.channel_id] : [];
      const provider = providers.find((p) => p.key === entry.provider);
      return (provider?.channels || []).map((c) => c.id).filter((id) => exists.has(id) && !ownEntry.has(id) && !placed.has(id));
    });
    const scale = expanded.reduce((acc, ids) => (ids.length > 1 ? lcm(acc, ids.length) : acc), 1);
    const ids = [];
    const weights = {};
    tier.forEach((entry, i) => {
      const list = expanded[i];
      const w = Math.max(1, entry.weight || 1) * scale;
      for (const id of list) {
        if (placed.has(id)) continue;
        placed.add(id);
        ids.push(id);
        weights[id] = entry.channel_id ? w : w / list.length;
      }
    });
    if (ids.length) groups.push({ ids, weights });
  }
  const rest = channels
    .map((c) => c.id)
    .filter((id) => !placed.has(id))
    .sort((a, b) => a - b);
  for (const id of rest) groups.push({ ids: [id], weights: { [id]: 1 } });
  return groups;
}

// The policy's tiers for an order: one channel entry per channel.
export function orderToTiers(groups = []) {
  return groups.map((g) => g.ids.map((id) => ({ channel_id: id, weight: Math.max(1, Math.round(g.weights[id] || 1)) })));
}

const clone = (groups) => groups.map((g) => ({ ids: [...g.ids], weights: { ...g.weights } }));
const indexOf = (groups, id) => groups.findIndex((g) => g.ids.includes(id));

// Takes a channel out of its group (dropping an emptied group) and returns it alone.
function takeOut(groups, id) {
  const gi = indexOf(groups, id);
  if (gi < 0) return null;
  groups[gi].ids = groups[gi].ids.filter((x) => x !== id);
  delete groups[gi].weights[id];
  if (groups[gi].ids.length === 0) groups.splice(gi, 1);
  return { ids: [id], weights: { [id]: 1 } };
}

// Moves a channel to its own position right before `beforeId` (or to the end when beforeId is
// empty). A channel dropped on its own group's member moves out of the group first.
export function moveBefore(groups, id, beforeId) {
  if (id === beforeId) return groups;
  const next = clone(groups);
  const item = takeOut(next, id);
  if (!item) return groups;
  const at = beforeId ? indexOf(next, beforeId) : -1;
  if (at < 0) next.push(item);
  else next.splice(at, 0, item);
  return next;
}

// Moves a channel's whole position one step up or down.
export function moveStep(groups, id, dir) {
  const gi = indexOf(groups, id);
  const to = dir === 'up' ? gi - 1 : gi + 1;
  if (gi < 0 || to < 0 || to >= groups.length) return groups;
  const next = clone(groups);
  [next[gi], next[to]] = [next[to], next[gi]];
  return next;
}

export const canMoveStep = (groups, id, dir) => {
  const gi = indexOf(groups, id);
  return dir === 'up' ? gi > 0 : gi >= 0 && gi < groups.length - 1;
};

// Joins a channel's position with the position above it, so they share traffic.
export function shareWithAbove(groups, id) {
  const gi = indexOf(groups, id);
  if (gi <= 0) return groups;
  const next = clone(groups);
  const [moved] = next.splice(gi, 1);
  next[gi - 1].ids.push(...moved.ids);
  Object.assign(next[gi - 1].weights, moved.weights);
  return next;
}

// Takes a channel out of a shared position into its own position right below it.
export function stopSharing(groups, id) {
  const gi = indexOf(groups, id);
  if (gi < 0 || groups[gi].ids.length < 2) return groups;
  const next = clone(groups);
  const item = takeOut(next, id);
  next.splice(gi + 1, 0, item);
  return next;
}

export function setWeight(groups, id, weight) {
  const gi = indexOf(groups, id);
  if (gi < 0) return groups;
  const next = clone(groups);
  next[gi].weights[id] = Math.max(1, Math.round(Number(weight)) || 1);
  return next;
}

// Per channel, how it is shown: { number, rank, clusterSize, clusterFirst, index, size, weight,
// share, peers }. Channels of one position only compete for the models they have in common, so a
// position falls apart into clusters of channels linked by a common model; each cluster gets the
// next number (a lone channel is a cluster of one). `rank` sorts rows: cluster first, then the
// channel's place in its position. share is the percentage inside the cluster, null for a
// cluster of one; peers are the other channels of the cluster.
export function orderInfo(groups = [], providers = []) {
  const info = new Map();
  let number = 0;
  let rank = 0;
  groups.forEach((g, gi) => {
    const tier = g.ids.map((id) => ({ channel_id: id, weight: g.weights[id] || 1 }));
    const split = g.ids.length > 1 ? tierSplit(tier, providers, [tier]) : g.ids.map(() => ({ group: 0, share: null, peers: [] }));
    const numberOf = new Map();
    const members = new Map();
    split.forEach((s, i) => {
      if (!members.has(s.group)) members.set(s.group, []);
      members.get(s.group).push(i);
    });
    for (const [key, list] of members) {
      number += 1;
      numberOf.set(key, number);
      // Models offered by at least two channels of the cluster: why they share.
      const seen = new Map();
      for (const i of list) for (const m of channelModels(providers, g.ids[i])) seen.set(m, (seen.get(m) || 0) + 1);
      const sharedModels = [...seen.values()].filter((n) => n > 1).length;
      list.forEach((i, k) => {
        const id = g.ids[i];
        info.set(id, {
          number,
          rank: rank++,
          clusterSize: list.length,
          clusterFirst: k === 0,
          index: gi,
          size: g.ids.length,
          weight: g.weights[id] || 1,
          share: split[i].share,
          sharedModels,
          peers: split[i].share !== null ? split[i].peers.map((x) => g.ids[x]) : []
        });
      });
    }
  });
  return info;
}

function channelModels(providers, id) {
  for (const p of providers) {
    const c = p.channels.find((x) => x.id === id);
    if (c) return c.models || [];
  }
  return [];
}

// Moves a channel into the position of `targetId`, sharing it (weight 1).
export function joinPosition(groups, id, targetId) {
  if (id === targetId) return groups;
  const next = clone(groups);
  const item = takeOut(next, id);
  const at = indexOf(next, targetId);
  if (!item || at < 0) return groups;
  next[at].ids.push(id);
  next[at].weights[id] = 1;
  return next;
}

export const sameOrder = (a, b) => JSON.stringify(orderToTiers(a)) === JSON.stringify(orderToTiers(b));

// Splits every position into the clusters of channels that share a model, each cluster a position
// of its own right where it was. Channels of one position that share no model never compete for a
// request, so this changes no routing; it makes every position a group that really shares.
export function normalizeOrder(groups = [], providers = []) {
  const out = [];
  for (const g of groups) {
    if (g.ids.length < 2) {
      out.push(g);
      continue;
    }
    const tier = g.ids.map((id) => ({ channel_id: id, weight: g.weights[id] || 1 }));
    const split = tierSplit(tier, providers, [tier]);
    const clusters = new Map();
    split.forEach((s, i) => {
      if (!clusters.has(s.group)) clusters.set(s.group, { ids: [], weights: {} });
      const c = clusters.get(s.group);
      c.ids.push(g.ids[i]);
      c.weights[g.ids[i]] = g.weights[g.ids[i]] || 1;
    });
    out.push(...clusters.values());
  }
  return out;
}

// Moves a channel to its own position right after `afterId`'s position.
export function moveAfter(groups, id, afterId) {
  if (id === afterId) return groups;
  const at = groups.findIndex((g) => g.ids.includes(afterId));
  const next = groups[at + 1]?.ids.find((x) => x !== id);
  if (next !== undefined && !groups[at + 1].ids.includes(id)) return moveBefore(groups, id, next);
  if (next !== undefined) return moveBefore(groups, id, groups[at + 2]?.ids[0] ?? null);
  return moveBefore(groups, id, groups[at + 2]?.ids[0] ?? null);
}
