// Pure helpers behind the channel overview. Input rows come from GET /api/channel/metrics:
// one row per channel × model × hour with request_count, success_count, latency_ms_sum
// (successful requests only), ttft_ms_sum / ttft_count (streams) and output_tokens /
// generation_ms.

const ratio = (num, den) => (den > 0 ? num / den : null);

function emptyTotals() {
  return { requests: 0, successes: 0, latency: 0, ttft: 0, ttftCount: 0, tokens: 0, genMs: 0 };
}

function addRow(acc, r) {
  acc.requests += r.request_count || 0;
  acc.successes += r.success_count || 0;
  acc.latency += r.latency_ms_sum || 0;
  acc.ttft += r.ttft_ms_sum || 0;
  acc.ttftCount += r.ttft_count || 0;
  acc.tokens += r.output_tokens || 0;
  acc.genMs += r.generation_ms || 0;
  return acc;
}

function finish(acc) {
  return {
    requests: acc.requests,
    successes: acc.successes,
    failures: Math.max(0, acc.requests - acc.successes),
    successRate: ratio(acc.successes, acc.requests),
    avgLatencyMs: ratio(acc.latency, acc.successes),
    avgTtftMs: ratio(acc.ttft, acc.ttftCount),
    tokensPerSec: acc.genMs > 0 ? acc.tokens / (acc.genMs / 1000) : null
  };
}

// Totals over every row: requests, successes, failures, success rate (0..1), average latency
// of successful requests, average time to first token and output speed. Rates are null when
// there is nothing to divide by.
export function summarizeMetrics(rows = []) {
  return finish(rows.reduce(addRow, emptyTotals()));
}

// One entry per model, busiest first.
export function metricsByModel(rows = []) {
  const map = new Map();
  for (const r of rows) {
    const key = r.model_name || '';
    map.set(key, addRow(map.get(key) || emptyTotals(), r));
  }
  return [...map.entries()].map(([model, acc]) => ({ model, ...finish(acc) })).sort((a, b) => b.requests - a.requests);
}

// Requests and failures per bucket over [start, end) (unix seconds), empty buckets included
// so the chart keeps an even time axis.
export function metricsSeries(rows = [], start, end, bucketSec) {
  const first = Math.floor(start / bucketSec) * bucketSec;
  const buckets = new Map();
  for (let ts = first; ts < end; ts += bucketSec) buckets.set(ts, { ts, requests: 0, failures: 0 });
  for (const r of rows) {
    const ts = Math.floor(r.bucket_ts / bucketSec) * bucketSec;
    const b = buckets.get(ts);
    if (!b) continue;
    b.requests += r.request_count || 0;
    b.failures += Math.max(0, (r.request_count || 0) - (r.success_count || 0));
  }
  return [...buckets.values()];
}

// Totals per channel (same figures as summarizeMetrics), keyed by channel id.
export function metricsByChannel(rows = []) {
  const map = new Map();
  for (const r of rows) {
    const id = Number(r.channel_id) || 0;
    map.set(id, addRow(map.get(id) || emptyTotals(), r));
  }
  return new Map([...map.entries()].map(([id, acc]) => [id, finish(acc)]));
}

// Requests per bucket and channel over [start, end) (unix seconds), for a chart with one line
// per channel. Every bucket is present and carries a count for every id in `channelIds` (0 when
// the channel had no traffic), so lines stay continuous. Rows of other channels are ignored.
export function channelSeries(rows = [], channelIds = [], start, end, bucketSec) {
  const ids = channelIds.map(Number);
  const wanted = new Set(ids);
  const first = Math.floor(start / bucketSec) * bucketSec;
  const buckets = new Map();
  for (let ts = first; ts < end; ts += bucketSec) {
    buckets.set(ts, { ts, values: Object.fromEntries(ids.map((id) => [id, 0])) });
  }
  for (const r of rows) {
    const id = Number(r.channel_id);
    if (!wanted.has(id)) continue;
    const b = buckets.get(Math.floor(r.bucket_ts / bucketSec) * bucketSec);
    if (b) b.values[id] += r.request_count || 0;
  }
  return [...buckets.values()];
}

// Prompt cache hit rate (cached read tokens over prompt tokens) and the share of requests that
// stayed on the channel their session was bound to. Null when there is no traffic.
export function cacheSummary(stat) {
  if (!stat) return { hitRate: null, affinityRate: null, requests: 0, mismatches: 0 };
  return {
    hitRate: ratio(stat.cached_read_tokens || 0, stat.prompt_tokens || 0),
    affinityRate: ratio(stat.affinity_hits || 0, stat.request_count || 0),
    requests: stat.request_count || 0,
    mismatches: stat.response_model_mismatches || 0
  };
}

// Upstream balance in the provider's currency: DeepSeek-style CNY types show ¥, the rest $.
const CNY_BALANCE_TYPES = new Set([28, 45]);
export function formatBalance(type, balance) {
  const n = Number(balance || 0);
  return CNY_BALANCE_TYPES.has(type) ? `¥${n.toFixed(2)}` : `$${n.toFixed(2)}`;
}
