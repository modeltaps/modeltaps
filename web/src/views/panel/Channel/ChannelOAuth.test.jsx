// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';

const startChannelOAuth = vi.fn();
const exchangeChannelOAuthCode = vi.fn();
const startCodexDeviceOAuth = vi.fn();
const pollCodexDeviceOAuth = vi.fn();
vi.mock('./channelApi', () => ({
  startChannelOAuth: (...args) => startChannelOAuth(...args),
  fetchChannelOAuthStatus: vi.fn(),
  exchangeChannelOAuthCode: (...args) => exchangeChannelOAuthCode(...args),
  startCodexDeviceOAuth: (...args) => startCodexDeviceOAuth(...args),
  pollCodexDeviceOAuth: (...args) => pollCodexDeviceOAuth(...args)
}));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

import ChannelOAuth from './ChannelOAuth';

const getValues = () => '';
const flush = () => act(() => Promise.resolve());

beforeEach(async () => {
  await i18n.changeLanguage('en_US');
  vi.spyOn(window, 'open').mockReturnValue(null);
  Object.assign(navigator, { clipboard: { writeText: vi.fn(() => Promise.resolve()) } });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
  [startChannelOAuth, exchangeChannelOAuthCode, startCodexDeviceOAuth, pollCodexDeviceOAuth].forEach((m) => m.mockReset());
});

describe('Codex device code login', () => {
  it('shows the code, polls at the server interval and fills the credentials', async () => {
    vi.useFakeTimers();
    startCodexDeviceOAuth.mockResolvedValue({
      success: true,
      data: {
        session_id: 'sid',
        user_code: 'ABCD-12345',
        verification_url: 'https://auth.openai.com/codex/device',
        interval: 5,
        expires_in: 900
      }
    });
    pollCodexDeviceOAuth
      .mockResolvedValueOnce({ success: true, data: { status: 'pending', interval: 5 } })
      .mockResolvedValueOnce({ success: true, data: { status: 'completed', credentials: '{"access_token":"a"}' } });
    const onCredentials = vi.fn();
    render(<ChannelOAuth type={59} getValues={getValues} onCredentials={onCredentials} />);

    fireEvent.click(screen.getByRole('button', { name: /OAuth Authorize/ }));
    await flush();
    expect(screen.getByTestId('device-user-code').textContent).toBe('ABCD-12345');
    expect(screen.getByText(/expires in 15:00/)).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Copy code and open the device page' }));
    expect(window.open).toHaveBeenCalledWith('https://auth.openai.com/codex/device', '_blank', 'noopener,noreferrer');
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('ABCD-12345');

    await act(() => vi.advanceTimersByTimeAsync(4900));
    expect(pollCodexDeviceOAuth).not.toHaveBeenCalled();
    await act(() => vi.advanceTimersByTimeAsync(200));
    expect(pollCodexDeviceOAuth).toHaveBeenCalledTimes(1);
    await act(() => vi.advanceTimersByTimeAsync(5000));
    expect(pollCodexDeviceOAuth).toHaveBeenCalledTimes(2);
    expect(onCredentials).toHaveBeenCalledWith('{"access_token":"a"}');
  });

  it('explains a refused login and offers a new code', async () => {
    vi.useFakeTimers();
    startCodexDeviceOAuth.mockResolvedValue({
      success: true,
      data: { session_id: 'sid', user_code: 'ABCD-12345', verification_url: 'https://x/codex/device', interval: 5, expires_in: 900 }
    });
    pollCodexDeviceOAuth.mockResolvedValue({
      success: false,
      code: 'channel_oauth_device_auth_failed',
      message: 'OpenAI refused: status 400: denied',
      params: { reason: 'status 400: denied' }
    });
    render(<ChannelOAuth type={59} getValues={getValues} onCredentials={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /OAuth Authorize/ }));
    await flush();
    await act(() => vi.advanceTimersByTimeAsync(5000));

    expect(screen.getByText('OpenAI refused the device code login.')).toBeTruthy();
    expect(screen.getByText('status 400: denied')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Get a new code' }));
    await flush();
    expect(startCodexDeviceOAuth).toHaveBeenCalledTimes(2);
  });
});

describe('paste flow', () => {
  const openPasteTab = async (type) => {
    render(<ChannelOAuth type={type} getValues={getValues} onCredentials={onCredentials} />);
    fireEvent.click(screen.getByRole('button', { name: /OAuth Authorize/ }));
    if (type === 59) fireEvent.click(screen.getByRole('tab', { name: 'Paste callback URL' }));
    await flush();
  };
  let onCredentials;

  beforeEach(() => {
    onCredentials = vi.fn();
    startChannelOAuth.mockResolvedValue({
      success: true,
      data: { auth_url: 'https://auth.openai.com/oauth/authorize?state=st', session_id: 'st', expires_in: 600 }
    });
  });

  it('flags a callback from another attempt and submits a matching paste by itself', async () => {
    exchangeChannelOAuthCode.mockResolvedValue({ success: true, data: { credentials: '{"k":1}' } });
    await openPasteTab(59);
    expect(startChannelOAuth).toHaveBeenCalledWith('codex', { channel_id: 0, proxy: '' });
    expect(window.open).not.toHaveBeenCalled();

    const box = screen.getByRole('textbox');
    fireEvent.change(box, { target: { value: 'http://localhost:1455/auth/callback?code=ac_abcdefghijkl&state=old' } });
    expect(screen.getByText(/earlier login attempt/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Submit code' }).disabled).toBe(true);

    const good = 'http://localhost:1455/auth/callback?code=ac_abcdefghijkl&state=st';
    fireEvent.paste(box, { clipboardData: { getData: () => good } });
    await flush();
    expect(exchangeChannelOAuthCode).toHaveBeenCalledWith('codex', 'st', good);
    expect(onCredentials).toHaveBeenCalledWith('{"k":1}');
  });

  it('shows a used code as an actionable error', async () => {
    exchangeChannelOAuthCode.mockResolvedValue({ success: false, code: 'channel_oauth_code_rejected', message: 'invalid_grant' });
    await openPasteTab(59);
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'http://localhost:1455/auth/callback?code=ac_abcdefghijkl&state=st' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Submit code' }));
    await flush();
    expect(screen.getByText(/already used or has expired/)).toBeTruthy();
    expect(onCredentials).not.toHaveBeenCalled();
  });

  it('opens the Claude page right away and accepts the code#state it shows', async () => {
    exchangeChannelOAuthCode.mockResolvedValue({ success: true, data: { credentials: '{"c":1}' } });
    await openPasteTab(58);
    expect(screen.queryByRole('tab')).toBeNull();
    expect(window.open).toHaveBeenCalledWith('https://auth.openai.com/oauth/authorize?state=st', '_blank', 'noopener,noreferrer');
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'abcdefghijklmnop#st' } });
    expect(screen.getByText('Authorization code found')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Submit code' }));
    await flush();
    expect(exchangeChannelOAuthCode).toHaveBeenCalledWith('claudecode', 'st', 'abcdefghijklmnop#st');
  });
});
