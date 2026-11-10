import { useCallback } from 'react';
import { useSearchParams } from 'react-router';

const KEYS = ['id', 'tab', 'new'];

// The object open in a list page's right-side panel, kept in the URL so it can be linked and
// survives a reload: `?id=<id>` opens its panel (`&tab=<key>` for a tab other than the first),
// `?new=1` opens the new-object sheet. Other parameters (search, filters) are left as they are.
// Opening and closing add a history entry; switching tabs replaces it.
export default function useObjectRoute(tabKeys = []) {
  const [searchParams, setSearchParams] = useSearchParams();
  const raw = searchParams.get('id') || '';
  const openId = /^\d+$/.test(raw) ? Number(raw) : 0;
  const isNew = searchParams.get('new') === '1';
  const tabParam = searchParams.get('tab') || '';
  const tab = tabKeys.includes(tabParam) ? tabParam : tabKeys[0] || '';

  const update = useCallback(
    (patch, replace = false) =>
      setSearchParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          for (const [k, v] of Object.entries(patch)) v ? p.set(k, String(v)) : p.delete(k);
          return p;
        },
        { replace }
      ),
    [setSearchParams]
  );

  const open = useCallback((id, nextTab) => update({ id, tab: nextTab || '', new: '' }), [update]);
  const openNew = useCallback(() => update({ id: '', tab: '', new: '1' }), [update]);
  const close = useCallback(() => update(Object.fromEntries(KEYS.map((k) => [k, '']))), [update]);
  const setTab = useCallback((next) => update({ tab: next === tabKeys[0] ? '' : next }, true), [update, tabKeys]);

  return { openId, isNew, tab, open, openNew, close, setTab };
}
