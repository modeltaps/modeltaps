// ==============================|| CHROME — SETTINGS ASSISTANT ||============================== //
// The settings assistant in the command palette (root only): a model answers a question about the platform
// settings (where a setting is, what to set, how settings combine) with the read-only tools of the
// admin MCP endpoint. The loop runs here: the model is called through the relay with the admin's
// playground key, so the question is billed to the admin and shows in the logs like any request;
// its tool calls go to /mcp/admin/call, which only reads. Nothing here changes a setting.

export const MAX_ROUNDS = 8;
// A tool result longer than this is cut before it goes back to the model.
export const MAX_TOOL_TEXT = 12000;

const LANGUAGE_NAMES = {
  zh_CN: 'Simplified Chinese',
  zh_HK: 'Traditional Chinese (Hong Kong)',
  ja_JP: 'Japanese',
  en_US: 'English'
};

export function systemPrompt(language) {
  const reply = LANGUAGE_NAMES[language] || 'English';
  return [
    'You are the settings assistant of the Modeltaps admin console, an AI gateway. The admin asks where a setting is,',
    'what to set, how settings work together, or describes a problem to fix. Look everything up with the tools before',
    'you answer: never guess a setting name, key, default, behavior or console location.',
    '',
    'How to look things up:',
    '- settings_search matches English words of the setting keys, names and descriptions. Search with English words',
    '  only, even when the question is in another language, and include the concrete signal of the problem: a status',
    '  code (429, 503), an error, a feature name. Try other words when nothing useful comes back.',
    '- Then read the candidates with settings_describe before you answer, and also the settings it lists as related:',
    '  the right setting is often the related one made for exactly this case.',
    '- settings_effective gives the value in effect (site settings, or one object such as a channel or model);',
    '  settings_docs explains how several settings work together.',
    '- When settings_search finds nothing, its result may list console_matches: settings the console matched by the',
    "  admin's own words; describe them with settings_describe.",
    '',
    'How to answer:',
    '- When the admin describes a problem, first say in one sentence what causes it, then recommend the setting made',
    '  for that case, before general ones. Prefer a targeted change over switching a protection off: never recommend',
    '  turning off retries, failover, cooldowns, limits or checks unless the admin asks for exactly that, and say what',
    '  is lost when you do.',
    '- When the answer is a change and you can choose a sensible value, propose it with propose_setting_changes right',
    '  away instead of asking the admin for a value; the admin reviews and applies it. Ask only when the right value',
    '  depends on something you cannot know, and then say what you need.',
    '- After proposing, write one or two sentences: the card under your answer already shows each setting with its',
    '  current and new value, so do not list them again. Never say a change is done.',
    '- Otherwise reply short: what to set, where, the value and why, with an example when it helps.',
    '- Tool results carry console_label and console_location, the names the admin sees in the console: use them, and',
    '  add the key in code. Link each setting you name with a markdown link to its console_url, and only to a',
    '  console_url a tool returned: never make up a link.',
    '- Sign-in, payment and secret settings are changed by hand: say where. If no setting does what the admin wants,',
    '  say so and name the closest ones.',
    `- Reply in ${reply}.`
  ].join('\n');
}

// Adds the console's own names (the admin's language) to the settings in a tool result, so the
// answer names what the admin sees. `describe(key)` → { label, location } or null.
export function localizeToolText(text, describe) {
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    return text;
  }
  const visit = (node) => {
    if (Array.isArray(node)) node.forEach(visit);
    else if (node && typeof node === 'object') {
      if (typeof node.key === 'string') {
        const names = describe(node.key);
        if (names?.label) node.console_label = names.label;
        if (names?.location) node.console_location = names.location;
      }
      Object.values(node).forEach(visit);
    }
  };
  visit(data);
  return JSON.stringify(data);
}

