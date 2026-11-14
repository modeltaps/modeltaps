import { describe, expect, it } from 'vitest';

import {
  followsChannel,
  pendingTarget,
  publicModelNames,
  channelGroups,
  countSuppliesByModel,
  defaultNewPriority,
  groupTiers,
  isPublicModelRow,
  modelSupplyPath,
  moveSupply,
  parseSupplyForm,
  pausesFor,
  previewEntryState,
  previewOutcome,
  previewShares,
  previewSupplyList,
  supplyCandidates,
  supplyState,
  supplyUpdateBody,
  upstreamFor
} from './supplyHelpers';

const supply = (id, priority, weight = 1, extra = {}) => ({
  id,
  model: 'gpt-a',
  channel_id: id * 10,
  upstream_model: 'gpt-a',
  priority,
  weight,
  enabled: true,
  channel_status: 1,
  ...extra
});

describe('isPublicModelRow', () => {
  it('accepts plain catalog rows only', () => {
    expect(isPublicModelRow({ catalogId: 3 })).toBe(true);
    expect(isPublicModelRow({ catalogId: 0 })).toBe(false);
    expect(isPublicModelRow({ catalogId: 3, alias_of: 'gpt-a' })).toBe(false);
    expect(isPublicModelRow({ catalogId: 3, kind: 'routing_policy' })).toBe(false);
    expect(isPublicModelRow(null)).toBe(false);
  });
});

describe('upstreamFor', () => {
  it('uses the mapping entry without the billing "+" prefix', () => {
    expect(upstreamFor({ model_mapping: '{"gpt-a":"+gpt-a-2025"}' }, 'gpt-a')).toBe('gpt-a-2025');
    expect(upstreamFor({ model_mapping: '{"gpt-a":"deploy-1"}' }, 'gpt-a')).toBe('deploy-1');
  });

  it('falls back to the model name', () => {
    expect(upstreamFor({ model_mapping: '{"other":"x"}' }, 'gpt-a')).toBe('gpt-a');
    expect(upstreamFor({ model_mapping: 'not json' }, 'gpt-a')).toBe('gpt-a');
    expect(upstreamFor({}, 'gpt-a')).toBe('gpt-a');
  });
});

describe('supplyCandidates', () => {
  it('drops channels already in use and lists the ones offering the model first', () => {
    const channels = [
      { id: 1, models: 'gpt-a,gpt-b' },
      { id: 2, models: 'gpt-b', model_mapping: '{"gpt-a":"x"}' },
      { id: 3, models: 'gpt-b' },
      { id: 4, models: 'gpt-a' }
    ];
    const { offering, others } = supplyCandidates(channels, [{ channel_id: 4 }], 'gpt-a');
    expect(offering.map((c) => c.id)).toEqual([1, 2]);
    expect(others.map((c) => c.id)).toEqual([3]);
  });
});

describe('groupTiers', () => {
  it('groups by priority, highest first, then by weight and id', () => {
    const tiers = groupTiers([supply(1, 0), supply(2, 10, 1), supply(3, 10, 5), supply(4, 10, 1)]);
    expect(tiers.map((t) => t.priority)).toEqual([10, 0]);
    expect(tiers[0].supplies.map((s) => s.id)).toEqual([3, 2, 4]);
  });
});

describe('moveSupply', () => {
  const list = [supply(1, 10), supply(2, 10), supply(3, 5), supply(4, 0)];

  it('moves into the neighbouring tier', () => {
    expect(moveSupply(list, 3, 'up')).toEqual([{ id: 3, priority: 10 }]);
    expect(moveSupply(list, 3, 'down')).toEqual([{ id: 3, priority: 0 }]);
  });

  it('opens a new top tier when the top tier is shared', () => {
    expect(moveSupply(list, 1, 'up')).toEqual([{ id: 1, priority: 11 }]);
  });

  it('cannot move a supply that is alone at the top or bottom', () => {
    expect(moveSupply(list, 4, 'down')).toEqual([]);
    expect(moveSupply([supply(1, 3), supply(2, 1)], 1, 'up')).toEqual([]);
    expect(moveSupply(list, 99, 'up')).toEqual([]);
  });

  it('opens a new bottom tier below a positive priority', () => {
    expect(moveSupply([supply(1, 4), supply(2, 4)], 2, 'down')).toEqual([{ id: 2, priority: 3 }]);
  });

  it('lifts the others instead of going below priority 0', () => {
    expect(moveSupply([supply(1, 5), supply(2, 0), supply(3, 0)], 3, 'down')).toEqual([
      { id: 1, priority: 6 },
      { id: 2, priority: 1 }
    ]);
  });
});

