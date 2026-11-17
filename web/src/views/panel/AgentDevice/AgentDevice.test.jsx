// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import AgentDevice, { formatUserCode } from './index';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));
vi.mock('contexts/OrgContext', () => ({
  useOrg: () => ({ orgEnabled: true, organizations: [{ id: 5, name: 'Acme', slug: 'acme' }] })
}));

const request = {
  user_code: 'BCDF-GHJK',
  client_name: 'OpenClaw on laptop',
  request_ip: '203.0.113.9',
  request_user_agent: 'openclaw/2',
  created_time: 1700000000,
  expires_at: 1700000900
};

beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation((url) => {
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
    for (const text of ['OpenClaw on laptop', '203.0.113.9', 'openclaw/2']) expect(card.textContent).toContain(text);
    expect(document.getElementById('device-name').value).toBe('OpenClaw on laptop');
  });

  it('approves with the chosen name, limit and expiry as a personal key', async () => {
    openPage();
    await screen.findByTestId('device-request');
    fireEvent.change(document.getElementById('device-name'), { target: { value: 'claw' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('agentDevice.approve') }));

    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    const [url, body] = API.post.mock.calls[0];
    expect(url).toBe('/api/agent/device/BCDF-GHJK/approve');
    expect(body.name).toBe('claw');
    expect(body.expired_time).toBe(-1);
    expect(body.unlimited_quota).toBe(true);
    expect(body.setting.limits.limit_model_setting).toEqual({ enabled: false, models: [] });
    expect((await screen.findByTestId('device-done')).textContent).toContain(i18n.t('agentDevice.approvedTitle'));
  });

  it('approves with an organization key when an organization is picked', async () => {
    openPage();
    await screen.findByTestId('device-request');
    fireEvent.click(screen.getByTestId('device-owner-trigger'));
    fireEvent.click(await screen.findByRole('option', { name: 'Acme', hidden: true }));
    await waitFor(() => expect(API.get).toHaveBeenCalledWith('/api/org/5/model_access'));
    fireEvent.click(screen.getByRole('button', { name: i18n.t('agentDevice.approve') }));
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
    fireEvent.change(document.getElementById('device-user-code'), { target: { value: 'zzzz-zzzz' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('agentDevice.lookup') }));
    expect((await screen.findByTestId('device-lookup-error')).textContent).toBe('invalid code');
    expect(screen.queryByTestId('device-request')).toBeNull();
  });
});
