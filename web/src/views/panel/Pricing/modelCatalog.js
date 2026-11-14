// ==============================|| MODEL CATALOG — PURE HELPERS ||============================== //
// 模型页(「模型」表 + 「厂商」页签)的纯函数:页签解析、旧地址映射、列偏好持久化、
// 价格与目录按模型名合并。无 React / API 依赖,便于单测。

export const CATALOG_TABS = ['models', 'vendors', 'paused', 'pending', 'rules'];
export const DEFAULT_CATALOG_TAB = 'models';

// 旧页签值(价格 / 目录 / 厂商)→ 新页签。
const LEGACY_TAB_MAP = { price: 'models', info: 'models', ownedby: 'vendors' };

export function resolveCatalogTab(requested) {
  if (CATALOG_TABS.includes(requested)) return requested;
  return LEGACY_TAB_MAP[requested] || DEFAULT_CATALOG_TAB;
}

// 旧独立页面地址 → 模型页新页签(供路由表 Navigate 使用)。
export const LEGACY_CATALOG_REDIRECTS = {
  model_info: '/panel/pricing?tab=models',
  model_ownedby: '/panel/pricing?tab=vendors'
};

// Column registry for the shared column picker: fixed columns always show and stay out of the
// menu; defaultHidden columns start hidden. Channels are one click away: the state column already
// says when a model has none.
export const MODEL_COLUMNS = [
  { id: 'select', fixed: true },
  { id: 'model', labelKey: 'modelInfoPage.model', fixed: true },
  { id: 'endpoints', labelKey: 'modelInfoPage.endpoints' },
  { id: 'channels', labelKey: 'modelInfoPage.channels', defaultHidden: true },
  { id: 'supply', labelKey: 'modelSupply.column' },
  { id: 'price', labelKey: 'modelInfoPage.price' },
  { id: 'state', labelKey: 'modelInfoPage.state', fixed: true },
  { id: 'visible', labelKey: 'modelsPage.visibleToUsers', fixed: true },
  { id: 'vendor', labelKey: 'modelInfoPage.vendor', defaultHidden: true },
  { id: 'context_length', labelKey: 'modelInfoPage.contextLength', defaultHidden: true },
  { id: 'max_tokens', labelKey: 'modelInfoPage.maxTokens', defaultHidden: true },
  { id: 'modalities', labelKey: 'modelInfoPage.modalities', defaultHidden: true },
  { id: 'tags', labelKey: 'modelInfoPage.tags', defaultHidden: true },
  { id: 'capabilities', labelKey: 'modelInfoPage.capabilities', defaultHidden: true },
  { id: 'mode', labelKey: 'modelInfoPage.mode', defaultHidden: true },
  { id: 'alias_of', labelKey: 'modelInfoPage.aliasOf', defaultHidden: true },
  { id: 'source', labelKey: 'modelInfoPage.source', defaultHidden: true },
  { id: 'synced_at', labelKey: 'modelInfoPage.syncedAt', defaultHidden: true },
  { id: 'actions', fixed: true }
];

// The column picker's storage key (v2: channels became optional); the old key is cleaned up.
export const COLUMN_STORAGE_KEY = 'model-catalog-columns-v2';
export const LEGACY_COLUMN_STORAGE_KEYS = ['model-catalog-columns'];

const CHANNEL_STATUS_ENABLED = 1;

export const defaultColumnVisibility = () => Object.fromEntries(MODEL_COLUMNS.map((c) => [c.id, !!c.fixed || !c.defaultHidden]));

export const PRICE_UNIT_STORAGE_KEY = 'model-catalog-price-unit';

// 价格单位 K / M;存储值非法或不可用时回落 M。
export function loadPriceUnit(storage = globalThis.localStorage) {
  try {
    return storage?.getItem(PRICE_UNIT_STORAGE_KEY) === 'K' ? 'K' : 'M';
  } catch {
    return 'M';
  }
}

export function savePriceUnit(unit, storage = globalThis.localStorage) {
  try {
    storage?.setItem(PRICE_UNIT_STORAGE_KEY, unit === 'K' ? 'K' : 'M');
  } catch {
    /* storage unavailable -> keep in-memory only */
  }
}

// 渠道列表(/api/channel/ 分页结果)→ 模型名 → [{ id, name, type }];与后端 bound_channels 一致,只计启用渠道。
export function indexChannelsByModel(channels = []) {
  const index = {};
  channels.forEach((channel) => {
    if (channel?.status !== CHANNEL_STATUS_ENABLED) return;
    String(channel?.models || '')
      .split(',')
      .map((name) => name.trim())
      .filter(Boolean)
      .forEach((name) => {
        (index[name] ||= []).push({ id: channel.id, name: channel.name, type: channel.type });
      });
  });
  return index;
}

