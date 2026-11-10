import { createContext, useContext, useEffect } from 'react';

// ==============================|| CHROME — PAGE HEADER OVERRIDE ||============================== //
// A detail page (e.g. one channel) replaces the nav-derived title with its own:
// `usePageHeader({ title, subtitle, back: { label, to } })`. Values are strings so the effect
// only re-runs when they change. MainLayout owns the state; unmounting or passing null
// restores the default header.

export const PageHeaderOverrideContext = createContext(null);

export default function usePageHeader(header) {
  const setHeader = useContext(PageHeaderOverrideContext);
  const title = header?.title;
  const subtitle = header?.subtitle;
  const backLabel = header?.back?.label;
  const backTo = header?.back?.to;
  const active = Boolean(header);
  useEffect(() => {
    if (!setHeader) return undefined;
    setHeader(active ? { title, subtitle, back: backTo ? { label: backLabel, to: backTo } : null } : null);
    return () => setHeader(null);
  }, [setHeader, active, title, subtitle, backLabel, backTo]);
}
