import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('./common', () => ({ showError: vi.fn() }));
vi.mock('store/index', () => ({ store: { dispatch: vi.fn() } }));
vi.mock('views/panel/Playground/chat/chatPersistence', () => ({ clearChatSession: vi.fn() }));

import i18n from 'i18n/i18n';
import { API } from './api';

const [handler] = API.interceptors.response.handlers;

// The interceptor translates coded messages once, so call sites keep showing `message` as before.
describe('API response interceptor', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('zh_CN');
  });

  it('translates a coded message in a 200 response body', () => {
    const response = { status: 200, data: { success: false, message: 'ids must not be empty', code: 'channel_ids_required' } };
    expect(handler.fulfilled(response).data.message).toBe('请至少选择一个渠道');
  });

  it('translates a coded message on an HTTP error and keeps plain messages', async () => {
    const coded = {
      name: 'AxiosError',
      message: 'Request failed',
      response: { status: 400, data: { message: 'invalid status', code: 'channel_tag_invalid_status' } }
    };
    await expect(handler.rejected(coded)).rejects.toBe(coded);
    expect(coded.message).toBe('未知的状态');

    const plain = { name: 'AxiosError', message: 'Request failed', response: { status: 400, data: { message: 'record not found' } } };
    await expect(handler.rejected(plain)).rejects.toBe(plain);
    expect(plain.message).toBe('record not found');
  });
});
