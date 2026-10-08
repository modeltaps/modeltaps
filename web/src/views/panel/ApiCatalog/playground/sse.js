// ==============================|| PLAYGROUND — SSE ||============================== //
// 流式对话的纯函数解析层：fetch 的 ReadableStream 切出来的文本块不保证落在事件边界上，
// 所以先把碎片攒进 buffer，只有见到空行才认一个完整事件；`data:` 可以有多行，按规范用
// 换行拼接。OpenAI 兼容流以 data: [DONE] 收尾，中途出错则在一帧里直接给 { error: {...} }。
// 这里不碰 DOM 也不碰 fetch，测试可以在 node 环境直接跑。

export const SSE_DONE = '[DONE]';

// 事件之间以空行分隔，CRLF 与 LF 都可能出现（网关、代理会改写换行）。
export function splitEvents(buffer) {
  const normalized = buffer.replace(/\r\n/g, '\n');
  const parts = normalized.split('\n\n');
  return { blocks: parts.slice(0, -1), rest: parts[parts.length - 1] };
}

// 一个事件块里只取 data 行：event / id / retry 与以 : 开头的心跳注释都跳过。
export function eventData(block) {
  return block
    .split('\n')
    .filter((line) => line.startsWith('data:'))
    .map((line) => line.slice(5).trim())
    .join('\n');
}

export const isDoneData = (data) => data === SSE_DONE;

const EMPTY_CHUNK = { text: '', reasoning: '', usage: null, error: null, payload: null };

// 单帧 JSON → 面板要的几样东西：增量文本、思考过程、用量、错误体。
// reasoning_content 是推理模型的增量思考（非 OpenAI 标准字段），面板单独折叠展示。
// 非 JSON 的帧（网关插的提示行）按「无内容」处理，不能因此中断整条流。
export function parseChunk(data) {
  let payload;
  try {
    payload = JSON.parse(data);
  } catch {
    return { ...EMPTY_CHUNK };
  }
  if (payload?.error) return { ...EMPTY_CHUNK, error: payload.error, payload };
  const delta = payload?.choices?.[0]?.delta || {};
  return {
    text: typeof delta.content === 'string' ? delta.content : '',
    reasoning: typeof delta.reasoning_content === 'string' ? delta.reasoning_content : '',
    usage: payload?.usage || null,
    error: null,
    payload
  };
}

export function createSseParser() {
  let buffer = '';

  return {
    // 返回本次能确定下来的 data 串（可能为空数组：块落在事件中间）。
    push(text) {
      buffer += text;
      const { blocks, rest } = splitEvents(buffer);
      buffer = rest;
      return blocks.map(eventData).filter((data) => data !== '');
    },
    // 流结束时 buffer 里可能还剩最后一个没跟空行的事件。
    flush() {
      const tail = buffer;
      buffer = '';
      const data = eventData(tail.replace(/\r\n/g, '\n'));
      return data ? [data] : [];
    }
  };
}
