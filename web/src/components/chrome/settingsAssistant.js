// ==============================|| CHROME — SETTINGS ASSISTANT ||============================== //
// "Ask AI" in the command palette (root only): a model answers a question about the platform
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
    'what to set, or how settings work together. Look everything up with the tools before you answer: never guess',
    'a setting name, key, default or console location.',
    'settings_search matches English words of the setting names and descriptions: search with English words even when',
    'the question is in another language, and try other words when nothing comes back. Use settings_describe for the',
    'full explanation and console URL, settings_effective for the value in effect (site settings, or one object such',
    'as a channel or model), and settings_docs for how several settings work together.',
    'Tool results carry console_label and console_location, the names the admin sees in the console: use them, and add',
    'the key in code. Link each setting you recommend with a markdown link to its console_url, and only to a',
    'console_url a tool returned: never make up a link.',
    'When settings_search finds nothing, its result may list console_matches: settings the console matched by the',
    "admin's own words; describe them with settings_describe.",
    `Reply in ${reply}, short: what to set, where, the value to use and why, with an example when it helps.`,
    'You cannot change settings yourself. When a change is wanted, propose it with propose_setting_changes: the',
    'admin reviews and applies it. Never say a change is done. Sign-in, payment and secret settings are changed by',
    'hand: say where.',
    'If no setting does what the admin wants, say so and name the closest ones.'
  ].join(' ');
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
// told each tool call. Returns { history, answer, steps, proposal } where proposal is the checked
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
      body: JSON.stringify({ model, messages, tools, tool_choice: last ? 'none' : 'auto', temperature: 0.2 })
    });
    const body = await readJSON(res);
    if (!res.ok) throw new AssistantError(body?.error?.message || body?.message || `HTTP ${res.status}`);
    const message = body?.choices?.[0]?.message;
    if (!message) throw new AssistantError('empty reply');
    const calls = Array.isArray(message.tool_calls) ? message.tool_calls : [];
    if (calls.length === 0 || last) {
      const answer = message.content || '';
      messages.push({ role: 'assistant', content: answer });
      return { history: messages, answer, steps, proposal };
    }

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
