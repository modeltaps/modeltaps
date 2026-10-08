import { useCallback, useMemo } from 'react';

import { createSseParser, isDoneData, parseChunk } from 'views/panel/ApiCatalog/playground/sse';
import { RelayError, relayErrorFromNetwork, relayErrorFromResponse } from 'views/panel/ApiCatalog/playground/useRelayRequest';
import { useConsole } from '../shared/ConsoleContext';
import { CHAT_RULE } from '../shared/fieldSchema';
import { createSessionStore, playgroundAuthHeaders, relaySend, useSessionState } from '../shared/sessionStore';
import useModalityModel from '../shared/useModalityModel';
import { REASONING_EFFORTS } from './chatFields';
import { createChatPersistence, loadChatSession, onChatSessionCleared, removeChatSession, safeStorage } from './chatPersistence';

// ==============================|| PLAYGROUND — CHAT SESSION ||============================== //
// 控制台对话模态的会话层：多轮消息、系统提示词、运行参数、流式读取与每轮计时。
// 同源 /v1/chat/completions、Authorization 头、include_usage，
// 每轮额外记录「首 token 延迟 / 总耗时 / 本轮用量 / 原始响应」。
// 状态走 reducer、流式读取走 consumeStream，两者都是纯的，单测不需要 DOM。
// 会话放模块级 store 并落 localStorage（见 chatPersistence）：切 tab、切模态、刷新都还在。

export const CHAT_PATH = '/v1/chat/completions';

// 数值字段一律「留空 = 不发送」：滑块只是取值方式，没拖过就不该替用户决定参数。
export const DEFAULT_SETTINGS = {
  temperature: '',
  topP: '',
  maxTokens: '',
  seed: '',
  thinking: false,
  reasoningEffort: '',
  thinkingBudget: '',
  stream: true,
  jsonMode: false
};

// 留空时滑块停在哪：只影响滑块位置，不进请求体。
export const SLIDER_FALLBACK = { temperature: 1, topP: 1, maxTokens: 1024, thinkingBudget: 8192 };

export const INITIAL_STATE = { turns: [], input: '', system: '', settings: DEFAULT_SETTINGS };

const toNumber = (value) => {
  if (value === '' || value === null || value === undefined) return null;
  const parsed = Number(value);
  return Number.isNaN(parsed) ? null : parsed;
};

// 运行设置 → 请求体附加字段。能力型参数只发给声明了对应 capability 的模型，
// 切模型后残留的开关不会跟着发出去。
export function buildParams(settings = {}, capabilities = []) {
  const params = {};
  const temperature = toNumber(settings.temperature);
  const topP = toNumber(settings.topP);
  const maxTokens = toNumber(settings.maxTokens);
  const seed = toNumber(settings.seed);
  if (temperature !== null) params.temperature = temperature;
  if (topP !== null) params.top_p = topP;
  if (maxTokens !== null) params.max_tokens = maxTokens;
  if (seed !== null) params.seed = seed;
  if (settings.stream !== false) {
    params.stream = true;
    params.stream_options = { include_usage: true };
  }
  // 思考深度与思考预算各自可空：都没给就只发 { enabled: true }，别替用户定死上游默认。
  if (settings.thinking && capabilities.includes('reasoning')) {
    const budget = toNumber(settings.thinkingBudget);
    const reasoning = {};
    if (REASONING_EFFORTS.includes(settings.reasoningEffort)) reasoning.effort = settings.reasoningEffort;
    if (budget !== null) reasoning.max_tokens = budget;
    params.reasoning = Object.keys(reasoning).length > 0 ? reasoning : { enabled: true };
  }
  if (settings.jsonMode && capabilities.includes('structured_output')) params.response_format = { type: 'json_object' };
  return params;
}

// 发出去的 messages：可选系统提示词 + 已有轮次 + 本次输入。空内容的轮次（回滚残留、
// 还没收到首个 token 的助手气泡）不进请求体。
export function buildMessages({ system = '', turns = [], pending = '' } = {}) {
  const history = turns
    .filter((turn) => turn?.role && String(turn.content ?? '').trim() !== '')
    .map((turn) => ({ role: turn.role, content: turn.content }));
  return [
    ...(system.trim() ? [{ role: 'system', content: system }] : []),
    ...history,
    ...(pending.trim() ? [{ role: 'user', content: pending }] : [])
  ];
}

const assistantTurn = () => ({
  role: 'assistant',
  content: '',
  reasoning: '',
  usage: null,
  firstTokenMs: null,
  durationMs: null,
  raw: '',
  pending: true,
  completed: false
});

const patchLast = (turns, patch) => turns.map((turn, index) => (index === turns.length - 1 ? { ...turn, ...patch } : turn));

