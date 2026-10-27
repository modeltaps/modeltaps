import { describe, it, expect } from 'vitest';
import { maskSecret } from './maskSecret';

// Mirrors TestMaskSecret in common/utils/mask_secret_test.go.
describe('maskSecret', () => {
  it.each([
    ['', ''],
    ['   ', ''],
    [null, ''],
    [undefined, ''],
    ['short', '****'],
    ['1234567', '****'],
    ['12345678', '****5678'],
    ['abcdefghijklmno', '****lmno'],
    ['abcdefghijklmnop', 'abcd****mnop'],
    ['sk-0123456789abcdefghijklmnopqrstuvwxyz', 'sk-0****wxyz'],
    ['  sk-0123456789abcdefghijklmnopqrstuvwxyz\n', 'sk-0****wxyz'],
    ['密钥密钥密钥密钥密钥密钥密钥密钥', '密钥密钥****密钥密钥']
  ])('masks %j as %j', (input, want) => {
    expect(maskSecret(input)).toBe(want);
  });
});
