import { useCallback, useEffect, useState } from 'react';

import { saveUserSetting, THEME_SYNC_EVENT } from 'utils/userSetting';

// localStorage key shared with the v1 MUI ThemeButton: explicit 'light'/'dark'
// are stored, while "system" is represented by the absence of the key.
const STORAGE_KEY = 'theme';

const readStored = () => {
  const v = typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) : null;
  return v === 'light' || v === 'dark' ? v : 'system';
};

const systemPrefersDark = () =>
  typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;

const resolve = (mode) => (mode === 'system' ? (systemPrefersDark() ? 'dark' : 'light') : mode);

const applyResolved = (resolved) => {
  const root = document.documentElement;
  root.classList.toggle('dark', resolved === 'dark');
  root.classList.toggle('light', resolved !== 'dark');
};

// ==============================|| THEME HOOK (light / dark / system) ||============================== //

export default function useTheme() {
  const [theme, setThemeState] = useState(readStored);

  useEffect(() => {
    applyResolved(resolve(theme));
  }, [theme]);

  useEffect(() => {
    if (theme !== 'system') return undefined;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = (e) => applyResolved(e.matches ? 'dark' : 'light');
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, [theme]);

  // 登录后服务端偏好应用到 localStorage 时，重读以同步已挂载实例的状态
  useEffect(() => {
    const handler = () => setThemeState(readStored());
    window.addEventListener(THEME_SYNC_EVENT, handler);
    return () => window.removeEventListener(THEME_SYNC_EVENT, handler);
  }, []);

  const setTheme = useCallback((mode) => {
    if (mode === 'system') localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, mode);
    setThemeState(mode);
    saveUserSetting({ theme: mode });
  }, []);

  const cycle = useCallback(() => {
    const next = theme === 'system' ? 'light' : theme === 'light' ? 'dark' : 'system';
    setTheme(next);
  }, [theme, setTheme]);

  return { theme, resolved: resolve(theme), setTheme, cycle };
}
