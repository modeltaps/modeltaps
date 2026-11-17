// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import AgentDevice, { formatUserCode } from './index';
import { recognizeClient } from './agentClient';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));
vi.mock('react-redux', () => ({
  useSelector: (pick) => pick({ siteInfo: { system_name: 'Modeltaps' }, account: { user: { username: 'ada', display_name: 'Ada' } } })
}));
vi.mock('contexts/OrgContext', () => ({
  useOrg: () => ({ orgEnabled: true, organizations: [{ id: 5, name: 'Acme', slug: 'acme' }] })
}));

const request = {
  user_code: 'BCDF-GHJK',
  client_name: 'OpenClaw on laptop',
  request_ip: '203.0.113.9',
  request_user_agent: 'openclaw/2.4.1',
  same_network: true,
  created_time: Math.floor(Date.now() / 1000),
  expires_at: Math.floor(Date.now() / 1000) + 900
};

let deviceStatus = { status: 'approved' };

// Approve is armed a moment after the request shows.
const approve = async () => {
  const button = screen.getByRole('button', { name: i18n.t('agentDevice.approve') });
  await waitFor(() => expect(button.disabled).toBe(false), { timeout: 2000 });
  fireEvent.click(button);
};

const adjust = () => fireEvent.click(screen.getByRole('button', { name: i18n.t('agentDevice.adjust') }));

beforeEach(() => {
  vi.clearAllMocks();
  deviceStatus = { status: 'approved' };
  API.get.mockImplementation((url) => {
    if (url === '/api/agent/device/BCDF-GHJK/status') return Promise.resolve({ data: { success: true, data: deviceStatus } });
    if (url === '/api/agent/device/BCDF-GHJK') return Promise.resolve({ data: { success: true, data: request } });
    if (url.startsWith('/api/agent/device/')) {
      return Promise.resolve({ data: { success: false, code: 'device_user_code_invalid', message: 'invalid code' } });
    }
    if (url === '/api/available_model') return Promise.resolve({ data: { success: true, data: { 'gpt-a': { owned_by: 'openai' } } } });
    return Promise.resolve({ data: { success: true, data: { scope: null } } });
  });
  API.post.mockResolvedValue({ data: { success: true, data: { id: 3, name: 'OpenClaw on laptop' } } });
});

afterEach(cleanup);

const openPage = (search = '?user_code=bcdf-ghjk') =>
  render(createElement(MemoryRouter, { initialEntries: [`/panel/device${search}`] }, createElement(AgentDevice)));

describe('recognizeClient', () => {
  it('names known agents from what they send', () => {
    expect(recognizeClient('my box', 'claude-cli/2.1.3 (external, cli)')).toMatchObject({
      name: 'Claude Code',
      brandKey: 'claude',
      version: '2.1.3'
    });
    expect(recognizeClient('Codex on laptop', 'curl/8.7.1')).toMatchObject({ name: 'Codex', version: '' });
    expect(recognizeClient('build bot', 'curl/8.7.1')).toBeNull();
  });
});

describe('formatUserCode', () => {
  it('normalizes what users type', () => {
    expect(formatUserCode('bcdf ghjk')).toBe('BCDF-GHJK');
    expect(formatUserCode('BCDFGHJK')).toBe('BCDF-GHJK');
    expect(formatUserCode(' short ')).toBe('short');
  });
});

