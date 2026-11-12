import { API } from 'utils/api';
import { toast } from '@/components/ui/sonner';

// Data + mutation helpers for the Channel admin page. Ported from the v1
// `views/Channel/index.jsx` so the API contract stays identical.

export const ORIGINAL_KEYWORD = {
  type: 0,
  status: 0,
  name: '',
  group: 'all',
  models: '',
  key: '',
  test_model: '',
  other: '',
  filter_tag: 0,
  tag: 'all',
  base_url: ''
};

export async function fetchChannelData(page, rowsPerPage, keyword, order, orderBy) {
  try {
    let resolvedOrderBy = orderBy;
    if (resolvedOrderBy) {
      resolvedOrderBy = order === 'desc' ? '-' + resolvedOrderBy : resolvedOrderBy;
    }
    const params = { page: page + 1, size: rowsPerPage, order: resolvedOrderBy, ...keyword };
    if (params.group === 'all') params.group = '';
    if (params.tag === 'all') params.tag = '';
    delete params._timestamp;
    const res = await API.get('/api/channel/', { params });
    const { success, message, data } = res.data;
    if (success) return data;
    toast.error(message);
  } catch (error) {
    console.error(error);
  }
  return false;
}

// Header stat chips: status values of the single-select `status` facet.
export const CHANNEL_STATUS_CHIPS = [
  { id: 'enabled', status: '1' },
  { id: 'manual', status: '2' },
  { id: 'auto', status: '3' }
];

// Params used to count one chip: the list's current filters with `status`
// replaced by the chip's value (null = all), so the count equals the rows the
// list shows after clicking that chip.
export function statusCountParams(keyword, status) {
  const params = { page: 1, size: 1, ...keyword };
  delete params.status;
  if (status) params.status = status;
  if (params.group === 'all') params.group = '';
  if (params.tag === 'all') params.tag = '';
  delete params._timestamp;
  return params;
}

export async function fetchChannelStatusCounts(keyword = {}) {
  const count = async (status) => {
    const res = await API.get('/api/channel/', { params: statusCountParams(keyword, status) });
    return res.data?.success ? (res.data.data?.total_count ?? 0) : null;
  };
  const [all, ...rest] = await Promise.all([count(null), ...CHANNEL_STATUS_CHIPS.map((c) => count(c.status))]);
  return { all, ...Object.fromEntries(CHANNEL_STATUS_CHIPS.map((c, i) => [c.id, rest[i]])) };
}

// Mirrors v1 manageChannel for the subset of actions the page needs.
export async function manageChannel(id, action, value) {
  const url = '/api/channel/';
  const data = { id };
  let res;
  try {
    switch (action) {
      case 'copy': {
        const oldRes = await API.get(`/api/channel/${id}`);
        const old = oldRes.data;
        if (!old.success) {
          toast.error(old.message);
          return { success: false, message: old.message };
        }
        const payload = old.data;
        delete payload.id;
        delete payload.test_time;
        delete payload.balance_updated_time;
        delete payload.used_quota;
        delete payload.response_time;
        delete payload.key_status;
        payload.name = payload.name + '_copy';
        // The detail API no longer returns the raw key; the backend copies it from the source channel.
        res = await API.post('/api/channel/', { ...payload }, { params: { copy_key_from: id } });
        break;
      }
      case 'delete':
        res = await API.delete(`${url}${id}`);
        break;
      case 'status':
        res = await API.put(url, { ...data, status: value });
        break;
      case 'priority':
      case 'weight':
        if (value === '') return { success: false, message: 'value required' };
        res = await API.put(url, { ...data, [action]: Number(value) });
        break;
      case 'test':
        res = await API.get(url + `test/${id}`, { params: value ? { model: value } : {} });
        break;
      case 'batch_delete':
        res = await API.delete('/api/channel/batch', { data: { value: 'batch_delete', ids: value } });
        break;
      default:
        return { success: false, message: 'invalid action' };
    }
    return res.data;
  } catch (error) {
    return { success: false, message: error.message };
  }
}

// Tests one model of a channel (`GET /api/channel/test/:id`). endpointType forces the endpoint
// (chat / response / embeddings / image); empty lets the backend infer it from the model name.
// A manual test never disables the channel.
export async function testChannelModel(id, model, endpointType) {
  const params = { model };
  if (endpointType) params.endpoint_type = endpointType;
  try {
    const res = await API.get(`/api/channel/test/${id}`, { params });
    return res?.data ?? { success: false, message: '' };
  } catch (error) {
    return { success: false, message: error.message };
  }
}