// Names any channel lists, enabled or not: the models the catalog page shows even without a
// catalog row. A name a supply of the same channel already sends upstream is left out
// (`suppliedUpstreams`, see suppliedUpstreamKeys): it is a platform spelling of a public model,
// such as "anthropic/claude-opus-5" for claude-opus-5, not a model of its own.
export function channelModelNames(channels = [], suppliedUpstreams = null) {
  const names = new Set();
  channels.forEach((channel) => {
    String(channel?.models || '')
      .split(',')
      .map((name) => name.trim())
      .filter(Boolean)
      .filter((name) => !suppliedUpstreams?.has(suppliedUpstreamKey(channel?.id, name)))
      .forEach((name) => names.add(name));
  });
  return names;
}

const suppliedUpstreamKey = (channelId, upstream) => `${channelId}\u0000${upstream}`;

// Supplies (/api/model_supply/) → the (channel, upstream name) pairs they send.
export function suppliedUpstreamKeys(supplies = []) {
  return new Set(
    supplies.filter((s) => s?.channel_id && s?.upstream_model).map((s) => suppliedUpstreamKey(s.channel_id, s.upstream_model))
  );
}

// 目录行、价格行与渠道可提供的模型名按模型名合并为一张表:
// - 目录行保留全部字段(含后端计算的 state / bound_channels),附上同名价格;
// - 目录无行、但有渠道列出的模型也出现(附上同名价格),状态按渠道判断(有启用渠道 = visible,否则 unrouted);
// - 只有价格、没有目录行也没有渠道列出的模型不出现:它们在模型库里,要用时从库里添加;
//   不传 listedModels 时(旧调用方)沿用价格表 + modelList 的名字;
// - key 为模型名(唯一),catalogId 为目录行 id(无目录行为 0)。
export function mergeCatalogRows({ catalog = [], prices = [], modelList = [], channelsByModel = {}, listedModels } = {}) {
  const priceByModel = {};
  prices.forEach((price) => {
    if (price?.model) priceByModel[price.model] = price;
  });

  const aliasesByTarget = {};
  catalog.forEach((info) => {
    if (info.alias_of) (aliasesByTarget[info.alias_of] ||= []).push(info.model);
  });

  const seen = new Set();
  const rows = catalog.map((info) => {
    seen.add(info.model);
    return {
      ...info,
      key: info.model,
      catalogId: info.id,
      price: priceByModel[info.model] || null,
      alias_count: aliasesByTarget[info.model]?.length || 0,
      bound_channels: Array.isArray(info.bound_channels) ? info.bound_channels : []
    };
  });

  const candidates = listedModels ? [...listedModels] : [...prices.map((p) => p?.model), ...modelList];
  const orphanNames = candidates.filter((name) => name && !seen.has(name));
  [...new Set(orphanNames)].sort().forEach((name) => {
    const channels = channelsByModel[name] || [];
    rows.push({
      key: name,
      catalogId: 0,
      id: 0,
      model: name,
      name: '',
      hidden: false,
      state: channels.length > 0 ? 'visible' : 'unrouted',
      bound_channels: channels,
      price: priceByModel[name] || null,
      alias_count: aliasesByTarget[name]?.length || 0
    });
  });

  return rows;
}

// 渠道列展示所需字段(白名单,key 等敏感字段一律不带出)。
const CHANNEL_DETAIL_FIELDS = ['id', 'name', 'type', 'status', 'group', 'base_url', 'priority', 'weight', 'tag'];

function pickChannelDetail(channel) {
  return Object.fromEntries(CHANNEL_DETAIL_FIELDS.filter((field) => channel[field] !== undefined).map((field) => [field, channel[field]]));
}

// 用完整渠道列表按 id 补全各行 bound_channels 的状态 / base_url / 优先级 / 权重,
// 并在末尾追加仍配置了该模型的非启用渠道(仅用于置灰展示,不影响 state 判定)。
// While supplies route requests (`supplyRouting`), bound_channels come from the supplies and
// already include disabled channels; a channel's own model list says nothing, so nothing is
// appended from it, and the supply's priority / weight win over the channel's.
export function attachChannelDetails(rows = [], channels = [], { supplyRouting = false } = {}) {
  if (!channels.length) return rows;
  const byId = new Map();
  const inactiveByModel = {};
  channels.forEach((channel) => {
    if (!channel?.id) return;
    const detail = pickChannelDetail(channel);
    byId.set(channel.id, detail);
    if (supplyRouting || channel.status === CHANNEL_STATUS_ENABLED) return;
    String(channel.models || '')
      .split(',')
      .map((name) => name.trim())
      .filter(Boolean)
      .forEach((name) => (inactiveByModel[name] ||= []).push(detail));
  });
  return rows.map((row) => {
    const bound = (row.bound_channels || []).map((summary) => ({ status: CHANNEL_STATUS_ENABLED, ...byId.get(summary.id), ...summary }));
    const boundIds = new Set(bound.map((channel) => channel.id));
    const inactive = (inactiveByModel[row.model] || []).filter((channel) => !boundIds.has(channel.id));
    return { ...row, bound_channels: [...bound, ...inactive] };
  });
}

