// Where the back row of 设置 leads: the last page opened outside 设置 in this tab (MainLayout
// records it), else the dashboard. Kept in sessionStorage so a reload inside 设置 keeps it.

const KEY = 'settings_return_path';
const FALLBACK = '/panel/dashboard';

export function rememberSettingsReturn(path) {
  try {
    sessionStorage.setItem(KEY, path);
  } catch {
    // storage disabled: the back row falls back to the dashboard
  }
}

export function settingsReturnPath() {
  try {
    return sessionStorage.getItem(KEY) || FALLBACK;
  } catch {
    return FALLBACK;
  }
}
