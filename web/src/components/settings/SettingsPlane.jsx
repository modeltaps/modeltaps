import { createContext, useCallback, useContext, useMemo, useState } from 'react';

// ==============================|| SETTINGS — SETTINGS PLANE STATE ||============================== //
// On 设置 (`/panel/settings/*`) the subnav of the settings page lives in the sidebar
// (SettingsSidebar) instead of on the page. MainLayout provides this state to both: the search
// text the sidebar edits, and what the page reports back (whether it is searchable, which groups
// hold unsaved cards, the match count of each group while searching). Null everywhere else, so a
// SettingsPage outside 设置 (platform settings) keeps its own subnav.

const IDLE = { searchable: false, dirtyGroups: [], counts: null };

export const SettingsPlaneContext = createContext(null);

export const useSettingsPlane = () => useContext(SettingsPlaneContext);

const sameStatus = (a, b) =>
  a.searchable === b.searchable &&
  a.dirtyGroups.join(',') === b.dirtyGroups.join(',') &&
  JSON.stringify(a.counts) === JSON.stringify(b.counts);

export function useSettingsPlaneState() {
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState(IDLE);
  // The page reports its status; null when it goes away.
  const publish = useCallback((next) => {
    const value = next ? { ...IDLE, ...next } : IDLE;
    setStatus((prev) => (sameStatus(prev, value) ? prev : value));
  }, []);
  return useMemo(() => ({ query, setQuery, publish, ...status }), [query, publish, status]);
}
