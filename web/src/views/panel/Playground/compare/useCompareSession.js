import { useCallback, useEffect, useMemo, useReducer, useRef } from 'react';

import { buildParams, buildMessages, CHAT_PATH, consumeStream, DEFAULT_SETTINGS, readCompletion } from '../chat/useChatSession';
import { useConsole } from '../shared/ConsoleContext';
import { CHAT_RULE } from '../shared/fieldSchema';
import { playgroundAuthHeaders, relaySend } from '../shared/sessionStore';
import { resolveModalityModel } from '../shared/useModalityModel';

// ==============================|| PLAYGROUND — COMPARE SESSION ||============================== //
// 对比模态的会话层：2–4 列并排，每列 { id, modelId, override, turns, running, error }，
// 共享系统提示词、全局运行参数与输入框。列数是动态的，所以不在循环里起 useChatSession：
// 状态走一个 reducer，请求走 relaySend，每列各持一个 AbortController，停止 / 报错互不影响。
// 列参数 = 全局 settings 叠上该列的 override（只开放温度 / 最大 tokens / 思考深度三项）。
// 列模型只在本页生效，不回写全局选中模型，也不写 ?model=。
// reducer 与 createCompareRunner 都不依赖 React，单测直接喂假的 send。

export const MIN_COLUMNS = 2;
export const MAX_COLUMNS = 4;
export const OVERRIDE_KEYS = ['temperature', 'maxTokens', 'reasoningEffort'];

const isBlank = (value) => value === '' || value === null || value === undefined;

const makeColumn = (id, modelId = '') => ({ id, modelId, override: {}, turns: [], running: false, error: null });

export const INITIAL_STATE = {
  columns: [makeColumn(1), makeColumn(2)],
  nextId: 3,
  initialized: false,
  input: '',
  system: '',
  settings: DEFAULT_SETTINGS,
  sync: true,
  activeId: 1
};

// 列参数：override 里非空的项盖过全局值；单独覆盖了思考深度就意味着这一列要开思考。
export function columnSettings(settings = {}, override = {}) {
  const merged = { ...settings };
  for (const key of OVERRIDE_KEYS) if (!isBlank(override[key])) merged[key] = override[key];
  if (!isBlank(override.reasoningEffort)) merged.thinking = true;
  return merged;
}

export const overrideCount = (override = {}) => OVERRIDE_KEYS.filter((key) => !isBlank(override[key])).length;

// 一列的等效请求体：发送与代码视图共用，保证代码里的 model / 参数与该列实际请求一致。
export function columnRequest({ system = '', settings } = {}, column, model, pending = '') {
  return {
    model: column?.modelId || '',
    messages: buildMessages({ system, turns: column?.turns || [], pending }),
    params: buildParams(columnSettings(settings, column?.override), model?.info?.capabilities || [])
  };
}

// 默认两列：全局选中模型（不满足对话规则时落到推荐项）+ 候选里下一个不同的模型。
export function defaultColumnModels(models, selectedModel) {
  const { options, model } = resolveModalityModel(models, CHAT_RULE, selectedModel);
  const second = options.find((item) => item.id !== model?.id);
  return [model?.id || '', second?.id || ''];
}

const assistantTurn = () => ({
  role: 'assistant',
  content: '',
  reasoning: '',
  usage: null,
  firstTokenMs: null,
  durationMs: null,
  raw: '',
  pending: true
});

const patchLast = (turns, patch) => turns.map((turn, index) => (index === turns.length - 1 ? { ...turn, ...patch } : turn));

const mapColumn = (state, id, fn) => ({ ...state, columns: state.columns.map((column) => (column.id === id ? fn(column) : column)) });

