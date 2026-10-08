import { describe, expect, it } from 'vitest';

import {
  COLUMN_STORAGE_KEY,
  STATUS_FILTERS,
  attachChannelDetails,
  LEGACY_CATALOG_REDIRECTS,
  billingType,
  buildCatalogExport,
  catalogStats,
  defaultColumnVisibility,
  PRICE_UNIT_STORAGE_KEY,
  indexChannelsByModel,
  isFullyUsable,
  isVisibleToUsers,
  loadColumnVisibility,
  loadPriceUnit,
  matchesStatusFilter,
  mergeCatalogRows,
  modelStatus,
  orphanModelNames,
  resolveCatalogTab,
  saveColumnVisibility,
  savePriceUnit
} from './modelCatalog';

const memoryStorage = (initial = {}) => {
  const data = { ...initial };
  return {
    getItem: (k) => (k in data ? data[k] : null),
    setItem: (k, v) => {
      data[k] = String(v);
    },
    data
  };
};

describe('legacy route redirects', () => {
  it('maps legacy pricing tabs to the new tabs', () => {
    expect(resolveCatalogTab('price')).toBe('models');
    expect(resolveCatalogTab('info')).toBe('models');
    expect(resolveCatalogTab('ownedby')).toBe('vendors');
  });

  it('keeps new tabs and falls back to models', () => {
    expect(resolveCatalogTab('models')).toBe('models');
    expect(resolveCatalogTab('vendors')).toBe('vendors');
    expect(resolveCatalogTab(null)).toBe('models');
    expect(resolveCatalogTab('bogus')).toBe('models');
  });

  it('redirects the standalone legacy pages to the new tabs', () => {
    expect(LEGACY_CATALOG_REDIRECTS.model_info).toBe('/panel/pricing?tab=models');
    expect(LEGACY_CATALOG_REDIRECTS.model_ownedby).toBe('/panel/pricing?tab=vendors');
  });
});

describe('column visibility persistence', () => {
  it('shows the default columns and hides the optional ones', () => {
    const v = defaultColumnVisibility();
    ['model', 'endpoints', 'channels', 'price', 'state', 'visible'].forEach((id) => expect(v[id]).toBe(true));
    ['vendor', 'context_length', 'max_tokens', 'modalities', 'tags', 'capabilities', 'mode', 'alias_of', 'source', 'synced_at'].forEach(
      (id) => expect(v[id]).toBe(false)
    );
  });

  it('round-trips a saved selection', () => {
    const storage = memoryStorage();
    saveColumnVisibility({ ...defaultColumnVisibility(), tags: true, vendor: true, price: false }, storage);
    const loaded = loadColumnVisibility(storage);
    expect(loaded.tags).toBe(true);
    expect(loaded.vendor).toBe(true);
    expect(loaded.price).toBe(false);
    expect(loaded.endpoints).toBe(true);
  });

  it('never hides the fixed model, state and visibility columns', () => {
    const storage = memoryStorage({ [COLUMN_STORAGE_KEY]: JSON.stringify({ model: false, state: false, visible: false }) });
    const loaded = loadColumnVisibility(storage);
    expect(loaded.model).toBe(true);
    expect(loaded.state).toBe(true);
    expect(loaded.visible).toBe(true);
  });

  it('falls back to defaults on corrupted or missing storage', () => {
    expect(loadColumnVisibility(memoryStorage({ [COLUMN_STORAGE_KEY]: '{oops' }))).toEqual(defaultColumnVisibility());
    expect(loadColumnVisibility(undefined)).toEqual(defaultColumnVisibility());
  });
});

