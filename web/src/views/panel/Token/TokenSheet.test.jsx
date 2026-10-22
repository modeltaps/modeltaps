// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import TokenSheet from './TokenSheet';

// 开启模型限制但一个模型都没选时，服务端会拒绝该 key 的全部请求，表单必须在保存前拦下。

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
  it('blocks saving when the limit is on with no model selected', async () => {
    openNewToken();
    fireEvent.click(modelLimitSwitch());
    submit();

    expect(await screen.findByText(i18n.t('token_index.limitModelsRequired'))).toBeTruthy();
    expect(API.post).not.toHaveBeenCalled();
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
