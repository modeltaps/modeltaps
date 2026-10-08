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

// Streams per-model availability results from `POST /api/sse/channel/check`.
// Parses the accumulated SSE text on each progress tick and invokes
// `onResult(data)` for every `result` event (same contract as v1).
// `signal` (AbortSignal) cancels the in-flight stream, e.g. on dialog close.
export async function checkChannelModels(id, models, onResult, signal) {
  const readEventText = (progressEvent) =>
    progressEvent?.event?.target?.response ?? progressEvent?.event?.currentTarget?.response ?? progressEvent?.currentTarget?.response ?? '';

  const res = await API.post(
    '/api/sse/channel/check',
    { id, models },
    {
      responseType: 'text',
      signal,
      onDownloadProgress: (progressEvent) => {
        const text = readEventText(progressEvent);
        text.split('\n').forEach((line) => {
          if (line.trim() === '' || !line.startsWith('data:')) return;
          try {
            const eventData = JSON.parse(line.slice(5));
            if (eventData.type === 'result') onResult(eventData.data);
          } catch (e) {
            // ignore partial/invalid SSE lines
          }
        });
      }
    }
  );
  if (res.status !== 200) {
    throw new Error(res.data?.message || 'check failed');
  }
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