async function readJSON(res) {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

export class AssistantError extends Error {}

// Reads a chat completion: a streamed one (server-sent events) as it arrives, telling onText the
// text so far, or a plain JSON one. Returns the message { content, tool_calls }.
export async function readChatMessage(res, onText) {
  const type = res.headers?.get?.('content-type') || '';
  if (!type.includes('text/event-stream') || !res.body?.getReader) {
    const body = await readJSON(res);
    const message = body?.choices?.[0]?.message;
    if (!message) throw new AssistantError(body?.error?.message || 'empty reply');
    if (message.content) onText?.(message.content);
    return message;
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let content = '';
  let seen = false;
  const calls = [];
  const take = (line) => {
    if (!line.startsWith('data:')) return;
    const data = line.slice(5).trim();
    if (!data || data === '[DONE]') return;
    let chunk;
    try {
      chunk = JSON.parse(data);
    } catch {
      return;
    }
    if (chunk.error) throw new AssistantError(chunk.error.message || 'stream error');
    const delta = chunk.choices?.[0]?.delta;
    if (!delta) return;
    seen = true;
    if (delta.content) {
      content += delta.content;
      onText?.(content);
    }
    for (const part of delta.tool_calls || []) {
      const index = part.index ?? calls.length;
      const call = (calls[index] ||= { id: '', type: 'function', function: { name: '', arguments: '' } });
      if (part.id) call.id = part.id;
      if (part.function?.name) call.function.name += part.function.name;
      if (part.function?.arguments) call.function.arguments += part.function.arguments;
    }
  };
  for (;;) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
    const lines = buffer.split(/\r?\n/);
    buffer = done ? '' : lines.pop();
    lines.forEach(take);
    if (done) break;
  }
  if (!seen) throw new AssistantError('empty reply');
  const toolCalls = calls.filter(Boolean);
  return { role: 'assistant', content, ...(toolCalls.length ? { tool_calls: toolCalls } : {}) };
}

// The one tool the browser answers itself: the model proposes setting changes; the admin reviews
// them in the conversation and applies the ticked ones through the settings page's own save. The
// proposal is checked by /mcp/admin/plan, which writes nothing.
export const PROPOSE_TOOL = 'propose_setting_changes';

export const proposeTool = {
  type: 'function',
  function: {
    name: PROPOSE_TOOL,
    description:
      'Propose changes of platform settings for the admin to review. You never apply them: the admin sees each with its ' +
      'current and new value, ticks the ones to apply and applies them. Use it when the admin wants a setting changed or ' +
      'when the answer is a change. Only platform settings (scope site) found with the other tools; write each value as ' +
      'the setting takes it (true or false, a number, one of its enum values, JSON). The result says which changes can be ' +
      'applied and why the others cannot; sign-in, payment and secret settings are never changed this way.',
    parameters: {
      type: 'object',
      properties: {
        changes: {
          type: 'array',
          items: {
            type: 'object',
            properties: {
              key: { type: 'string', description: 'The setting key, e.g. RetryTimes.' },
              value: { type: 'string', description: 'The new value.' },
              reason: { type: 'string', description: 'Why, in one short sentence in the reply language.' }
            },
            required: ['key', 'value']
          }
        }
      },
      required: ['changes']
    }
  }
};

// Starts a conversation: the system prompt only. Each turn appends to it.
export const newConversation = (language) => [{ role: 'system', content: systemPrompt(language) }];

// One turn: the question is added to `history` (the conversation so far, with the model's tool
// calls), the model is asked and its tool calls run until it answers. onStep({ name, args }) is
// told each tool call, onText(text) the reply of the current round as it streams (a round that
// ends in tool calls is followed by onText('')). Returns { history, answer, steps, proposal } where proposal is the checked
// plan of the last propose_setting_changes call of the turn, or null.
export async function askSettingsAssistant({
  question,
  history,
  model,
  language,
  getKey,
  describe = () => null,
  augment = (name, args, text) => text,
  onStep,
  onText,
  signal,
  fetchImpl = (...args) => fetch(...args),
  maxRounds = MAX_ROUNDS
}) {
  const toolsRes = await fetchImpl('/mcp/admin/tools', { credentials: 'same-origin', signal });
  const toolsBody = await readJSON(toolsRes);
  if (!toolsRes.ok || !toolsBody?.success) throw new AssistantError(toolsBody?.message || `tools: HTTP ${toolsRes.status}`);
  const tools = [...toolsBody.data, proposeTool];

  const messages = [...(history?.length ? history : newConversation(language)), { role: 'user', content: question }];
  const steps = [];
  let proposal = null;

  const runTool = async (name, args) => {
    if (name === PROPOSE_TOOL) {
      let changes;
      try {
        changes = JSON.parse(args || '{}').changes || [];
      } catch {
        return 'error: the arguments are not valid JSON';
      }
      const res = await fetchImpl('/mcp/admin/plan', {
        method: 'POST',
        signal,
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ changes })
      });
      const body = await readJSON(res);
      if (!body?.success) return `error: ${body?.message || `HTTP ${res.status}`}`;
      proposal = body.data;
      return JSON.stringify({
        proposal: body.data,
        note: 'Shown to the admin below your answer to review and apply. Tell the admin to review it; do not say it is applied.'
      });
    }
    const res = await fetchImpl('/mcp/admin/call', {
      method: 'POST',
      signal,
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, arguments: args })
    });
    const body = await readJSON(res);
    return body?.success ? body.data?.text || '' : `error: ${body?.message || `HTTP ${res.status}`}`;
  };

  for (let round = 0; round <= maxRounds; round += 1) {
    const last = round === maxRounds;
    const key = await getKey();
    const res = await fetchImpl('/v1/chat/completions', {
      method: 'POST',
      signal,
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${key}` },
      body: JSON.stringify({ model, messages, tools, tool_choice: last ? 'none' : 'auto', temperature: 0.2, stream: true })
    });
    if (!res.ok) {
      const body = await readJSON(res);
      throw new AssistantError(body?.error?.message || body?.message || `HTTP ${res.status}`);
    }
    const message = await readChatMessage(res, onText);
    const calls = Array.isArray(message.tool_calls) ? message.tool_calls : [];
    if (calls.length === 0 || last) {
      const answer = message.content || '';
      messages.push({ role: 'assistant', content: answer });
      return { history: messages, answer, steps, proposal };
    }

    // Text written before tool calls ("let me look that up") is not the answer.
    onText?.('');
    messages.push({ role: 'assistant', content: message.content || '', tool_calls: calls });
    for (const call of calls) {
      const name = call.function?.name || '';
      const args = call.function?.arguments || '{}';
      steps.push({ name, args });
      onStep?.({ name, args });
      const text = await runTool(name, args);
      const content = localizeToolText(augment(name, args, text), describe);
      messages.push({ role: 'tool', tool_call_id: call.id, content: content.slice(0, MAX_TOOL_TEXT) });
    }
  }
  return { history: messages, answer: '', steps, proposal };
}

// For a settings_search that found nothing: the settings the console's own search finds for the
// query (labels in the admin's language and the palette's synonyms), as console_matches.
// `match(query)` → [{ key, console_url }].
export function withConsoleMatches(name, args, text, match) {
  if (name !== 'settings_search') return text;
  let data;
  let query;
  try {
    data = JSON.parse(text);
    query = JSON.parse(args || '{}').query;
  } catch {
    return text;
  }
  if (!query || data?.total > 0) return text;
  const matches = match(String(query));
  if (!matches.length) return text;
  return JSON.stringify({ ...data, console_matches: matches });
}
