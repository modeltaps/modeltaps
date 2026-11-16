import { isAdminPath, workbench } from 'routes/paths';
// Where the back row of 平台管理 leads: the last workbench page (personal or organization, outside
// 设置) opened in this tab (MainLayout records it), else the dashboard. Kept in sessionStorage so a
// reload inside platform admin keeps it.

const KEY = 'admin_return_path';
const FALLBACK = workbench.dashboard();

// A platform admin page is never a place to return to. Until the signed-in user loads, an admin
// page looks like a workbench page to MainLayout; recording it then left the back row pointing at
// platform admin itself.
const usable = (path) => Boolean(path) && !isAdminPath(path.split(/[?#]/)[0]);

export function rememberAdminReturn(path) {
  if (!usable(path)) return;
  try {
    sessionStorage.setItem(KEY, path);
  } catch {
    // storage disabled: the back row falls back to the dashboard
  }
}

export function adminReturnPath() {
  try {
    const path = sessionStorage.getItem(KEY);
    return usable(path) ? path : FALLBACK;
  } catch {
    return FALLBACK;
  }
}
