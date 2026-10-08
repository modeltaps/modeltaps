// ==============================|| PLAYGROUND — CHAT PERSISTENCE ||============================== //
// 对话模态的当前会话落 localStorage：刷新、切模态回来都还在。只存白名单字段（轮次、输入草稿、
// 系统提示词、运行参数），个人 playground key 与进行中的请求状态一律不落盘。
// 本文件不依赖 React 与会话层，退出登录 / 会话失效（hooks/useLogin、utils/api）也从这里清。

export const CHAT_STORAGE_KEY = 'playground.chat.session.v1';

// 原始响应体只留个头，够排查即可，别把配额吃光。
export const RAW_LIMIT = 16 * 1024;

const isObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);
const numberOrNull = (value) => (typeof value === 'number' && Number.isFinite(value) ? value : null);

// 隐私模式 / 被禁用时连读 localStorage 这个属性都可能抛。
export function safeStorage() {
  try {
    return typeof localStorage !== 'undefined' ? localStorage : null;
  } catch {
    return null;
  }
}

const storedTurn = (turn) => ({
  role: turn.role,
  content: turn.content,
  ...(turn.role === 'assistant' && {
    reasoning: turn.reasoning || '',
    usage: turn.usage || null,
    firstTokenMs: numberOrNull(turn.firstTokenMs),
    durationMs: numberOrNull(turn.durationMs),
    raw: String(turn.raw || '').slice(0, RAW_LIMIT),
    pending: Boolean(turn.pending),
    completed: Boolean(turn.completed),
    interrupted: Boolean(turn.interrupted)
  })
});

export function serializeChatSession(state) {
  return JSON.stringify({
    turns: (state.turns || []).map(storedTurn),
    input: state.input || '',
    system: state.system || '',
    settings: state.settings || {}
  });
}

// 读回来的轮次：格式不对返回 null（整份丢弃）。没标记 completed 的助手轮一律当「已中断」，
// 不能卡在生成中，也不能看起来像正常收完。没有 completed 字段的旧数据按 pending / interrupted 判断。
const restoreTurn = (turn) => {
  if (!isObject(turn) || (turn.role !== 'user' && turn.role !== 'assistant') || typeof turn.content !== 'string') return null;
  if (turn.role === 'user') return { role: 'user', content: turn.content };
  const interrupted = typeof turn.completed === 'boolean' ? !turn.completed : Boolean(turn.interrupted || turn.pending);
  return {
    role: 'assistant',
    content: turn.content,
    reasoning: typeof turn.reasoning === 'string' ? turn.reasoning : '',
    usage: isObject(turn.usage) ? turn.usage : null,
    firstTokenMs: numberOrNull(turn.firstTokenMs),
    durationMs: numberOrNull(turn.durationMs),
    raw: typeof turn.raw === 'string' ? turn.raw : '',
    pending: false,
    completed: !interrupted,
    interrupted
  };
};

// 运行参数只认 defaults 里有的键、且类型一致的值，其余回落默认。
const restoreSettings = (settings, defaults) =>
  Object.fromEntries(
    Object.entries(defaults).map(([key, fallback]) => [key, typeof settings?.[key] === typeof fallback ? settings[key] : fallback])
  );

export function parseChatSession(text, defaults = {}) {
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    return null;
  }
  if (!isObject(data) || !Array.isArray(data.turns)) return null;
  if (typeof (data.input ?? '') !== 'string' || typeof (data.system ?? '') !== 'string') return null;
  if (data.settings !== undefined && !isObject(data.settings)) return null;
  let turns = data.turns.map(restoreTurn);
  if (turns.some((turn) => turn === null)) return null;
  let input = data.input || '';
  // 中断时一个字都没回来：同运行失败的回滚，撤掉这一轮，提问退回输入框以便直接重试。
  const last = turns[turns.length - 1];
  const prompt = turns[turns.length - 2];
  if (last?.role === 'assistant' && last.interrupted && !last.content && !last.reasoning && prompt?.role === 'user') {
    turns = turns.slice(0, -2);
    if (!input.trim()) input = prompt.content;
  }
  return { turns, input, system: data.system || '', settings: restoreSettings(data.settings, defaults) };
}

// 读失败或格式不对都当没有；坏数据顺手删掉，下次不再读到。
export function loadChatSession(storage, defaults) {
  if (!storage) return null;
  let text;
  try {
    text = storage.getItem(CHAT_STORAGE_KEY);
  } catch {
    return null;
  }
  if (!text) return null;
  const session = parseChatSession(text, defaults);
  if (!session) removeChatSession(storage);
  return session;
}

export function saveChatSession(storage, state) {
  try {
    storage?.setItem(CHAT_STORAGE_KEY, serializeChatSession(state));
  } catch {
    /* 配额满 / 隐私模式：只在本页内存里保留 */
  }
}

export function removeChatSession(storage) {
  try {
    storage?.removeItem(CHAT_STORAGE_KEY);
  } catch {
    /* 同上 */
  }
}

const PERSISTED = ['turns', 'input', 'system', 'settings'];

// 订阅会话 store，落盘字段变了才写，并做节流：平时 300ms 合一次，流式输出期间 1s 一次。
// running / error 这类进行中状态的变化不触发写入。
export function createChatPersistence({ store, getStorage = safeStorage, idleDelay = 300, streamingDelay = 1000 }) {
  let timer = null;
  let last = store.getState();

  const flush = () => {
    if (timer) clearTimeout(timer);
    timer = null;
    saveChatSession(getStorage(), store.getState());
  };

  const cancel = () => {
    if (timer) clearTimeout(timer);
    timer = null;
    last = store.getState();
  };

  store.subscribe(() => {
    const state = store.getState();
    const changed = PERSISTED.some((key) => state[key] !== last[key]);
    last = state;
    if (!changed || timer) return;
    const streaming = Boolean(state.turns[state.turns.length - 1]?.pending);
    timer = setTimeout(flush, streaming ? streamingDelay : idleDelay);
  });

  return { flush, cancel, pending: () => timer !== null };
}

// 退出登录：删掉本地副本，并通知会话层把内存里的也清掉。
const clearListeners = new Set();

export const onChatSessionCleared = (listener) => {
  clearListeners.add(listener);
  return () => clearListeners.delete(listener);
};

export function clearChatSession() {
  clearListeners.forEach((listener) => listener());
  removeChatSession(safeStorage());
}
