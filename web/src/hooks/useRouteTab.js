import { useCallback, useEffect } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router';

// ==============================|| ROUTE TABS ||============================== //
// A page whose tabs are path segments: the default tab is the bare page (`/panel/logs`), the
// others a trailing segment (`/panel/logs/tasks`) read from the route's `:tab` param.
//   tabs      tab ids in order; the first is the default
//   segments  optional id → segment map when an id differs from its URL segment
//   legacy    optional old `?tab=` value → tab id, or { tab, query } to also set query parameters;
//             such links are rewritten to the path
// An unknown segment falls back to the default tab (the URL is corrected with replace). The query
// string is kept when switching tabs, so filters survive. While an old or unknown URL is being
// rewritten `redirecting` is true: render nothing then, so the tab's content does not mount and
// write its own query parameters over the rewrite.
export default function useRouteTab({ tabs, segments = {}, legacy = {} }) {
  const { tab: param } = useParams();
  const { pathname, search, hash } = useLocation();
  const navigate = useNavigate();

  const segmentOf = useCallback((id) => segments[id] || id, [segments]);
  const idOf = (segment) => tabs.find((id) => segmentOf(id) === segment);

  const defaultTab = tabs[0];
  const requested = param === undefined ? defaultTab : idOf(param);
  const tab = requested || defaultTab;

  // The page's own path without the tab segment.
  const base = param === undefined ? pathname.replace(/\/$/, '') : pathname.slice(0, pathname.length - param.length - 1);

  const hrefFor = useCallback(
    (id, query = search) => `${id === defaultTab ? base : `${base}/${segmentOf(id)}`}${query}`,
    [base, defaultTab, search, segmentOf]
  );

  // Old `?tab=x` links and unknown segments → the canonical path.
  const params = new URLSearchParams(search);
  const legacyTab = params.get('tab');
  const legacyEntry = legacyTab != null ? legacy[legacyTab] || (tabs.includes(legacyTab) ? legacyTab : defaultTab) : null;
  const legacyTarget = typeof legacyEntry === 'object' && legacyEntry ? legacyEntry.tab : legacyEntry;
  const legacyQuery = typeof legacyEntry === 'object' && legacyEntry ? JSON.stringify(legacyEntry.query || {}) : '{}';
  useEffect(() => {
    if (legacyTarget) {
      const rest = new URLSearchParams(search);
      rest.delete('tab');
      Object.entries(JSON.parse(legacyQuery)).forEach(([k, v]) => rest.set(k, v));
      const query = rest.toString();
      navigate(`${hrefFor(legacyTarget, query ? `?${query}` : '')}${hash}`, { replace: true });
    } else if (param !== undefined && !requested) {
      navigate(`${hrefFor(defaultTab)}${hash}`, { replace: true });
    }
  }, [legacyTarget, legacyQuery, param, requested, hrefFor, defaultTab, navigate, search, hash]);

  const setTab = useCallback((id) => navigate(hrefFor(id)), [navigate, hrefFor]);

  const redirecting = Boolean(legacyTarget) || (param !== undefined && !requested);

  return { tab, setTab, hrefFor, redirecting };
}
