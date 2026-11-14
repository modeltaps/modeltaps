import { describe, expect, it } from 'vitest';

import { isGone, matchesState, offerOf, snapshotOf, stateCounts, unlistedOf } from './channelModelStates';

describe('offerOf', () => {
  it('is offered when an enabled supply serves the model', () => {
    expect(
      offerOf({
        public: [
          { model: 'a', enabled: false },
          { model: 'b', enabled: true }
        ]
      })
    ).toEqual({ offered: true });
  });
  it('says why a model is not offered', () => {
    expect(offerOf({ public: [{ model: 'a', enabled: false }] })).toEqual({ offered: false, reason: 'off' });
    expect(offerOf({ public: [], unoffered: 'no_supply', suggested: 'gpt-5' })).toEqual({
      offered: false,
      reason: 'no_supply',
      suggested: 'gpt-5'
    });
    expect(offerOf({ public: [], unoffered: 'wildcard' })).toEqual({ offered: false, reason: 'wildcard', suggested: '' });
    expect(
      offerOf({ public: [], unoffered: 'duplicate', suggested: 'gpt-5.5-fast', same_as: 'gpt-5.5', supplied_by: 'openai/gpt-5.5' })
    ).toEqual({ offered: false, reason: 'duplicate', suggested: 'gpt-5.5-fast', sameAs: 'gpt-5.5', suppliedBy: 'openai/gpt-5.5' });
  });
  it('is unknown before the details load', () => {
    expect(offerOf(undefined)).toBeNull();
    expect(offerOf({})).toBeNull();
  });
});

describe('upstream snapshot', () => {
  const upstream = { checked_at: 1, models: [{ name: 'openai/gpt-5' }, { name: 'deploy-x' }, { name: 'xai/grok-6' }] };

  it('marks nothing when the upstream was never read', () => {
    expect(snapshotOf({ checked_at: 0, models: [] })).toBeNull();
    expect(snapshotOf({ checked_at: 5, models: [] })).toBeNull();
    expect(isGone('old/model', {}, null)).toBe(false);
    expect(unlistedOf({ checked_at: 0, models: [] }, [], {})).toEqual([]);
  });

  it('finds listed models the upstream dropped, through the mapping, never wildcards', () => {
    const snap = snapshotOf(upstream);
    expect(isGone('openai/gpt-5', {}, snap)).toBe(false);
    expect(isGone('alias', { alias: '+deploy-x' }, snap)).toBe(false);
    expect(isGone('old/model', {}, snap)).toBe(true);
    expect(isGone('claude-*', {}, snap)).toBe(false);
  });

  it('lists upstream models the channel lists neither by name nor as a mapping target', () => {
    expect(unlistedOf(upstream, ['openai/gpt-5', 'alias'], { alias: 'deploy-x' }).map((m) => m.name)).toEqual(['xai/grok-6']);
  });
});

describe('state filters', () => {
  const rows = [
    { offer: { offered: true }, gone: false },
    { offer: { offered: false, reason: 'not_in_catalog' }, gone: true },
    { offer: null, gone: false }
  ];
  it('counts each state', () => {
    expect(stateCounts(rows, [{ name: 'x' }])).toEqual({ offered: 1, unoffered: 1, gone: 1, unlisted: 1 });
  });
  it('matches rows to a filter', () => {
    expect(rows.filter((r) => matchesState(r, 'offered'))).toHaveLength(1);
    expect(rows.filter((r) => matchesState(r, 'unoffered'))).toHaveLength(1);
    expect(rows.filter((r) => matchesState(r, 'gone'))).toHaveLength(1);
  });
});
