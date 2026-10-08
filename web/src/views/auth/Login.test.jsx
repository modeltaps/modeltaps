import { describe, expect, it, vi } from 'vitest';

import { renderAuthPage, silenceRouterLayoutEffectWarning } from './authPageTestUtils';
import Login from './Login';

vi.mock('@/components/auth/AuthShell', async () => {
  const { createElement: h } = await import('react');
  return { default: ({ title, children }) => h('section', { 'data-title': title }, children) };
});

silenceRouterLayoutEffectWarning();

// 登录页只有一套骨架,按 /api/status 的账号体系与有效开关增减段落:
//   builtin  —— 社交登录按钮 → 本站表单(密码 / 邮箱验证码)→ 通行密钥 → 注册;
//   external —— 不画任何登录框,直达身份提供方;提供方没配好时只给提示与 root 应急入口。

const render = (siteInfo, path = '/login') => renderAuthPage(Login, path, siteInfo);

const AUTH_SHELL = 'data-title';
const SPINNER = 'animate-spin';
const USERNAME_FIELD = 'id="username"';
const PASSWORD_FIELD = 'id="password"';
const EMAIL_FIELD = 'id="email"';
const CODE_FIELD = 'id="code"';
const FORGOT_LINK = 'href="/forgot-password"';
const REGISTER_LINK = 'href="/register"';
const ADMIN_LINK = 'href="/login/admin"';
const PASSKEY_BUTTON = '使用通行密钥登录';
const EMAIL_CODE_LINK = '用邮箱验证码登录';
const GITHUB_BUTTON = '使用 Github 登录';

describe('Login — 内置账号(旧后端缺 key 也按它)', () => {
  it('默认渲染邮箱 / 用户名 + 密码表单、忘记密码、通行密钥与注册入口', () => {
    const html = render({});
    expect(html).toContain(AUTH_SHELL);
    expect(html).toContain('<form');
    expect(html).toContain(USERNAME_FIELD);
    expect(html).toContain(PASSWORD_FIELD);
    expect(html).toContain(FORGOT_LINK);
    expect(html).toContain(PASSKEY_BUTTON);
    expect(html).toContain(REGISTER_LINK);
    expect(html).not.toContain(EMAIL_CODE_LINK);
    expect(html).not.toContain(SPINNER);
  });

  it('开启社交登录时按钮在表单之前,按钮上只有第三方账号名', () => {
    const html = render({ github_oauth: true, github_client_id: 'x' });
    expect(html).toContain(GITHUB_BUTTON);
    expect(html.indexOf(GITHUB_BUTTON)).toBeLessThan(html.indexOf(USERNAME_FIELD));
  });

  it('开启邮箱验证码登录时给出切换入口,默认仍是密码', () => {
    const html = render({ email_code_login: true });
    expect(html).toContain(EMAIL_CODE_LINK);
    expect(html).toContain(PASSWORD_FIELD);
    expect(html).not.toContain(CODE_FIELD);
  });

  it('密码登录关闭、邮箱验证码开启:直接落到邮箱 + 验证码表单,没有忘记密码', () => {
    const html = render({ password_login: false, email_code_login: true });
    expect(html).toContain(EMAIL_FIELD);
    expect(html).toContain(CODE_FIELD);
    expect(html).not.toContain(PASSWORD_FIELD);
    expect(html).not.toContain(FORGOT_LINK);
    expect(html).not.toContain(EMAIL_CODE_LINK);
  });

  it('密码与验证码都关闭:没有表单,只剩社交登录与通行密钥', () => {
    const html = render({ password_login: false, github_oauth: true, github_client_id: 'x' });
    expect(html).not.toContain('<form');
    expect(html).toContain(GITHUB_BUTTON);
    expect(html).toContain(PASSKEY_BUTTON);
  });

  it('通行密钥关闭时没有该按钮;密码注册关闭时没有注册入口', () => {
    const html = render({ passkey_login: false, password_register: false });
    expect(html).not.toContain(PASSKEY_BUTTON);
    expect(html).not.toContain(REGISTER_LINK);
    expect(html).toContain(PASSWORD_FIELD);
  });

  it('内置模式下提供方不是登录按钮:哪怕后端还下发了提供方,也不出现供应商名', () => {
    const html = render({ oidc_providers: [{ slug: 'authany', display_name: 'Authany', first_party: true }] });
    expect(html).not.toContain('Authany');
    expect(html).not.toContain('authany');
    expect(html).toContain(PASSWORD_FIELD);
  });

  it('?local=1 并入 /login/admin(Navigate 不产出内容)', () => {
    const html = render({}, '/login?local=1');
    expect(html).not.toContain(AUTH_SHELL);
    expect(html).not.toContain('<form');
  });
});

describe('Login — 站点状态还没回来', () => {
  it('只转圈:不画本站表单(外部模式下那会先闪一帧),也没有标题', () => {
    const html = render(undefined);
    expect(html).toContain(SPINNER);
    expect(html).not.toContain(AUTH_SHELL);
    expect(html).not.toContain('<form');
    expect(html).not.toContain(USERNAME_FIELD);
    expect(html).not.toContain(PASSWORD_FIELD);
    expect(html).not.toContain(PASSKEY_BUTTON);
  });

  it('?local=1 的并入判定仍排在转圈之前', () => {
    const html = render(undefined, '/login?local=1');
    expect(html).not.toContain(SPINNER);
    expect(html).not.toContain(AUTH_SHELL);
  });
});

describe('Login — 外部身份提供方', () => {
  const external = { account_system: 'external', oidc_providers: [{ slug: 'authany', display_name: 'Authany', first_party: true }] };

  it('提供方配好时只渲染转圈:没有标题、表单、按钮,也不露出供应商名', () => {
    const html = render(external);
    expect(html).toContain(SPINNER);
    expect(html).not.toContain(AUTH_SHELL);
    expect(html).not.toContain('<form');
    expect(html).not.toContain('Authany');
    expect(html).not.toContain(PASSKEY_BUTTON);
    expect(html).not.toContain('登录');
  });

  it('提供方没配好:只给提示与管理员应急入口,不画登录框', () => {
    const html = render({ account_system: 'external', oidc_providers: [] });
    expect(html).toContain(AUTH_SHELL);
    expect(html).toContain('登录服务尚未配置');
    expect(html).toContain(ADMIN_LINK);
    expect(html).not.toContain('<form');
    expect(html).not.toContain(PASSKEY_BUTTON);
  });

  it('后台关闭了应急登录时提示里没有管理员入口', () => {
    const html = render({ account_system: 'external', admin_login_enabled: false });
    expect(html).toContain('登录服务尚未配置');
    expect(html).not.toContain(ADMIN_LINK);
  });

  it('外部模式下后端漏发的开关也不会画出本站表单', () => {
    const html = render({ ...external, oidc_providers: [], password_login: true, github_oauth: true, email_code_login: true });
    expect(html).not.toContain('<form');
    expect(html).not.toContain(GITHUB_BUTTON);
  });
});
