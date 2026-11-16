// ==============================|| SYSTEM LOGS — HELPERS ||============================== //

export const LEVELS = ['error', 'warn', 'info', 'debug'];

const LEVEL_MAP = {
  err: 'error',
  error: 'error',
  fatal: 'error',
  panic: 'error',
  dpanic: 'error',
  warn: 'warn',
  warning: 'warn',
  debug: 'debug'
};

export function normalizeLevel(level) {
  return LEVEL_MAP[String(level || '').toLowerCase()] || 'info';
}

// Lines read "<source> | <text>", where the source is `[SYS]`, a job name or a request id.
const SOURCE_RE = /^(\S+) \| ([\s\S]*)$/;

// Access lines are `GIN request<TAB>{json}`; show them as one request line keyed by its request id.
const ACCESS_RE = /^GIN request\s+(\{[\s\S]*\})$/;

function accessLine(message) {
  const m = ACCESS_RE.exec(message);
  if (!m) return null;
  try {
    const r = JSON.parse(m[1]);
    const path = r.query ? `${r.path}?${r.query}` : r.path;
    const parts = [`${r.method} ${path}`, `→ ${r.status}`, r.latency];
    if (r.user_id) parts.push(`user_id=${r.user_id}`);
    if (r.requested_model) parts.push(`model=${r.requested_model}`);
    return { source: r.request_id || 'GIN', text: parts.filter(Boolean).join('  ') };
  } catch {
    return null;
  }
}

// Normalize a backend entry ({ Timestamp, Level, Message }). `timestamp` stays the raw value the
// server sent, so the context lookup can match it exactly.
export function processLogEntry(entry) {
  const message = String(entry?.Message ?? '').replace(/\s+$/, '');
  const access = accessLine(message);
  if (access) return { timestamp: entry?.Timestamp, level: normalizeLevel(entry?.Level), ...access, message };
  const match = SOURCE_RE.exec(message);
  let source = match ? match[1] : '';
  if (source === '[SYS]') source = 'SYS';
  return {
    timestamp: entry?.Timestamp,
    level: normalizeLevel(entry?.Level),
    source,
    text: match ? match[2] : message,
    message
  };
}

const pad = (n) => String(n).padStart(2, '0');

// `11-16 15:17:11`; the year is in the title (formatFullTimestamp).
export function formatTimestamp(timestamp) {
  const d = new Date(timestamp);
  if (Number.isNaN(d.getTime())) return '';
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export function formatFullTimestamp(timestamp) {
  const d = new Date(timestamp);
  if (Number.isNaN(d.getTime())) return '';
  return `${d.getFullYear()}-${formatTimestamp(timestamp)}`;
}
