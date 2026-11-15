import { describe, expect, it } from 'vitest';

import {
  CUSTOM_PROVIDER,
  MAIL_PROVIDERS,
  detectMailProvider,
  fromAddressPatch,
  missingMailFields,
  providerPatch,
  savedSecretApplies,
  smtpTestPayload
} from './mailProviders';

describe('mail provider presets', () => {
  it('have unique ids, a logo and at least one server each', () => {
    const ids = MAIL_PROVIDERS.map((p) => p.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const p of MAIL_PROVIDERS) {
      expect(p.logo, p.id).toBeTruthy();
      expect(p.servers.length, p.id).toBeGreaterThan(0);
      expect(['ssl', 'starttls']).toContain(p.tls);
    }
  });

  it('detects the saved choice, then the server, then custom', () => {
    expect(detectMailProvider({ SMTPProvider: 'gmail', SMTPServer: 'smtp.resend.com' })).toBe('gmail');
    expect(detectMailProvider({ SMTPServer: 'SMTP.Resend.com' })).toBe('resend');
    expect(detectMailProvider({ SMTPServer: 'email-smtp.eu-west-1.amazonaws.com' })).toBe('ses');
    expect(detectMailProvider({ SMTPServer: 'mail.example.com' })).toBe(CUSTOM_PROVIDER);
    expect(detectMailProvider({})).toBe('');
  });

  it('fills server, port, encryption and a fixed username when a preset is picked', () => {
    expect(providerPatch('resend', { SMTPAccount: 'me@example.com', SMTPFrom: 'noreply@example.com' })).toEqual({
      SMTPProvider: 'resend',
      SMTPServer: 'smtp.resend.com',
      SMTPPort: '465',
      SMTPTLSMode: 'ssl',
      SMTPAccount: 'resend',
      SMTPToken: ''
    });
  });

  it('keeps a region already chosen and clears a username the previous preset fixed', () => {
    const patch = providerPatch('ses', { SMTPServer: 'smtp.resend.com', SMTPAccount: 'resend' });
    expect(patch.SMTPServer).toBe('email-smtp.us-east-1.amazonaws.com');
    expect(patch.SMTPAccount).toBe('');
    expect(providerPatch('ses', { SMTPProvider: 'ses', SMTPServer: 'email-smtp.ap-northeast-1.amazonaws.com' }).SMTPServer).toBe(
      'email-smtp.ap-northeast-1.amazonaws.com'
    );
  });

  it('uses the mailbox address as the username for mailbox providers', () => {
    expect(providerPatch('gmail', { SMTPFrom: 'me@gmail.com' }).SMTPAccount).toBe('me@gmail.com');
    expect(fromAddressPatch('gmail', 'a@gmail.com')).toEqual({ SMTPFrom: 'a@gmail.com', SMTPAccount: 'a@gmail.com' });
    expect(fromAddressPatch('resend', 'a@example.com')).toEqual({ SMTPFrom: 'a@example.com' });
  });

  it('switching to custom keeps every field', () => {
    expect(providerPatch(CUSTOM_PROVIDER, { SMTPServer: 'smtp.qq.com' })).toEqual({ SMTPProvider: CUSTOM_PROVIDER, SMTPToken: '' });
  });

  it('lists what is still missing', () => {
    expect(missingMailFields('resend', { SMTPFrom: 'a@example.com' }, true)).toEqual([]);
    expect(missingMailFields('resend', { SMTPFrom: 'a@example.com' }, false)).toEqual(['SMTPToken']);
    expect(missingMailFields('ses', { SMTPFrom: 'a@example.com', SMTPToken: 'x' }, false)).toEqual(['SMTPAccount']);
    expect(missingMailFields(CUSTOM_PROVIDER, { SMTPPort: '' }, true)).toEqual(['SMTPServer', 'SMTPPort', 'SMTPFrom']);
    expect(missingMailFields('', {}, false)).toEqual([]);
  });

  it('builds the test payload from the form', () => {
    expect(
      smtpTestPayload(
        { SMTPServer: 'smtp.resend.com', SMTPPort: '465', SMTPAccount: 'resend', SMTPFrom: 'a@b.c', SMTPFromName: 'Site' },
        'me@x.y'
      )
    ).toEqual({
      to: 'me@x.y',
      server: 'smtp.resend.com',
      port: 465,
      account: 'resend',
      from: 'a@b.c',
      from_name: 'Site',
      token: '',
      tls_mode: 'auto'
    });
  });

  it('counts the saved password only for the saved server and username', () => {
    const origin = { SMTPServer: 'smtp.resend.com', SMTPAccount: 'resend' };
    expect(savedSecretApplies({ SMTPServer: 'smtp.resend.com', SMTPAccount: 'resend' }, origin, true)).toBe(true);
    expect(savedSecretApplies({ SMTPServer: 'smtp.gmail.com', SMTPAccount: 'me@gmail.com' }, origin, true)).toBe(false);
    expect(savedSecretApplies(origin, origin, false)).toBe(false);
  });
});
