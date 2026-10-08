import { describe, expect, it, vi } from 'vitest';

import { renderAuthPage, silenceRouterLayoutEffectWarning } from './authPageTestUtils';
import Register from './Register';

vi.mock('@/components/auth/AuthShell', async () => {
  const { createElement: h } = await import('react');
  return { default: ({ title, children }) => h('section', { 'data-title': title }, children) };
});

silenceRouterLayoutEffectWarning();

// 直接访问 /register 时按 password_register 优雅降级(UX-33):
// 关闭 → 不渲染注册表单,改为提示 + 社交登录按钮(无分隔线)+ 「登录」链接。
// 外部账号体系下注册在身份提供方完成,/register 并入 /login。

const github = { github_oauth: true, github_client_id: 'x' };
const render = (siteInfo) => renderAuthPage(Register, '/register', siteInfo);

const USERNAME_FIELD = 'id="username"';
const PASSWORD_FIELD = 'id="password"';
const LOGIN_LINK = 'href="/login"';
const OR_DIVIDER = 'uppercase tracking-wide';
const SOCIAL_BUTTON = '使用 Github 登录';
const DISABLED_NOTICE = '管理员关闭了通过密码注册';

describe('Register — password_register 缺失或为 true', () => {
  it('key 缺失(旧后端)时渲染注册表单', () => {
    const html = render({});
    expect(html).toContain('<form');
    expect(html).toContain(USERNAME_FIELD);
    expect(html).toContain(PASSWORD_FIELD);
    expect(html).toContain(LOGIN_LINK);
    expect(html).not.toContain(DISABLED_NOTICE);
  });

  it('true 时渲染注册表单', () => {
    const html = render({ password_register: true });
    expect(html).toContain('<form');
    expect(html).not.toContain(DISABLED_NOTICE);
  });
});

describe('Register — 密码注册关闭', () => {
  const siteInfo = { password_register: false, ...github };

  it('不渲染注册表单', () => {
    const html = render(siteInfo);
    expect(html).not.toContain('<form');
    expect(html).not.toContain(USERNAME_FIELD);
    expect(html).not.toContain(PASSWORD_FIELD);
  });

  it('显示关闭提示,并保留「登录」链接', () => {
    const html = render(siteInfo);
    expect(html).toContain(DISABLED_NOTICE);
    expect(html).toContain(LOGIN_LINK);
  });

  it('提供社交登录按钮,且没有分隔线', () => {
    const html = render(siteInfo);
    expect(html).toContain(SOCIAL_BUTTON);
    expect(html).not.toContain(OR_DIVIDER);
  });

  it('没有第三方登录时也只显示提示与「登录」链接', () => {
    const html = render({ password_register: false });
    expect(html).toContain(DISABLED_NOTICE);
    expect(html).toContain(LOGIN_LINK);
    expect(html).not.toContain('<form');
  });
});

// 外部账号体系下直访 /register 直接并入 /login(那里再跳去身份提供方):
// 静态渲染下 <Navigate> 不产出内容,断言既没有降级提示也没有表单。
describe('Register — 外部身份提供方', () => {
  it('不渲染降级提示与注册表单', () => {
    const html = render({ account_system: 'external', oidc_providers: [{ slug: 'authany', display_name: 'Authany', first_party: true }] });
    expect(html).not.toContain(DISABLED_NOTICE);
    expect(html).not.toContain('<form');
    expect(html).not.toContain('Authany');
  });

  it('内置模式下密码注册关闭仍渲染降级提示', () => {
    const html = render({ password_register: false, password_login: false });
    expect(html).toContain(DISABLED_NOTICE);
  });
});
