import { apiMessage } from 'utils/apiError';

// Recognizes what an admin pasted back from a ClaudeCode / Codex authorization, following the
// same rules as controller/oauth_paste.go, so the dialog can say what is wrong before submitting
// (and submit by itself when the paste is a valid callback for this session).

// Returns { code, state, error, errorDescription } or null when no code or error is present.
export function parseOAuthPaste(input) {
  const s = (input || '').trim();
  if (!s) return null;

  let query;
  if (/^https?:\/\//i.test(s)) {
    let url;
    try {
      url = new URL(s);
    } catch {
      return null;
    }
    query = url.search.replace(/^\?/, '');
    if (!/(^|&)(code|error)=/.test(query)) query = url.hash.replace(/^#/, '');
  } else if (s.includes('code=') || s.includes('error=')) {
    query = s.replace(/^\?/, '');
  } else {
    const [code, state = ''] = s.split(/#(.*)/s);
    if (code.trim().length < 10 || /\s/.test(code.trim())) return null;
    return { code: code.trim(), state: state.trim(), error: '', errorDescription: '' };
  }

  const params = new URLSearchParams(query);
  const result = {
    code: (params.get('code') || '').trim(),
    state: (params.get('state') || '').trim(),
    error: (params.get('error') || '').trim(),
    errorDescription: (params.get('error_description') || '').trim()
  };
  return result.code || result.error ? result : null;
}

// Classifies a paste for the status line under the input:
//   empty | ok | stateMismatch | denied | authLink | invalid
export function describeOAuthPaste(input, sessionState) {
  if (!(input || '').trim()) return { kind: 'empty' };
  const parsed = parseOAuthPaste(input);
  if (!parsed) {
    return { kind: /\/oauth\/authorize\b/.test(input) ? 'authLink' : 'invalid' };
  }
  if (parsed.state && sessionState && parsed.state !== sessionState) return { kind: 'stateMismatch' };
  if (parsed.error) {
    return { kind: 'denied', error: parsed.errorDescription ? `${parsed.error} (${parsed.errorDescription})` : parsed.error };
  }
  return { kind: 'ok', code: parsed.code };
}

// Turns a failed `{ success: false, message, code, params }` response into { code, text, detail }:
// the translated message (see utils/apiError.js) and the provider's own reason, shown under it.
export function oauthErrorText(t, data, fallbackKey) {
  return {
    code: typeof data?.code === 'string' ? data.code : '',
    text: apiMessage(data, t(fallbackKey)),
    detail: typeof data?.params?.reason === 'string' ? data.params.reason : ''
  };
}

// mm:ss for the expiry countdown.
export function formatCountdown(seconds) {
  const s = Math.max(0, Math.floor(seconds));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
