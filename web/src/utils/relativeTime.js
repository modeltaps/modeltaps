// 相对时间（unix 秒）：本地化 Intl.RelativeTimeFormat，就近取最大单位
const RELATIVE_TIME_UNITS = [
  ['year', 31536000],
  ['month', 2592000],
  ['week', 604800],
  ['day', 86400],
  ['hour', 3600],
  ['minute', 60],
  ['second', 1]
];
export function relativeTime(rtf, tsSec) {
  const diffSec = tsSec - Math.floor(Date.now() / 1000);
  const absSec = Math.abs(diffSec);
  for (const [unit, secs] of RELATIVE_TIME_UNITS) {
    if (absSec >= secs || unit === 'second') return rtf.format(Math.round(diffSec / secs), unit);
  }
  return rtf.format(0, 'second');
}
