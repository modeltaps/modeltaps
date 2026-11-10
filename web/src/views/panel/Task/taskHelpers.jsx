// ==============================|| TASK — HELPERS ||============================== //
// Ported from the v1 Task view (type/Type.js + component/TableRow.jsx). Pure logic +
// small presentational helpers so the table/detail components stay lean.

// status value → { i18n text key, badge color token }. Mirrors v1 STATUS_TYPE.
export const STATUS_TYPE = {
  SUCCESS: { labelKey: 'taskPage.statusType.success', color: 'success' },
  NOT_START: { labelKey: 'taskPage.statusType.notStart', color: 'default' },
  SUBMITTED: { labelKey: 'taskPage.statusType.submitted', color: 'secondary' },
  IN_PROGRESS: { labelKey: 'taskPage.statusType.inProgress', color: 'primary' },
  FAILURE: { labelKey: 'taskPage.statusType.failure', color: 'orange' },
  QUEUED: { labelKey: 'taskPage.statusType.queued', color: 'default' },
  UNKNOWN: { labelKey: 'taskPage.statusType.unknown', color: 'default' }
};

// color token → Tailwind classes for soft badges. Same palette as the Log view:
// one quiet neutral style; the failure state (orange) keeps the error tint.
const NEUTRAL_BADGE = 'bg-muted text-foreground';
const BADGE_CLASSES = {
  default: NEUTRAL_BADGE,
  primary: NEUTRAL_BADGE,
  orange: 'bg-destructive/10 text-destructive',
  info: NEUTRAL_BADGE,
  secondary: 'bg-muted text-muted-foreground',
  success: NEUTRAL_BADGE,
  error: 'bg-destructive/10 text-destructive'
};

export const badgeClass = (color) =>
  `inline-flex items-center rounded-sm px-2 py-0.5 text-xs font-medium ${BADGE_CLASSES[color] || BADGE_CLASSES.default}`;

export function statusMeta(status) {
  return STATUS_TYPE[status] || null;
}

// finish_time - submit_time (seconds). Mirrors v1 request_time computation.
export function deriveTaskDuration(item) {
  if (!item.finish_time || item.finish_time <= 0) {
    return { requestTime: 0, requestTimeStr: '' };
  }
  const requestTime = item.finish_time - item.submit_time;
  return { requestTime, requestTimeStr: `${requestTime.toFixed(2)} S` };
}

// suno MUSIC tasks carry a media array we render specially in the detail sheet.
export const isSunoMusic = (item) => item?.platform === 'suno' && item?.action === 'MUSIC';
