// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import UsageTab from './UsageTab';

// The usage tab groups usage by coding agent (logs.client) and filters it to one agent.

vi.mock('utils/api', () => ({ API: { get: vi.fn() } }));

const byClient = [
  { client: 'claude-code', request_count: 3, quota: 300, prompt_tokens: 30, completion_tokens: 3 },
  { request_count: 1, quota: 10, prompt_tokens: 1, completion_tokens: 1 }
];

beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation((url, { params } = {}) => {
    if (url.endsWith('/logs/stat')) return Promise.resolve({ data: { success: true, data: { quota: 310 } } });
    const data = params?.group_by === 'client' ? byClient : [];
    return Promise.resolve({ data: { success: true, data } });
  });
});

afterEach(cleanup);

const analyticsCalls = () => API.get.mock.calls.filter(([url]) => url.endsWith('/analytics')).map(([, opts]) => opts.params);

describe('UsageTab tool dimension', () => {
  it('fetches the by-tool grouping and labels its rows', async () => {
    render(createElement(UsageTab, { orgId: 7, isAdmin: true, detail: {} }));
    await waitFor(() => expect(analyticsCalls().some((p) => p.group_by === 'client')).toBe(true));
    expect(analyticsCalls().every((p) => !('client' in p))).toBe(true);

    const tab = screen.getByRole('tab', { name: i18n.t('orgPage.usage.byTool') });
    fireEvent.mouseDown(tab);
    fireEvent.click(tab);
    await waitFor(() => expect(screen.getAllByText('Claude Code').length).toBeGreaterThan(0));
    expect(screen.getAllByText(i18n.t('orgPage.usage.toolOther')).length).toBeGreaterThan(0);
  });

  it('filters every query to the chosen tool', async () => {
    render(createElement(UsageTab, { orgId: 7, isAdmin: false, detail: {} }));
    await waitFor(() => expect(analyticsCalls().length).toBeGreaterThan(0));

    fireEvent.click(screen.getByRole('button', { name: i18n.t('orgPage.usage.toolFilter') }));
    fireEvent.click(screen.getByRole('option', { name: 'Codex' }));
    await waitFor(() => expect(analyticsCalls().some((p) => p.client === 'codex')).toBe(true));
    const statCalls = API.get.mock.calls.filter(([url]) => url.endsWith('/logs/stat'));
    expect(statCalls[statCalls.length - 1][1].params.client).toBe('codex');
  });
});
