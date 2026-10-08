import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  RelayError,
  isRelayPath,
  parseRelayErrorBody,
  relayErrorFromNetwork,
  relayErrorFromResponse,
  statusErrorKey
} from './useRelayRequest';
import { PLAYGROUND_TOKEN_PATH, requestPlaygroundKey } from './usePlaygroundKey';
import { createSseParser, eventData, isDoneData, parseChunk, splitEvents } from './sse';

// 控制台请求层的回归：错误映射（relay 的 OpenAI 错误体 + HTTP 状态）、
// key 只在内存里流转（不写 localStorage / sessionStorage）、SSE 解析。

describe('statusErrorKey', () => {
  it('401/403 归为鉴权失败，429 归为限流', () => {
    expect(statusErrorKey(401)).toBe('playground.errors.unauthorized');
    expect(statusErrorKey(403)).toBe('playground.errors.unauthorized');
    expect(statusErrorKey(429)).toBe('playground.errors.rateLimited');
  });

  it('5xx 归为上游错误，其余 4xx 回落到通用文案', () => {
    expect(statusErrorKey(500)).toBe('playground.errors.upstream');
    expect(statusErrorKey(502)).toBe('playground.errors.upstream');
    expect(statusErrorKey(404)).toBe('playground.errors.notFound');
    expect(statusErrorKey(400)).toBe('playground.errors.request');
  });
});

describe('parseRelayErrorBody', () => {
  it('读 OpenAI 风格错误体', () => {
    expect(parseRelayErrorBody({ error: { message: 'no quota', type: 'insufficient_quota', code: 'x' } })).toEqual({
      message: 'no quota',
      type: 'insufficient_quota',
      code: 'x'
    });
  });

  it('网关只回 message 时也认', () => {
    expect(parseRelayErrorBody({ message: ' boom ' }).message).toBe('boom');
  });

  it('空体不炸', () => {
    expect(parseRelayErrorBody(null)).toEqual({ message: '', type: '', code: '' });
  });
});

describe('relayErrorFromResponse', () => {
  it('状态与错误体合成面板内错误对象', () => {
    const err = relayErrorFromResponse(429, { error: { message: 'slow down', type: 'rate_limit' } });
    expect(err).toBeInstanceOf(RelayError);
    expect(err.i18nKey).toBe('playground.errors.rateLimited');
    expect(err.status).toBe(429);
    expect(err.type).toBe('rate_limit');
    expect(err.message).toBe('slow down');
    expect(err.aborted).toBe(false);
  });

  it('上游渠道认证失败按错误码映射，不落到通用上游错误', () => {
    const err = relayErrorFromResponse(503, { error: { message: 'auth failed', type: 'system_error', code: 'upstream_auth_failed' } });
    expect(err.i18nKey).toBe('playground.errors.upstreamAuth');
    expect(err.status).toBe(503);
  });

  it('其他 503 仍按状态映射', () => {
    const err = relayErrorFromResponse(503, { error: { message: 'busy', code: 'service_unavailable' } });
    expect(err.i18nKey).toBe('playground.errors.upstream');
  });
});

describe('relayErrorFromNetwork', () => {
  it('中止单独标记，供调用方静默处理', () => {
    const abort = Object.assign(new Error('aborted'), { name: 'AbortError' });
    const err = relayErrorFromNetwork(abort);
    expect(err.i18nKey).toBe('playground.errors.aborted');
    expect(err.aborted).toBe(true);
  });

  it('其余 fetch 失败归为网络错误', () => {
    const err = relayErrorFromNetwork(new TypeError('Failed to fetch'));
    expect(err.i18nKey).toBe('playground.errors.network');
    expect(err.aborted).toBe(false);
  });
});

describe('isRelayPath', () => {
  it('只接受同源的 /v1/ 相对路径', () => {
    expect(isRelayPath('/v1/chat/completions')).toBe(true);
    expect(isRelayPath('/v1/images/generations')).toBe(true);
  });

  it('拒绝绝对地址、协议相对地址与 /v1 之外的站内接口', () => {
    expect(isRelayPath('https://evil.example.com/v1/chat/completions')).toBe(false);
    expect(isRelayPath('//evil.example.com/v1/chat/completions')).toBe(false);
    expect(isRelayPath('/api/token/playground')).toBe(false);
    expect(isRelayPath('/v1')).toBe(false);
    expect(isRelayPath('/v1/chat completions')).toBe(false);
    expect(isRelayPath(undefined)).toBe(false);
  });

  it('拒绝前缀之后的回溯段与反斜杠', () => {
    expect(isRelayPath('/v1/../api/token/playground')).toBe(false);
    expect(isRelayPath('/v1/./chat/completions')).toBe(false);
    expect(isRelayPath('/v1/chat/../../api/user/self')).toBe(false);
    expect(isRelayPath('/v1/chat\\completions')).toBe(false);
  });

  it('拒绝百分号编码的回溯段', () => {
    expect(isRelayPath('/v1/%2e%2e/api/token/playground')).toBe(false);
    expect(isRelayPath('/v1/%2E%2E/api/user/self')).toBe(false);
    expect(isRelayPath('/v1/chat/%2e/completions')).toBe(false);
    expect(isRelayPath('/v1/.%2e/api/status')).toBe(false);
  });
});

