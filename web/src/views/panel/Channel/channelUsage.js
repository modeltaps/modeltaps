import { groupsOverlap, splitList } from './channelGroups';

// "Usage" of a channel next to the other channels that serve the same requests: same provider
// type, sharing at least one group (a channel open to all groups shares every group), enabled. Priority decides which tier is tried first (higher
// first, lower tiers only when every channel above is unavailable) and weight splits traffic
// inside a tier. The options turn that into words relative to the highest-priority peer.

// Peers of `channel` ({ id?, type, group|groups }) among `all`: enabled channels of the same
// type sharing a group, highest priority first, then highest weight.
export function usagePeers(channel, all = []) {
  const groups = channel.groups || splitList(channel.group);
  return all
    .filter(
      (c) => c.id !== channel.id && c.status === 1 && Number(c.type) === Number(channel.type) && groupsOverlap(groups, splitList(c.group))
    )
    .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0) || (b.weight ?? 1) - (a.weight ?? 1) || a.id - b.id);
}

// Options for a channel among `peers` (from usagePeers). Without peers the channel is the only
// provider and keeps its numbers. With peers, relative to the top one:
//   backup - one tier below it, used only when it fails
//   share  - same tier, weight 1; `share` is this channel's traffic share in percent
//   first  - one tier above it, tried first
// `custom` is always last and carries the current numbers.
export function usageOptions(current, peers = []) {
  const priority = Number(current?.priority) || 0;
  const weight = Number(current?.weight) || 1;
  const top = peers[0] || null;
  const custom = { key: 'custom', priority, weight };
  if (!top) return { top: null, options: [{ key: 'only', priority, weight }, custom] };
  const topPriority = Number(top.priority) || 0;
  const tierWeight = peers.filter((p) => (Number(p.priority) || 0) === topPriority).reduce((a, p) => a + (Number(p.weight) || 1), 0);
  return {
    top,
    options: [
      { key: 'backup', priority: topPriority - 1, weight: 1 },
      { key: 'share', priority: topPriority, weight: 1, share: Math.round(100 / (tierWeight + 1)) },
      { key: 'first', priority: topPriority + 1, weight: 1 },
      custom
    ]
  };
}

// Which option the channel's current numbers correspond to; 'custom' when none matches exactly.
export function usageOf(current, peers = []) {
  const { options } = usageOptions(current, peers);
  const priority = Number(current?.priority) || 0;
  const weight = Number(current?.weight) || 1;
  const hit = options.find((o) => o.key !== 'custom' && o.priority === priority && o.weight === weight);
  return hit ? hit.key : 'custom';
}

// Priority and weight to save for a chosen option; custom uses the entered numbers.
export function usageNumbers(key, current, peers = [], custom = {}) {
  if (key === 'custom') {
    return { priority: Number(custom.priority) || 0, weight: Math.max(1, Number(custom.weight) || 1) };
  }
  const opt = usageOptions(current, peers).options.find((o) => o.key === key);
  if (!opt) return { priority: Number(current?.priority) || 0, weight: Number(current?.weight) || 1 };
  return { priority: opt.priority, weight: opt.weight };
}
