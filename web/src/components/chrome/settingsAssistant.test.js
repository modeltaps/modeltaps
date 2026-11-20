import { describe, expect, it, vi } from 'vitest';

import { askSettingsAssistant, localizeToolText, systemPrompt, withConsoleMatches } from './settingsAssistant';
import { commandFilter, looksLikeQuestion } from './CommandMenu';

const json = (body, ok = true, status = 200) => Promise.resolve({ ok, status, json: () => Promise.resolve(body) });

// A fake relay and admin tool endpoint: the model first searches, then answers.
function fakeFetch(replies) {
  const calls = [];
  const fetchImpl = vi.fn((url, init = {}) => {
    calls.push({ url, body: init.body ? JSON.parse(init.body) : null, headers: init.headers });
    if (url === '/mcp/admin/tools') return json({ success: true, data: [{ type: 'function', function: { name: 'settings_search' } }] });
    if (url === '/mcp/admin/call') return json({ success: true, data: { text: '{"results":[{"key":"RetryTimes","label":"Retries"}]}' } });
    if (url === '/mcp/admin/plan')
      return json({ success: true, data: [{ key: 'RetryTimes', value: '3', current: '2', allowed: true, label: 'Retries' }] });
    if (url === '/v1/chat/completions') return json({ choices: [{ message: replies.shift() }] });
    return json({}, false, 404);
  });
  return { fetchImpl, calls };
}

describe('askSettingsAssistant', () => {
  it('runs the tool calls through the admin endpoint and returns the answer', async () => {
    const { fetchImpl, calls } = fakeFetch([
      { content: '', tool_calls: [{ id: 'c1', type: 'function', function: { name: 'settings_search', arguments: '{"query":"retry"}' } }] },
      { content: 'Set **Retries** to 2.' }
    ]);
    const onStep = vi.fn();
    const { answer, steps } = await askSettingsAssistant({
      question: '上游 429 怎么换渠道？',
      model: 'gpt-x',
      language: 'zh_CN',
      getKey: async () => 'sk-play',
      describe: (key) => (key === 'RetryTimes' ? { label: '重试次数', location: '网关 › 重试' } : null),
      onStep,
      fetchImpl
    });
    expect(answer).toBe('Set **Retries** to 2.');
    expect(steps).toEqual([{ name: 'settings_search', args: '{"query":"retry"}' }]);
    expect(onStep).toHaveBeenCalledOnce();

    const chats = calls.filter((c) => c.url === '/v1/chat/completions');
    expect(chats).toHaveLength(2);
    expect(chats[0].headers.Authorization).toBe('Bearer sk-play');
    expect(chats[0].body).toMatchObject({ model: 'gpt-x', tool_choice: 'auto' });
    expect(chats[0].body.tools.map((x) => x.function.name)).toEqual(['settings_search', 'propose_setting_changes']);
    expect(chats[0].body.messages[0].content).toContain('Simplified Chinese');
    // The tool result goes back with the console's names of the setting.
    const tool = chats[1].body.messages.find((m) => m.role === 'tool');
    expect(tool.tool_call_id).toBe('c1');
    expect(JSON.parse(tool.content).results[0]).toMatchObject({
      key: 'RetryTimes',
      console_label: '重试次数',
      console_location: '网关 › 重试'
    });
  });

  it('stops calling tools after the last round', async () => {
    const loop = { content: '', tool_calls: [{ id: 'c', function: { name: 'settings_search', arguments: '{}' } }] };
    const { fetchImpl, calls } = fakeFetch([loop, loop, { content: 'done' }]);
    const { answer } = await askSettingsAssistant({ question: 'q', model: 'm', getKey: async () => 'k', fetchImpl, maxRounds: 2 });
    expect(answer).toBe('done');
    const chats = calls.filter((c) => c.url === '/v1/chat/completions');
    expect(chats.map((c) => c.body.tool_choice)).toEqual(['auto', 'auto', 'none']);
  });

  it('surfaces a relay error', async () => {
    const fetchImpl = vi.fn((url) =>
      url === '/mcp/admin/tools' ? json({ success: true, data: [] }) : json({ error: { message: 'insufficient quota' } }, false, 403)
    );
    await expect(askSettingsAssistant({ question: 'q', model: 'm', getKey: async () => 'k', fetchImpl })).rejects.toThrow(
      'insufficient quota'
    );
  });
});

