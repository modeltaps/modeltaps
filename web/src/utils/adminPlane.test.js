import { beforeEach, describe, expect, it } from 'vitest';

import { clearReturnPath, getUserReturnPath, saveUserReturnPath } from './adminPlane';

// 管理面的「返回工作台」落点:记的是最近一次离开工作台面的路径,设置面路径不算来源,
// 所以从设置面进管理后台再返回绝不会落回 /panel/settings/*。
// 测试跑在 node 环境,自备一份 sessionStorage 替身。

const memoryStorage = () => {
  const store = new Map();
  return {
    getItem: (key) => (store.has(key) ? store.get(key) : null),
    setItem: (key, value) => store.set(key, String(value)),
    removeItem: (key) => store.delete(key)
  };
};

describe('getUserReturnPath', () => {
  beforeEach(() => {
    globalThis.sessionStorage = memoryStorage();
    clearReturnPath('admin');
  });

  it('记住进入管理后台前所在的工作台路径', () => {
    saveUserReturnPath('/panel/token');
    expect(getUserReturnPath()).toBe('/panel/token');
  });

  it('从设置面进管理后台:不记设置路径,没有工作台来源就回落首页', () => {
    saveUserReturnPath('/panel/settings/account/security');
    expect(getUserReturnPath()).toBe('/panel/dashboard');
  });

  it('从设置面进管理后台:保留上一次的工作台来源', () => {
    saveUserReturnPath('/panel/token');
    saveUserReturnPath('/panel/settings/org/1/members');
    expect(getUserReturnPath()).toBe('/panel/token');
  });
});
