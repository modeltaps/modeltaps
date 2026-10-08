import { describe, expect, it } from 'vitest';

import { formatFullAmount } from './compactAmount';

// 侧栏页脚的可用额度:始终显示完整金额(千分位、两位小数)。

describe('formatFullAmount', () => {
  it('始终给完整值', () => {
    expect(formatFullAmount(1234567.891)).toBe('$1,234,567.89');
    expect(formatFullAmount(-5)).toBe('-$5.00');
    expect(formatFullAmount(null)).toBe('$0.00');
    expect(formatFullAmount('x')).toBe('—');
  });

  it('负数保留符号,非数字显示占位', () => {
    expect(formatFullAmount(-12345)).toBe('-$12,345.00');
    expect(formatFullAmount(NaN)).toBe('—');
    expect(formatFullAmount(undefined)).toBe('—');
  });
});
