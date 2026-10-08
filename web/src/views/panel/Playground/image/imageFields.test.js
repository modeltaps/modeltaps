import { describe, expect, it } from 'vitest';

import { aspectRatio, paramsSummary, sizeTier } from './imageFields';

// 参数 chip 的摘要：尺寸压成「宽高比 · 分辨率档位」，再接张数与质量。

describe('aspectRatio', () => {
  it('把像素尺寸约成宽高比，非法取值给空串', () => {
    expect(aspectRatio('1024x1024')).toBe('1:1');
    expect(aspectRatio('1024x1536')).toBe('2:3');
    expect(aspectRatio('1792x1024')).toBe('7:4');
    expect(aspectRatio('auto')).toBe('');
  });
});

describe('sizeTier', () => {
  it('长边 ≥1024 用 k 记，小图直接写像素', () => {
    expect(sizeTier('1024x1024')).toBe('1k');
    expect(sizeTier('1024x1536')).toBe('1.5k');
    expect(sizeTier('512x512')).toBe('512px');
    expect(sizeTier('')).toBe('');
  });
});

describe('paramsSummary', () => {
  it('尺寸 · 档位 · 张数，quality 的 auto 不写进摘要', () => {
    expect(paramsSummary({ size: '1024x1024', n: 1, quality: 'auto' })).toBe('1:1 · 1k · 1x');
    expect(paramsSummary({ size: '1536x1024', n: 2, quality: 'high' })).toBe('3:2 · 1.5k · 2x · high');
  });

  it('什么都没有时给空串，由调用方回落成「参数」二字', () => {
    expect(paramsSummary({})).toBe('');
  });
});
