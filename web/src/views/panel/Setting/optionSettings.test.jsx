// @vitest-environment jsdom
import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { API } from 'utils/api';
import { useOptionSettings } from './parts';

// One option state for the whole platform settings page: saving a card writes only that card's
// changed keys and keeps the unsaved edits of every other card; Undo restores a card's keys.

vi.mock('utils/api', () => ({ API: { get: vi.fn(), put: vi.fn() } }));
vi.mock('utils/common', () => ({ copy: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }));

let stored;
beforeEach(() => {
  vi.clearAllMocks();
  stored = { RetryTimes: '2', SMTPServer: 'smtp.example.com', MjNotifyEnabled: 'false', ChatLinks: '[]' };
  API.get.mockImplementation(() =>
    Promise.resolve({ data: { success: true, data: Object.entries(stored).map(([key, value]) => ({ key, value })), secrets: {} } })
  );
  API.put.mockImplementation((_url, { key, value }) => {
    stored[key] = value;
    return Promise.resolve({ data: { success: true, message: '' } });
  });
});

const load = async () => {
  const hook = renderHook(() => useOptionSettings());
  await waitFor(() => expect(hook.result.current.inputs.RetryTimes).toBe('2'));
  return hook.result;
};

describe('useOptionSettings', () => {
  it('saves one card and keeps the other cards’ unsaved edits', async () => {
    const result = await load();
    act(() => result.current.setField('RetryTimes', '5'));
    act(() => result.current.setField('SMTPServer', 'smtp2.example.com'));
    act(() => result.current.setBool('MjNotifyEnabled', true));
    expect(result.current.isDirty(['MjNotifyEnabled'])).toBe(true);

    await act(() => result.current.saveKeys(['MjNotifyEnabled']));
    expect(API.put.mock.calls.map(([, body]) => body)).toEqual([{ key: 'MjNotifyEnabled', value: 'true' }]);
    expect(result.current.isDirty(['MjNotifyEnabled'])).toBe(false);
    expect(result.current.inputs.RetryTimes).toBe('5');
    expect(result.current.isDirty(['RetryTimes', 'SMTPServer'])).toBe(true);
  });

  it('Undo puts a card’s keys back', async () => {
    const result = await load();
    act(() => result.current.setField('RetryTimes', '5'));
    act(() => result.current.setField('SMTPServer', 'smtp2.example.com'));
    act(() => result.current.resetKeys(['RetryTimes']));
    expect(result.current.inputs.RetryTimes).toBe('2');
    expect(result.current.isDirty(['SMTPServer'])).toBe(true);
  });

  it('collections save a value right away without a pending edit', async () => {
    const result = await load();
    const links = JSON.stringify([{ name: 'Chat', url: 'https://chat.example.com' }]);
    await act(() => result.current.saveKeys(['ChatLinks'], { values: { ChatLinks: links } }));
    expect(API.put.mock.calls.map(([, body]) => body)).toEqual([{ key: 'ChatLinks', value: links }]);
    expect(result.current.origin.ChatLinks).toBe(links);
    expect(result.current.isDirty(['ChatLinks'])).toBe(false);
  });

  it('a failed validation saves nothing', async () => {
    const result = await load();
    act(() => result.current.setField('RetryTimes', 'x'));
    const ok = await act(() => result.current.saveKeys(['RetryTimes'], { validate: () => 'bad' }));
    expect(ok).toBe(false);
    expect(API.put).not.toHaveBeenCalled();
  });
});
