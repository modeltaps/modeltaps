// Where the back row of 平台管理 leads: the last workbench page (personal or organization, outside
// 设置) opened in this tab (MainLayout records it), else the dashboard. Kept in sessionStorage so a
// reload inside platform admin keeps it.

const KEY = 'admin_return_path';
const FALLBACK = '/panel/dashboard';

export function rememberAdminReturn(path) {
  try {
    sessionStorage.setItem(KEY, path);
  } catch {
    // storage disabled: the back row falls back to the dashboard
  }
}

export function adminReturnPath() {
  try {
    return sessionStorage.getItem(KEY) || FALLBACK;
  } catch {
    return FALLBACK;
  }
}
