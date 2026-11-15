import { describe, expect, it } from 'vitest';
import { costDrift, formatCost, formatDrift, isFlagged, summarizeUpstreamCosts } from './upstreamCosts';

describe('upstream costs', () => {
  it('measures the difference against the expected cost', () => {
    expect(costDrift(1.2, 1)).toBeCloseTo(0.2);
    expect(costDrift(0.9, 1)).toBeCloseTo(-0.1);
    expect(costDrift(1, 0)).toBeNull();
  });

  it('flags a difference of 5% or more either way', () => {
    expect(isFlagged(0.2)).toBe(true);
    expect(isFlagged(-0.05)).toBe(true);
    expect(isFlagged(0.049)).toBe(false);
    expect(isFlagged(null)).toBe(false);
  });

  it('sums the rows of a channel', () => {
    const { models, total } = summarizeUpstreamCosts([
      { model_name: 'a', request_count: 2, upstream_cost: 1.8, expected_cost: 1.5 },
      { model_name: 'b', request_count: 1, upstream_cost: 0.1, expected_cost: 0.1 }
    ]);
    expect(models.map((m) => m.model)).toEqual(['a', 'b']);
    expect(models[0].drift).toBeCloseTo(0.2);
    expect(total.requests).toBe(3);
    expect(total.upstream).toBeCloseTo(1.9);
    expect(total.drift).toBeCloseTo(0.1875);
  });

  it('formats', () => {
    expect(formatDrift(0.2)).toBe('+20.0%');
    expect(formatDrift(-0.032)).toBe('-3.2%');
    expect(formatDrift(0)).toBe('0.0%');
    expect(formatDrift(null)).toBe('—');
    expect(formatCost(0.0048)).toBe('$0.0048');
    expect(formatCost(12.345)).toBe('$12.35');
  });
});
