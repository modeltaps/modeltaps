import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CODE_LANGUAGES, buildSnippets } from '../shared/codegen';
import { createSessionStore } from '../shared/sessionStore';
import {
  CHAT_STORAGE_KEY,
  clearChatSession,
  createChatPersistence,
  loadChatSession,
  parseChatSession,
  saveChatSession,
  serializeChatSession
} from './chatPersistence';
import {
  DEFAULT_SETTINGS,
  INITIAL_STATE,
  abortChat,
  buildMessages,
  buildParams,
  chatReducer,
  chatStore,
  clearChat,
  consumeStream,
  flushChatSession,
  readCompletion,
  runChat
} from './useChatSession';

// 会话层的纯部分：messages 拼装、reducer（发起 / 增量 / 清空）、读流（计时与中止清理）。
// 测试跑在 node 环境（见 vite.config.mjs），所以流用一个假的 reader 喂字节。

const encoder = new TextEncoder();

// 每次 read() 交出一块；chunks 用完后 done。error 在指定次数后抛出（模拟中止）。
const streamResponse = (chunks, { throwAfter = -1, error } = {}) => {
  let index = 0;
  return {
    status: 200,
    body: {
      getReader: () => ({
        read: async () => {
          if (index === throwAfter) throw error;
          if (index >= chunks.length) return { done: true, value: undefined };
          const value = encoder.encode(chunks[index]);
          index += 1;
          return { done: false, value };
        }
      })
    }
  };
};

const sse = (payload) => `data: ${JSON.stringify(payload)}\n\n`;
const delta = (content) => sse({ choices: [{ delta: { content } }] });

describe('buildMessages', () => {
  it('系统提示词 + 历史 + 输入框里这一条按序拼装', () => {
    const messages = buildMessages({
      system: '你是接入顾问',
      turns: [
        { role: 'user', content: 'hi' },
        { role: 'assistant', content: 'hello' }
      ],
      pending: '再来一条'
    });

    expect(messages).toEqual([
      { role: 'system', content: '你是接入顾问' },
      { role: 'user', content: 'hi' },
      { role: 'assistant', content: 'hello' },
      { role: 'user', content: '再来一条' }
    ]);
  });

  it('系统提示词为空不占位，空内容的轮次不发出去', () => {
    const messages = buildMessages({
      system: '   ',
      turns: [
        { role: 'user', content: 'hi' },
        { role: 'assistant', content: '' }
      ],
      pending: '  '
    });

    expect(messages).toEqual([{ role: 'user', content: 'hi' }]);
  });
});

describe('buildParams', () => {
  it('留空的数值不进请求体，流式恒带 include_usage', () => {
    expect(buildParams({ temperature: '', maxTokens: '512', stream: true })).toEqual({
      max_tokens: 512,
      stream: true,
      stream_options: { include_usage: true }
    });
  });

  it('能力型参数只发给声明了该能力的模型', () => {
    const settings = { stream: false, thinking: true, thinkingBudget: '2048', jsonMode: true };
    expect(buildParams(settings, [])).toEqual({});
    expect(buildParams(settings, ['reasoning', 'structured_output'])).toEqual({
      reasoning: { max_tokens: 2048 },
      response_format: { type: 'json_object' }
    });
  });

  it('思考深度与思考预算各自可空，都没给就只发 enabled', () => {
    const base = { stream: false, thinking: true };
    expect(buildParams({ ...base, reasoningEffort: 'high' }, ['reasoning'])).toEqual({ reasoning: { effort: 'high' } });
    expect(buildParams({ ...base, thinkingBudget: '2048' }, ['reasoning'])).toEqual({ reasoning: { max_tokens: 2048 } });
    expect(buildParams({ ...base, reasoningEffort: 'low', thinkingBudget: '2048' }, ['reasoning'])).toEqual({
      reasoning: { effort: 'low', max_tokens: 2048 }
    });
    expect(buildParams({ ...base, reasoningEffort: '' }, ['reasoning'])).toEqual({ reasoning: { enabled: true } });
  });

  it('取值集合外的思考深度不发出去', () => {
    expect(buildParams({ stream: false, thinking: true, reasoningEffort: 'turbo' }, ['reasoning'])).toEqual({
      reasoning: { enabled: true }
    });
  });

  // 「查看代码」吃的就是这份 params，示例代码里也得带上思考深度。
  it('思考深度进「查看代码」生成的三语言示例', () => {
    const snippets = buildSnippets({
      modality: 'chat',
      baseUrl: 'https://api.example.com',
      model: 'm',
      messages: [{ role: 'user', content: 'hi' }],
      params: buildParams({ stream: false, thinking: true, reasoningEffort: 'high' }, ['reasoning'])
    });
    for (const language of CODE_LANGUAGES) expect(snippets[language]).toContain('"effort": "high"');
  });
});

