import { describe, expect, it } from 'vitest';

import {
  accountSystem,
  adminLoginEnabled,
  emailCodeLoginEnabled,
  externalProvider,
  isExternalAccountSystem,
  oidcAutoRedirectTarget,
  passkeyLoginEnabled,
  passwordLoginEnabled,
  passwordRegisterEnabled,
  siteInfoLoaded,
  socialLoginEnabled
} from './authAvailability';
import { initialState } from 'store/siteInfoReducer';

// 账号体系二选一:builtin(旧后端缺 key 时也按它)与 external;各入口的可用性都由它推导。

describe('siteInfoLoaded', () => {
  it('只有 SET_SITE_INFO 写过(isLoading:false)才算已加载', () => {
    expect(siteInfoLoaded({ isLoading: false })).toBe(true);
    expect(siteInfoLoaded(initialState)).toBe(false);
    expect(siteInfoLoaded({})).toBe(false);
    expect(siteInfoLoaded(undefined)).toBe(false);
  });

  it('未加载时各入口判定都落在内置模式的默认值上,跳转必须等它', () => {
    expect(isExternalAccountSystem(initialState)).toBe(false);
    expect(adminLoginEnabled(initialState)).toBe(false);
  });
});

describe('accountSystem', () => {
  it('明确下发 external 才是外部模式,其余(含缺失 / 未加载)都是内置', () => {
    expect(accountSystem({ account_system: 'external' })).toBe('external');
    expect(accountSystem({ account_system: 'builtin' })).toBe('builtin');
    expect(accountSystem({})).toBe('builtin');
    expect(accountSystem(undefined)).toBe('builtin');
    expect(isExternalAccountSystem({ account_system: 'external' })).toBe(true);
    expect(isExternalAccountSystem(null)).toBe(false);
  });
});

describe('passwordLoginEnabled / passwordRegisterEnabled', () => {
  it('内置模式下后端明确下发 false 时关闭,缺失视为开启', () => {
    expect(passwordLoginEnabled({ password_login: false })).toBe(false);
    expect(passwordLoginEnabled({ password_login: true })).toBe(true);
    expect(passwordLoginEnabled({})).toBe(true);
    expect(passwordLoginEnabled(undefined)).toBe(true);
    expect(passwordRegisterEnabled({ password_register: false })).toBe(false);
    expect(passwordRegisterEnabled({})).toBe(true);
  });

  it('两个开关互不影响', () => {
    expect(passwordRegisterEnabled({ password_login: false })).toBe(true);
    expect(passwordLoginEnabled({ password_register: false })).toBe(true);
  });

  it('外部模式下一律关闭,哪怕后端漏了有效值', () => {
    expect(passwordLoginEnabled({ account_system: 'external', password_login: true })).toBe(false);
    expect(passwordRegisterEnabled({ account_system: 'external' })).toBe(false);
  });
});

describe('emailCodeLoginEnabled / passkeyLoginEnabled', () => {
  it('邮箱验证码是新增能力:只有明确下发 true 才开', () => {
    expect(emailCodeLoginEnabled({ email_code_login: true })).toBe(true);
    expect(emailCodeLoginEnabled({})).toBe(false);
    expect(emailCodeLoginEnabled({ account_system: 'external', email_code_login: true })).toBe(false);
  });

  it('通行密钥缺失视为开启,外部模式下对普通用户关闭', () => {
    expect(passkeyLoginEnabled({})).toBe(true);
    expect(passkeyLoginEnabled({ passkey_login: false })).toBe(false);
    expect(passkeyLoginEnabled({ account_system: 'external' })).toBe(false);
  });
});

describe('socialLoginEnabled', () => {
  it('内置模式下任一社交登录开启即为真;外部模式下恒假', () => {
    expect(socialLoginEnabled({ github_oauth: true })).toBe(true);
    expect(socialLoginEnabled({ lark_login: true })).toBe(true);
    expect(socialLoginEnabled({})).toBe(false);
    expect(socialLoginEnabled({ account_system: 'external', github_oauth: true })).toBe(false);
  });
});

describe('externalProvider / oidcAutoRedirectTarget / adminLoginEnabled', () => {
  const authany = [{ slug: 'authany', display_name: 'Authany', first_party: true }];

  it('外部模式且提供方配好时直达该 slug', () => {
    expect(externalProvider({ account_system: 'external', oidc_providers: authany })?.slug).toBe('authany');
    expect(oidcAutoRedirectTarget({ account_system: 'external', oidc_providers: authany })).toBe('authany');
  });

  it('内置模式下提供方不是登录入口,永不直达', () => {
    expect(externalProvider({ oidc_providers: authany })).toBeNull();
    expect(oidcAutoRedirectTarget({ oidc_auto_redirect: true, password_login: false, oidc_providers: authany })).toBe('');
  });

  it('外部模式但提供方没配好时返回空,登录页改为提示', () => {
    expect(oidcAutoRedirectTarget({ account_system: 'external', oidc_providers: [] })).toBe('');
    expect(oidcAutoRedirectTarget({ account_system: 'external' })).toBe('');
    expect(oidcAutoRedirectTarget(undefined)).toBe('');
  });

  it('/login/admin 只在外部模式下存在,且可被后台关闭', () => {
    expect(adminLoginEnabled({ account_system: 'external' })).toBe(true);
    expect(adminLoginEnabled({ account_system: 'external', admin_login_enabled: false })).toBe(false);
    expect(adminLoginEnabled({ admin_login_enabled: true })).toBe(false);
  });
});
