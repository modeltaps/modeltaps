import { useEffect, useState } from 'react';

import { API } from 'utils/api';

// Event the alerts page sends after it changed or reloaded the list, so the badge follows.
export const ALERTS_CHANGED = 'modeltaps:alerts-changed';
const REFRESH_MS = 60000;

// The number of open alerts (`/api/alert/count`) while `enabled`, read again every minute, when
// the window comes back into view and when the alerts page says the list changed. 0 until read
// and when it cannot be read: the badge is a hint, not a source of truth.
export default function useOpenAlertCount(enabled) {
  const [count, setCount] = useState(0);
  useEffect(() => {
    if (!enabled) return undefined;
    let alive = true;
    const load = async () => {
      try {
        const res = await API.get('/api/alert/count');
        if (alive && res.data?.success) setCount(res.data.data?.open || 0);
      } catch {
        // keep the last count
      }
    };
    load();
    const timer = setInterval(load, REFRESH_MS);
    const onVisible = () => document.visibilityState === 'visible' && load();
    window.addEventListener(ALERTS_CHANGED, load);
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      alive = false;
      clearInterval(timer);
      window.removeEventListener(ALERTS_CHANGED, load);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [enabled]);
  return enabled ? count : 0;
}