describe('chatReducer', () => {
  it('发起一轮时补上空的助手气泡并清空输入框', () => {
    const state = chatReducer({ ...INITIAL_STATE, input: 'hi' }, { type: 'start', prompt: 'hi' });
    expect(state.input).toBe('');
    expect(state.turns.map((turn) => turn.role)).toEqual(['user', 'assistant']);
    expect(state.turns[1]).toMatchObject({ content: '', pending: true, firstTokenMs: null });
  });

  it('「清空对话」连系统提示词一起清，运行参数保留', () => {
    const before = {
      ...INITIAL_STATE,
      system: '你是接入顾问',
      turns: [{ role: 'user', content: 'hi' }],
      settings: { ...INITIAL_STATE.settings, temperature: '0.7' }
    };
    const after = chatReducer(before, { type: 'clear' });

    expect(after.turns).toEqual([]);
    expect(after.system).toBe('');
    expect(after.settings.temperature).toBe('0.7');
  });

  it('回滚撤掉这一轮并把提问退回输入框', () => {
    const before = chatReducer({ ...INITIAL_STATE, input: 'hi' }, { type: 'start', prompt: 'hi' });
    const after = chatReducer(before, { type: 'rollback', prompt: 'hi' });

    expect(after.turns).toEqual([]);
    expect(after.input).toBe('hi');
  });

  it('finish 把耗时与用量写回最后一条助手消息', () => {
    const started = chatReducer(INITIAL_STATE, { type: 'start', prompt: 'hi' });
    const state = chatReducer(started, {
      type: 'finish',
      result: {
        text: 'hello',
        reasoning: '',
        usage: { prompt_tokens: 3 },
        firstTokenMs: 120,
        durationMs: 900,
        raw: 'data: …',
        completed: true
      }
    });

    expect(state.turns[1]).toMatchObject({
      content: 'hello',
      firstTokenMs: 120,
      durationMs: 900,
      pending: false,
      completed: true,
      interrupted: false
    });
  });

  it('没收完就收尾（中止 / 断流）的助手轮标成「已中断」', () => {
    const started = chatReducer(INITIAL_STATE, { type: 'start', prompt: 'hi' });
    const state = chatReducer(started, { type: 'finish', result: { text: 'hal', reasoning: '', completed: false } });

    expect(state.turns[1]).toMatchObject({ content: 'hal', pending: false, completed: false, interrupted: true });
  });
});

