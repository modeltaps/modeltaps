import { useCallback, useRef, useState } from 'react';

// ==============================|| PLAYGROUND — RELAY REQUEST ||============================== //
// 试用面板对同源 /v1/* 的 fetch 封装：JSON 与 multipart 两种请求体、一个 AbortController、
// 统一的错误映射。relay 的错误体是 OpenAI 风格（{ error: { message, type, code } }），
// 网关层偶尔只回 { message }，两种都收；HTTP 状态另给一句可读文案兜底。
// 错误只回到面板内（返回 / 抛 RelayError），不走全局 showError——试用失败是面板里的一次
// 调用失败，不是应用级故障。

export class RelayError extends Error {
  constructor(detail = {}) {
    super(detail.message || detail.i18nKey || 'relay request failed');
    this.name = 'RelayError';
    this.i18nKey = detail.i18nKey || 'playground.errors.request';
    this.status = detail.status ?? 0;
    this.type = detail.type || '';
    this.code = detail.code ?? '';
    this.payload = detail.payload ?? null;
    this.aborted = Boolean(detail.aborted);
  }
}

// HTTP 状态 → 面板文案。relay 的配额不足 / 无可用渠道都落在 403 与 5xx 上，
// 具体原因由 error.message 补充，这里只保证「没有 message 时也有人话」。
export function statusErrorKey(status) {
  if (status === 401 || status === 403) return 'playground.errors.unauthorized';
  if (status === 404) return 'playground.errors.notFound';
  if (status === 429) return 'playground.errors.rateLimited';
  if (status >= 500) return 'playground.errors.upstream';
  return 'playground.errors.request';
}

export function parseRelayErrorBody(payload) {
  const error = payload && typeof payload.error === 'object' ? payload.error : null;
  const message = error?.message || payload?.message || (typeof payload?.error === 'string' ? payload.error : '') || '';
  return { message: String(message).trim(), type: error?.type || '', code: error?.code ?? '' };
}

// 错误码比状态码更具体时优先按码映射：上游渠道认证失败以 503 返回，按状态会落到「上游供应商返回错误」。
const CODE_ERROR_KEYS = { upstream_auth_failed: 'playground.errors.upstreamAuth' };

export function relayErrorFromResponse(status, payload) {
  const body = parseRelayErrorBody(payload);
  const i18nKey = CODE_ERROR_KEYS[body.code] || statusErrorKey(status);
  return new RelayError({ ...body, i18nKey, status, payload });
}

// 面板只允许打同源的 relay 入口：绝对 URL、协议相对的 //host、以及 /v1/ 之外的站内接口
// 都在发请求前拒掉——试用面板会把个人 key 放进 Authorization 头，路径一旦被构造成外部地址
// 就等于把 key 送出去。前缀之后还要挡住 ../ 与 ./ 这类回溯段，否则 /v1/../api/... 能绕出 relay。
export const RELAY_PATH_PREFIX = '/v1/';

// 段里的 %2e / %2E 是点号的百分号编码，先还原再判断，否则 /v1/%2e%2e/api/... 照样能回溯。
const decodeDots = (segment) => segment.replace(/%2e/gi, '.');

export function isRelayPath(path) {
  if (typeof path !== 'string' || !path.startsWith(RELAY_PATH_PREFIX)) return false;
  if (/[\\\s]/.test(path)) return false;
  return !path
    .split(/[?#]/)[0]
    .split('/')
    .some((segment) => {
      const decoded = decodeDots(segment);
      return decoded === '.' || decoded === '..';
    });
}

const isAbort = (err) => err?.name === 'AbortError' || err?.code === 20;

// fetch 层面的失败（断网、被中止、CORS）没有 HTTP 状态，单独映射。
export function relayErrorFromNetwork(err) {
  if (isAbort(err)) return new RelayError({ i18nKey: 'playground.errors.aborted', aborted: true });
  return new RelayError({ i18nKey: 'playground.errors.network', message: err?.message || '' });
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

export default function useRelayRequest() {
  const controllerRef = useRef(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState(null);

  const release = useCallback((controller) => {
    if (controller && controllerRef.current !== controller) return;
    controllerRef.current = null;
    setRunning(false);
  }, []);

  const abort = useCallback(() => {
    controllerRef.current?.abort();
    release(null);
  }, [release]);

  const clearError = useCallback(() => setError(null), []);

  // parse:
  //   'json'  解析并返回响应体（默认）
  //   'raw'   直接交出 Response，由调用方读流；读完必须调用 done()
  //   'blob'  二进制（图像 / 音频）
  const send = useCallback(
    async ({ path, method = 'POST', headers, json, form, parse = 'json' }) => {
      if (!isRelayPath(path)) {
        const invalid = new RelayError({ i18nKey: 'playground.errors.request', message: `invalid relay path: ${String(path)}` });
        setError(invalid);
        throw invalid;
      }
      controllerRef.current?.abort();
      const controller = new AbortController();
      controllerRef.current = controller;
      setRunning(true);
      setError(null);

      const init = { method, signal: controller.signal, headers: { ...headers } };
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
        if (parse === 'blob') {
          const blob = await response.blob();
          release(controller);
          return blob;
        }
        const payload = await readPayload(response);
        // relay 偶尔以 HTTP 200 回 OpenAI 错误体（上游把错误当正常响应转出）。
        if (payload?.error) throw relayErrorFromResponse(response.status, payload);
        release(controller);
        return payload;
      } catch (err) {
        const mapped = err instanceof RelayError ? err : relayErrorFromNetwork(err);
        if (!mapped.aborted) setError(mapped);
        release(controller);
        throw mapped;
      }
    },
    [release]
  );

  // 流式请求读完后由调用方收尾（'raw' 不在 send 里结束，否则「运行中」会在首字节就熄灭）。
  const done = useCallback(() => release(null), [release]);

  return { running, error, send, abort, done, clearError, setError };
}