describe('defaultNewPriority', () => {
  it('puts a new supply below the lowest tier, never below 0', () => {
    expect(defaultNewPriority([])).toBe(0);
    expect(defaultNewPriority([supply(1, 10), supply(2, 4)])).toBe(3);
    expect(defaultNewPriority([supply(1, 0)])).toBe(0);
  });
});

describe('supplyState', () => {
  const now = 1000;

  it('reports a disabled channel before anything else', () => {
    expect(supplyState(supply(1, 0, 1, { channel_status: 2, enabled: false }), [], now).kind).toBe('channel_off');
  });

  it('reports a switched-off supply', () => {
    expect(supplyState(supply(1, 0, 1, { enabled: false }), [], now).kind).toBe('disabled');
  });

  it('reports the longest active pause on the model or the whole channel', () => {
    const s = supply(1, 0);
    const pauses = [
      { id: 1, channel_id: 10, model: 'gpt-a', until: 1500 },
      { id: 2, channel_id: 10, model: '', until: 2000 },
      { id: 3, channel_id: 10, model: 'gpt-b', until: 9000 },
      { id: 4, channel_id: 20, model: '', until: 9000 }
    ];
    const state = supplyState(s, pauses, now);
    expect(state.kind).toBe('paused');
    expect(state.pause.id).toBe(2);
    expect(state.pauses.map((p) => p.id)).toEqual([1, 2]);
  });

  it('matches a pause on the upstream name (before supplies route)', () => {
    const s = supply(1, 0, 1, { upstream_model: 'gpt-a-2025' });
    expect(pausesFor(s, [{ channel_id: 10, model: 'gpt-a-2025', until: 2000 }])).toHaveLength(1);
  });

  it('waits for the recovery test once a probing pause is due', () => {
    expect(supplyState(supply(1, 0), [{ id: 1, channel_id: 10, model: 'gpt-a', until: 900, probe: true }], now).kind).toBe('probing');
    expect(supplyState(supply(1, 0), [{ id: 1, channel_id: 10, model: 'gpt-a', until: 900 }], now).kind).toBe('ok');
  });
});

describe('previewEntryState', () => {
  const entry = (state, extra = {}) => ({ ...supply(1, 0), state, ...extra });

  it('maps the preview states to row states', () => {
    expect(previewEntryState(entry('channel_disabled')).kind).toBe('channel_off');
    expect(previewEntryState(entry('disabled')).kind).toBe('disabled');
    expect(previewEntryState(entry('available')).kind).toBe('ok');
    expect(previewEntryState(entry('cooling_down', { until: 1200 }))).toEqual({ kind: 'cooling', until: 1200, pauses: [] });
  });

  it('takes the pause from the pause list when it is there', () => {
    const pauses = [
      { id: 7, channel_id: 10, model: '', until: 2000, rule: 'quota' },
      { id: 8, channel_id: 10, model: 'gpt-a', until: 1500, rule: 'manual' }
    ];
    const state = previewEntryState(entry('paused', { pause_id: 7, until: 2000 }), pauses);
    expect(state.kind).toBe('paused');
    expect(state.pause.model).toBe('');
    expect(state.pauses.map((p) => p.id)).toEqual([7, 8]);
  });

  it('builds the pause from the preview otherwise', () => {
    const state = previewEntryState(entry('awaiting_test', { pause_id: 9, until: 900, pause_rule: 'quota', pause_reason: 'r' }));
    expect(state.kind).toBe('probing');
    expect(state.pause).toMatchObject({ id: 9, model: 'gpt-a', rule: 'quota', reason: 'r', probe: true });
    expect(state.pauses.map((p) => p.id)).toEqual([9]);
  });
});

describe('preview summary', () => {
  const preview = (extra = {}) => ({
    tiers: [
      { priority: 10, supplies: [{ id: 1, share: 0 }] },
      {
        priority: 0,
        supplies: [
          { id: 2, share: 0.75 },
          { id: 3, share: 0.25 },
          { id: 4, share: 0 }
        ]
      },
      { priority: -1, supplies: [{ id: 5, share: 0 }] }
    ],
    ...extra
  });

  it('lists the supplies in order', () => {
    expect(previewSupplyList(preview()).map((s) => s.id)).toEqual([1, 2, 3, 4, 5]);
  });

  it('gives the shares of the tier that takes requests', () => {
    expect(previewShares(preview())).toEqual({ priority: 0, shares: { 2: 75, 3: 25, 4: 0 }, count: 2 });
    expect(previewShares({ tiers: [{ priority: 0, supplies: [{ id: 1, share: 0 }] }] }).count).toBe(0);
  });

  it('names the first choice, a random split or no supply', () => {
    expect(previewOutcome(preview({ first_choice: 3 }))).toEqual({ kind: 'first', supply: { id: 3, share: 0.25 } });
    expect(previewOutcome(preview())).toEqual({ kind: 'split', count: 2 });
    expect(previewOutcome({ tiers: [] })).toEqual({ kind: 'none' });
  });
});