describe('merging prices with the catalog by model name', () => {
  const catalog = [
    { id: 1, model: 'gpt-4o', name: 'GPT-4o', state: 'visible', bound_channels: [{ id: 9, name: 'openai', type: 1 }] },
    { id: 2, model: '4o', alias_of: 'gpt-4o', state: 'visible', bound_channels: [] }
  ];
  const prices = [
    { model: 'gpt-4o', type: 'tokens', input: 1, output: 2 },
    { model: 'priced-only', type: 'tokens', input: 3, output: 4 }
  ];

  it('attaches the price to the matching catalog row', () => {
    const rows = mergeCatalogRows({ catalog, prices });
    const row = rows.find((r) => r.model === 'gpt-4o');
    expect(row.catalogId).toBe(1);
    expect(row.price.input).toBe(1);
    expect(row.alias_count).toBe(1);
    expect(rows.find((r) => r.model === '4o').price).toBeNull();
  });

  it('adds priced or routable models missing from the catalog, state judged by channels', () => {
    const channelsByModel = indexChannelsByModel([
      { id: 5, name: 'or', type: 20, status: 1, models: 'channel-only, gpt-4o' },
      { id: 6, name: 'off', type: 20, status: 2, models: 'priced-only' }
    ]);
    const rows = mergeCatalogRows({ catalog, prices, modelList: ['channel-only', 'gpt-4o'], channelsByModel });
    const pricedOnly = rows.find((r) => r.model === 'priced-only');
    expect(pricedOnly).toMatchObject({ catalogId: 0, state: 'unrouted', bound_channels: [] });
    expect(pricedOnly.price.output).toBe(4);
    const channelOnly = rows.find((r) => r.model === 'channel-only');
    expect(channelOnly).toMatchObject({ catalogId: 0, state: 'visible', price: null });
    expect(channelOnly.bound_channels).toEqual([{ id: 5, name: 'or', type: 20 }]);
    expect(rows.filter((r) => r.model === 'gpt-4o')).toHaveLength(1);
  });

  it('lists only models without a catalog row as orphans', () => {
    expect(orphanModelNames({ catalog, prices, modelList: ['gpt-4o', 'channel-only'] }).sort()).toEqual(['channel-only', 'priced-only']);
  });
});

describe('attaching channel details for the channels column', () => {
  const channels = [
    { id: 5, name: 'or', type: 20, status: 1, models: 'm1,m2', base_url: 'https://or.example', priority: 3, weight: 1, key: 'sk-secret' },
    { id: 6, name: 'off', type: 1, status: 2, models: 'm1', priority: 0, weight: 1, key: 'sk-secret' }
  ];

  it('enriches bound channels by id and appends disabled channels carrying the model', () => {
    const rows = [
      { model: 'm1', state: 'visible', bound_channels: [{ id: 5, name: 'or', type: 20 }] },
      { model: 'm3', state: 'unrouted', bound_channels: [] }
    ];
    const [m1, m3] = attachChannelDetails(rows, channels);
    expect(m1.state).toBe('visible');
    expect(m1.bound_channels).toEqual([
      { id: 5, name: 'or', type: 20, status: 1, base_url: 'https://or.example', priority: 3, weight: 1 },
      { id: 6, name: 'off', type: 1, status: 2, priority: 0, weight: 1 }
    ]);
    expect(m3.bound_channels).toEqual([]);
  });

  it('never carries channel keys and keeps backend summaries for channels missing from the list', () => {
    const [row] = attachChannelDetails(
      [
        {
          model: 'm2',
          bound_channels: [
            { id: 5, name: 'or', type: 20 },
            { id: 77, name: 'tagged', type: 1 }
          ]
        }
      ],
      channels
    );
    expect(row.bound_channels.some((c) => 'key' in c)).toBe(false);
    expect(row.bound_channels[1]).toEqual({ id: 77, name: 'tagged', type: 1, status: 1 });
  });

  it('returns rows untouched when no channel list is loaded', () => {
    const rows = [{ model: 'm1', bound_channels: [{ id: 5, name: 'or', type: 20 }] }];
    expect(attachChannelDetails(rows, [])).toBe(rows);
  });
});