// Fetches upstream provider models for the check dialog (same endpoint as v1
// `component/ChannelCheck.jsx` getProviderModels). Keeps every model the
// provider reports, including audio models, without vendor-prefix filtering.
export async function fetchProviderModels(channel) {
  const res = await API.post('/api/channel/provider_models_list', {
    ...channel,
    models: '',
    model_mapping: ''
  });
  const { success, message, data } = res.data;
  if (!success || !data) {
    throw new Error(message || 'fetch provider models failed');
  }
  return [...new Set(data)];
}

// Pulls real per-model pricing from the channel's upstream API (e.g. OpenRouter
// /v1/models) and upserts it into the price table. Returns the synced count,
// channel models without an upstream price, and models whose upstream pricing
// could not be converted (per request / audio duration / audio tokens).
export async function syncChannelPricing(channel) {
  const res = await API.post('/api/channel/sync_pricing', {
    ...channel,
    models: '',
    model_mapping: ''
  });
  const { success, message, data } = res.data;
  if (!success) {
    throw new Error(message || 'sync pricing failed');
  }
  return {
    synced: data?.synced ?? 0,
    unmatched: data?.unmatched ?? [],
    unconvertible: data?.unconvertible ?? []
  };
}

// Previews a channel price sync: per-model changes against the price table for the
// channel's models, and user groups priced below the channel's cost ratio. Writes nothing.
export async function previewChannelPricing(channel) {
  const res = await API.post('/api/channel/sync_pricing', { ...channel, model_mapping: '', sync_preview: true });
  const { success, message, data } = res.data;
  if (!success) {
    throw new Error(message || 'sync pricing failed');
  }
  return { changes: data?.changes ?? [], losses: data?.losses ?? [], unmatched: data?.unmatched ?? [] };
}

// Writes the selected models of a channel price sync.
export async function applyChannelPricing(channel, models) {
  const res = await API.post('/api/channel/sync_pricing', { ...channel, model_mapping: '', sync_models: models });
  const { success, message, data } = res.data;
  if (!success) {
    throw new Error(message || 'sync pricing failed');
  }
  return { synced: data?.synced ?? 0 };
}

// Fetches the raw upstream model list for the channel form's model selector.
// Also keeps every model the provider reports. `channel` carries the current form
// values (id/type/key/base_url/other); when `key` is blank the backend falls
// back to the stored key via `id`.
export async function fetchUpstreamModelList(channel) {
  const res = await API.post('/api/channel/provider_models_list', {
    ...channel,
    models: '',
    model_mapping: '',
    model_headers: ''
  });
  const { success, message, data } = res.data;
  if (!success || !data) {
    throw new Error(message || 'fetch provider models failed');
  }
  return [...new Set(data)];
}

// Fetches the upstream model list with picker details (`?detail=1`): per-model
// family / latest / catalog metadata / price, and the families ranked by usage.
export async function fetchUpstreamModelPicker(channel) {
  const res = await API.post('/api/channel/provider_models_list?detail=1', {
    ...channel,
    models: '',
    model_mapping: '',
    model_headers: ''
  });
  const { success, message, data } = res.data;
  if (!success || !data) {
    throw new Error(message || 'fetch provider models failed');
  }
  return { models: data.models || [], families: data.families || [] };
}

// Runs every check against one model of a channel (`GET /api/channel/check/:id?model=`) and
// resolves to its result `{ model, status, items }`. Each call sends up to five real requests to
// the upstream. Rejects with the server message when the check could not run.
export async function checkChannelModel(id, model) {
  const res = await API.get(`/api/channel/check/${id}`, { params: { model } });
  const { success, message, data } = res?.data || {};
  if (!success || !data) {
    throw new Error(message || 'check failed');
  }
  return data;
}

// Triggers a manual model-drift check (MODEL-1b `POST /api/channel/check_drift`,
// AdminAuth). With `id` (>0) checks that single channel and returns a
// ModelDriftResult `{ checked_at, missing_models, ok }`; omit `id` for a full
// sweep of enabled channels, returning a summary `{ checked, drifted, channels }`.
// Returns raw `{ success, message, data }` so callers can branch/toast.
export async function checkChannelDrift(id) {
  const res = await API.post('/api/channel/check_drift', id ? { id } : {});
  return res.data;
}

