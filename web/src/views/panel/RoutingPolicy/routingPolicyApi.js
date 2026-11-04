import { API } from 'utils/api';

// 自动路由策略的后台接口封装；返回值统一为 res.data（{ success, message, code, data }）。
const BASE = '/api/routing_policy/';

export async function listRoutingPolicies() {
  const res = await API.get(BASE);
  return res.data;
}

export async function getRoutingPolicy(id) {
  const res = await API.get(`${BASE}${id}`);
  return res.data;
}

export async function createRoutingPolicy(policy) {
  const res = await API.post(BASE, policy);
  return res.data;
}

export async function updateRoutingPolicy(id, policy) {
  const res = await API.put(`${BASE}${id}`, policy);
  return res.data;
}

export async function setRoutingPolicyEnabled(id, enabled) {
  const res = await API.put(`${BASE}${id}/status`, { enabled });
  return res.data;
}

export async function deleteRoutingPolicy(id) {
  const res = await API.delete(`${BASE}${id}`);
  return res.data;
}

// 策略效果统计（近 7 / 30 天）与可回放的真实首条消息样本（需要开了请求明细留存）。
export async function getRoutingPolicyStats(id, days = 7) {
  const res = await API.get(`${BASE}${id}/stats`, { params: { days } });
  return res.data;
}

export async function getRoutingPolicySamples(id, limit = 20) {
  const res = await API.get(`${BASE}${id}/samples`, { params: { limit } });
  return res.data;
}

// 「试一试」：对一份（可能未保存的）策略草稿与一段首条消息，用若干候选决策模型各跑一次分类。
export async function tryRoutingPolicy({ policy, text, decisionModels }) {
  const res = await API.post(`${BASE}try`, { policy, text, decision_models: decisionModels });
  return res.data;
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
    threshold: num(form.threshold, 0.5),
    pin_ttl_seconds: num(form.pin_ttl_seconds, 3600),
    max_input_chars: num(form.max_input_chars, 0),
    decision_timeout_ms: num(form.decision_timeout_ms, 0)
  };
}
