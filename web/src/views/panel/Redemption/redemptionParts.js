// Shared by the redemption panel and the new code sheet.

export const SCOPES = ['any', 'personal', 'org'];

// unix(seconds) <-> <input type="datetime-local"> (local "YYYY-MM-DDTHH:mm")
export const unixToInput = (ts) => {
  if (!ts || ts <= 0) return '';
  const d = new Date(ts * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

export const inputToUnix = (s) => (s ? Math.floor(new Date(s).getTime() / 1000) : 0);

// 1 enabled, 2 disabled, 3 redeemed.
export function statusLabel(t, status) {
  if (status === 1) return t('common.enable');
  if (status === 2) return t('common.disable');
  if (status === 3) return t('analytics_index.used');
  return t('common.unknown');
}
