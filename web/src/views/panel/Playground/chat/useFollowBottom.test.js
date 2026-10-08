import { describe, expect, it } from 'vitest';

import { THRESHOLD, watchFollow } from './useFollowBottom';

// 窗口缩放后的贴底：跟随态下容器高度变化会重新滚到底，手动上翻后不拉回。
// 测试跑在 node 环境，用假的滚动容器与 ResizeObserver。

const fakeContainer = ({ scrollHeight = 1000, clientHeight = 400, scrollTop = 600 } = {}) => {
  const listeners = {};
  return {
    scrollHeight,
    clientHeight,
    scrollTop,
    addEventListener: (type, fn) => {
      listeners[type] = fn;
    },
    removeEventListener: (type) => {
      delete listeners[type];
    },
    fire: (type) => listeners[type]?.(),
    listeners
  };
};

const fakeObserverClass = () => {
  const instances = [];
  class FakeObserver {
    constructor(callback) {
      this.callback = callback;
      this.disconnected = false;
      instances.push(this);
    }
    observe(el) {
      this.el = el;
    }
    disconnect() {
      this.disconnected = true;
    }
  }
  return { FakeObserver, instances };
};

describe('watchFollow', () => {
  it('跟随态下容器变矮后重新贴底', () => {
    const el = fakeContainer();
    const following = { current: true };
    const { FakeObserver, instances } = fakeObserverClass();
    watchFollow(el, following, FakeObserver);

    el.clientHeight = 300;
    instances[0].callback();
    expect(el.scrollTop).toBe(el.scrollHeight);
  });

  it('手动上翻超过阈值后，缩放不再拉回', () => {
    const el = fakeContainer();
    const following = { current: true };
    const { FakeObserver, instances } = fakeObserverClass();
    watchFollow(el, following, FakeObserver);

    el.scrollTop = 600 - THRESHOLD - 50;
    el.fire('scroll');
    expect(following.current).toBe(false);

    const before = el.scrollTop;
    el.clientHeight = 300;
    instances[0].callback();
    expect(el.scrollTop).toBe(before);
  });

  it('清理时摘掉 scroll 监听并断开观察', () => {
    const el = fakeContainer();
    const { FakeObserver, instances } = fakeObserverClass();
    const cleanup = watchFollow(el, { current: true }, FakeObserver);
    cleanup();
    expect(el.listeners.scroll).toBeUndefined();
    expect(instances[0].disconnected).toBe(true);
  });

  it('没有 ResizeObserver 的环境照常挂 scroll 监听', () => {
    const el = fakeContainer();
    const cleanup = watchFollow(el, { current: true }, undefined);
    expect(el.listeners.scroll).toBeTypeOf('function');
    cleanup();
  });
});
