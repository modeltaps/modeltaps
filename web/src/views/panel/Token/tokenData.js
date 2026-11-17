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
  // Organization keys come with whether their request contents are stored (SEC-51).
  return { success: true, data: normalizeToken(token), logIO: res.data.log_io ?? null };
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

// Expiry choices for a key: never, or one day, week or (calendar) month from now.
export const EXPIRY_PRESETS = ['never', 'day', 'week', 'month'];
export const expiryFromPreset = (preset, now = new Date()) => {
  if (preset === 'day') return Math.floor(now.getTime() / 1000) + 86400;
  if (preset === 'week') return Math.floor(now.getTime() / 1000) + 7 * 86400;
  if (preset === 'month') {
    const d = new Date(now.getTime());
    const day = d.getDate();
    d.setMonth(d.getMonth() + 1);
    // Jan 31 + 1 month would roll into March; stay on the last day of the next month instead.
    if (d.getDate() < day) d.setDate(0);
    return Math.floor(d.getTime() / 1000);
  }
  return -1;
};

// Key types offered when creating a key (AGENT-4): an API key calls models; a management key
// (read-only or read-write) calls the management API and MCP and never models.
export const KEY_TYPES = ['api', 'read', 'write'];
export const isManagementKeyType = (type) => type === 'read' || type === 'write';

// A management key must expire: 90 days by default, at most a year (the server refuses more).
export const MANAGEMENT_EXPIRY_PRESETS = ['month', 'quarter', 'halfYear', 'year'];
export const MANAGEMENT_EXPIRY_DEFAULT = 'quarter';
const MANAGEMENT_EXPIRY_DAYS = { month: 30, quarter: 90, halfYear: 180, year: 365 };
export const managementExpiryFromPreset = (preset, now = new Date()) =>
  Math.floor(now.getTime() / 1000) + (MANAGEMENT_EXPIRY_DAYS[preset] || MANAGEMENT_EXPIRY_DAYS[MANAGEMENT_EXPIRY_DEFAULT]) * 86400;

// The i18n key of a management key's type tag ("legacy" is a migrated access token).
export const managementKeyTypeLabel = (scope) => `apiKey.type.${scope === 'read' || scope === 'write' ? scope : 'legacy'}`;
