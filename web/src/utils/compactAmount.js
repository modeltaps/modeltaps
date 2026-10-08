// 金额显示:侧栏页脚给完整值(千分位、两位小数),非数字显示占位符。

export function formatFullAmount(value, { currency = '$' } = {}) {
  const n = Number(value);
  if (!Number.isFinite(n)) return '—';
  return `${n < 0 ? '-' : ''}${currency}${Math.abs(n).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}