// Everything the channel's upstream listed at the last model check, with the public model each
// name belongs to and whether the channel supplies it (GET /api/channel/:id/upstream_models).
// Returns `{ checked_at, models, counts }`; throws on failure.
export async function fetchChannelUpstreamModels(id) {
  const res = await API.get(`/api/channel/${id}/upstream_models`);
  const { success, message, data } = res.data;
  if (!success) throw new Error(message);
  return { checked_at: data?.checked_at || 0, models: data?.models || [], counts: data?.counts || {} };
}

// Clears the "new upstream models" badge for one channel (MODEL-3
// `POST /api/channel/dismiss_new_models`, AdminAuth). Keeps the upstream
// snapshot so the same batch is not reported again. Returns raw
// `{ success, message, data }`; `data` is the updated ModelDriftResult.
export async function dismissChannelNewModels(id) {
  const res = await API.post('/api/channel/dismiss_new_models', { id });
  return res.data;
}

// OAuth helpers for the channel form (GeminiCli 57 / ClaudeCode 58 /
// Codex 59 / Antigravity 60). `endpoint` is the backend route segment:
// geminicli | claudecode | codex | antigravity. All return raw `res.data`
// so callers can branch on success/status themselves.
export async function startChannelOAuth(endpoint, payload) {
  const res = await API.post(`/api/${endpoint}/oauth/start`, payload);
  return res.data;
}

// Polls the popup-flow result (57/60 only have this endpoint). Returns
// `{ success, status: 'pending'|'completed', result?, credentials?, message? }`.
export async function fetchChannelOAuthStatus(endpoint, state) {
  const res = await API.get(`/api/${endpoint}/oauth/status/${state}`);
  return res.data;
}

// Exchanges the pasted callback URL / auth code for credentials (58/59).
export async function exchangeChannelOAuthCode(endpoint, sessionId, callbackUrl) {
  const res = await API.post(`/api/${endpoint}/oauth/exchange-code`, {
    session_id: sessionId,
    callback_url: callbackUrl
  });
  return res.data;
}

// Codex device code login: start returns `{ session_id, user_code, verification_url,
// interval, expires_in }`; poll returns `status: 'pending' | 'completed'` (with credentials) or
// a coded failure `{ success: false, message, code, params }`.
export async function startCodexDeviceOAuth(payload) {
  const res = await API.post('/api/codex/oauth/device/start', payload);
  return res.data;
}

export async function pollCodexDeviceOAuth(sessionId) {
  const res = await API.post('/api/codex/oauth/device/poll', { session_id: sessionId });
  return res.data;
}

export async function fetchGroups() {
  try {
    const res = await API.get('/api/group/');
    const list = Array.isArray(res.data.data) ? res.data.data.map((g) => (typeof g === 'string' ? g : g.symbol)) : [];
    return list.sort((a, b) => String(a).localeCompare(String(b)));
  } catch (error) {
    return [];
  }
}

export async function fetchTags() {
  try {
    const res = await API.get('/api/channel_tag/_all');
    const { success, data } = res.data;
    return success ? data : [];
  } catch (error) {
    return [];
  }
}

// Lists the channels under one tag (`GET /api/channel_tag/:tag/list`). Used
// by the expanded tag row. The endpoint returns a paginated `{ data, page, size,
// total_count }` object; this unwraps it to the channel array. Throws on failure
// so callers can surface the error.
export async function fetchTagChannels(tag) {
  const res = await API.get(`/api/channel_tag/${encodeURIComponent(tag)}/list`);
  const { success, message, data } = res.data;
  if (!success) {
    throw new Error(message || 'fetch tag channels failed');
  }
  if (Array.isArray(data)) return data;
  return Array.isArray(data?.data) ? data.data : [];
}

// Tag-level management, mirroring v1 manageChannel(tag=true) branches:
// priority / status (enable|disable) / delete / delete_disabled.
export async function manageTag(tag, action, value) {
  const base = `/api/channel_tag/${encodeURIComponent(tag)}`;
  let res;
  try {
    switch (action) {
      case 'priority':
        if (value === '') return { success: false, message: 'value required' };
        res = await API.put(`${base}/priority`, { type: 'priority', value: Number(value) });
        break;
      case 'status':
        res = await API.put(`${base}/status/${value}`);
        break;
      case 'delete':
        res = await API.delete(base);
        break;
      case 'delete_disabled':
        res = await API.delete(`${base}/disabled`);
        break;
      default:
        return { success: false, message: 'invalid action' };
    }
    return res.data || { success: true, message: '' };
  } catch (error) {
    return { success: false, message: error.message };
  }
}

