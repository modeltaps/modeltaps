import i18n from 'i18n/i18n';

// Admin APIs may send a stable `code` and `params` next to the English `message` (see
// common/api_error.go). A code with a key under apiError.* in locales/modeltaps is shown in the
// UI language; any other response keeps the server message.
const NAMESPACE = 'apiError';
const CODE_PATTERN = /^[a-z0-9_]+$/;

// i18next reads these from the options object; a server param must never steer the lookup.
const RESERVED_OPTIONS = [
  'lng',
  'lngs',
  'ns',
  'context',
  'defaultValue',
  'returnObjects',
  'joinArrays',
  'interpolation',
  'replace',
  'postProcess'
];

// Admin bodies are { success, message, code, params }; relay-style bodies (such as the 429 from
// the rate limiter) nest the same fields under `error`.
function payloadOf(data) {
  if (!data || typeof data !== 'object') return null;
  if (data.error && typeof data.error === 'object') return data.error;
  return data;
}

// The i18n key for a response, or null when it has no code the UI knows.
export function apiMessageKey(data) {
  const code = payloadOf(data)?.code;
  if (typeof code !== 'string' || !CODE_PATTERN.test(code)) return null;
  const key = `${NAMESPACE}.${code}`;
  return i18n.exists(key) ? key : null;
}

// The params of a response, or an empty object.
export function apiMessageParams(data) {
  const params = payloadOf(data)?.params;
  return params && typeof params === 'object' && !Array.isArray(params) ? params : {};
}

// The text to show for a response: the translation of its code, else its message, else fallback.
export function apiMessage(data, fallback = '') {
  const payload = payloadOf(data);
  const message = typeof payload?.message === 'string' && payload.message ? payload.message : fallback;
  const key = apiMessageKey(data);
  if (!key) return message;
  const options = { ...apiMessageParams(data) };
  RESERVED_OPTIONS.forEach((name) => delete options[name]);
  // The text is rendered as React text, so values such as upstream reasons stay unescaped.
  return i18n.t(key, { ...options, defaultValue: message, interpolation: { escapeValue: false } });
}

// Replaces data.message with its translation in place, so call sites that show `message` from a
// response body need no change. Bodies without a known code are left as they are.
export function localizeApiMessage(data) {
  if (!data || typeof data !== 'object' || typeof data.message !== 'string') return data;
  if (!apiMessageKey(data) || data.error) return data;
  data.message = apiMessage(data);
  return data;
}
