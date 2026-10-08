// ==============================|| SYSTEM LOGS — HELPERS ||============================== //
// Shared utilities for the system log viewer. Mirrors the v1 SystemLogs logic.

const LEVEL_MAP = {
  error: 'error',
  err: 'error',
  fatal: 'error',
  warn: 'warning',
  warning: 'warning',
  debug: 'debug',
  info: 'info'
};

// Normalize a backend log entry ({ Level, Timestamp, Message }) into the shape
// the viewer renders. Falls back to an error entry if parsing fails.
export function processLogEntry(entry) {
  try {
    const level = entry.Level.toLowerCase();
    return {
      timestamp: new Date(entry.Timestamp).toISOString(),
      type: LEVEL_MAP[level] || 'info',
      message: entry.Message
    };
  } catch {
    return {
      timestamp: new Date().toISOString(),
      type: 'error',
      message: `Failed to process log entry: ${JSON.stringify(entry)}`
    };
  }
}

// Filter logs by search term. Regex mode falls back to plain text on invalid input.
export function filterLogs(logs, searchTerm, useRegex) {
  if (!searchTerm.trim()) return logs;
  const plain = () => {
    const lower = searchTerm.toLowerCase();
    return logs.filter((log) => log.message.toLowerCase().includes(lower));
  };
  if (!useRegex) return plain();
  try {
    const regex = new RegExp(searchTerm, 'i');
    return logs.filter((log) => regex.test(log.message));
  } catch {
    return plain();
  }
}

export function formatTimestamp(timestamp) {
  const date = new Date(timestamp);
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

// Map a log level to a shadcn Badge variant (+ optional className for warning).
export function levelBadge(type) {
  switch ((type || '').toLowerCase()) {
    case 'error':
      return { variant: 'destructive', className: '' };
    case 'warn':
    case 'warning':
      return { variant: 'outline', className: 'border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400' };
    case 'debug':
      return { variant: 'secondary', className: '' };
    case 'info':
    default:
      return { variant: 'default', className: '' };
  }
}
