import { describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';

import { describeOAuthPaste, formatCountdown, oauthErrorText, parseOAuthPaste } from './oauthPaste';

describe('parseOAuthPaste', () => {
  it('reads a Codex callback URL', () => {
    expect(parseOAuthPaste(' http://localhost:1455/auth/callback?code=ac_abcdefghijkl&scope=openid&state=st-1 \n')).toEqual({
      code: 'ac_abcdefghijkl',
      state: 'st-1',
      error: '',
      errorDescription: ''
    });
  });

  it('reads a query string and a fragment', () => {
    expect(parseOAuthPaste('?code=ac_abcdefghijkl&state=st-1')).toMatchObject({ code: 'ac_abcdefghijkl', state: 'st-1' });
    expect(parseOAuthPaste('https://example.com/cb#code=abcdefghijkl&state=st-3')).toMatchObject({ code: 'abcdefghijkl', state: 'st-3' });
  });

  it('reads a Claude code#state pair and a bare code', () => {
    expect(parseOAuthPaste('abcdefghijklmnop#st-2')).toMatchObject({ code: 'abcdefghijklmnop', state: 'st-2' });
    expect(parseOAuthPaste('abcdefghijklmnop')).toMatchObject({ code: 'abcdefghijklmnop', state: '' });
  });

  it('reads a provider error', () => {
    expect(
      parseOAuthPaste('http://localhost:1455/auth/callback?error=access_denied&error_description=User+cancelled&state=s')
    ).toMatchObject({
      error: 'access_denied',
      errorDescription: 'User cancelled',
      code: ''
    });
  });

  it('rejects input without a code', () => {
    for (const input of ['', '  ', 'short', 'two words here', 'https://auth.openai.com/oauth/authorize?client_id=x&state=s']) {
      expect(parseOAuthPaste(input)).toBeNull();
    }
  });
});

describe('describeOAuthPaste', () => {
  const cb = (state) => `http://localhost:1455/auth/callback?code=ac_abcdefghijkl&state=${encodeURIComponent(state)}`;

  it('accepts a callback for this session', () => {
    expect(describeOAuthPaste(cb('abc=='), 'abc==')).toEqual({ kind: 'ok', code: 'ac_abcdefghijkl' });
    expect(describeOAuthPaste('abcdefghijklmnop', 'abc')).toEqual({ kind: 'ok', code: 'abcdefghijklmnop' });
  });

  it('flags an earlier attempt, a denial, the auth link and junk', () => {
    expect(describeOAuthPaste(cb('old'), 'new').kind).toBe('stateMismatch');
    expect(describeOAuthPaste('abcdefghijklmnop#old', 'new').kind).toBe('stateMismatch');
    expect(describeOAuthPaste('http://localhost:1455/auth/callback?error=access_denied&state=new', 'new')).toEqual({
      kind: 'denied',
      error: 'access_denied'
    });
    expect(describeOAuthPaste('https://auth.openai.com/oauth/authorize?client_id=x', 'new').kind).toBe('authLink');
    expect(describeOAuthPaste('hello', 'new').kind).toBe('invalid');
    expect(describeOAuthPaste('   ', 'new').kind).toBe('empty');
  });
});

describe('oauthErrorText', () => {
  const t = (key) => `t:${key}`;

  it('translates known codes and keeps the provider reason apart', async () => {
    await i18n.changeLanguage('en_US');
    expect(
      oauthErrorText(t, { code: 'channel_oauth_exchange_failed', message: 'Failed: status 500', params: { reason: 'status 500' } }, 'fb')
    ).toEqual({
      code: 'channel_oauth_exchange_failed',
      text: 'Could not exchange the authorization code for credentials.',
      detail: 'status 500'
    });
  });

  it('falls back to the server message, then to the fallback key', () => {
    expect(oauthErrorText(t, { message: 'boom' }, 'fb')).toEqual({ code: '', text: 'boom', detail: '' });
    expect(oauthErrorText(t, {}, 'fb').text).toBe('t:fb');
  });
});

it('formats the countdown', () => {
  expect(formatCountdown(905)).toBe('15:05');
  expect(formatCountdown(-3)).toBe('0:00');
});