export function compareReducer(state, action) {
  switch (action.type) {
    case 'init':
      if (state.initialized) return state;
      return {
        ...state,
        initialized: true,
        columns: state.columns.map((column, i) => ({ ...column, modelId: column.modelId || action.modelIds[i] || '' }))
      };
    case 'add':
      if (state.columns.length >= MAX_COLUMNS) return state;
      return { ...state, nextId: state.nextId + 1, columns: [...state.columns, makeColumn(state.nextId, action.modelId || '')] };
    case 'remove': {
      if (state.columns.length <= MIN_COLUMNS) return state;
      const columns = state.columns.filter((column) => column.id !== action.id);
      const activeId = columns.some((column) => column.id === state.activeId) ? state.activeId : columns[0].id;
      return { ...state, columns, activeId };
    }
    case 'model':
      return mapColumn(state, action.id, (column) => ({ ...column, modelId: action.modelId }));
    case 'override':
      return mapColumn(state, action.id, (column) => ({ ...column, override: { ...column.override, ...action.patch } }));
    case 'active':
      return { ...state, activeId: action.id };
    case 'input':
      return { ...state, input: action.value };
    case 'system':
      return { ...state, system: action.value };
    case 'settings':
      return { ...state, settings: { ...state.settings, ...action.patch } };
    case 'sync':
      return { ...state, sync: Boolean(action.value) };
    case 'start':
      return mapColumn(state, action.id, (column) => ({
        ...column,
        running: true,
        error: null,
        turns: [...column.turns, { role: 'user', content: action.prompt }, assistantTurn()]
      }));
    case 'delta':
      return mapColumn(state, action.id, (column) => ({
        ...column,
        turns: patchLast(column.turns, { content: action.text, reasoning: action.reasoning })
      }));
    // 收尾：有内容就落定助手轮；一个字都没回来就撤掉空气泡（提问留着，错误挂在列上）。
    case 'finish':
      return mapColumn(state, action.id, (column) => {
        const result = action.result;
        const hasContent = Boolean(result && (result.text || result.reasoning));
        const turns = hasContent
          ? patchLast(column.turns, {
              content: result.text,
              reasoning: result.reasoning,
              usage: result.usage,
              firstTokenMs: result.firstTokenMs,
              durationMs: result.durationMs,
              raw: result.raw,
              pending: false
            })
          : column.turns.slice(0, -1);
        return { ...column, turns, running: false, error: action.error || null };
      });
    // 「清空」连系统提示词一起清（同对话页），列与模型、参数保留。
    case 'clear':
      return { ...state, system: '', columns: state.columns.map((column) => ({ ...column, turns: [], running: false, error: null })) };
    default:
      return state;
  }
}

// 发送与停止：getState 读当前快照，modelOf 把 modelId 解析成候选里的模型（不在候选里的列不发），
// controllers 按列 id 存 AbortController。每列一条独立请求，一列失败只落在它自己的 error 上。
export function createCompareRunner({
  getState,
  dispatch,
  modelOf,
  send = relaySend,
  authHeaders = playgroundAuthHeaders,
  controllers = new Map(),
  clock = () => Date.now()
}) {
  const runColumn = async (column, prompt, state, headersPromise) => {
    const model = modelOf(column.modelId);
    const controller = new AbortController();
    controllers.set(column.id, controller);
    const { messages: outgoing, params } = columnRequest(state, column, model, prompt);
    const streaming = params.stream === true;
    dispatch({ type: 'start', id: column.id, prompt });
    const startedAt = clock();
    const finish = (result, error) => {
      if (controllers.get(column.id) === controller) controllers.delete(column.id);
      dispatch({ type: 'finish', id: column.id, result, error: error && !error.aborted ? error : null });
    };

    let response;
    try {
      const headers = await headersPromise;
      response = await send({
        path: CHAT_PATH,
        headers,
        json: { model: column.modelId, messages: outgoing, ...params },
        parse: streaming ? 'raw' : 'json',
        signal: controller.signal
      });
    } catch (err) {
      finish(null, err);
      return;
    }

    if (!streaming) {
      finish({ ...readCompletion(response), durationMs: clock() - startedAt }, null);
      return;
    }
    const result = await consumeStream({
      response,
      clock,
      startedAt,
      onProgress: (partial) => dispatch({ type: 'delta', id: column.id, text: partial.text, reasoning: partial.reasoning })
    });
    finish(result, result.error);
  };

  const run = async (targets, text) => {
    const state = getState();
    const prompt = typeof text === 'string' ? text : state.input;
    const ready = targets.filter((column) => column && modelOf(column.modelId) && !column.running && !controllers.has(column.id));
    if (!prompt.trim() || ready.length === 0) return;
    dispatch({ type: 'input', value: '' });
    const headersPromise = Promise.resolve().then(authHeaders);
    // 列共用一次取 key；取 key 失败时每列各自报错，不抛给调用方。
    headersPromise.catch(() => {});
    await Promise.all(ready.map((column) => runColumn(column, prompt, state, headersPromise)));
  };

  const columnAt = (index) => getState().columns[index];

  return {
    sendAll: (text) => run(getState().columns, text),
    sendTo: (index, text) => run([columnAt(index)], text),
    stop: (index) => {
      const column = columnAt(index);
      if (column) controllers.get(column.id)?.abort();
    },
    stopAll: () => controllers.forEach((controller) => controller.abort())
  };
}

