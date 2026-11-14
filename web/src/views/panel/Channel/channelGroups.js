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

// A channel group of "*" means every user group, including groups created later.
export const ALL_GROUPS = '*';

// Whether a group list (array) means every user group.
export const isAllGroups = (groups = []) => groups.includes(ALL_GROUPS);

// Whether a channel serves `group`: it lists the group or is open to all groups.
export function channelInGroup(channel, group) {
  const groups = splitList(channel?.group);
  return isAllGroups(groups) || groups.includes(group);
}

// Whether two group lists share a group; a list open to all groups shares every group.
export function groupsOverlap(a = [], b = []) {
  if (!a.length || !b.length) return false;
  if (isAllGroups(a) || isAllGroups(b)) return true;
  return a.some((g) => b.includes(g));
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

// Issue condition values: any open issue, or one kind of it.
export const ISSUE_FILTERS = ['any', 'lowBalance', 'keyMissing', 'drift'];

// Whether a channel has an open issue of the given condition value.
function matchesIssue(channel, issue, dismissed) {
  const open = openIssues([channel], dismissed);
  if (issue === 'any') return open.length > 0;
  if (issue === 'drift') return open.some((i) => i.kind === 'driftNew' || i.kind === 'driftMissing');
  return open.some((i) => i.kind === issue);
}

// Channels passing the filters: `status` (a STATUS_FILTERS key), `tag` (a tag name or NO_TAG),
// `group` (one of the channel's groups), `model` (part of a model name) and `issue` (an
// ISSUE_FILTERS value). Empty values do not filter.
export function filterChannels(channels = [], { status, tag, group, model, issue } = {}, dismissed = new Set()) {
  const m = String(model || '')
    .trim()
    .toLowerCase();
  return channels.filter((c) => {
    if (status && STATUS_FILTERS[status] !== c.status) return false;
    if (tag === NO_TAG && c.tag) return false;
    if (tag && tag !== NO_TAG && c.tag !== tag) return false;
    if (group && !channelInGroup(c, group)) return false;
    if (m && !splitList(c.models).some((x) => x.toLowerCase().includes(m))) return false;
    if (issue && !matchesIssue(c, issue, dismissed)) return false;
    return true;
  });
}

// Count per status filter (plus `all`) over channels that pass every other filter, so each number
// equals the rows shown after picking that status.
export function statusCounts(channels = [], conditions, dismissed = new Set()) {
  const rest = typeof conditions === 'string' || conditions == null ? { tag: conditions } : { ...conditions, status: '' };
  const pool = filterChannels(channels, rest, dismissed);
  const counts = { all: pool.length };
  for (const [key, value] of Object.entries(STATUS_FILTERS)) counts[key] = pool.filter((c) => c.status === value).length;
  return counts;
}

// Groups named by channels, sorted. "All groups" is not a group to filter by: those channels
// match every group filter.
export function groupOptions(channels = []) {
  return Array.from(new Set(channels.flatMap((c) => splitList(c.group))))
    .filter((g) => g !== ALL_GROUPS)
    .sort((a, b) => a.localeCompare(b));
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
