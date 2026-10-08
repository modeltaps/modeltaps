import { describe, expect, it } from 'vitest';

import { GATE_CHAT_DISABLED, GATE_ORG, resolveConsoleGate } from './useConsoleGate';

// 门禁三种输入:个人上下文且试用区开启 → 不拦;组织上下文 → 拦(去向个人上下文);
// 站点关闭内置试用区 → 拦(去向接口示例)。

describe('resolveConsoleGate', () => {
  it('个人上下文 + 试用区开启:不拦', () => {
    expect(resolveConsoleGate({ orgActive: false, chatEnabled: true })).toEqual({ gated: false, reason: null, actionHref: null });
  });

  it('组织上下文:拦运行,给回个人上下文的去向', () => {
    expect(resolveConsoleGate({ orgActive: true, chatEnabled: true })).toEqual({
      gated: true,
      reason: GATE_ORG,
      actionHref: '/panel/organization'
    });
  });

  it('站点关闭内置试用区:拦运行,给接口示例的去向', () => {
    expect(resolveConsoleGate({ orgActive: false, chatEnabled: false })).toEqual({
      gated: true,
      reason: GATE_CHAT_DISABLED,
      actionHref: '/panel/api/docs/chat'
    });
  });

  it('两者同时命中时组织上下文优先', () => {
    expect(resolveConsoleGate({ orgActive: true, chatEnabled: false }).reason).toBe(GATE_ORG);
  });

  it('缺省参数按「不拦」处理', () => {
    expect(resolveConsoleGate().gated).toBe(false);
  });
});