export function chatReducer(state, action) {
  switch (action.type) {
    case 'input':
      return { ...state, input: action.value };
    case 'system':
      return { ...state, system: action.value };
    case 'settings':
      return { ...state, settings: { ...state.settings, ...action.patch } };
    // 发起一轮：用户消息 + 空的助手气泡一起进列表，输入框就地清空。
    case 'start':
      return { ...state, input: '', turns: [...state.turns, { role: 'user', content: action.prompt }, assistantTurn()] };
    case 'delta':
      return { ...state, turns: patchLast(state.turns, { content: action.text, reasoning: action.reasoning }) };
    case 'finish':
      return {
        ...state,
        turns: patchLast(state.turns, {
          content: action.result.text,
          reasoning: action.result.reasoning,
          usage: action.result.usage,
          firstTokenMs: action.result.firstTokenMs,
          durationMs: action.result.durationMs,
          raw: action.result.raw,
          pending: false,
          completed: Boolean(action.result.completed),
          interrupted: !action.result.completed
        })
      };
    // 一个字都没回来：撤掉这一轮（含空气泡），提问退回输入框以便直接重试。
    case 'rollback':
      return {
        ...state,
        turns: state.turns.slice(0, -2),
        input: state.input.trim() ? state.input : action.prompt
      };
    // 「清空对话」连系统提示词一起清，运行参数保留。
    case 'clear':
      return { ...state, turns: [], system: '' };
    default:
      return state;
  }
}

// 读流：把增量文本 / 思考过程 / 用量攒出来，并记下首 token 延迟与本轮总耗时。
// 中止与网络错误都不抛给调用方，而是落在返回值的 aborted / error 上——已经吐出来的
// 半截回答要留在界面上。只有见到 [DONE] 或 finish_reason 才算 completed，连接被掐断不算。
export async function consumeStream({ response, clock = () => Date.now(), startedAt = clock(), onProgress } = {}) {
  const parser = createSseParser();
  const decoder = new TextDecoder();
  const result = {
    text: '',
    reasoning: '',
    raw: '',
    usage: null,
    firstTokenMs: null,
    durationMs: 0,
    aborted: false,
    error: null,
    completed: false
  };

  const consume = (events) => {
    for (const data of events) {
      if (isDoneData(data)) {
        result.completed = true;
        continue;
      }
      const parsed = parseChunk(data);
      if (parsed.error) throw relayErrorFromResponse(response?.status ?? 0, parsed.payload);
      if (parsed.payload?.choices?.[0]?.finish_reason) result.completed = true;
      if (parsed.text || parsed.reasoning) {
        if (result.firstTokenMs === null) result.firstTokenMs = clock() - startedAt;
        result.text += parsed.text;
        result.reasoning += parsed.reasoning;
        onProgress?.(result);
      }
      if (parsed.usage) result.usage = parsed.usage;
    }
  };

  try {
    const reader = response?.body?.getReader();
    if (!reader) throw new Error('missing response stream');
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      const chunk = decoder.decode(value, { stream: true });
      result.raw += chunk;
      consume(parser.push(chunk));
    }
    consume(parser.flush());
  } catch (err) {
    const mapped = err instanceof RelayError ? err : relayErrorFromNetwork(err);
    result.aborted = mapped.aborted;
    result.error = mapped;
    result.completed = false;
  } finally {
    result.durationMs = clock() - startedAt;
  }
  return result;
}

// 非流式回包：一次性拿到整条回答，没有首 token 延迟可言。
export function readCompletion(payload) {
  const message = payload?.choices?.[0]?.message || {};
  return {
    text: typeof message.content === 'string' ? message.content : '',
    reasoning: typeof message.reasoning_content === 'string' ? message.reasoning_content : '',
    usage: payload?.usage || null,
    raw: payload ? JSON.stringify(payload, null, 2) : '',
    firstTokenMs: null,
    completed: true
  };
}

// running / error 只活在内存里，不落盘。
const RUNTIME = { running: false, error: null };

export const chatStore = createSessionStore({
  ...INITIAL_STATE,
  ...loadChatSession(safeStorage(), DEFAULT_SETTINGS),
  ...RUNTIME
});

const persistence = createChatPersistence({ store: chatStore });

// 关页 / 刷新前把节流中的那次写掉，别丢最后几个字。
if (typeof window !== 'undefined') window.addEventListener('pagehide', () => persistence.pending() && persistence.flush());

export const flushChatSession = () => persistence.flush();

const dispatch = (action) => chatStore.setState((state) => chatReducer(state, action));

let controller = null;

export function abortChat() {
  controller?.abort();
}