// ---- 状态 / 可见性 / 统计 ----
// 状态徽章三态:无启用渠道优先于未定价;「对用户可见」只看目录行的 hidden(无目录行不可切换,视为可见)。
export const enabledChannels = (row) =>
  (row?.bound_channels || []).filter((c) => (c?.status ?? CHANNEL_STATUS_ENABLED) === CHANNEL_STATUS_ENABLED);

// While supplies route requests (`row.supplyRouting`), a public model is listed only when
// published: its status replaces the hidden flag.
export const followsStatus = (row) => Boolean(row?.supplyRouting && row.catalogId > 0 && !row.alias_of && !row.kind);

// A draft cannot be called while supplies route requests, so a priced draft with channels is
// "unpublished", not "ok".
export function modelStatus(row) {
  if (enabledChannels(row).length === 0) return 'nochannel';
  if (!row?.price) return 'unpriced';
  if (followsStatus(row) && row.status === 'draft') return 'unpublished';
  return 'ok';
}

export const isVisibleToUsers = (row) => {
  if (followsStatus(row)) return row.status === 'published';
  return !(row?.catalogId > 0 && row.hidden);
};

// 渠道分组串(逗号分隔)→ 去空数组。
export const splitChannelGroups = (group) =>
  String(group || '')
    .split(',')
    .map((g) => g.trim())
    .filter(Boolean);

// Groups the model is available in: the union of its enabled channels' groups, sorted. A channel
// open to all groups ("*") makes it ["*"].
export const modelGroups = (row) => {
  const groups = [...new Set(enabledChannels(row).flatMap((c) => splitChannelGroups(c.group)))].sort();
  return groups.includes('*') ? ['*'] : groups;
};

// Default order of the catalog: enabled models first, then models some channel can serve, then
// the rest; by name within each group. A gateway that lists hundreds of models then still shows
// what users can call at the top.
export function sortCatalogRows(rows = []) {
  const rank = (row) => (isVisibleToUsers(row) ? 0 : 2) + (enabledChannels(row).length > 0 ? 0 : 1);
  return rows
    .map((row) => ({ row, rank: rank(row) }))
    .sort((a, b) => a.rank - b.rank || String(a.row.model).localeCompare(String(b.row.model)))
    .map(({ row }) => row);
}

// 完整可用 = 有启用渠道 + 已定价 + 对用户可见。
export const isFullyUsable = (row) => modelStatus(row) === 'ok' && isVisibleToUsers(row);

// 统计 chip 与「状态」分面共用的筛选值。各分面独立判定、可以重叠(与徽章的单值优先级不同)。
export const STATUS_FILTERS = ['usable', 'nochannel', 'unpriced', 'hidden'];

export function matchesStatusFilter(row, filter) {
  if (filter === 'usable') return isFullyUsable(row);
  if (filter === 'hidden') return !isVisibleToUsers(row);
  if (filter === 'nochannel') return enabledChannels(row).length === 0;
  if (filter === 'unpriced') return !row?.price;
  return true;
}

// 页头 chip 计数:按全量合并行计算,与单独应用对应筛选后的条数一致。
export function catalogStats(rows = []) {
  const stats = { all: rows.length };
  STATUS_FILTERS.forEach((filter) => {
    stats[filter] = rows.filter((row) => matchesStatusFilter(row, filter)).length;
  });
  return stats;
}

// 计费方式:按量 / 按次 / 未定价。
export const billingType = (row) => {
  if (!row?.price) return 'none';
  return row.price.type === 'tokens' ? 'tokens' : 'times';
};

const parseJsonArray = (value) => {
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

// 导出为导入对话框可读的格式({ data: [{ model, model_info, price }] });只导出目录行与价格,不含渠道。
export function buildCatalogExport(rows = []) {
  return {
    data: rows.map((row) => ({
      model: row.model,
      model_info: row.catalogId
        ? {
            model: row.model,
            name: row.name || row.model,
            description: row.description || '',
            context_length: row.context_length || 0,
            max_tokens: row.max_tokens || 0,
            input_modalities: parseJsonArray(row.input_modalities),
            output_modalities: parseJsonArray(row.output_modalities),
            tags: parseJsonArray(row.tags)
          }
        : null,
      price: row.price
        ? { type: row.price.type, channel_type: row.price.channel_type, input: row.price.input, output: row.price.output }
        : null
    }))
  };
}

// 需要额外取渠道列表的无目录行模型(用于决定是否请求 /api/channel/)。
export function orphanModelNames({ catalog = [], prices = [], modelList = [] } = {}) {
  const inCatalog = new Set(catalog.map((info) => info.model));
  return [...new Set([...prices.map((p) => p?.model), ...modelList])].filter((name) => name && !inCatalog.has(name));
}
