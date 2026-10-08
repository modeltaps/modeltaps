import { useCallback, useRef, useState } from 'react';

import { RelayError } from './useRelayRequest';

// ==============================|| PLAYGROUND — API KEY ||============================== //
// 试用面板用个人的 sys_playground 长期 token 调同源 /v1（后端 GET /api/token/playground
// 自动建 / 复用）。key 只活在本 hook 的 ref 里，只经 Authorization 头发出：
// 不落任何浏览器存储、不进 URL、不进 Redux、不渲染到 DOM。
// 面板里展示的等效 curl 一律用占位符，不回填真实 key。
// 取 key 走裸 fetch 而不是 utils/api 的 API 实例：后者的响应拦截器会额外弹一次全局 toast，
// 而取 key 失败在这里只是面板内的一次失败，由面板的错误条呈现。

export const PLAYGROUND_TOKEN_PATH = '/api/token/playground';

// 后端回的是裸 key，调用方要的是 sk- 前缀的完整值。
export async function requestPlaygroundKey(fetchImpl) {
  const doFetch = fetchImpl || ((...args) => fetch(...args));
  let res;
  try {
    res = await doFetch(PLAYGROUND_TOKEN_PATH, {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' }
    });
  } catch (err) {
    throw new RelayError({ i18nKey: 'playground.errors.keyFailed', message: err?.message || '' });
  }
  const body = await res.json().catch(() => null);
  const { success, message, data } = body || {};
  if (!res.ok || !success || !data) throw new RelayError({ i18nKey: 'playground.errors.keyFailed', message: message || '' });
  return `sk-${data}`;
}

export default function usePlaygroundKey() {
  const keyRef = useRef('');
  const pendingRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // 懒取：页面打开不动 key，用户第一次点「运行」才请求，之后整个会话复用同一份内存副本。
  const ensureKey = useCallback(async () => {
    if (keyRef.current) return keyRef.current;
    if (!pendingRef.current) {
      setLoading(true);
      setError(null);
      pendingRef.current = requestPlaygroundKey()
        .then((key) => {
          keyRef.current = key;
          return key;
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
