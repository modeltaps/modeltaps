import { API } from 'utils/api';

// Routing policy endpoints; every call resolves to res.data ({ success, message, code, data }).
// createRoutingPolicyApi(base) builds the same client for one scope: the platform console uses
// /api/routing_policy/, an organization's settings use /api/org/:id/routing_policy/. The organization
// scope has no log sample replay (getSamples is null) and lists its own usable models.
export function createRoutingPolicyApi(base, { listModels, samples = true, billedTry = false } = {}) {
  return {
    // review: the platform console's read-only view of organization policies (organizationReviewApi).
    review: false,
    // billedTry: "Try it" and the probe are charged (organization scope).
    billedTry,
    list: async (params) => (await API.get(base, params ? { params } : undefined)).data,
    get: async (id) => (await API.get(`${base}${id}`)).data,
    create: async (policy) => (await API.post(base, policy)).data,
    update: async (id, policy) => (await API.put(`${base}${id}`, policy)).data,
    setEnabled: async (id, enabled) => (await API.put(`${base}${id}/status`, { enabled })).data,
    remove: async (id) => (await API.delete(`${base}${id}`)).data,
    // 策略效果统计（近 7 / 30 天）。
    getStats: async (id, days = 7) => (await API.get(`${base}${id}/stats`, { params: { days } })).data,
    // 可回放的真实首条消息样本（需要开了请求明细留存）。
    getSamples: samples ? async (id, limit = 20) => (await API.get(`${base}${id}/samples`, { params: { limit } })).data : null,
    // 「试一试」：对一份（可能未保存的）策略草稿与一段首条消息，用若干候选决策模型各跑一次分类。
    tryPolicy: async ({ policy, text, decisionModels }) =>
      (await API.post(`${base}try`, { policy, text, decision_models: decisionModels })).data,
    // 探测一个决策模型在当前渠道上能否返回 logprobs；data.supported 为 null 时探测本身失败（data.error）。
    probe: async (decisionModel) => (await API.post(`${base}probe`, { decision_model: decisionModel })).data,
    // 决策 / 目标 / 兜底的下拉建议。
    listModels: listModels || (async () => [])
  };
}

// 管理员目录：全部主名模型（去掉别名行与策略行），供决策 / 目标 / 兜底的下拉建议。
export async function listCatalogModelNames() {
  const res = await API.get('/api/model_info/catalog');
  const { success, data } = res.data || {};
  if (!success || !Array.isArray(data)) return [];
  return data
    .filter((row) => !row.alias_of && row.kind !== 'routing_policy')
    .map((row) => row.model)
    .sort((a, b) => a.localeCompare(b));
}

export const platformRoutingPolicyApi = createRoutingPolicyApi('/api/routing_policy/', { listModels: listCatalogModelNames });

// organizationReviewApi: every organization's policies as the platform console reviews them
// (?organization_id=-1, rows carry organization_name and overrides_platform). Review is read-only:
// an admin can open a policy and switch it off, nothing else.
export const organizationReviewApi = {
  ...platformRoutingPolicyApi,
  review: true,
  list: () => platformRoutingPolicyApi.list({ organization_id: -1 }),
  getSamples: null
};

// orgRoutingPolicyApi the client for one organization's policies (Owner / Admin only).
export function orgRoutingPolicyApi(orgId) {
  const base = `/api/org/${orgId}/routing_policy/`;
  return createRoutingPolicyApi(base, {
    samples: false,
    billedTry: true,
    listModels: async () => {
      const res = await API.get(`${base}models`);
      const { success, data } = res.data || {};
      return success && Array.isArray(data) ? data : [];
    }
  });
}

// 打分方式：auto 先用 logprobs，不支持时改用自报；logprobs 只用概率；self_reported 只用自报。
export const SCORING_MODES = ['auto', 'logprobs', 'self_reported'];

// 空策略：新建表单的初始值。数值字段 0 = 用后端默认（截断长度、超时）；pin_ttl_seconds 0 = 每次分类。
export const EMPTY_POLICY = {
  name: '',
  display_name: '',
  description: '',
  enabled: true,
  decision_model: '',
  question: '',
  routes: [
    { label: '', description: '', target_model: '' },
    { label: '', description: '', target_model: '' }
  ],
  fallback_model: '',
  scoring: 'auto',
  threshold: 0.5,
  pin_ttl_seconds: 3600,
  max_input_chars: 4000,
  decision_timeout_ms: 3000
};

// 提交前整理：去首尾空白、数值转数字、去掉完全为空的路由行。
export function normalizePolicy(form) {
  const num = (v, fallback) => {
    const n = Number(v);
    return Number.isFinite(n) ? n : fallback;
  };
  const routes = (form.routes || [])
    .map((r) => ({
      label: (r.label || '').trim(),
      description: (r.description || '').trim(),
      target_model: (r.target_model || '').trim()
    }))
    .filter((r) => r.label || r.description || r.target_model);
  return {
    name: (form.name || '').trim(),
    display_name: (form.display_name || '').trim(),
    description: (form.description || '').trim(),
    enabled: Boolean(form.enabled),
    decision_model: (form.decision_model || '').trim(),
    question: (form.question || '').trim(),
    routes,
    fallback_model: (form.fallback_model || '').trim(),
    scoring: form.scoring || 'auto',
    threshold: num(form.threshold, 0.5),
    pin_ttl_seconds: num(form.pin_ttl_seconds, 3600),
    max_input_chars: num(form.max_input_chars, 0),
    decision_timeout_ms: num(form.decision_timeout_ms, 0)
  };
}
