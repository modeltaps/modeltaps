// What the upstream reported it charged next to what the gateway expected, per model of a
// channel (GET /api/channel/upstream_costs). Only requests whose upstream reported a cost count.

// A difference at or above this share of the expected cost is flagged.
export const COST_DRIFT_FLAG = 0.05;

// (upstream - expected) / expected; null when nothing was expected.
export function costDrift(upstream, expected) {
  const e = Number(expected || 0);
  if (e <= 0) return null;
  return (Number(upstream || 0) - e) / e;
}

export function isFlagged(drift) {
  return drift !== null && Math.abs(drift) >= COST_DRIFT_FLAG;
}

// Rows of one channel with their drift, and the totals over them.
export function summarizeUpstreamCosts(rows = []) {
  const models = rows.map((r) => ({
    model: r.model_name,
    requests: Number(r.request_count || 0),
    upstream: Number(r.upstream_cost || 0),
    expected: Number(r.expected_cost || 0),
    drift: costDrift(r.upstream_cost, r.expected_cost)
  }));
  const total = models.reduce(
    (sum, m) => ({ requests: sum.requests + m.requests, upstream: sum.upstream + m.upstream, expected: sum.expected + m.expected }),
    { requests: 0, upstream: 0, expected: 0 }
  );
  return { models, total: { ...total, drift: costDrift(total.upstream, total.expected) } };
}

// "+20.0%" / "-3.2%" / "—".
export function formatDrift(drift) {
  if (drift === null) return '—';
  const pct = drift * 100;
  return (pct > 0 ? '+' : '') + pct.toFixed(1) + '%';
}

// Costs of single requests are small: four decimals below a dollar, two from there on.
export function formatCost(usd) {
  const n = Number(usd || 0);
  return '$' + n.toFixed(n >= 1 ? 2 : 4);
}
