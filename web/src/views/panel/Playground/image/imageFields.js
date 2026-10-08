// ==============================|| PLAYGROUND — IMAGE PARAM SUMMARY ||============================== //
// 输入框里那颗参数 chip 的摘要：把 IMAGE_FIELDS 求值后的尺寸 / 张数 / 质量压成一行
// 「1:1 · 1k · 1x」。字段本身仍在 shared/fieldSchema（与请求体同源），这里只管怎么写在 chip 上。

const gcd = (a, b) => (b === 0 ? a : gcd(b, a % b));

export function parseSize(size) {
  const [width, height] = String(size || '')
    .toLowerCase()
    .split('x')
    .map(Number);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null;
  return { width, height };
}

// 「1024x1536」在 chip 上写成宽高比，比原始像素好认。
export function aspectRatio(size) {
  const parsed = parseSize(size);
  if (!parsed) return '';
  const divisor = gcd(parsed.width, parsed.height);
  return `${parsed.width / divisor}:${parsed.height / divisor}`;
}

// 分辨率档位取长边：≥1k 用 k 记（1536 → 1.5k），小图直接写像素。
export function sizeTier(size) {
  const parsed = parseSize(size);
  if (!parsed) return '';
  const longest = Math.max(parsed.width, parsed.height);
  if (longest < 1024) return `${longest}px`;
  return `${String(Math.round((longest / 1024) * 10) / 10)}k`;
}

// quality 的 auto 等于「不选」，写进摘要没有信息量。
export function paramsSummary(values = {}) {
  const parts = [aspectRatio(values.size), sizeTier(values.size)];
  const count = Number(values.n);
  if (Number.isFinite(count) && count > 0) parts.push(`${count}x`);
  if (values.quality && values.quality !== 'auto') parts.push(values.quality);
  return parts.filter(Boolean).join(' · ');
}