// 「清空」：内存里的轮次与系统提示词清掉，本地副本整份删除。
export function clearChat() {
  dispatch({ type: 'clear' });
  chatStore.setState({ error: null });
  persistence.cancel();
  removeChatSession(safeStorage());
}

// 退出登录：中止进行中的请求，内存回到初始状态（本地副本由 clearChatSession 删）。
onChatSessionCleared(() => {
  controller?.abort();
  controller = null;
  chatStore.setState({ ...INITIAL_STATE, ...RUNTIME });
  persistence.cancel();
});

// 一次运行：history 显式传入时以它为历史（「重跑」丢掉最后一轮后再发）。
// send / authHeaders 可注入，单测不用真的发请求。
export async function runChat({ prompt, history, modelId, params = {}, send = relaySend, authHeaders = playgroundAuthHeaders }) {
  const state = chatStore.getState();
  if (!prompt?.trim() || !modelId || state.running) return;

  const outgoing = buildMessages({ system: state.system, turns: history ?? state.turns, pending: prompt });
  const streaming = state.settings.stream !== false;
  const current = new AbortController();
  controller = current;
  dispatch({ type: 'start', prompt });
  // 「进行中」立刻落盘：刷新 / 关页来不及写任何东西时，读回来也知道这一轮没收完。
  persistence.flush();
  chatStore.setState({ running: true, error: null });
  const rollback = () => dispatch({ type: 'rollback', prompt });
  const startedAt = Date.now();

  // 退出登录会换掉 controller：旧请求的收尾不能再改新会话。
  const active = () => controller === current;
  const settle = (err) => {
    if (!active()) return;
    controller = null;
    chatStore.setState({ running: false, error: err && !err.aborted ? err : null });
  };

  let response;
  try {
    const headers = await authHeaders();
    response = await send({
      path: CHAT_PATH,
      headers,
      json: { model: modelId, messages: outgoing, ...params },
      parse: streaming ? 'raw' : 'json',
      signal: current.signal
    });
  } catch (err) {
    if (!active()) return;
    rollback();
    settle(err instanceof RelayError ? err : relayErrorFromNetwork(err));
    return;
  }

  if (!streaming) {
    if (!active()) return;
    const result = { ...readCompletion(response), durationMs: Date.now() - startedAt };
    if (!result.text && !result.reasoning) rollback();
    else dispatch({ type: 'finish', result });
    settle(null);
    return;
  }

  const result = await consumeStream({
    response,
    startedAt,
    onProgress: (partial) => active() && dispatch({ type: 'delta', text: partial.text, reasoning: partial.reasoning })
  });
  if (!active()) return;
  if (!result.text && !result.reasoning) rollback();
  else dispatch({ type: 'finish', result });
  settle(result.error);
}

export default function useChatSession() {
  const state = useSessionState(chatStore);
  const { running, error } = state;
  const { setSelectedModel } = useConsole();

  // 本模态只认声明了 chat 接口的模型，不让对话打到生图模型上。
  const { options, model } = useModalityModel(CHAT_RULE);

  const modelId = model?.id || '';
  const capabilities = useMemo(() => model?.info?.capabilities || [], [model]);
  const params = useMemo(() => buildParams(state.settings, capabilities), [state.settings, capabilities]);
  // 「代码」视图与等效请求都看这一份：历史 + 输入框里还没发出去的那条。
  const messages = useMemo(
    () => buildMessages({ system: state.system, turns: state.turns, pending: state.input }),
    [state.system, state.turns, state.input]
  );

  const setInput = useCallback((value) => dispatch({ type: 'input', value }), []);
  const setSystem = useCallback((value) => dispatch({ type: 'system', value }), []);
  const updateSettings = useCallback((patch) => dispatch({ type: 'settings', patch }), []);

  const run = useCallback(
    (override, history) =>
      runChat({ prompt: typeof override === 'string' ? override : chatStore.getState().input, history, modelId, params }),
    [modelId, params]
  );

  // 「重跑」：丢掉最后一轮（提问 + 回答），用同一条提问再发一次。
  const rerun = useCallback(() => {
    const { turns, running: busy } = chatStore.getState();
    const last = turns[turns.length - 2];
    if (!last || last.role !== 'user' || busy || !modelId) return;
    dispatch({ type: 'rollback', prompt: '' });
    run(last.content, turns.slice(0, -2));
  }, [modelId, run]);

  return {
    ...state,
    model,
    options,
    setSelectedModel,
    modelId,
    capabilities,
    params,
    messages,
    running,
    error,
    setInput,
    setSystem,
    updateSettings,
    run,
    rerun,
    abort: abortChat,
    clear: clearChat
  };
}
