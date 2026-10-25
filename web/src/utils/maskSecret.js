// Display mask for keys, passwords and other secrets. Same rule as the backend
// utils.MaskSecret (common/utils/mask.go), so a value reads the same everywhere:
// 16+ characters show the first 4 and last 4, 8–15 show only the last 4, shorter
// values show nothing.
const STARS = '****';
const EDGE = 4;
const MIN_HEAD = 16;
const MIN_TAIL = 8;

export function maskSecret(secret) {
  const chars = Array.from(String(secret ?? '').trim());
  if (chars.length === 0) return '';
  if (chars.length < MIN_TAIL) return STARS;
  const tail = chars.slice(-EDGE).join('');
  if (chars.length < MIN_HEAD) return STARS + tail;
  return chars.slice(0, EDGE).join('') + STARS + tail;
}