describe('channelGroups', () => {
  it('splits the channel group list', () => {
    expect(channelGroups({ channel_group: 'default, vip,,' })).toEqual(['default', 'vip']);
    expect(channelGroups({ groups: ['a'], channel_group: 'b' })).toEqual(['a']);
    expect(channelGroups({})).toEqual([]);
  });
});

describe('countSuppliesByModel', () => {
  it('counts all and usable supplies per model', () => {
    const counts = countSuppliesByModel([
      supply(1, 0),
      supply(2, 0, 1, { enabled: false }),
      supply(3, 0, 1, { channel_status: 2 }),
      supply(4, 0, 1, { model: 'gpt-b' })
    ]);
    expect(counts).toEqual({ 'gpt-a': { total: 3, usable: 1 }, 'gpt-b': { total: 1, usable: 1 } });
  });
});

describe('supplyUpdateBody', () => {
  it('sends every editable field with the changes applied', () => {
    expect(supplyUpdateBody(supply(1, 5, 2, { note: 'n' }), { enabled: false })).toEqual({
      id: 1,
      upstream_model: 'gpt-a',
      priority: 5,
      weight: 2,
      enabled: false,
      note: 'n'
    });
  });
});

describe('parseSupplyForm', () => {
  it('parses valid input', () => {
    expect(parseSupplyForm({ upstream_model: ' x ', priority: '3', weight: '0' })).toEqual({ upstream_model: 'x', priority: 3, weight: 0 });
  });

  it('rejects missing or invalid fields', () => {
    expect(parseSupplyForm({ upstream_model: ' ', priority: '1', weight: '1' }).error).toBe('modelSupply.upstreamRequired');
    expect(parseSupplyForm({ upstream_model: 'x', priority: '1.5', weight: '1' }).error).toBe('modelSupply.priorityInvalid');
    expect(parseSupplyForm({ upstream_model: 'x', priority: '', weight: '1' }).error).toBe('modelSupply.priorityInvalid');
    expect(parseSupplyForm({ upstream_model: 'x', priority: '1', weight: '-1' }).error).toBe('modelSupply.weightInvalid');
  });
});

describe('modelSupplyPath', () => {
  it('links to the supplies tab of the model page', () => {
    expect(modelSupplyPath('a/b c')).toBe('/panel/pricing?tab=models&model=a%2Fb%20c&panel=supplies');
  });
});

describe('pending upstream models', () => {
  const rows = [
    { model: 'claude-opus-5' },
    { model: 'opus', alias_of: 'claude-opus-5' },
    { model: 'opus-latest', alias_of: 'opus' },
    { model: 'auto/coder', kind: 'routing_policy' },
    { model: 'a-model' }
  ];

  it('lists public model names only', () => {
    expect(publicModelNames(rows)).toEqual(['a-model', 'claude-opus-5']);
  });

  it('tells what attaching to a name does', () => {
    expect(pendingTarget('claude-opus-5', rows)).toEqual({ kind: 'public', model: 'claude-opus-5' });
    expect(pendingTarget(' opus-latest ', rows)).toEqual({ kind: 'alias', model: 'opus-latest', canonical: 'claude-opus-5' });
    expect(pendingTarget('auto/coder', rows)).toEqual({ kind: 'policy', model: 'auto/coder' });
    expect(pendingTarget('brand-new', rows)).toEqual({ kind: 'new', model: 'brand-new' });
    expect(pendingTarget('  ', rows).kind).toBe('empty');
  });
});

describe('followsChannel', () => {
  const base = { channel_name: 'c', channel_priority: 10, channel_weight: 3 };
  it('is true while the supply has the channel numbers', () => {
    expect(followsChannel({ ...base, priority: 10, weight: 3 })).toBe(true);
    expect(followsChannel({ ...base, channel_weight: 1, priority: 10, weight: 0 })).toBe(true);
  });
  it('is false once priority or weight differs', () => {
    expect(followsChannel({ ...base, priority: 9, weight: 3 })).toBe(false);
    expect(followsChannel({ ...base, priority: 10, weight: 1 })).toBe(false);
  });
  it('does not flag supplies without channel numbers', () => {
    expect(followsChannel({ priority: 1, weight: 1 })).toBe(true);
    expect(followsChannel({ channel_priority: 5, channel_weight: 1, priority: 1, weight: 1 })).toBe(true);
  });
});