describe('consumeStream', () => {
  it('记录首 token 延迟与本轮总耗时', async () => {
    const clock = vi.fn();
    // 依次：起点 0、首个增量 300、第二个增量之后不再取时间、收尾 1000。
    clock.mockReturnValueOnce(0).mockReturnValueOnce(300).mockReturnValueOnce(1000);
    const response = streamResponse([delta('he'), delta('llo'), sse({ usage: { total_tokens: 9 } })]);

    const result = await consumeStream({ response, clock });

    expect(result.text).toBe('hello');
    expect(result.firstTokenMs).toBe(300);
    expect(result.durationMs).toBe(1000);
    expect(result.usage).toEqual({ total_tokens: 9 });
  });

  it('见到 [DONE] 或 finish_reason 才算 completed；连接直接断开不算', async () => {
    const done = await consumeStream({ response: streamResponse([delta('hi'), 'data: [DONE]\n\n']), clock: () => 0 });
    const finished = await consumeStream({
      response: streamResponse([delta('hi'), sse({ choices: [{ delta: {}, finish_reason: 'stop' }] })]),
      clock: () => 0
    });
    const cut = await consumeStream({ response: streamResponse([delta('hi')]), clock: () => 0 });

    expect(done.completed).toBe(true);
    expect(finished.completed).toBe(true);
    expect(cut).toMatchObject({ text: 'hi', completed: false, error: null });
  });

  it('中止时保留已经吐出来的半截回答，只标记 aborted 且不抛', async () => {
    const abort = Object.assign(new Error('aborted'), { name: 'AbortError' });
    const response = streamResponse([delta('half')], { throwAfter: 1, error: abort });

    const result = await consumeStream({ response, clock: () => 0 });

    expect(result.text).toBe('half');
    expect(result.aborted).toBe(true);
    expect(result.completed).toBe(false);
    expect(result.error.i18nKey).toBe('playground.errors.aborted');
    expect(result.durationMs).toBe(0);
  });

  it('流中途的错误体中断本次调用并映射为面板内错误', async () => {
    const response = streamResponse([sse({ error: { message: 'upstream died' } })]);

    const result = await consumeStream({ response, clock: () => 0 });

    expect(result.aborted).toBe(false);
    expect(result.error.message).toBe('upstream died');
  });
});

describe('readCompletion', () => {
  it('非流式回包取整条回答与用量', () => {
    const result = readCompletion({ choices: [{ message: { content: 'hi' } }], usage: { total_tokens: 4 } });
    expect(result).toMatchObject({ text: 'hi', usage: { total_tokens: 4 }, firstTokenMs: null });
  });
});

const fakeStorage = (initial = {}) => {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: vi.fn((key) => (data.has(key) ? data.get(key) : null)),
    setItem: vi.fn((key, value) => data.set(key, String(value))),
    removeItem: vi.fn((key) => data.delete(key))
  };
};

const finishedTurns = [
  { role: 'user', content: 'hi' },
  {
    role: 'assistant',
    content: 'hello',
    reasoning: '',
    usage: { total_tokens: 9 },
    firstTokenMs: 120,
    durationMs: 900,
    raw: 'data: …',
    pending: false
  }
];

