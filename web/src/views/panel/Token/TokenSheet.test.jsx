// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import TokenSheet from './TokenSheet';

// 模型限制：列表为空即不限制（服务端写入时归一为未开启），表单给出说明；选中模型后按列表限制。

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));
vi.mock('react-redux', () => ({ useSelector: (select) => select({ siteInfo: {} }) }));

beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation((url) => {
    if (url === '/api/available_model') {
      return Promise.resolve({ data: { success: true, data: { 'gpt-a': { owned_by: 'openai', groups: [] } } } });
    }
    return Promise.resolve({ data: { success: true, data: [] } });
  });
  API.post.mockResolvedValue({ data: { success: true, message: '' } });
});

afterEach(cleanup);

const openNewToken = () => {
  render(createElement(TokenSheet, { open: true, onOpenChange: vi.fn(), onSaved: vi.fn() }));
  fireEvent.change(document.getElementById('token-name'), { target: { value: 'agent key' } });
};

const modelLimitSwitch = () => {
  const label = screen.getByText(i18n.t('token_index.limits_models_switch'));
  return label.closest('.space-y-3').querySelector('[role="switch"]');
};

const submit = () => fireEvent.click(screen.getByRole('button', { name: i18n.t('token_index.submit') }));

describe('TokenSheet model limit', () => {
  it('saves the limit on with no model selected and explains it does not restrict', async () => {
    openNewToken();
    fireEvent.click(modelLimitSwitch());
    expect(screen.getByText(i18n.t('token_index.limitModelsEmptyHint'))).toBeTruthy();
    submit();

    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    expect(API.post.mock.calls[0][1].setting.limits.limit_model_setting).toEqual({ enabled: true, models: [] });
  });

  it('saves once a model is selected', async () => {
    openNewToken();
    fireEvent.click(modelLimitSwitch());
    fireEvent.click(await screen.findByRole('option', { name: /gpt-a/ }));
    submit();

    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    const payload = API.post.mock.calls[0][1];
    expect(payload.setting.limits.limit_model_setting).toEqual({ enabled: true, models: ['gpt-a'] });
  });

  it('saves without a model list when the limit is off', async () => {
    openNewToken();
    submit();

    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    expect(API.post.mock.calls[0][1].setting.limits.limit_model_setting.enabled).toBe(false);
  });
});

describe('TokenSheet available models', () => {
  it('lists the saved key’s available models when editing', async () => {
    API.get.mockImplementation((url) => {
      if (url === '/api/available_model') return Promise.resolve({ data: { success: true, data: {} } });
      if (url === '/api/token/5') {
        return Promise.resolve({
          data: { success: true, data: { id: 5, name: 'k', remain_quota: 0, unlimited_quota: true, expired_time: -1, setting: {} } }
        });
      }
      if (url === '/api/token/5/models') return Promise.resolve({ data: { success: true, data: ['gpt-a', 'gpt-b'] } });
      return Promise.resolve({ data: { success: true, data: [] } });
    });
    render(createElement(TokenSheet, { open: true, tokenId: 5, onOpenChange: vi.fn(), onSaved: vi.fn() }));

    expect(await screen.findByText(i18n.t('token_index.availableModels', { count: 2 }))).toBeTruthy();
    expect(screen.getByText('gpt-a')).toBeTruthy();
    expect(screen.getByText('gpt-b')).toBeTruthy();
  });

  it('does not show available models for a new key', async () => {
    openNewToken();
    await waitFor(() => expect(API.get).toHaveBeenCalledWith('/api/available_model'));
    expect(screen.queryByTestId('token-available-models')).toBeNull();
  });
});
