// Pure helpers behind the provider page: channels grouped by provider type, the order of rows
// inside a provider, and the attention items a channel raises.

// Comma-separated list field (groups, models) as a trimmed, de-duplicated array.
export function splitList(value) {
  return Array.from(
    new Set(
      String(value || '')
        .split(/[,\n]/)
        .map((s) => s.trim())
        .filter(Boolean)
    )
  );
}

// Rows of one provider: highest priority first, then highest weight, then oldest.
export function sortChannels(channels = []) {
  return [...channels].sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0) || (b.weight ?? 1) - (a.weight ?? 1) || a.id - b.id);
}

// One entry per channel type present: { type, channels }. Providers with more channels come
// first, ties keep the lower type id first, so the default selection is stable.
export function groupChannelsByType(channels = []) {
  const map = new Map();
  for (const c of channels) {
    const type = Number(c.type) || 0;
    if (!map.has(type)) map.set(type, []);
    map.get(type).push(c);
  }
  return [...map.entries()]
    .map(([type, list]) => ({ type, channels: sortChannels(list) }))
    .sort((a, b) => b.channels.length - a.channels.length || a.type - b.type);
}

// Low balance: an upstream balance was read and it is below the channel's alert threshold.
// Subscription channels (usage windows) have no money balance and never count.
export function hasLowBalance(channel) {
  if (!channel || channel.usage_windows?.windows?.length) return false;
  const threshold = Number(channel.balance_threshold) || 0;
  return threshold > 0 && channel.balance_updated_time > 0 && Number(channel.balance || 0) < threshold;
}

// Attention items of one channel, in display order. Each is { key, kind, channel, count? }:
// autoDisabled (status 3), keyMissing (no stored key), lowBalance, driftNew (models the
// upstream added) and driftMissing (models the upstream no longer lists). `key` identifies the
// item for dismissal and changes when the underlying data changes, so a dismissed item comes
// back once there is something new to say.
export function channelIssues(channel) {
  if (!channel) return [];
  const issues = [];
  const push = (kind, sig, extra = {}) => issues.push({ key: `${channel.id}:${kind}:${sig}`, kind, channel, ...extra });
  if (channel.status === 3) push('autoDisabled', channel.test_time || 0);
  if (channel.key_status && !channel.key_status.configured) push('keyMissing', 0);
  if (hasLowBalance(channel)) push('lowBalance', channel.balance_updated_time);
  const drift = channel.model_drift;
  const added = drift?.new_models || [];
  const missing = drift?.missing_models || [];
  if (added.length) push('driftNew', added.join(','), { count: added.length });
  if (missing.length) push('driftMissing', missing.join(','), { count: missing.length });
  return issues;
}

// Issues of several channels minus the dismissed ones.
export function openIssues(channels = [], dismissed = new Set()) {
  return channels.flatMap(channelIssues).filter((i) => !dismissed.has(i.key));
}

// Status dot of a row: off (disabled by hand), warn (auto-disabled or an open issue) or live.
export function channelDot(channel, dismissed = new Set()) {
  if (channel.status === 3) return 'warn';
  if (channel.status !== 1) return 'off';
  return openIssues([channel], dismissed).length ? 'warn' : 'live';
}

// Host of a base URL for search; the raw value when it does not parse.
function hostText(url) {
  if (!url) return '';
  try {
    return new URL(url).host;
  } catch {
    return String(url);
  }
}

// Whether a channel matches a search query: name, models, tag, the visible suffix of its masked
// key and the host of its base URL, case-insensitive. An empty query matches everything.
export function channelMatches(channel, query) {
  const q = String(query || '')
    .trim()
    .toLowerCase();
  if (!q) return true;
  const keys = (channel.key_status?.masked || []).map((m) => String(m).split('*').pop());
  const fields = [channel.name, channel.tag, hostText(channel.base_url), ...splitList(channel.models), ...keys];
  return fields.some((f) => f && String(f).toLowerCase().includes(q));
}

// Status filter values: enabled (1), manual (2, disabled by hand), auto (3, disabled by the
// system).
export const STATUS_FILTERS = { enabled: 1, manual: 2, auto: 3 };
// Tag filter value for channels without a tag.
export const NO_TAG = '\u0000';

// Channels passing the status and tag filters. `status` is a STATUS_FILTERS key or empty;
// `tag` a tag name, NO_TAG, or empty.
export function filterChannels(channels = [], { status, tag } = {}) {
  return channels.filter((c) => {
    if (status && STATUS_FILTERS[status] !== c.status) return false;
    if (tag === NO_TAG) return !c.tag;
    if (tag) return c.tag === tag;
    return true;
  });
}

// Count per status filter (plus `all`) over channels that pass the tag filter, so each number
// equals the rows shown after picking that status.
export function statusCounts(channels = [], tag) {
  const pool = filterChannels(channels, { tag });
  const counts = { all: pool.length };
  for (const [key, value] of Object.entries(STATUS_FILTERS)) counts[key] = pool.filter((c) => c.status === value).length;
  return counts;
}

// Tags present among channels, sorted, and whether any channel has no tag.
export function tagOptions(channels = []) {
  const tags = Array.from(new Set(channels.map((c) => c.tag).filter(Boolean))).sort((a, b) => a.localeCompare(b));
  return { tags, hasUntagged: channels.some((c) => !c.tag) };
}

// Header checkbox state for the visible rows: none, some or all of them selected.
export function selectionState(visibleIds = [], selected = new Set()) {
  const n = visibleIds.filter((id) => selected.has(id)).length;
  if (!n) return 'none';
  return n === visibleIds.length ? 'all' : 'some';
}
