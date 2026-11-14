import { describe, expect, it, vi } from 'vitest';

import { isServerUnavailable, retryWhileUnavailable } from './serverRetry';

const noWait = () => Promise.resolve();
const httpError = (status) => Object.assign(new Error(`HTTP ${status}`), { response: { status } });

describe('isServerUnavailable', () => {
  it('is true without a response or for a 5xx', () => {
    expect(isServerUnavailable(new Error('Network Error'))).toBe(true);
    expect(isServerUnavailable(httpError(502))).toBe(true);
  });
  it('is false for an answer such as 401', () => {
    expect(isServerUnavailable(httpError(401))).toBe(false);
    expect(isServerUnavailable(null)).toBe(false);
  });
});

describe('retryWhileUnavailable', () => {
  it('tries again until the server answers', async () => {
    const fn = vi.fn().mockRejectedValueOnce(httpError(502)).mockRejectedValueOnce(new Error('Network Error')).mockResolvedValue('ok');
    await expect(retryWhileUnavailable(fn, [1, 1, 1], noWait)).resolves.toBe('ok');
    expect(fn).toHaveBeenCalledTimes(3);
  });
  it('does not retry a 401', async () => {
    const fn = vi.fn().mockRejectedValue(httpError(401));
    await expect(retryWhileUnavailable(fn, [1, 1], noWait)).rejects.toThrow('HTTP 401');
    expect(fn).toHaveBeenCalledTimes(1);
  });
  it('gives up after the last delay', async () => {
    const fn = vi.fn().mockRejectedValue(httpError(503));
    await expect(retryWhileUnavailable(fn, [1, 1], noWait)).rejects.toThrow('HTTP 503');
    expect(fn).toHaveBeenCalledTimes(3);
  });
});