describe('AgentDevice confirmation page (AGENT-7)', () => {
  it('looks up the code from the link and shows who is asking', async () => {
    openPage();
    const card = await screen.findByTestId('device-request');
    expect(API.get).toHaveBeenCalledWith('/api/agent/device/BCDF-GHJK');
    for (const text of ['203.0.113.9', 'openclaw/2.4.1']) expect(card.textContent).toContain(text);
    expect(screen.getByTestId('device-identity').textContent).toContain('OpenClaw 2.4.1');
    expect(screen.getByTestId('device-network').textContent).toBe(i18n.t('agentDevice.sameNetwork'));
    expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('OpenClaw on laptop');
    expect(screen.getByTestId('device-code-shown').getAttribute('aria-label')).toContain('BCDF-GHJK');
    // What the key may do is a summary until the user adjusts it.
    expect(screen.getByTestId('device-summary').textContent).toContain('OpenClaw on laptop');
    expect(document.getElementById('device-name')).toBeNull();
    adjust();
    expect(document.getElementById('device-name').value).toBe('OpenClaw on laptop');
  });

  it('approves with the chosen name, limit and expiry as a personal key', async () => {
    openPage();
    await screen.findByTestId('device-request');
    adjust();
    fireEvent.change(document.getElementById('device-name'), { target: { value: 'claw' } });
    await approve();

    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    const [url, body] = API.post.mock.calls[0];
    expect(url).toBe('/api/agent/device/BCDF-GHJK/approve');
    expect(body.name).toBe('claw');
    expect(body.expired_time).toBe(-1);
    expect(body.unlimited_quota).toBe(true);
    expect(body.setting.limits.limit_model_setting).toEqual({ enabled: false, models: [] });
    expect((await screen.findByTestId('device-done')).textContent).toContain(i18n.t('agentDevice.approvedTitle'));
    expect(screen.getByTestId('device-status').textContent).toContain(i18n.t('agentDevice.stepWaiting'));
  });

  it('follows the approval until the agent has its key', async () => {
    openPage();
    await screen.findByTestId('device-request');
    await approve();
    await screen.findByTestId('device-done');
    expect(API.get).toHaveBeenCalledWith('/api/agent/device/BCDF-GHJK/status');
    deviceStatus = { status: 'claimed', claim: { claimed_at: 1700000000, ip: '203.0.113.9', user_agent: 'openclaw/2.4.1' } };
    await waitFor(() => expect(screen.getByTestId('device-done').textContent).toContain(i18n.t('agentDevice.connectedTitle')), {
      timeout: 5000
    });
    expect(screen.getByTestId('device-status').textContent).toContain('203.0.113.9');
  });

  it('approves with an organization key when an organization is picked', async () => {
    openPage();
    await screen.findByTestId('device-request');
    fireEvent.click(screen.getByRole('radio', { name: /Acme/ }));
    await waitFor(() => expect(API.get).toHaveBeenCalledWith('/api/org/5/model_access'));
    await approve();
    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    expect(API.post.mock.calls[0][0]).toBe('/api/org/5/agent/device/BCDF-GHJK/approve');
  });

  it('denies', async () => {
    API.post.mockResolvedValue({ data: { success: true } });
    openPage();
    await screen.findByTestId('device-request');
    fireEvent.click(screen.getByRole('button', { name: i18n.t('agentDevice.deny') }));
    await waitFor(() => expect(API.post).toHaveBeenCalledWith('/api/agent/device/BCDF-GHJK/deny', undefined));
    expect((await screen.findByTestId('device-done')).textContent).toContain(i18n.t('agentDevice.deniedTitle'));
  });

  it('shows why an unknown code cannot be answered', async () => {
    openPage('');
    // A complete code is looked up as soon as the last letter is typed.
    fireEvent.change(document.getElementById('device-user-code'), { target: { value: 'zzzz-zzzz' } });
    expect((await screen.findByTestId('device-lookup-error')).textContent).toBe('invalid code');
    expect(API.get).toHaveBeenCalledWith('/api/agent/device/ZZZZ-ZZZZ');
    expect(screen.queryByTestId('device-request')).toBeNull();
  });

  it('keeps Continue off until all eight letters are in', () => {
    openPage('');
    fireEvent.change(document.getElementById('device-user-code'), { target: { value: 'bcd-1f' } });
    expect(document.getElementById('device-user-code').value).toBe('BCDF');
    expect(screen.getByRole('button', { name: i18n.t('agentDevice.lookup') }).disabled).toBe(true);
  });

  it('turns into an expired notice when the request runs out', async () => {
    const past = Math.floor(Date.now() / 1000) - 1;
    API.get.mockImplementation((url) =>
      url === '/api/agent/device/BCDF-GHJK'
        ? Promise.resolve({ data: { success: true, data: { ...request, expires_at: past } } })
        : Promise.resolve({ data: { success: true, data: {} } })
    );
    openPage();
    await screen.findByTestId('device-expired');
    fireEvent.click(screen.getByRole('button', { name: i18n.t('agentDevice.enterAnother') }));
    expect(document.getElementById('device-user-code').value).toBe('');
  });
});