describe('chatPersistence', () => {
  it('只落白名单字段：轮次、草稿、系统提示词、参数；运行态与多余字段不进存储', () => {
    const text = serializeChatSession({
      ...INITIAL_STATE,
      turns: [{ ...finishedTurns[0], headers: { Authorization: 'Bearer sk-x' } }, finishedTurns[1]],
      input: 'draft',
      system: 'sys',
      running: true,
      error: { message: 'boom' },
      apiKey: 'sk-x'
    });

    expect(text).not.toContain('sk-x');
    expect(text).not.toContain('boom');
    expect(JSON.parse(text)).toMatchObject({
      input: 'draft',
      system: 'sys',
      turns: [{ role: 'user', content: 'hi' }, { content: 'hello' }]
    });
  });

  it('读回时没收完的助手轮标成「已中断」，不再卡在生成中', () => {
    const turns = [finishedTurns[0], { ...finishedTurns[1], content: 'hal', pending: true }];
    const session = parseChatSession(JSON.stringify({ turns, input: '', system: '' }), DEFAULT_SETTINGS);

    expect(session.turns[1]).toMatchObject({ content: 'hal', pending: false, interrupted: true });
  });

  it('读回时按 completed 判断：没收到结束标记的轮次即使 pending 已落定也算「已中断」', () => {
    const turns = [
      finishedTurns[0],
      { ...finishedTurns[1], completed: true },
      finishedTurns[0],
      { ...finishedTurns[1], content: 'hal', completed: false }
    ];
    const session = parseChatSession(JSON.stringify({ turns, input: '', system: '' }), DEFAULT_SETTINGS);

    expect(session.turns[1]).toMatchObject({ completed: true, interrupted: false });
    expect(session.turns[3]).toMatchObject({ content: 'hal', completed: false, interrupted: true });
  });

  it('没有 completed 字段的旧数据：已落定的轮次按正常完成处理', () => {
    const session = parseChatSession(JSON.stringify({ turns: finishedTurns, input: '', system: '' }), DEFAULT_SETTINGS);

    expect(session.turns[1]).toMatchObject({ completed: true, interrupted: false });
  });

  it('中断时一个字都没回来：撤掉这一轮，提问退回输入框', () => {
    const turns = [finishedTurns[0], { role: 'assistant', content: '', pending: true }];
    const session = parseChatSession(JSON.stringify({ turns, input: '', system: '' }), DEFAULT_SETTINGS);

    expect(session.turns).toEqual([]);
    expect(session.input).toBe('hi');
  });

  it('运行参数只认已知键且类型一致，其余回落默认', () => {
    const settings = { temperature: '0.3', stream: 'yes', unknown: 1 };
    const session = parseChatSession(JSON.stringify({ turns: [], settings }), DEFAULT_SETTINGS);

    expect(session.settings).toEqual({ ...DEFAULT_SETTINGS, temperature: '0.3' });
  });

  it('格式不对的数据整份丢弃并从存储里删掉', () => {
    for (const bad of ['{not json', '[]', JSON.stringify({ turns: [{ role: 'tool', content: 'x' }] }), JSON.stringify({ turns: 'x' })]) {
      const storage = fakeStorage({ [CHAT_STORAGE_KEY]: bad });
      expect(loadChatSession(storage, DEFAULT_SETTINGS)).toBeNull();
      expect(storage.data.has(CHAT_STORAGE_KEY)).toBe(false);
    }
  });

  it('存储读写抛错时静默降级', () => {
    const broken = {
      getItem: () => {
        throw new Error('denied');
      },
      setItem: () => {
        throw new Error('quota');
      },
      removeItem: () => {
        throw new Error('denied');
      }
    };
    expect(loadChatSession(broken, DEFAULT_SETTINGS)).toBeNull();
    expect(() => saveChatSession(broken, INITIAL_STATE)).not.toThrow();
  });

  describe('createChatPersistence', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it('落盘字段变了才写，并做节流；运行态变化不触发写入', () => {
      const storage = fakeStorage();
      const store = createSessionStore({ ...INITIAL_STATE, running: false });
      createChatPersistence({ store, getStorage: () => storage });

      store.setState({ running: true });
      vi.advanceTimersByTime(1000);
      expect(storage.setItem).not.toHaveBeenCalled();

      store.setState({ input: 'a' });
      store.setState({ input: 'ab' });
      vi.advanceTimersByTime(299);
      expect(storage.setItem).not.toHaveBeenCalled();
      vi.advanceTimersByTime(1);
      expect(storage.setItem).toHaveBeenCalledTimes(1);
      expect(JSON.parse(storage.data.get(CHAT_STORAGE_KEY)).input).toBe('ab');
    });

    it('cancel 丢掉节流中的那次写入', () => {
      const storage = fakeStorage();
      const store = createSessionStore(INITIAL_STATE);
      const persistence = createChatPersistence({ store, getStorage: () => storage });

      store.setState({ input: 'a' });
      persistence.cancel();
      vi.advanceTimersByTime(2000);
      expect(storage.setItem).not.toHaveBeenCalled();
    });
  });
});