describe('status, visibility and stat chip counts', () => {
  const on = { id: 1, name: 'on', type: 1, status: 1 };
  const off = { id: 2, name: 'off', type: 1, status: 2 };
  const price = { model: 'x', type: 'tokens', input: 1, output: 2 };
  const rows = [
    { key: 'a', model: 'a', catalogId: 1, hidden: false, price, bound_channels: [on] },
    { key: 'b', model: 'b', catalogId: 2, hidden: true, price, bound_channels: [on] },
    { key: 'c', model: 'c', catalogId: 3, hidden: false, price: null, bound_channels: [on, off] },
    { key: 'd', model: 'd', catalogId: 4, hidden: false, price, bound_channels: [off] },
    { key: 'e', model: 'e', catalogId: 0, hidden: false, price: null, bound_channels: [] },
    {
      key: 'f',
      model: 'f',
      catalogId: 0,
      hidden: false,
      price: { ...price, type: 'times' },
      bound_channels: [{ id: 3, name: 'legacy', type: 1 }]
    }
  ];

  it('derives the status badge from enabled channels first, then price', () => {
    expect(rows.map(modelStatus)).toEqual(['ok', 'ok', 'unpriced', 'nochannel', 'nochannel', 'ok']);
  });

  it('treats only hidden catalog rows as invisible to users', () => {
    expect(rows.map(isVisibleToUsers)).toEqual([true, false, true, true, true, true]);
    expect(isVisibleToUsers({ catalogId: 0, hidden: true })).toBe(true);
  });

  it('requires channel, price and visibility at once for fully usable', () => {
    expect(rows.filter(isFullyUsable).map((r) => r.key)).toEqual(['a', 'f']);
  });

  it('treats chip facets as independent and overlapping', () => {
    const keys = (filter) => rows.filter((r) => matchesStatusFilter(r, filter)).map((r) => r.key);
    expect(keys('unpriced')).toEqual(['c', 'e']);
    expect(keys('nochannel')).toEqual(['d', 'e']);
    expect(keys('hidden')).toEqual(['b']);
    expect(keys('usable')).toEqual(['a', 'f']);
  });

  it('counts every chip exactly as many rows as its filter returns', () => {
    const stats = catalogStats(rows);
    expect(stats).toEqual({ all: 6, usable: 2, nochannel: 2, unpriced: 2, hidden: 1 });
    STATUS_FILTERS.forEach((filter) => expect(rows.filter((r) => matchesStatusFilter(r, filter))).toHaveLength(stats[filter]));
    expect(catalogStats([])).toEqual({ all: 0, usable: 0, nochannel: 0, unpriced: 0, hidden: 0 });
  });

  it('classifies billing type', () => {
    expect(rows.map(billingType)).toEqual(['tokens', 'tokens', 'none', 'tokens', 'none', 'times']);
  });
});

describe('price unit persistence', () => {
  it('defaults to M and round-trips K', () => {
    const storage = memoryStorage();
    expect(loadPriceUnit(storage)).toBe('M');
    savePriceUnit('K', storage);
    expect(storage.data[PRICE_UNIT_STORAGE_KEY]).toBe('K');
    expect(loadPriceUnit(storage)).toBe('K');
  });

  it('falls back to M on invalid or missing storage', () => {
    expect(loadPriceUnit(memoryStorage({ [PRICE_UNIT_STORAGE_KEY]: 'X' }))).toBe('M');
    expect(loadPriceUnit(undefined)).toBe('M');
  });
});

describe('catalog export', () => {
  it('writes the import format with catalog fields and prices, and no channel data', () => {
    const out = buildCatalogExport([
      {
        model: 'gpt-a',
        catalogId: 5,
        name: 'GPT A',
        context_length: 1000,
        max_tokens: 10,
        input_modalities: '["text","image"]',
        output_modalities: '["text"]',
        tags: 'bad json',
        price: { model: 'gpt-a', type: 'tokens', channel_type: 1, input: 1, output: 2, locked: true },
        bound_channels: [{ id: 1, name: 'secret-channel' }]
      },
      { model: 'orphan', catalogId: 0, price: null, bound_channels: [] }
    ]);
    expect(out.data[0].model_info).toMatchObject({ model: 'gpt-a', name: 'GPT A', input_modalities: ['text', 'image'], tags: [] });
    expect(out.data[0].price).toEqual({ type: 'tokens', channel_type: 1, input: 1, output: 2 });
    expect(out.data[1]).toEqual({ model: 'orphan', model_info: null, price: null });
    expect(JSON.stringify(out)).not.toContain('secret-channel');
  });
});
