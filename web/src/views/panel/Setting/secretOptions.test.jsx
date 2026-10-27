// @vitest-environment jsdom
import { act, cleanup, render, renderHook, screen, waitFor } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import { TextRow, useOptionSettings } from './parts';

// 系统设置的密钥项：后端只下发回显(secrets)，输入框保持空白，留空保存表示不修改。

vi.mock('utils/api', () => ({ API: { get: vi.fn(), put: vi.fn() } }));
vi.mock('utils/common', () => ({ copy: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }));

beforeAll(() => i18n.changeLanguage('en_US'));
afterAll(() => i18n.changeLanguage('zh_CN'));
beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockResolvedValue({
    data: {
      success: true,
      data: [{ key: 'SMTPServer', value: 'smtp.example.com' }],
      secrets: { SMTPToken: 'smtp****cdef', TurnstileSecretKey: '' }
    }
  });
  API.put.mockResolvedValue({ data: { success: true, message: '' } });
});
afterEach(cleanup);

describe('useOptionSettings secrets', () => {
  it('exposes the masks and leaves secret inputs empty', async () => {
    const { result } = renderHook(() => useOptionSettings());
    await waitFor(() => expect(result.current.secrets.SMTPToken).toBe('smtp****cdef'));
    expect(result.current.inputs.SMTPToken).toBeUndefined();
    expect(result.current.isDirty(['SMTPServer', 'SMTPToken'])).toBe(false);
  });

  it('does not save a blank secret but saves a new one', async () => {
    const { result } = renderHook(() => useOptionSettings());
    await waitFor(() => expect(result.current.secrets.SMTPToken).toBe('smtp****cdef'));

    act(() => result.current.setField('SMTPServer', 'smtp2.example.com'));
    act(() => result.current.setField('SMTPToken', ''));
    await act(() => result.current.saveKeys(['SMTPServer', 'SMTPToken']));
    expect(API.put.mock.calls.map(([, body]) => body)).toEqual([{ key: 'SMTPServer', value: 'smtp2.example.com' }]);

    API.put.mockClear();
    act(() => result.current.setField('SMTPToken', 'new-token'));
    await act(() => result.current.saveKeys(['SMTPToken']));
    expect(API.put.mock.calls.map(([, body]) => body)).toEqual([{ key: 'SMTPToken', value: 'new-token' }]);
  });
});

describe('TextRow secret', () => {
  it('shows the mask and the keep hint when the secret is set', () => {
    render(<TextRow id="s" label="Token" value="" onChange={() => {}} placeholder="Enter token" secret="smtp****cdef" />);
    expect(screen.getByText('Configured smtp****cdef')).toBeTruthy();
    expect(screen.getByText('Leave blank to keep the existing value')).toBeTruthy();
    expect(screen.getByLabelText('Token').getAttribute('placeholder')).toBe('••••••••');
  });

  it('renders like an ordinary field when the secret is unset', () => {
    render(<TextRow id="s" label="Token" value="" onChange={() => {}} placeholder="Enter token" secret="" />);
    expect(screen.queryByText(/^Configured /)).toBeNull();
    expect(screen.getByLabelText('Token').getAttribute('placeholder')).toBe('Enter token');
  });
});