describe('chat session store', () => {
  let storage;

  beforeEach(() => {
    storage = fakeStorage();
    vi.stubGlobal('localStorage', storage);
    chatStore.setState({ ...INITIAL_STATE, running: false, error: null });
  });

  afterEach(() => {
    clearChat();
    vi.unstubAllGlobals();
  });

  const authHeaders = async () => ({ Authorization: 'Bearer sk-secret' });

  it('一轮跑完后本地副本里有轮次与用量，但没有个人 key', async () => {
    const send = vi.fn(async () => streamResponse([delta('he'), delta('llo'), sse({ usage: { total_tokens: 9 } }), 'data: [DONE]\n\n']));

    await runChat({ prompt: 'hi', modelId: 'gpt-x', send, authHeaders });
    flushChatSession();

    expect(send.mock.calls[0][0].headers.Authorization).toBe('Bearer sk-secret');
    const stored = storage.data.get(CHAT_STORAGE_KEY);
    expect(stored).not.toContain('sk-secret');
    expect(JSON.parse(stored).turns).toMatchObject([
      { role: 'user', content: 'hi' },
      { role: 'assistant', content: 'hello', usage: { total_tokens: 9 }, pending: false, completed: true, interrupted: false }
    ]);
    expect(chatStore.getState().running).toBe(false);
  });

  it('发起时立刻落盘进行中的一轮：页面中途被关，读回来就是「已中断」', async () => {
    let snapshot;
    const send = vi.fn(async () => {
      snapshot = storage.data.get(CHAT_STORAGE_KEY);
      return streamResponse([delta('hal')]);
    });

    await runChat({ prompt: 'hi', modelId: 'gpt-x', send, authHeaders });

    expect(JSON.parse(snapshot).turns[1]).toMatchObject({ role: 'assistant', pending: true, completed: false });
    expect(chatStore.getState().turns[1]).toMatchObject({ content: 'hal', completed: false, interrupted: true });
  });

  it('点「停止」：保留已生成的部分，标成「已中断」且不报错', async () => {
    const send = vi.fn(async ({ signal }) => {
      let index = 0;
      return {
        status: 200,
        body: {
          getReader: () => ({
            read: async () => {
              if (index === 0) {
                index += 1;
                return { done: false, value: encoder.encode(delta('hal')) };
              }
              abortChat();
              if (signal.aborted) throw Object.assign(new Error('aborted'), { name: 'AbortError' });
              return { done: true, value: undefined };
            }
          })
        }
      };
    });

    await runChat({ prompt: 'hi', modelId: 'gpt-x', send, authHeaders });
    flushChatSession();

    expect(chatStore.getState()).toMatchObject({ running: false, error: null });
    expect(chatStore.getState().turns[1]).toMatchObject({ content: 'hal', completed: false, interrupted: true });
    expect(JSON.parse(storage.data.get(CHAT_STORAGE_KEY)).turns[1]).toMatchObject({ completed: false, interrupted: true });
  });

  it('请求失败：撤回这一轮、提问退回输入框、错误只在内存里', async () => {
    const send = vi.fn(async () => {
      throw new TypeError('network down');
    });

    await runChat({ prompt: 'hi', modelId: 'gpt-x', send, authHeaders });
    flushChatSession();

    const state = chatStore.getState();
    expect(state).toMatchObject({ turns: [], input: 'hi', running: false });
    expect(state.error).not.toBeNull();
    expect(storage.data.get(CHAT_STORAGE_KEY)).not.toContain('network down');
  });

  it('「清空」删掉本地副本', () => {
    chatStore.setState({ turns: finishedTurns, system: 'sys' });
    flushChatSession();
    expect(storage.data.has(CHAT_STORAGE_KEY)).toBe(true);

    clearChat();

    expect(storage.data.has(CHAT_STORAGE_KEY)).toBe(false);
    expect(chatStore.getState()).toMatchObject({ turns: [], system: '' });
  });

  it('退出登录清掉本地副本与内存里的会话', () => {
    chatStore.setState({ turns: finishedTurns, input: 'draft', settings: { ...DEFAULT_SETTINGS, temperature: '0.2' } });
    flushChatSession();

    clearChatSession();

    expect(storage.data.has(CHAT_STORAGE_KEY)).toBe(false);
    expect(chatStore.getState()).toMatchObject({ turns: [], input: '', settings: DEFAULT_SETTINGS });
  });
});
