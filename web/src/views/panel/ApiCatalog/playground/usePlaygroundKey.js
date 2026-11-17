import { useCallback, useRef, useState } from 'react';

import { RelayError } from './useRelayRequest';

// ==============================|| PLAYGROUND — API KEY ||============================== //
// The playground calls the same-origin relay (/v1) with a short-lived playground key: POST
// /api/token/playground (signed-in session) issues one that spends on the user's playground API
// key and expires after an hour (model.PlaygroundKey). Long-lived API keys are never fetched,
// they cannot be read back. The key only lives in memory and only goes out in the Authorization
// header: never in browser storage, URLs, Redux or the DOM; the code shown next to the
// playground uses a placeholder. The request uses a bare fetch rather than utils/api: that
// instance's interceptor would pop a global toast, while a failure here is one failure inside the
// panel, shown by its error bar.

export const PLAYGROUND_TOKEN_PATH = '/api/token/playground';

// Renew the key this many seconds before it expires, so a request never goes out with a key
// about to lapse.
export const PLAYGROUND_KEY_RENEW_SECONDS = 120;

// The backend returns the key without its prefix; callers want the full "sk-" value and when it
// expires (Unix seconds).
export async function requestPlaygroundKey(fetchImpl) {
  const doFetch = fetchImpl || ((...args) => fetch(...args));
  let res;
  try {
    res = await doFetch(PLAYGROUND_TOKEN_PATH, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' }
    });
  } catch (err) {
    throw new RelayError({ i18nKey: 'playground.errors.keyFailed', message: err?.message || '' });
  }
  const body = await res.json().catch(() => null);
  const { success, message, data } = body || {};
  if (!res.ok || !success || !data?.key) throw new RelayError({ i18nKey: 'playground.errors.keyFailed', message: message || '' });
  return { key: `sk-${data.key}`, expiresAt: Number(data.expires_at) || 0 };
}

// True while a key from requestPlaygroundKey can still be used.
export function playgroundKeyFresh(entry, nowSeconds = Date.now() / 1000) {
  return Boolean(entry?.key) && entry.expiresAt - PLAYGROUND_KEY_RENEW_SECONDS > nowSeconds;
}

export default function usePlaygroundKey() {
  const keyRef = useRef(null);
  const pendingRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // Lazy: nothing is requested until the first run; the key is then reused until it nears expiry.
  const ensureKey = useCallback(async () => {
    if (playgroundKeyFresh(keyRef.current)) return keyRef.current.key;
    if (!pendingRef.current) {
      setLoading(true);
      setError(null);
      pendingRef.current = requestPlaygroundKey()
        .then((entry) => {
          keyRef.current = entry;
          return entry.key;
        })
        .catch((err) => {
          setError(err);
          throw err;
        })
        .finally(() => {
          pendingRef.current = null;
          setLoading(false);
        });
    }
    return pendingRef.current;
  }, []);

  const authHeaders = useCallback(async () => ({ Authorization: `Bearer ${await ensureKey()}` }), [ensureKey]);

  return { ensureKey, authHeaders, loading, error };
}
