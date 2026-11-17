// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import TotpStep from './TotpStep';

// The second sign-in step: six digits are sent at once, a recovery code on submit, a timed-out
// sign-in goes back to the password form with its reason.

beforeEach(() => i18n.changeLanguage('en_US'));
afterEach(cleanup);

const setup = (result = { success: false, message: 'The code is incorrect or has already been used' }) => {
  const props = { onVerify: vi.fn().mockResolvedValue(result), onExpired: vi.fn(), onCancel: vi.fn() };
  render(createElement(TotpStep, props));
  return props;
};
const input = () => document.getElementById('totp-code');

describe('TotpStep', () => {
  it('sends six digits as soon as they are in and shows a wrong code', async () => {
    const { onVerify } = setup();
    expect(screen.getByText(i18n.t('login.totpHint'))).toBeTruthy();
    expect(input().getAttribute('autocomplete')).toBe('one-time-code');
    fireEvent.change(input(), { target: { value: '12345' } });
    expect(onVerify).not.toHaveBeenCalled();
    fireEvent.change(input(), { target: { value: '123456' } });
    await screen.findByText('The code is incorrect or has already been used');
    expect(onVerify).toHaveBeenCalledWith('123456');
    expect(input().value).toBe('');
  });

  it('takes a recovery code on submit', async () => {
    const { onVerify } = setup({ success: true });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('login.useRecoveryCode') }));
    expect(screen.getByText(i18n.t('login.totpRecoveryHint'))).toBeTruthy();
    fireEvent.change(input(), { target: { value: 'abcd-efgh-ijkl-mnop' } });
    expect(onVerify).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: i18n.t('login.totpVerify') }));
    await waitFor(() => expect(onVerify).toHaveBeenCalledWith('abcd-efgh-ijkl-mnop'));
  });

  it('goes back to the password form when the sign-in timed out', async () => {
    const { onExpired, onCancel } = setup({ success: false, code: 'totp_challenge_expired', message: 'Sign-in timed out.' });
    fireEvent.change(input(), { target: { value: '654321' } });
    await waitFor(() => expect(onExpired).toHaveBeenCalledWith('Sign-in timed out.'));
    fireEvent.click(screen.getByRole('button', { name: i18n.t('login.backToSignIn') }));
    expect(onCancel).toHaveBeenCalled();
  });
});
