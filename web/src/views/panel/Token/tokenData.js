import { API } from 'utils/api';

// Loading and saving one API key for its panel. The update endpoints take the whole key, so a save
// starts from the key as stored now and replaces only the fields of the tab being saved.

export const DEFAULT_SETTING = {
  heartbeat: { enabled: false, timeout_seconds: 30 },
  limits: {
    limit_model_setting: { enabled: false, models: [] },
    limits_ip_setting: { enabled: false, whitelist: [] }
  }
};

// Fills in the setting blocks an older key may lack, so tabs can read them without guards.
export function normalizeToken(d) {
  const s = d.setting || {};
  const limits = s.limits || {};
  return {
    ...d,
    setting: {
      ...s,
      heartbeat: { ...DEFAULT_SETTING.heartbeat, ...(s.heartbeat || {}) },
      limits: {
        ...limits,
        limit_model_setting: { ...DEFAULT_SETTING.limits.limit_model_setting, ...(limits.limit_model_setting || {}) },
        limits_ip_setting: { ...DEFAULT_SETTING.limits.limits_ip_setting, ...(limits.limits_ip_setting || {}) }
      }
    }
  };
}

// An administrator looking at someone else's key goes through the admin endpoints.
export async function fetchToken(id, adminMode) {
  const res = adminMode
    ? await API.get('/api/token/admin/search', { params: { token_id: id, page: 1, size: 1 } })
    : await API.get(`/api/token/${id}`);
  const { success, message, data } = res.data;
  const token = adminMode ? data?.data?.[0] : data;
  if (!success || !token) return { success: false, message };
  return { success: true, data: normalizeToken(token) };
}

// `patch` holds top-level fields and an optional partial `setting`; `setting.quota_reset: null`
// removes the reset period.
export async function saveToken(id, patch, adminMode) {
  const fresh = await fetchToken(id, adminMode);
  if (!fresh.success) return fresh;
  const { setting: settingPatch, ...fields } = patch;
  const setting = { ...fresh.data.setting, ...(settingPatch || {}) };
  if (settingPatch?.limits) setting.limits = { ...fresh.data.setting.limits, ...settingPatch.limits };
  if (setting.quota_reset === null) delete setting.quota_reset;
  const body = { ...fresh.data, ...fields, setting, id };
  if (body.user_id != null) body.user_id = parseInt(body.user_id, 10);
  const res = adminMode ? await API.put('/api/token/admin', body) : await API.put('/api/token/', body);
  return res.data;
}

// The key's spending limit as one amount and an optional reset period (the form shown in the
// panel and the create dialog), and back. An empty amount without a period means no limit; an
// amount without a period is what is left to spend; with a period it is the cap per period.
export function quotaDraft(token) {
  const period = token.setting?.quota_reset?.period || '';
  if (period) return { quota: token.setting.quota_reset.limit ?? '', period };
  return { quota: token.unlimited_quota ? '' : (token.remain_quota ?? ''), period: '' };
}

export function quotaPatch({ quota, period }, t) {
  const empty = quota === '' || quota == null;
  const value = empty ? 0 : parseInt(quota, 10);
  if (!empty && (!Number.isFinite(value) || value < 0)) return { error: t('redemption_edit.requiredQuota') };
  if (period) {
    if (empty || value <= 0) return { error: t('token_index.quotaResetLimitRequired') };
    return { unlimited_quota: true, remain_quota: 0, setting: { quota_reset: { period, limit: value } } };
  }
  if (empty) return { unlimited_quota: true, remain_quota: 0, setting: { quota_reset: null } };
  return { unlimited_quota: false, remain_quota: value, setting: { quota_reset: null } };
}

export const RESET_PERIODS = ['daily', 'weekly', 'monthly'];

// Expiry presets of the create dialog, in days; 0 = never.
export const EXPIRY_PRESETS = [0, 7, 30, 90, 365];
export const expiryFromPreset = (days) => (days ? Math.floor(Date.now() / 1000) + days * 86400 : -1);

const pad = (n) => String(n).padStart(2, '0');
export const toLocalInput = (unix) => {
  const d = new Date(unix * 1000);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
export const fromLocalInput = (s) => Math.floor(new Date(s).getTime() / 1000);
