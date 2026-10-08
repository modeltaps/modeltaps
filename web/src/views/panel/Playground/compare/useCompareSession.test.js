import { describe, expect, it } from 'vitest';

import { RelayError } from 'views/panel/ApiCatalog/playground/useRelayRequest';
import {
  INITIAL_STATE,
  MAX_COLUMNS,
  columnRequest,
  columnSettings,
  compareReducer,
  createCompareRunner,
  defaultColumnModels,
  overrideCount
} from './useCompareSession';

// 对比会话的纯部分：reducer（增删列边界、清空）、列参数合并，以及 runner 的发送 / 停止 / 错误隔离。
// 测试跑在 node 环境，请求用假的 send 代替。

const chatModel = (id) => ({ id, endpoints: ['chat'], info: { capabilities: [] } });
const MODELS = [chatModel('gpt-a'), chatModel('gpt-b'), chatModel('gpt-c')];

const completion = (content) => ({ choices: [{ message: { content } }] });

// 一个最小的 store：reducer 直接跑，runner 读写同一份 state。
const harness = (send, patch = {}) => {
  let state = {
    ...INITIAL_STATE,
    settings: { ...INITIAL_STATE.settings, stream: false },
    columns: INITIAL_STATE.columns.map((column, i) => ({ ...column, modelId: MODELS[i].id })),
    ...patch
  };
  const dispatch = (action) => {
    state = compareReducer(state, action);
  };
  const runner = createCompareRunner({
    getState: () => state,
    dispatch,
    modelOf: (id) => MODELS.find((m) => m.id === id) || null,
    send,
    authHeaders: async () => ({ Authorization: 'Bearer test' })
  });
  return {
    runner,
    dispatch,
    get state() {
      return state;
    }
  };
};

// 被中止前一直挂起的请求：abort 时按 relaySend 的方式抛出 aborted 的 RelayError。
const hanging = (signal) =>
  new Promise((_, reject) => {
    signal.addEventListener('abort', () => reject(new RelayError({ i18nKey: 'playground.errors.aborted', aborted: true })));
  });

describe('compareReducer 增删列', () => {
  it('最多 4 列，最少 2 列', () => {
    let state = INITIAL_STATE;
    for (let i = 0; i < 5; i += 1) state = compareReducer(state, { type: 'add' });
    expect(state.columns).toHaveLength(MAX_COLUMNS);
    expect(new Set(state.columns.map((c) => c.id)).size).toBe(MAX_COLUMNS);

    for (const column of [...state.columns]) state = compareReducer(state, { type: 'remove', id: column.id });
    expect(state.columns).toHaveLength(2);
  });

  it('删掉激活列时激活落到第一列', () => {
    let state = compareReducer(INITIAL_STATE, { type: 'add' });
    state = compareReducer(state, { type: 'active', id: 3 });
    state = compareReducer(state, { type: 'remove', id: 3 });
    expect(state.activeId).toBe(state.columns[0].id);
  });

  it('默认两列：全局模型 + 下一个不同模型；只有一个模型时第二列留空', () => {
    expect(defaultColumnModels(MODELS, MODELS[1])).toEqual(['gpt-b', 'gpt-a']);
    expect(defaultColumnModels([MODELS[0]], null)).toEqual(['gpt-a', '']);
  });
});

describe('columnSettings', () => {
  it('非空覆盖项盖过全局值，覆盖思考深度即开启思考', () => {
    const merged = columnSettings(
      { temperature: 0.2, maxTokens: 100, thinking: false },
      { temperature: 1.5, maxTokens: '', reasoningEffort: 'high' }
    );
    expect(merged).toMatchObject({ temperature: 1.5, maxTokens: 100, thinking: true, reasoningEffort: 'high' });
    expect(overrideCount({ temperature: 1.5, maxTokens: '', reasoningEffort: 'high' })).toBe(2);
  });
});

describe('createCompareRunner', () => {
  it('sendAll 每列各发一次，列覆盖的温度只进该列请求体', async () => {
    const calls = [];
    const h = harness(async (req) => {
      calls.push(req.json);
      return completion(`from ${req.json.model}`);
    });
    h.dispatch({ type: 'settings', patch: { temperature: 0.3 } });
    h.dispatch({ type: 'override', id: h.state.columns[1].id, patch: { temperature: 1.2 } });

    await h.runner.sendAll('hi');

    expect(calls.map((json) => json.model)).toEqual(['gpt-a', 'gpt-b']);
    expect(calls[0].temperature).toBe(0.3);
    expect(calls[1].temperature).toBe(1.2);
    expect(h.state.columns.map((c) => c.turns[1].content)).toEqual(['from gpt-a', 'from gpt-b']);
    expect(h.state.input).toBe('');
  });

  it('columnRequest 与实际发出的请求体一致（代码视图共用）', async () => {
    const calls = [];
    const h = harness(async (req) => {
      calls.push(req.json);
      return completion('x');
    });
    h.dispatch({ type: 'system', value: 'be brief' });
    h.dispatch({ type: 'override', id: h.state.columns[1].id, patch: { temperature: 1.1, maxTokens: 64 } });
    const before = h.state;

    await h.runner.sendAll('hi');

    before.columns.forEach((column, i) => {
      const { model, messages, params } = columnRequest(before, column, MODELS[i], 'hi');
      expect(calls[i]).toEqual({ model, messages, ...params });
    });
    expect(calls[1]).toMatchObject({ temperature: 1.1, max_tokens: 64 });
  });

  it('一列 4xx 报错，另一列照常完成', async () => {
    const h = harness(async (req) => {
      if (req.json.model === 'gpt-a') throw new RelayError({ i18nKey: 'playground.errors.request', status: 400, message: 'bad' });
      return completion('ok');
    });

    await h.runner.sendAll('hi');

    const [a, b] = h.state.columns;
    expect(a.error?.status).toBe(400);
    expect(a.turns).toEqual([{ role: 'user', content: 'hi' }]);
    expect(b.error).toBeNull();
    expect(b.turns[1].content).toBe('ok');
    expect(h.state.columns.some((c) => c.running)).toBe(false);
  });

  it('stop 只中止一列，另一列不受影响', async () => {
    let release;
    const h = harness((req) =>
      req.json.model === 'gpt-a' ? hanging(req.signal) : new Promise((resolve) => (release = () => resolve(completion('done'))))
    );

    const pending = h.runner.sendAll('hi');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(h.state.columns.every((c) => c.running)).toBe(true);

    h.runner.stop(0);
    release();
    await pending;

    const [a, b] = h.state.columns;
    expect(a.running).toBe(false);
    expect(a.error).toBeNull();
    expect(b.turns[1].content).toBe('done');
  });

  it('sendTo 只发给指定列；未选模型的列不参与发送', async () => {
    const calls = [];
    const h = harness(async (req) => {
      calls.push(req.json.model);
      return completion('x');
    });
    h.dispatch({ type: 'model', id: h.state.columns[1].id, modelId: '' });

    await h.runner.sendTo(0, 'one');
    await h.runner.sendAll('two');

    expect(calls).toEqual(['gpt-a', 'gpt-a']);
    expect(h.state.columns[1].turns).toEqual([]);
  });

  it('clear 清掉所有列的消息与错误，列与模型保留', async () => {
    const h = harness(async () => completion('x'));
    await h.runner.sendAll('hi');
    h.dispatch({ type: 'clear' });

    expect(h.state.columns.map((c) => c.turns)).toEqual([[], []]);
    expect(h.state.columns.map((c) => c.modelId)).toEqual(['gpt-a', 'gpt-b']);
  });
});
