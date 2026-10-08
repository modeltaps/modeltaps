import { describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { initialState } from 'store/siteInfoReducer';
import { localLoginIsDeadEnd, oidcFailureExit, oidcFailureKey } from './oidcFailure';

// 后端错误码 → 前端文案 key。冒号后的原文(旧后端带供应商名)不参与展示。
describe('oidcFailureKey', () => {
  it('三个错误码各自映射到前端文案 key', () => {
    expect(oidcFailureKey('OIDC_LINK_CONFLICT: 邮箱 a@b.c 已绑定 Authany 的另一身份')).toBe('login.oidcLinkConflict');
    expect(oidcFailureKey('OIDC_REGISTER_DISABLED: 该提供方不允许首登建号')).toBe('login.oidcRegisterDisabled');
    expect(oidcFailureKey('OIDC_REGISTER_CLOSED: 管理员关闭了新用户注册')).toBe('login.oidcRegisterClosed');
  });

  it('无前缀 / 空 / 非字符串走通用失败分支', () => {
    expect(oidcFailureKey('管理员关闭了新用户注册')).toBe('');
    expect(oidcFailureKey('')).toBe('');
    expect(oidcFailureKey(undefined)).toBe('');
    expect(oidcFailureKey(null)).toBe('');
  });
});

// 外部账号体系下 /login 直接送回身份提供方,回落过去就是死循环;内置账号下回本站登录页有意义。
describe('localLoginIsDeadEnd / oidcFailureExit', () => {
  const external = { account_system: 'external', oidc_providers: [{ slug: 'authany', first_party: true }] };

  it('外部模式为死路,停在回调页', () => {
    expect(localLoginIsDeadEnd(external, 'authany')).toBe(true);
    expect(oidcFailureExit(external, 'authany')).toBe('retry');
  });

  it('内置模式(含旧后端缺 key)不算死路,回本站登录页', () => {
    expect(localLoginIsDeadEnd({ password_login: false, oidc_providers: external.oidc_providers }, 'authany')).toBe(false);
    expect(localLoginIsDeadEnd(undefined, 'authany')).toBe(false);
    expect(oidcFailureExit({}, 'authany')).toBe('login');
    expect(oidcFailureExit(undefined, 'authany')).toBe('login');
  });

  // 站点状态未加载时算出来的是「去登录」,回调页必须等状态回来后按外部模式改判为 retry,
  // 否则外部模式下点「去登录」会被 /login 原样送回 IdP,再次撞回同一堵墙。
  it('状态未加载时算出的出口不可用,加载后按外部模式改判为 retry', () => {
    expect(oidcFailureExit(initialState, 'authany')).toBe('login');
    expect(oidcFailureExit({ ...initialState, ...external, isLoading: false }, 'authany')).toBe('retry');
  });
});

// 回调页任何状态下都不露出「OIDC」/ 供应商名,失败文案统一指向「联系管理员」。
describe('回调页失败文案', () => {
  const keys = [
    'login.oidcLinkConflict',
    'login.oidcRegisterDisabled',
    'login.oidcRegisterClosed',
    'login.oidcError',
    'login.signInFailed',
    'login.retryLogin',
    'login.backHome',
    'oauthInvite.cancelled'
  ];
  const locales = ['zh_CN', 'zh_HK', 'en_US', 'ja_JP'];

  it('四语言 key 齐全', () => {
    for (const lng of locales) {
      for (const key of keys) {
        expect(i18n.getFixedT(lng)(key), `${lng}/${key}`).not.toBe(key);
      }
    }
  });

  it('不含 OIDC / 供应商名 / 「原方式登录」措辞', () => {
    for (const lng of locales) {
      const t = i18n.getFixedT(lng);
      for (const key of keys) {
        const text = t(key);
        expect(text, `${lng}/${key}`).not.toMatch(/OIDC|Authany/i);
        expect(text, `${lng}/${key}`).not.toMatch(/原方式|原有方式|重定向/);
      }
    }
  });
});
