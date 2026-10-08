// ==============================|| PLAYGROUND — MODEL INDEX ||============================== //
// 把 /api/available_model（经 useCatalogModels 归一）的目录数据整理成选择器要的行结构：
//   { provider, id, contextLength?, pricing?, capabilities[], availability, channels? }
// 目录接口没有的字段一律返回 undefined，由 UI 隐藏对应列——「未知」不冒充「有」。
// 当前目录接口不带逐渠道报价，channels 恒为 undefined；后端补上同名字段后自动生效。
// 本文件只放纯函数：不 import react，也不 import 取数层，供组件与单测直接使用。

export const AVAILABILITY = {
  available: 'available',
  degraded: 'degraded',
  unavailable: 'unavailable'
};

const trimmed = (value) => (typeof value === 'string' ? value.trim() : '');

// 上下文长度缺省（后端 0 表示未填）时留空，不显示成「0」。
const toContextLength = (value) => (typeof value === 'number' && value > 0 ? value : undefined);

// 价格：未配置价格的兜底 Price 不是「免费」，整列留空交给 UI 说明。
const toPricing = (price) => {
  if (!price || price.unconfigured) return undefined;
  const input = typeof price.input === 'number' ? price.input : undefined;
  const output = typeof price.output === 'number' ? price.output : undefined;
  if (input === undefined && output === undefined) return undefined;
  return { type: price.type || 'tokens', input, output };
};

// 逐渠道报价：目录接口不返回，缺字段即 undefined（UI 不画渠道数与「+N more」）。
const toChannels = (channels) => (Array.isArray(channels) && channels.length > 0 ? channels : undefined);

// 可用性：目录接口只返回「已发布 + 可路由」的模型，所以在本站分组里有归属即可用；
// 一个可见分组都没有的模型只能是不可用。degraded 需要渠道级信号，目前无法判定。
const toAvailability = (model) => {
  if (model?.availability && AVAILABILITY[model.availability]) return model.availability;
  return model?.groups?.length > 0 ? AVAILABILITY.available : AVAILABILITY.unavailable;
};

export function normalizeModel(model) {
  return {
    id: model?.id || '',
    provider: trimmed(model?.vendor) || trimmed(model?.ownedBy).toLowerCase(),
    contextLength: toContextLength(model?.info?.contextLength),
    pricing: toPricing(model?.price),
    capabilities: model?.info?.capabilities || [],
    availability: toAvailability(model),
    channels: toChannels(model?.channels),
    aliases: model?.aliases || []
  };
}

// 供应商侧栏：按模型数降序、同数按名称升序，空供应商归到列表末尾。
export function buildProviders(rows) {
  const counts = new Map();
  (rows || []).forEach((row) => {
    counts.set(row.provider, (counts.get(row.provider) || 0) + 1);
  });
  return [...counts.entries()]
    .map(([id, count]) => ({ id, count }))
    .sort((a, b) => {
      if (!a.id !== !b.id) return a.id ? -1 : 1;
      if (a.count !== b.count) return b.count - a.count;
      return a.id.localeCompare(b.id);
    });
}

// 能力 chip 过滤项：目录里实际出现过的能力词，按字典序。
export function collectCapabilities(rows) {
  const seen = new Set();
  (rows || []).forEach((row) => (row.capabilities || []).forEach((cap) => seen.add(cap)));
  return [...seen].sort((a, b) => a.localeCompare(b));
}

const matchesSearch = (row, search) => {
  const q = search.trim().toLowerCase();
  if (!q) return true;
  if (row.id.toLowerCase().includes(q)) return true;
  if (row.provider.toLowerCase().includes(q)) return true;
  return (row.aliases || []).some((alias) => String(alias).toLowerCase().includes(q));
};

// 过滤顺序与 UI 一致：搜索 → 供应商 → 能力（多选取交集）→ 隐藏不可用。
export function filterRows(rows, { search = '', provider = '', capabilities = [], hideUnavailable = false } = {}) {
  return (rows || []).filter((row) => {
    if (!matchesSearch(row, search)) return false;
    if (provider && row.provider !== provider) return false;
    if (capabilities.length && !capabilities.every((cap) => (row.capabilities || []).includes(cap))) return false;
    if (hideUnavailable && row.availability === AVAILABILITY.unavailable) return false;
    return true;
  });
}

// 上下键在列表内循环；空列表停在 -1（没有可高亮的行）。
export function nextActiveIndex(current, delta, length) {
  if (!length) return -1;
  const base = current < 0 ? (delta > 0 ? -1 : 0) : current;
  return (((base + delta) % length) + length) % length;
}
