// ==============================|| API CATALOG — AVAILABILITY ||============================== //
// 能力可用性只看后端目录数据（/api/available_model 的 endpoints 与 price.model_info），
// 不看 channel_type，也不维护任何静态渠道表。
// 规则字段（都可省略，省略即不约束）：
//   endpoints          后端接口词表，任一命中即可（chat / responses / images /
//                      audio.speech / audio.transcription / embeddings / rerank）
//   inputModalities    输入模态，必须全部命中（如图片理解 = chat ∩ 输入含 image）
//   outputModalities   输出模态，必须全部命中
//   capabilities       能力词表（tool_call / reasoning / structured_output），必须全部命中
//   vendors            厂商 slug（anthropic / google / openai …），任一命中即可。
//                      用于协议兼容节：Claude 兼容只该列 Anthropic 的模型，否则模型数会
//                      把全部对话模型都算进去，示例也会拿 Anthropic SDK 去调 GPT。

const lower = (list) => (Array.isArray(list) ? list.map((v) => String(v).toLowerCase()) : []);

const hasAny = (have, want) => want.some((w) => have.includes(w));
const hasAll = (have, want) => want.every((w) => have.includes(w));

// 厂商名归一化成 slug 形态：小写、去空白。后端没有 slug 时只能拿展示名退化，
// 「Google Gemini」→「googlegemini」，靠下面的前缀比较仍能被规则里的 google 命中。
export const toVendorSlug = (value) => (typeof value === 'string' ? value : '').trim().toLowerCase().replace(/\s+/g, '');

// /api/available_model 单条记录的厂商 slug：后端已给 slug 就用 slug，
// 否则退化到展示名 / owned_by。vendor 是对象时不能整体插值，否则得到 "[object Object]"。
export function resolveVendorSlug(raw) {
  const vendor = raw?.vendor;
  const source = raw?.vendor_slug || (typeof vendor === 'string' ? vendor : vendor?.slug || vendor?.name) || raw?.owned_by || '';
  return toVendorSlug(source);
}

// 厂商比较：规则 slug 与模型 slug 相等，或模型 slug 以规则 slug 打头
// （规则写 google，命中后端的 google-gemini / googlegemini）。
const matchesVendor = (model, vendors) => {
  const have = [model.vendor, model.ownedBy].map(toVendorSlug).filter(Boolean);
  const want = vendors.map(toVendorSlug).filter(Boolean);
  return want.some((w) => have.some((h) => h === w || h.startsWith(w)));
};

// 判断单个模型是否满足某条可用性规则。缺字段的模型（未填 endpoints / model_info）
// 只要规则对该维度有要求就判为不满足——「未知」不冒充「支持」。
export function matchesRule(model, rule) {
  if (!model) return false;
  if (!rule) return true;

  if (rule.endpoints?.length && !hasAny(lower(model.endpoints), lower(rule.endpoints))) return false;
  if (rule.inputModalities?.length && !hasAll(lower(model.info?.inputModalities), lower(rule.inputModalities))) return false;
  if (rule.outputModalities?.length && !hasAll(lower(model.info?.outputModalities), lower(rule.outputModalities))) return false;
  if (rule.capabilities?.length && !hasAll(lower(model.info?.capabilities), lower(rule.capabilities))) return false;
  // 厂商未知（vendor / owned_by 都没填）时同样判不满足。
  if (rule.vendors?.length && !matchesVendor(model, rule.vendors)) return false;

  return true;
}

// 「0 个可用模型」有四种完全不同的原因，文案必须分开（chip、小节徽标、模态卡共用这一份）：
//   status: 'planned'  后端根本没有这条路由（如视频生成 /v1/videos）——「即将上线」。
//                      这条路由不存在是静态事实，与取数成败无关，取数失败时也照说「即将上线」。
//   loading            模型列表还没回来——「加载中」
//   error              取数失败，0 只代表我们不知道——「可用性未知」
//   其余                路由在，只是本站当下没有对应模型——「暂无可用模型」
// 概览页的模态卡不对应具体能力，传 capability = null 走后三条分支。
export function capabilityEmptyKey(capability, { loading = false, error = false } = {}) {
  if (capability?.status === 'planned') return 'apiCatalogPage.comingSoon';
  if (loading) return 'common.loading';
  if (error) return 'apiCatalogPage.availabilityUnknown';
  return 'apiCatalogPage.noModels';
}

// OpenRouter 的 latest 别名（~anthropic/...，请求时才解析到具体模型）与路由模型
// （openrouter/auto 等）不是具体模型，排在目录末尾，也不作为默认 / 推荐的回退项。
export const isAliasOrRouterModel = (id) => typeof id === 'string' && (id.startsWith('~') || id.startsWith('openrouter/'));

// 目录排序：具体模型在前、别名 / 路由模型在后，组内按模型名。
export const compareCatalogModels = (a, b) =>
  Number(isAliasOrRouterModel(a.id)) - Number(isAliasOrRouterModel(b.id)) || a.id.localeCompare(b.id);

// 按规则筛出可用模型，保持入参顺序（hook 已按模型名排序）。
export function filterModels(models, rule) {
  if (!Array.isArray(models)) return [];
  return models.filter((m) => matchesRule(m, rule));
}

// 推荐模型，按顺序回退：
//   1. preferredModels 里第一个真实可用的（名单会随后端目录变动而失效，见 catalog 的种子单测）
//   2. 名单全落空时，规则点名的厂商里的首项——协议兼容节不能推荐别家厂商的模型
//   3. 仍取不到就取列表首项
// 2、3 两步优先跳过别名 / 路由模型，只剩它们时才取。
export function pickPreferredModel(models, preferredModels, vendors) {
  if (!Array.isArray(models) || models.length === 0) return null;
  for (const name of preferredModels || []) {
    const hit = models.find((m) => m.id === name);
    if (hit) return hit;
  }
  const concrete = models.filter((m) => !isAliasOrRouterModel(m.id));
  const pool = concrete.length ? concrete : models;
  if (vendors?.length) {
    const hit = pool.find((m) => matchesVendor(m, vendors));
    if (hit) return hit;
  }
  return pool[0];
}

// 能力可能挂在一个站点级开关上（如 Claude / Gemini 协议入口）。开关显式关掉时整节不算数：
// 后端会 403 掉这些路由，页面再展示模型数与示例就是假的。siteInfo 尚未加载（字段为
// undefined）时按开着处理，避免首屏把默认开启的能力闪掉。
const capabilityEnabled = (capability, siteInfo) => !capability.enabledFlag || siteInfo?.[capability.enabledFlag] !== false;

// 一个模态的全部能力 + 各自的可用模型。模态页页头的 chip 与概览页的模态卡共用这一份
// 求值，保证两处的能力数量恒等。
export function buildCapabilitySections(capabilities, models, siteInfo) {
  return (capabilities || [])
    .filter((capability) => capabilityEnabled(capability, siteInfo))
    .map((capability) => ({ capability, models: filterModels(models, capability.availability) }));
}

// 模态级推荐模型：取「首个有可用模型的能力」的推荐项。主能力可能暂无模型（如图像 Images
// 路由、语音 TTS），此时顺延到下一个能力，避免整张推荐卡消失。
export function pickSectionRecommended(sections) {
  for (const { capability, models } of sections || []) {
    const picked = pickPreferredModel(models, capability.preferredModels, capability.availability?.vendors);
    if (picked) return picked;
  }
  return null;
}
