import { describe, expect, it, vi } from 'vitest';

import { renderAuthPage, silenceRouterLayoutEffectWarning } from './authPageTestUtils';
import AdminLogin from './AdminLogin';

vi.mock('@/components/auth/AuthShell', async () => {
  const { createElement: h } = await import('react');
  return { default: ({ title, children }) => h('section', { 'data-title': title }, children) };
});

silenceRouterLayoutEffectWarning();

// /login/admin:外部账号体系下 root 的应急入口,只走本站密码或通行密钥;
// 内置模式或后台关闭应急登录时并入 /login。

const render = (siteInfo) => renderAuthPage(AdminLogin, '/login/admin', siteInfo);

describe('AdminLogin', () => {
  it('外部模式:标题「管理员登录」+ 说明,账号密码表单默认展开,通行密钥为次要入口,底部只有返回登录', () => {
    const html = render({ account_system: 'external' });
    expect(html).toContain('data-title="管理员登录"');
    expect(html).toContain('此入口不依赖外部登录服务');
    expect(html).toContain('id="username"');
    expect(html).toContain('id="password"');
    expect(html).toContain('type="submit"');
    expect(html).toContain('使用通行密钥登录');
    expect(html).toContain('href="/login"');
    expect(html).not.toContain('href="/register"');
    expect(html).not.toContain('href="/forgot-password"');
    expect(html).not.toContain('Authany');
  });

  it('内置模式或关闭了应急登录:并入 /login(不产出内容)', () => {
    expect(render({})).not.toContain('data-title');
    expect(render({ account_system: 'external', admin_login_enabled: false })).not.toContain('data-title');
  });

  it('站点状态还没回来:只转圈,不跳去 /login(那会被 /login 再直达身份提供方)', () => {
    const html = render(undefined);
    expect(html).toContain('animate-spin');
    expect(html).not.toContain('data-title');
    expect(html).not.toContain('<form');
  });
});
