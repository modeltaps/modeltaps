import { LoginCheckAPI } from './api';
import { store } from 'store/index';
import i18n, { LANGUAGE_STORAGE_KEY, SUPPORTED_LANGUAGES } from 'i18n/i18n';

// 服务端用户级偏好（GET/PUT /api/user/setting，仅 self）。
// 登录后服务端值优先；未登录或拉取失败时回退 localStorage 现状。

const THEME_KEY = 'theme';

// useTheme 实例监听该事件以重读 localStorage（服务端值应用后同步组件状态）
export const THEME_SYNC_EVENT = 'user-setting:theme';

const isLoggedIn = () => Boolean(store.getState()?.account?.user);

const applyTheme = (theme) => {
  // 与 useTheme 的存储约定一致：system 以"无 key"表示
  if (theme === 'system') localStorage.removeItem(THEME_KEY);
  else localStorage.setItem(THEME_KEY, theme);
  const dark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  const root = document.documentElement;
  root.classList.toggle('dark', dark);
  root.classList.toggle('light', !dark);
  window.dispatchEvent(new Event(THEME_SYNC_EVENT));
};

// 登录（或会话恢复）后调用：拉取服务端偏好并应用
export async function syncUserSettingFromServer() {
  try {
    const res = await LoginCheckAPI.get('/api/user/setting');
    const { success, data } = res.data;
    if (!success || !data) return;
    if (data.theme === 'system' || data.theme === 'light' || data.theme === 'dark') {
      applyTheme(data.theme);
    }
    if (SUPPORTED_LANGUAGES.includes(data.language) && data.language !== i18n.language) {
      localStorage.setItem(LANGUAGE_STORAGE_KEY, data.language);
      i18n.changeLanguage(data.language);
    }
  } catch {
    // 静默降级：拉取失败时维持 localStorage 现状
  }
}

// 用户主动修改偏好时调用：登录状态下同步到服务端，未登录静默跳过
export function saveUserSetting(partial) {
  if (!isLoggedIn()) return;
  LoginCheckAPI.put('/api/user/setting', partial).catch(() => {});
}

// A language the user picked explicitly: remembered in this browser and, when signed in, saved to
// the account so it follows the user to other devices. Until then the browser language is used.
export function setAppLanguage(lng) {
  if (!SUPPORTED_LANGUAGES.includes(lng)) return;
  localStorage.setItem(LANGUAGE_STORAGE_KEY, lng);
  i18n.changeLanguage(lng);
  saveUserSetting({ language: lng });
}
