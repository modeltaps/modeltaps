// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import TokenPanel from './TokenPanel';
import NewTokenDialog from './NewTokenDialog';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

const stored = {
  id: 7,
  name: 'agent key',
  key: 'abcdefghijklmnopqrstuvwxyz0123456789abcdefghijkl',
  status: 1,
  expired_time: -1,
  unlimited_quota: true,
  remain_quota: 123,
  used_quota: 0,
  group: '',
  backup_group: '',
  setting: { limits: { limit_model_setting: { enabled: false, models: [] } } }
};

beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation((url) => {
    if (url === '/api/available_model') {
      return Promise.resolve({ data: { success: true, data: { 'gpt-a': { owned_by: 'openai', groups: [] } } } });
    }
    if (url === '/api/token/7') return Promise.resolve({ data: { success: true, data: structuredClone(stored) } });
    return Promise.resolve({ data: { success: true, data: [] } });
  });
  API.put.mockResolvedValue({ data: { success: true, message: '' } });
  API.post.mockResolvedValue({ data: { success: true, message: '', data: { id: 9, key: 'newkey' } } });
});

afterEach(cleanup);

const openPanel = (tab) =>
  render(
    createElement(
      MemoryRouter,
      null,
      createElement(TokenPanel, {
        tokenId: 7,
        tab,
        onTabChange: vi.fn(),
        onClose: vi.fn(),
        onChanged: vi.fn(),
        groupOptions: [],
        userGroup: {},
        onToggleStatus: vi.fn(),
        onCopyKey: vi.fn(),
        onCopyText: vi.fn(),
        onDelete: vi.fn(),
        onShowUsage: vi.fn(),
        onConnectOpenClaw: vi.fn()
      })
    )
  );

const save = () => fireEvent.click(screen.getByRole('button', { name: i18n.t('objects.panel.save') }));

describe('TokenPanel', () => {
  it('saves the access tab with the model limit on and no model, keeping the other stored fields', async () => {
    openPanel('access');
    fireEvent.click(await screen.findByRole('switch', { name: i18n.t('token_index.limits_models_switch') }));
    expect(screen.getByText(i18n.t('token_index.limitModelsEmptyHint'))).toBeTruthy();
    save();

    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    const [url, body] = API.put.mock.calls[0];
    expect(url).toBe('/api/token/');
    expect(body.id).toBe(7);
    expect(body.name).toBe('agent key');
    expect(body.setting.limits.limit_model_setting).toEqual({ enabled: true, models: [] });
    expect(body.setting.heartbeat).toEqual({ enabled: false, timeout_seconds: 30 });
  });

  it('turns a limit with a reset period into a per-period cap', async () => {
    openPanel('basics');
    fireEvent.click(await screen.findByRole('radio', { name: i18n.t('token_index.quotaReset_daily') }));
    fireEvent.change(document.querySelector('input[name="tk-quota"]'), { target: { value: '5000' } });
    save();

    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    const body = API.put.mock.calls[0][1];
    expect(body.unlimited_quota).toBe(true);
    expect(body.setting.quota_reset).toMatchObject({ period: 'daily' });
    expect(body.setting.quota_reset.limit).toBeGreaterThan(0);
  });
});

describe('NewTokenDialog', () => {
  it('creates with only a name and shows the new key', async () => {
    const onCreated = vi.fn();
    render(createElement(NewTokenDialog, { onClose: vi.fn(), onCreated, onOpen: vi.fn(), onCopy: vi.fn(), baseUrl: 'http://x/v1' }));
    fireEvent.change(document.getElementById('nt-name'), { target: { value: 'ci' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('apiKey.new.create') }));

    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    const body = API.post.mock.calls[0][1];
    expect(body).toMatchObject({ name: 'ci', expired_time: -1, unlimited_quota: true, group: '' });
    expect(await screen.findByTestId('new-token-key')).toHaveProperty('textContent', 'sk-newkey');
    expect(onCreated).toHaveBeenCalled();
  });

  it('shows a taken name under the name field', async () => {
    API.post.mockResolvedValueOnce({ data: { success: false, code: 'token_name_taken', message: 'name taken' } });
    render(
      createElement(NewTokenDialog, { onClose: vi.fn(), onCreated: vi.fn(), onOpen: vi.fn(), onCopy: vi.fn(), baseUrl: 'http://x/v1' })
    );
    fireEvent.change(document.getElementById('nt-name'), { target: { value: 'ci' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('apiKey.new.create') }));

    expect(await screen.findByText('name taken')).toBeTruthy();
    expect(document.getElementById('nt-name').getAttribute('aria-invalid')).toBe('true');
  });
});