// 测试跑在 node 环境（见 vite.config.mjs 的 test.environment），没有真实 Storage，
// 这里塞两个假的，用「有没有被写过」来守住「key 不落盘」。
const fakeStorage = () => ({ setItem: vi.fn(), getItem: vi.fn(), removeItem: vi.fn() });

const jsonResponse = (body, ok = true) => ({ ok, json: () => Promise.resolve(body) });

describe('requestPlaygroundKey', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('取到 key 后补 sk- 前缀，且不写任何存储', async () => {
    const local = fakeStorage();
    const session = fakeStorage();
    vi.stubGlobal('localStorage', local);
    vi.stubGlobal('sessionStorage', session);
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ success: true, data: 'abc' }));

    const key = await requestPlaygroundKey(fetchImpl);

    expect(fetchImpl).toHaveBeenCalledWith(PLAYGROUND_TOKEN_PATH, expect.objectContaining({ method: 'GET', credentials: 'same-origin' }));
    expect(key).toBe('sk-abc');
    expect(local.setItem).not.toHaveBeenCalled();
    expect(session.setItem).not.toHaveBeenCalled();
  });

  it('后端 success=false 时给可读错误', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ success: false, message: 'nope' }));
    await expect(requestPlaygroundKey(fetchImpl)).rejects.toMatchObject({
      i18nKey: 'playground.errors.keyFailed',
      message: 'nope'
    });
  });

  it('HTTP 失败同样落到面板内的 keyFailed', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(jsonResponse({ message: 'unauthorized' }, false));
    await expect(requestPlaygroundKey(fetchImpl)).rejects.toBeInstanceOf(RelayError);
  });

  it('请求本身失败时也不外泄原始异常', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new Error('500'));
    await expect(requestPlaygroundKey(fetchImpl)).rejects.toBeInstanceOf(RelayError);
  });
});

describe('splitEvents', () => {
  it('只交出完整事件，未闭合的尾巴留在 rest 里', () => {
    const { blocks, rest } = splitEvents('data: a\n\ndata: b');
    expect(blocks).toEqual(['data: a']);
    expect(rest).toBe('data: b');
  });

  it('CRLF 换行同样切得开', () => {
    const { blocks } = splitEvents('data: a\r\n\r\n');
    expect(blocks).toEqual(['data: a']);
  });
});

describe('eventData', () => {
  it('多行 data 按换行拼接，心跳注释与 event 行忽略', () => {
    expect(eventData(': ping\nevent: message\ndata: one\ndata: two')).toBe('one\ntwo');
  });

  it('只有注释的事件不产出数据', () => {
    expect(eventData(': keep-alive')).toBe('');
  });
});

describe('parseChunk', () => {
  it('取 delta 文本与用量', () => {
    const chunk = parseChunk(JSON.stringify({ choices: [{ delta: { content: 'hi' } }], usage: { total_tokens: 7 } }));
    expect(chunk.text).toBe('hi');
    expect(chunk.usage).toEqual({ total_tokens: 7 });
    expect(chunk.error).toBeNull();
  });

  it('流中途的错误体单独交出，供调用方中断', () => {
    const chunk = parseChunk(JSON.stringify({ error: { message: 'upstream died', type: 'server_error' } }));
    expect(chunk.error).toEqual({ message: 'upstream died', type: 'server_error' });
    expect(chunk.payload).toEqual({ error: { message: 'upstream died', type: 'server_error' } });
  });

  it('推理模型的 reasoning_content 单独交出', () => {
    const chunk = parseChunk(JSON.stringify({ choices: [{ delta: { reasoning_content: 'thinking' } }] }));
    expect(chunk.reasoning).toBe('thinking');
    expect(chunk.text).toBe('');
  });

  it('非 JSON 的帧按无内容处理，不抛异常', () => {
    expect(parseChunk('not json')).toEqual({ text: '', reasoning: '', usage: null, error: null, payload: null });
  });
});

describe('createSseParser', () => {
  it('事件被切在半路时跨块拼回来', () => {
    const parser = createSseParser();
    expect(parser.push('data: {"choices":[{"delta":{"content":"he')).toEqual([]);
    expect(parser.push('llo"}}]}\n\n')).toEqual(['{"choices":[{"delta":{"content":"hello"}}]}']);
    expect(parseChunk('{"choices":[{"delta":{"content":"hello"}}]}').text).toBe('hello');
  });

  it('一次推入多个事件时按序返回，[DONE] 可识别', () => {
    const parser = createSseParser();
    const out = parser.push('data: {"a":1}\n\n: ping\n\ndata: [DONE]\n\n');
    expect(out).toEqual(['{"a":1}', '[DONE]']);
    expect(isDoneData(out[1])).toBe(true);
  });

  it('流结束时 flush 补出最后一个没跟空行的事件', () => {
    const parser = createSseParser();
    expect(parser.push('data: {"a":1}')).toEqual([]);
    expect(parser.flush()).toEqual(['{"a":1}']);
    expect(parser.flush()).toEqual([]);
  });
});
