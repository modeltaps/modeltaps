// Static config objects (channel types, channel plugins, payment gateways) hold i18n keys instead of
// display text, so the text follows the active language. A value is either a key or a
// [key, params] pair; an empty value renders as an empty string.
export function configText(t, value) {
  if (!value) return '';
  if (Array.isArray(value)) return t(value[0], value[1]);
  return t(value);
}