export async function fetchModels() {
  try {
    const res = await API.get('/api/channel/models');
    const { data } = res.data;
    data.sort((a, b) => {
      const cmp = a.owned_by.localeCompare(b.owned_by);
      return cmp === 0 ? a.id.localeCompare(b.id) : cmp;
    });
    return data.map((model) => ({ id: model.id, group: model.owned_by }));
  } catch (error) {
    return [];
  }
}

export function statusLabel(t, status) {
  switch (status) {
    case 1:
      return t('channel_index.enabled');
    case 2:
      return t('channel_row.manual');
    case 3:
      return t('channel_row.auto');
    default:
      return t('common.unknown');
  }
}

// One channel with its key hidden (key_status carries the safe projection).
export async function fetchChannel(id) {
  const res = await API.get(`/api/channel/${id}`);
  return res.data;
}

// Hourly metrics rows for one channel in [start, end] (unix seconds).
export async function fetchChannelMetrics(id, start, end) {
  const res = await API.get('/api/channel/metrics', { params: { channel_id: id, start_timestamp: start, end_timestamp: end } });
  return res.data?.success ? res.data.data || [] : [];
}

// Prompt cache and session affinity counters for one channel in [start, end].
export async function fetchChannelCacheStat(id, start, end) {
  const res = await API.get('/api/channel/cache_stats', { params: { channel_id: id, start_timestamp: start, end_timestamp: end } });
  if (!res.data?.success) return null;
  const list = res.data.data || [];
  return list.find((s) => s.channel_id === Number(id)) || null;
}

// Saves part of a channel. PUT /api/channel/ writes the whole row, so the patch is merged into
// the channel as loaded (its key is hidden, and an empty key keeps the stored one). Runtime
// fields the backend maintains are left out.
export async function updateChannel(channel, patch) {
  // pause_rules is sent only when a tab changes it: the backend keeps the stored value otherwise.
  // eslint-disable-next-line no-unused-vars
  const { key_status, model_drift, usage_windows, quota_snapshot, pause_rules, ...rest } = channel;
  const res = await API.put('/api/channel/', { ...rest, key: '', ...patch, id: channel.id });
  return res.data;
}

const PAGE_SIZE = 100;

// Every row of a paginated channel list endpoint, page by page.
async function fetchPaged(url, params = {}) {
  const rows = [];
  for (let page = 1; ; page++) {
    const res = await API.get(url, { params: { ...params, page, size: PAGE_SIZE } });
    const { success, message, data } = res.data || {};
    if (!success) throw new Error(message || 'fetch channels failed');
    const list = Array.isArray(data) ? data : data?.data || [];
    rows.push(...list);
    const total = Array.isArray(data) ? list.length : (data?.total_count ?? list.length);
    if (list.length < PAGE_SIZE || rows.length >= total) return rows;
  }
}

// Every channel, keys hidden. The list endpoint folds each tag into one row, so channels
// without a tag come from it and tagged channels from their tag's list.
export async function fetchAllChannels() {
  const untagged = await fetchPaged('/api/channel/', { filter_tag: 1 });
  const tags = await fetchTags();
  const tagged = await Promise.all(
    (tags || []).filter((t) => t?.tag).map((t) => fetchPaged(`/api/channel_tag/${encodeURIComponent(t.tag)}/list`))
  );
  const byId = new Map();
  for (const c of [...untagged, ...tagged.flat()]) byId.set(c.id, c);
  return [...byId.values()];
}

// User groups with their description and price ratio: [{ symbol, name, description, ratio }].
export async function fetchGroupOptions() {
  try {
    const res = await API.get('/api/group/');
    const list = Array.isArray(res.data?.data) ? res.data.data : [];
    return list
      .map((g) => (typeof g === 'string' ? { symbol: g } : g))
      .filter((g) => g?.symbol)
      .sort((a, b) => String(a.symbol).localeCompare(String(b.symbol)));
  } catch {
    return [];
  }
}

// Creates channels (one per key line). Resolves to { success, message, ids }.
export async function createChannel(body) {
  const res = await API.post('/api/channel/', body);
  const { success, message, data } = res.data || {};
  return { success: !!success, message, ids: data?.ids || [] };
}
