import { useSyncExternalStore } from 'react';

import { requestPlaygroundKey } from 'views/panel/ApiCatalog/playground/usePlaygroundKey';
import { RelayError, isRelayPath, relayErrorFromNetwork, relayErrorFromResponse } from 'views/panel/ApiCatalog/playground/useRelayRequest';

// ==============================|| PLAYGROUND — SESSION STORE ||============================== //
// 控制台把「表单 | 代码」两个视图挂在同一层（见 index.jsx），切 tab 会卸载另一侧，所以模态的
// 会话状态不能待在组件里：每个模态建一个模块级 store，工作区 / 设置栏 / 代码视图三处订阅同
// 一份状态，切 tab、切模态回来都还在。
// 请求同样放在模块级：一次运行的「进行中 / 结果」不随工作区卸载而丢失，个人 key 也只取一次。
// 本文件属 shared 层：不 import 任何 React 组件。

export function createSessionStore(initialState) {
  let state = initialState;
  const listeners = new Set();

  const getState = () => state;

  const setState = (patch) => {
    const next = typeof patch === 'function' ? patch(state) : patch;
    state = { ...state, ...next };
    listeners.forEach((listener) => listener());
  };

  const subscribe = (listener) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  };

  return { getState, setState, subscribe };
}

export const useSessionState = (store) => useSyncExternalStore(store.subscribe, store.getState, store.getState);

// 个人 playground key：整页只取一次，只活在本模块的闭包里，只经 Authorization 头发出——
// 不落浏览器存储、不进 URL、不进 Redux、不渲染到 DOM（等效代码一律用占位符）。
let cachedKey = '';
let pendingKey = null;

export async function playgroundAuthHeaders() {
  if (!cachedKey) {
    if (!pendingKey) {
      pendingKey = requestPlaygroundKey()
        .then((key) => {
          cachedKey = key;
          return key;
        })
        .finally(() => {
          pendingKey = null;
        });
    }
    await pendingKey;
  }
  return { Authorization: `Bearer ${cachedKey}` };
}

// 错误体可能不是 JSON（网关 502 的 HTML 页、空 body），解析失败时退回纯文本。
async function readPayload(response) {
  const text = await response.text().catch(() => '');
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return { message: text.slice(0, 500) };
  }
}

// useRelayRequest.send 的无状态版本：同一套路径守卫与错误映射，进行中状态由调用方的 store 记。
// parse 'raw' 直接交出 Response，由调用方读流。
export async function relaySend({ path, method = 'POST', headers, json, form, parse = 'json', signal }) {
  if (!isRelayPath(path)) throw new RelayError({ i18nKey: 'playground.errors.request', message: `invalid relay path: ${String(path)}` });

  const init = { method, signal, headers: { ...headers } };
  // multipart 不能自己写 Content-Type，boundary 由浏览器补。
  if (form) init.body = form;
  else if (json !== undefined) {
    init.headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(json);
  }

  try {
    const response = await fetch(path, init);
    if (!response.ok) throw relayErrorFromResponse(response.status, await readPayload(response));
    if (parse === 'raw') return response;
    if (parse === 'blob') return await response.blob();
    const payload = await readPayload(response);
    // relay 偶尔以 HTTP 200 回 OpenAI 错误体（上游把错误当正常响应转出）。
    if (payload?.error) throw relayErrorFromResponse(response.status, payload);
    return payload;
  } catch (err) {
    throw err instanceof RelayError ? err : relayErrorFromNetwork(err);
  }
}
