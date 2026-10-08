// UX-2: 记住进入「管理后台」前所在的用户面路径,退出时回到原处而非固定首页。
// 整页设置面(/panel/settings/*)复用同一机制。
// 管理面用 sessionStorage(标签页级,不可用时静默回退到用户面首页);设置面的来源
// 只在本次页面会话内有意义(刷新 = 直达,必须回落首页),所以放内存,刷新自然清零。

const KEYS = {
  admin: 'modeltaps.userReturnPath'
};
const DEFAULT = '/panel/dashboard';
const SETTINGS_ROOT = '/panel/settings';

let settingsReturnPath = null;

const isSettings = (path) => path === SETTINGS_ROOT || path.startsWith(`${SETTINGS_ROOT}/`);

export function saveReturnPath(plane, path) {
  if (!path || !path.startsWith('/panel')) return;
  if (plane === 'settings') {
    settingsReturnPath = path;
    return;
  }
  // 管理面的「返回工作台」永远回工作台:从设置面进管理后台时不把设置路径记成来源,
  // 否则返回会落到 /panel/settings/*(保留上一次的工作台路径,没有就回落首页)。
  if (isSettings(path)) return;
  const key = KEYS[plane];
  try {
    if (key) sessionStorage.setItem(key, path);
  } catch {
    /* sessionStorage 不可用时忽略 */
  }
}

export function getReturnPath(plane) {
  if (plane === 'settings') return settingsReturnPath || DEFAULT;
  const key = KEYS[plane];
  try {
    return (key && sessionStorage.getItem(key)) || DEFAULT;
  } catch {
    return DEFAULT;
  }
}

export function clearReturnPath(plane) {
  if (plane === 'settings') {
    settingsReturnPath = null;
    return;
  }
  const key = KEYS[plane];
  try {
    if (key) sessionStorage.removeItem(key);
  } catch {
    /* sessionStorage 不可用时忽略 */
  }
}

// 这些旧路径自己会重定向进设置面(见 MainRoutes 的 ProfileRedirect),记成来源会让
// 「返回」回环,所以到了这些路径就把来源清掉,「返回」回落用户面首页。
const SETTINGS_REDIRECT_PATHS = ['/panel/profile'];

// 每次渲染在设置面外刷新来源,所以设置面侧边栏渲染期读到的就是最新值(不能放 effect,
// 那样一次导航直达设置路径时首屏会读到旧值)。
export function trackSettingsReturnPath(pathname, settingsPlane) {
  if (settingsPlane) return;
  if (SETTINGS_REDIRECT_PATHS.includes(pathname)) clearReturnPath('settings');
  else saveReturnPath('settings', pathname);
}

export const saveUserReturnPath = (path) => saveReturnPath('admin', path);

export const getUserReturnPath = () => getReturnPath('admin');