describe('askSettingsAssistant conversations and proposals', () => {
  it('carries the conversation into the next question', async () => {
    const { fetchImpl, calls } = fakeFetch([{ content: 'first answer' }, { content: 'second answer' }]);
    const first = await askSettingsAssistant({ question: 'q1', model: 'm', language: 'en_US', getKey: async () => 'k', fetchImpl });
    const second = await askSettingsAssistant({ question: 'q2', history: first.history, model: 'm', getKey: async () => 'k', fetchImpl });
    expect(second.answer).toBe('second answer');
    const sent = calls.filter((c) => c.url === '/v1/chat/completions')[1].body.messages;
    expect(sent.map((m) => `${m.role}:${m.role === 'system' ? '' : m.content}`)).toEqual([
      'system:',
      'user:q1',
      'assistant:first answer',
      'user:q2'
    ]);
  });

  it('checks a proposal with the plan route and returns it, never applying it', async () => {
    const { fetchImpl, calls } = fakeFetch([
      {
        content: '',
        tool_calls: [
          { id: 'p1', function: { name: 'propose_setting_changes', arguments: '{"changes":[{"key":"RetryTimes","value":"3"}]}' } }
        ]
      },
      { content: 'Review the change below.' }
    ]);
    const { proposal, answer } = await askSettingsAssistant({
      question: 'set retries to 3',
      model: 'm',
      getKey: async () => 'k',
      fetchImpl
    });
    expect(answer).toBe('Review the change below.');
    expect(proposal).toEqual([{ key: 'RetryTimes', value: '3', current: '2', allowed: true, label: 'Retries' }]);
    expect(calls.find((c) => c.url === '/mcp/admin/plan').body).toEqual({ changes: [{ key: 'RetryTimes', value: '3' }] });
    expect(calls.some((c) => c.url === '/mcp/admin/call' || c.url.startsWith('/api/option'))).toBe(false);
    const toolList = calls.find((c) => c.url === '/v1/chat/completions').body.tools.map((x) => x.function.name);
    expect(toolList).toContain('propose_setting_changes');
  });
});

describe('settings assistant helpers', () => {
  it('adds the console matches to a search that found nothing', () => {
    const match = vi.fn(() => [{ key: 'QuotaForNewUser', console_url: '/panel/admin/settings/billing?highlight=QuotaForNewUser' }]);
    const empty = withConsoleMatches('settings_search', '{"query":"新用户额度"}', '{"total":0,"results":[]}', match);
    expect(JSON.parse(empty).console_matches[0].key).toBe('QuotaForNewUser');
    expect(match).toHaveBeenCalledWith('新用户额度');
    const found = '{"total":1,"results":[{"key":"RetryTimes"}]}';
    expect(withConsoleMatches('settings_search', '{"query":"retry"}', found, match)).toBe(found);
    expect(withConsoleMatches('settings_describe', '{}', '{"total":0}', match)).toBe('{"total":0}');
  });

  it('leaves text that is not JSON alone', () => {
    expect(localizeToolText('error: keys is required', () => ({ label: 'x' }))).toBe('error: keys is required');
  });

  it('asks for English searches and read-only answers', () => {
    const prompt = systemPrompt('ja_JP');
    expect(prompt).toContain('Japanese');
    expect(prompt).toContain('English words');
    expect(prompt).toContain('propose_setting_changes');
    expect(prompt).toContain('Never say a change is done');
  });

  it('puts the settings assistant first for a question and last otherwise', () => {
    expect(looksLikeQuestion('怎么让请求自动换渠道')).toBe(true);
    expect(looksLikeQuestion('how do I add a channel')).toBe(true);
    expect(looksLikeQuestion('重试')).toBe(false);
    expect(commandFilter('ask-ai:', '怎么设置重试？')).toBe(1);
    expect(commandFilter('ask-ai:', '重试')).toBeGreaterThan(0);
    expect(commandFilter('ask-ai:', '重试')).toBeLessThan(0.1);
    expect(commandFilter('ask-ai:', '重')).toBe(0);
  });
});
