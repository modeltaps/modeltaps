// 紧凑货币格式化(展示层)。极小金额在窄屏卡片/紧凑容器里的长小数会溢出边界,
// 参考 OpenRouter 的处理:有效位数截断 + "<$0.0001" 下限表达,完整精确值放
// tooltip/title,信息不丢失。输入为已换算成货币的数值(USD),非原始 quota。

const CURRENCY_FLOOR = 0.0001; // 低于此值只给下限表达,不再展开长小数

// 完整精确值(不含符号):最多 8 位小数、去尾零,供 title 悬浮展示。
function fullDecimals(abs) {
  const s = abs.toFixed(8).replace(/0+$/, '').replace(/\.$/, '');
  return s === '' ? '0' : s;
}

// 有效位数截断(不含符号):如 0.00123 → "0.0012"、0.0089 → "0.0089"。
function toSignificant(abs, sig) {
  return String(Number(abs.toPrecision(sig)));
}

// 紧凑金额:返回 { text, title }。
// - text  紧凑展示串(带 $),窄容器不溢出
// - title 完整精确值(带 $),供 hover 展示,信息不丢失
// 规则:
//   0            → $0.00
//   ≥ $0.01      → $X.XX(2 位小数,常规金额显示不受影响)
//   [floor,0.01) → 2 位有效数字截断,如 $0.0012
//   (0,floor)    → "<$0.0001"(负值为 ">-$0.0001")
export function compactCurrency(amount) {
  const num = Number(amount) || 0;
  const abs = Math.abs(num);
  const sign = num < 0 ? '-' : '';

  if (abs === 0) {
    return { text: '$0.00', title: '$0.00' };
  }

  const title = sign + '$' + fullDecimals(abs);

  if (abs >= 0.01) {
    return { text: sign + '$' + abs.toFixed(2), title };
  }

  if (abs < CURRENCY_FLOOR) {
    return { text: (sign ? '>-$' : '<$') + '0.0001', title };
  }

  return { text: sign + '$' + toSignificant(abs, 2), title };
}
