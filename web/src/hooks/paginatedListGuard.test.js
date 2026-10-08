// ==============================|| 分页请求守卫 — 竞态断言(UX-13) ||============================== //

import { describe, it, expect, vi } from 'vitest';
import { createRequestGuard, runGuardedFetch } from './paginatedListGuard.js';

// 手动控制 resolve/reject 时机的 deferred,模拟乱序完成的网络请求。
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

describe('createRequestGuard', () => {
  it('only the latest begun request is current', () => {
    const guard = createRequestGuard();
    const a = guard.begin();
    const b = guard.begin();
    expect(guard.isCurrent(a)).toBe(false);
    expect(guard.isCurrent(b)).toBe(true);
  });
});

describe('runGuardedFetch', () => {
  it('applies only the newest of two out-of-order fetches (old resolves last)', async () => {
    const guard = createRequestGuard();
    const applied = [];
    const finallyCalls = vi.fn();
    const dA = deferred();
    const dB = deferred();

    const pA = runGuardedFetch(guard, () => dA.promise, { onResult: (r) => applied.push(r), onFinally: finallyCalls });
    const pB = runGuardedFetch(guard, () => dB.promise, { onResult: (r) => applied.push(r), onFinally: finallyCalls });

    // 新请求 B 先完成,旧请求 A 后完成——A 的响应必须被丢弃。
    dB.resolve('B');
    await pB;
    dA.resolve('A');
    await pA;

    expect(applied).toEqual(['B']);
    // onFinally 也受守卫保护:只有最新请求 B 收尾,旧请求 A 不得清掉 loading 态。
    expect(finallyCalls).toHaveBeenCalledTimes(1);
  });

  it('stale request must not clear the loading state of the in-flight newer request', async () => {
    const guard = createRequestGuard();
    let searching = false;
    const opts = {
      onStart: () => {
        searching = true;
      },
      onFinally: () => {
        searching = false;
      }
    };
    const dA = deferred();
    const dB = deferred();

    const pA = runGuardedFetch(guard, () => dA.promise, opts);
    const pB = runGuardedFetch(guard, () => dB.promise, opts);

    // 旧请求 A 先完成:B 仍在途,searching 必须保持 true。
    dA.resolve('A');
    await pA;
    expect(searching).toBe(true);

    dB.resolve('B');
    await pB;
    expect(searching).toBe(false);
  });

  it('suppresses errors from stale requests', async () => {
    const guard = createRequestGuard();
    const onError = vi.fn();
    const onResult = vi.fn();
    const dA = deferred();
    const dB = deferred();

    const pA = runGuardedFetch(guard, () => dA.promise, { onError });
    const pB = runGuardedFetch(guard, () => dB.promise, { onResult });

    dB.resolve('B');
    await pB;
    dA.reject(new Error('boom'));
    await pA;

    expect(onError).not.toHaveBeenCalled();
    expect(onResult).toHaveBeenCalledWith('B');
  });

  it('reports errors from the current request', async () => {
    const guard = createRequestGuard();
    const onError = vi.fn();
    const onFinally = vi.fn();
    const err = new Error('boom');

    await runGuardedFetch(guard, () => Promise.reject(err), { onError, onFinally });

    expect(onError).toHaveBeenCalledWith(err);
    expect(onFinally).toHaveBeenCalledTimes(1);
  });
});
