// Pure helpers for the subscription usage alerts on the channel's monitoring tab: which tiers
// apply to a channel, and reading the tiers an admin types.

// Claude Code reports its usage on each response rather than through an endpoint.
const CHANNEL_TYPE_CLAUDE_CODE = 58;

// The tiers the site falls back to when none are set (model.DefaultUsageAlertTiers).
export const DEFAULT_USAGE_TIERS = [50, 75, 100];

// Reads tiers typed as "50, 75, 100" (a % sign or a full-width comma are fine). Returns
// { value } in the stored form ("50,75,100", "0" for off, '' for not set) or { error: true }.
// Same rules as model.ParseUsageAlertTiers.
export function parseUsageTiers(text) {
  const raw = String(text ?? '').trim();
  if (!raw) return { value: '' };
  if (raw === '0' || raw.toLowerCase() === 'off') return { value: '0' };
  const parts = raw.split(/[\s,，%]+/).filter(Boolean);
  if (parts.length === 0) return { error: true };
  const tiers = [];
  for (const part of parts) {
    const tier = Number(part);
    if (!Number.isFinite(tier) || tier < 1 || tier > 100) return { error: true };
    if (!tiers.includes(tier)) tiers.push(tier);
  }
  tiers.sort((a, b) => a - b);
  return { value: tiers.join(',') };
}

// The tiers in force for a channel and where they come from:
//   { tiers: [50, 75, 100], source: 'channel' | 'site' }; tiers is empty when the alerts are off.
// `siteTiers` is usage_alert.tiers from GET /api/channel/capabilities (null while loading).
export function effectiveUsageTiers(channelTiers, siteTiers) {
  const own = parseUsageTiers(channelTiers);
  if (!own.error && own.value) {
    return { tiers: own.value === '0' ? [] : own.value.split(',').map(Number), source: 'channel' };
  }
  return { tiers: Array.isArray(siteTiers) ? siteTiers : DEFAULT_USAGE_TIERS, source: 'site' };
}

export const formatTiers = (tiers) => tiers.map((tier) => `${tier}%`).join(', ');

// Where the channel's usage comes from: 'headers' (Claude Code responses) or 'api' (queried).
export const usageSource = (type) => (Number(type) === CHANNEL_TYPE_CLAUDE_CODE ? 'headers' : 'api');