export default function useCompareSession() {
  const { models, selectedModel } = useConsole();
  const [state, dispatch] = useReducer(compareReducer, INITIAL_STATE);

  // 列的候选只按对话规则过滤，不做全局同步。
  const options = useMemo(() => resolveModalityModel(models, CHAT_RULE, selectedModel).options, [models, selectedModel]);

  const stateRef = useRef(state);
  stateRef.current = state;
  const optionsRef = useRef(options);
  optionsRef.current = options;

  useEffect(() => {
    if (!state.initialized && options.length > 0) dispatch({ type: 'init', modelIds: defaultColumnModels(models, selectedModel) });
  }, [state.initialized, options.length, models, selectedModel]);

  const runner = useMemo(
    () =>
      createCompareRunner({
        getState: () => stateRef.current,
        dispatch,
        modelOf: (id) => (id ? optionsRef.current.find((item) => item.id === id) || null : null)
      }),
    []
  );

  // 离开页面时中止所有列的请求。
  useEffect(() => () => runner.stopAll(), [runner]);

  const idAt = (index) => stateRef.current.columns[index]?.id;
  const addColumn = useCallback((modelId = '') => dispatch({ type: 'add', modelId }), []);
  const removeColumn = useCallback(
    (index) => {
      runner.stop(index);
      dispatch({ type: 'remove', id: idAt(index) });
    },
    [runner]
  );
  const setColumnModel = useCallback((index, modelId) => dispatch({ type: 'model', id: idAt(index), modelId }), []);
  const setOverride = useCallback((index, patch) => dispatch({ type: 'override', id: idAt(index), patch }), []);
  const setActive = useCallback((index) => dispatch({ type: 'active', id: idAt(index) }), []);
  const setInput = useCallback((value) => dispatch({ type: 'input', value }), []);
  const setSystem = useCallback((value) => dispatch({ type: 'system', value }), []);
  const updateSettings = useCallback((patch) => dispatch({ type: 'settings', patch }), []);
  const setSync = useCallback((value) => dispatch({ type: 'sync', value }), []);
  const clearAll = useCallback(() => {
    runner.stopAll();
    dispatch({ type: 'clear' });
  }, [runner]);

  const running = state.columns.some((column) => column.running);
  const hasContent = state.columns.some((column) => column.turns.length > 0 || column.error);
  const activeIndex = Math.max(
    0,
    state.columns.findIndex((column) => column.id === state.activeId)
  );

  return {
    ...state,
    options,
    running,
    hasContent,
    activeIndex,
    addColumn,
    removeColumn,
    setColumnModel,
    setOverride,
    setActive,
    setInput,
    setSystem,
    updateSettings,
    setSync,
    sendAll: runner.sendAll,
    sendTo: runner.sendTo,
    stop: runner.stop,
    stopAll: runner.stopAll,
    clearAll
  };
}
