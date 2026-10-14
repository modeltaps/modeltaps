// Helpers for the channel check dialog (ChannelCheckDialog.jsx).

// Each model runs the base check first and then up to four checks in parallel on the server, so
// two models at a time keep at most eight requests in flight against one upstream.
export const CHECK_CONCURRENCY = 2;

// Upper bound of upstream requests per model (the relay check is not sent when it cannot apply).
export const REQUESTS_PER_MODEL = 5;

// Column order of the checks; keys match `key` of the backend's check items.
export const PROBE_KEYS = ['base', 'image', 'json', 'tool', 'relay'];

// Same values as the backend's check_channel status constants.
export const STATUS = { FAILED: 0, SUCCESS: 1, WARNING: 2, SKIPPED: 3 };

// rowOutcome folds a row result into passed / warning / failed; a request that itself failed counts as failed.
export function rowOutcome(result) {
  if (!result || result.status === 'error' || result.data?.status === STATUS.FAILED) return 'failed';
  if (result.data?.status === STATUS.WARNING) return 'warning';
  return 'passed';
}

export const isFinished = (result) => result?.status === 'done' || result?.status === 'error';

// Params are upstream text, numbers and model names: they are shown as they are. Arrays (the
// tool calls) are joined. Escaping stays off because React already renders them as text.
function interpolationValues(params) {
  const values = {};
  for (const [key, value] of Object.entries(params || {})) {
    values[key] = Array.isArray(value) ? value.join(', ') : value;
  }
  return values;
}

// describeResult turns one backend verdict into its label and message. The backend sends stable
// keys (name, code) and raw params; an unknown code falls back to the code and its params so a
// newer backend never shows an empty line.
export function describeResult(t, result) {
  const values = interpolationValues(result.params);
  const fallback = Object.keys(values).length > 0 ? `${result.code} ${JSON.stringify(values)}` : result.code;
  return {
    label: t(`channelCheck.result.${result.name}`, { defaultValue: result.name }),
    message: t(`channelCheck.code.${result.code}`, {
      ...values,
      defaultValue: fallback,
      interpolation: { escapeValue: false }
    })
  };
}

// describeResults renders all verdicts of a check as "label: message" lines, used as a tooltip.
export const describeResults = (t, results) =>
  (results || [])
    .map((r) => {
      const { label, message } = describeResult(t, r);
      return `${label}: ${message}`;
    })
    .join('\n');
