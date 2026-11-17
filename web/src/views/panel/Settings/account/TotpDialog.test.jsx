// @vitest-environment jsdom
import { createElement } from 'react';
import { Provider } from 'react-redux';
import { legacy_createStore as createStore } from 'redux';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import reducer from 'store/reducer';
import { API } from 'utils/api';
import TotpDialog from './TotpDialog';

// Two-step verification on the account page: setup shows the QR code and the key, a code turns it
// on and the recovery codes are shown once (the dialog cannot be dismissed until "I've saved
// them"); when on, a code turns it off.

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn() } }));
vi.mock('utils/common', () => ({ copy: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(), timestamp2string: () => '2026-11-17 10:00:00' }));
vi.mock('react-qrcode-logo', async () => {
  const { createElement: h } = await import('react');
  return { QRCode: ({ value }) => h('canvas', { 'data-value': value }) };
});

const tk = (k) => i18n.t(`settingsPage.security.${k}`);
const recoveryCodes = Array.from({ length: 10 }, (_, i) => `AAAA-BBBB-CCCC-${String(i).padStart(4, '0')}`);

beforeEach(() => {
  vi.clearAllMocks();
  i18n.changeLanguage('en_US');
});
afterEach(cleanup);

const open = (props) => {
  const onOpenChange = vi.fn();
  const onChanged = vi.fn();
  render(
    createElement(
      Provider,
      { store: createStore(reducer) },
      createElement(TotpDialog, { open: true, onOpenChange, onChanged, account: 'alice', ...props })
    )
  );
  return { onOpenChange, onChanged };
};

const codeInput = () => document.getElementById('totp-dialog-code');

describe('TotpDialog', () => {
  it('sets up: QR code and key, a code turns it on, recovery codes stay until confirmed', async () => {
    API.post.mockImplementation((path) => {
      if (path === '/api/user/totp/setup') {
        return Promise.resolve({ data: { success: true, data: { secret: 'JBSWY3DPEHPK3PXP', otpauth_url: 'otpauth://totp/x' } } });
      }
      return Promise.resolve({ data: { success: true, data: { recovery_codes: recoveryCodes } } });
    });
    const { onOpenChange, onChanged } = open({ status: { enabled: false } });

    await screen.findByText('JBSWY3DPEHPK3PXP');
    expect(document.querySelector('canvas').getAttribute('data-value')).toBe('otpauth://totp/x');

    // Without a code nothing is sent.
    fireEvent.click(screen.getByRole('button', { name: tk('enable') }));
    expect(API.post).toHaveBeenCalledTimes(1);

    fireEvent.change(codeInput(), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: tk('enable') }));
    await screen.findByText(recoveryCodes[9]);
    expect(API.post).toHaveBeenLastCalledWith('/api/user/totp/enable', { code: '123456' });
    expect(onChanged).toHaveBeenCalled();

    // Escape does not dismiss the codes.
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onOpenChange).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: tk('totpRecoverySaved') }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('shows the error of a wrong code and stays on the setup', async () => {
    API.post.mockImplementation((path) =>
      Promise.resolve(
        path === '/api/user/totp/setup'
          ? { data: { success: true, data: { secret: 'S', otpauth_url: 'otpauth://totp/x' } } }
          : { data: { success: false, message: 'The code is incorrect or has already been used' } }
      )
    );
    open({ status: { enabled: false } });
    await waitFor(() => expect(codeInput()).not.toBeNull());
    fireEvent.change(codeInput(), { target: { value: '000000' } });
    fireEvent.click(screen.getByRole('button', { name: tk('enable') }));
    await screen.findByText('The code is incorrect or has already been used');
    expect(screen.queryByText(tk('totpRecoveryTitle'))).toBeNull();
  });

  it('when on, a code turns it off without starting a new setup', async () => {
    API.post.mockResolvedValue({ data: { success: true } });
    const { onOpenChange, onChanged } = open({ status: { enabled: true, enabled_time: 1, recovery_codes_left: 7 } });

    expect(screen.getByText(i18n.t('settingsPage.security.totpOn', { count: 7 }))).toBeTruthy();
    fireEvent.change(codeInput(), { target: { value: 'aaaa-bbbb-cccc-dddd' } });
    fireEvent.click(screen.getByRole('button', { name: tk('totpTurnOff') }));
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(API.post).toHaveBeenCalledTimes(1);
    expect(API.post).toHaveBeenCalledWith('/api/user/totp/disable', { code: 'aaaa-bbbb-cccc-dddd' });
    expect(onChanged).toHaveBeenCalled();
  });
});
