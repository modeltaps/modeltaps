import { describe, expect, it } from 'vitest';

import {
  cacheSummary,
  channelSeries,
  metricsByChannel,
  metricsByModel,
  metricsSeries,
  modelSignal,
  summarizeMetrics
} from './channelMetrics';

const rows = [
  {
    bucket_ts: 3600,
    model_name: 'a',
    request_count: 10,
    success_count: 8,
    latency_ms_sum: 8000,
    ttft_ms_sum: 900,
    ttft_count: 3,
    output_tokens: 500,
    generation_ms: 5000
  },
  {
    bucket_ts: 7200,
    model_name: 'b',
    request_count: 30,
    success_count: 30,
    latency_ms_sum: 15000,
    ttft_ms_sum: 0,
    ttft_count: 0,
    output_tokens: 0,
    generation_ms: 0
  },
  {
    bucket_ts: 7200,
    model_name: 'a',
    request_count: 2,
    success_count: 2,
    latency_ms_sum: 2000,
    ttft_ms_sum: 300,
    ttft_count: 1,
    output_tokens: 100,
    generation_ms: 1000
  }
];

describe('summarizeMetrics', () => {
  it('totals the rows and derives the rates', () => {
    const s = summarizeMetrics(rows);
    expect(s.requests).toBe(42);
    expect(s.failures).toBe(2);
    expect(s.successRate).toBeCloseTo(40 / 42);
    expect(s.avgLatencyMs).toBeCloseTo(25000 / 40);
    expect(s.avgTtftMs).toBe(300);
    expect(s.tokensPerSec).toBe(100);
  });

  it('leaves the rates empty without traffic', () => {
    const s = summarizeMetrics([]);
    expect(s.requests).toBe(0);
    expect(s.successRate).toBeNull();
    expect(s.avgLatencyMs).toBeNull();
    expect(s.tokensPerSec).toBeNull();
  });
});

describe('metricsByModel', () => {
  it('groups per model, busiest first', () => {
    const m = metricsByModel(rows);
    expect(m.map((x) => x.model)).toEqual(['b', 'a']);
    expect(m[1].requests).toBe(12);
    expect(m[1].successRate).toBeCloseTo(10 / 12);
  });
});

describe('metricsSeries', () => {
  it('fills empty buckets and drops rows outside the range', () => {
    const s = metricsSeries([...rows, { bucket_ts: 99999, request_count: 5, success_count: 5 }], 0, 3 * 3600, 3600);
    expect(s.map((b) => b.ts)).toEqual([0, 3600, 7200]);
    expect(s.map((b) => b.requests)).toEqual([0, 10, 32]);
    expect(s.map((b) => b.failures)).toEqual([0, 2, 0]);
  });

  it('merges hourly rows into wider buckets', () => {
    const s = metricsSeries(rows, 0, 4 * 3600, 2 * 3600);
    expect(s.map((b) => b.requests)).toEqual([10, 32]);
  });
});

describe('cacheSummary', () => {
  it('computes hit and affinity rates', () => {
    const c = cacheSummary({
      request_count: 4,
      prompt_tokens: 1000,
      cached_read_tokens: 750,
      affinity_hits: 3,
      response_model_mismatches: 1
    });
    expect(c.hitRate).toBe(0.75);
    expect(c.affinityRate).toBe(0.75);
    expect(c.mismatches).toBe(1);
  });

  it('handles a channel without traffic', () => {
    expect(cacheSummary(undefined).hitRate).toBeNull();
    expect(cacheSummary({ request_count: 0, prompt_tokens: 0 }).hitRate).toBeNull();
  });
});

describe('per-channel helpers', () => {
  const multi = [
    { bucket_ts: 0, channel_id: 1, request_count: 4, success_count: 3, latency_ms_sum: 300 },
    { bucket_ts: 1800, channel_id: 1, request_count: 2, success_count: 2, latency_ms_sum: 200 },
    { bucket_ts: 3600, channel_id: 2, request_count: 5, success_count: 5, latency_ms_sum: 500 },
    { bucket_ts: 3600, channel_id: 9, request_count: 99, success_count: 99, latency_ms_sum: 0 }
  ];

  it('channelSeries buckets requests per channel with zeros filled in', () => {
    expect(channelSeries(multi, [1, 2], 0, 7200, 3600)).toEqual([
      { ts: 0, values: { 1: 6, 2: 0 } },
      { ts: 3600, values: { 1: 0, 2: 5 } }
    ]);
  });

  it('metricsByChannel sums per channel', () => {
    const m = metricsByChannel(multi);
    expect(m.get(1).requests).toBe(6);
    expect(m.get(1).successRate).toBeCloseTo(5 / 6);
    expect(m.get(1).avgLatencyMs).toBe(100);
    expect(m.get(2).requests).toBe(5);
  });
});

describe('modelSignal', () => {
  it('flags nothing when the channel had no traffic', () => {
    expect(modelSignal(undefined, false)).toBe(null);
  });
  it('flags a model without requests as idle', () => {
    expect(modelSignal(undefined, true)).toBe('idle');
    expect(modelSignal({ requests: 0, successRate: null }, true)).toBe('idle');
  });
  it('flags a model whose requests mostly fail', () => {
    expect(modelSignal({ requests: 10, successRate: 0.5 }, true)).toBe('failing');
  });
  it('needs enough requests before calling a model failing', () => {
    expect(modelSignal({ requests: 4, successRate: 0 }, true)).toBe(null);
  });
  it('leaves a healthy model alone', () => {
    expect(modelSignal({ requests: 10, successRate: 0.8 }, true)).toBe(null);
  });
});
