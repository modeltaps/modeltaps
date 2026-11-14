import { createContext, useCallback, useContext, useMemo, useState } from 'react';

// ==============================|| SETTINGS — SETTINGS PLANE STATE ||============================== //
// On 设置 (`/panel/settings/*`) the subnav of the settings page lives in the sidebar
// (SettingsSidebar) instead of on the page. MainLayout provides this state to both: the page
// reports which groups hold unsaved cards, and the sidebar tree marks them with a dot. Null
// everywhere else, so a SettingsPage outside 设置 (platform settings) keeps its own subnav.
// Search is not part of it: it is the panel's search palette (chrome/SearchContext).

const IDLE = { dirtyGroups: [] };

export const SettingsPlaneContext = createContext(null);

export const useSettingsPlane = () => useContext(SettingsPlaneContext);

export function useSettingsPlaneState() {
  const [status, setStatus] = useState(IDLE);
  // The page reports its status; null when it goes away.
  const publish = useCallback((next) => {
    const value = next ? { ...IDLE, ...next } : IDLE;
    setStatus((prev) => (prev.dirtyGroups.join(',') === value.dirtyGroups.join(',') ? prev : value));
  }, []);
  return useMemo(() => ({ publish, ...status }), [publish, status]);
}
