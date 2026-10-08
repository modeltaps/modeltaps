import { describe, expect, it, vi } from 'vitest';

import { renderAuthPage, silenceRouterLayoutEffectWarning } from 'views/auth/authPageTestUtils';
import Security from './Security';

// 登录会话卡自己拉接口,与这里的判定无关,换成空壳。
vi.mock('./SessionsCard', () => ({ default: () => null }));

// useSelfUser 走 /api/user/self,测试直接注入 inputs。
const selfUser = { inputs: {}, reloadUser: () => {} };
vi.mock('./useSelfUser', () => ({ default: () => selfUser }));

silenceRouterLayoutEffectWarning();

// 账号安全页按账号体系增减区块:
//   内置 —— 「登录验证」卡是密码 / 邮箱验证码 / 通行密钥,「账号标识」卡是邮箱 / 手机号 /
//           第三方账号小节,动作都在本站完成;
//   外部 —— 同一套行,动作跳到登录服务的设置页深链,root 多一块「管理员应急登录」。
// 任何形态下都不出现 WebAuthn / OIDC / 供应商名。

const render = (siteInfo, inputs = {}) => {
  selfUser.inputs = inputs;
  return renderAuthPage(Security, '/panel/settings', siteInfo);
};

const links = {
  slug: 'authany',
  display_name: 'Authany',
  first_party: true,
  account_settings_url: 'https://auth.example.com/settings',
  password_url: 'https://auth.example.com/settings/change_password',
  mfa_url: 'https://auth.example.com/settings/mfa',
  passkey_url: 'https://auth.example.com/settings/passkey',
  identity_url: 'https://auth.example.com/settings/identity'
};
const external = { account_system: 'external', oidc_providers: [links], server_address: 'https://modeltaps.test' };

describe('Security — 内置账号', () => {
  it('登录验证:密码、通行密钥成行,右侧只有按钮;账号标识卡可更换邮箱;没有应急区块', () => {
    const html = render({ smtp_configured: true }, { has_password: true, email: 'alice@example.test', email_verified: true, role: 1 });
    expect(html).toContain('登录验证');
    expect(html).not.toContain('登录方式');
    expect(html).toContain('>修改<');
    expect(html).toContain('通行密钥');
    expect(html).toContain('>管理<');
    expect(html).toContain('账号标识');
    expect(html).toContain('alice@example.test · 已验证');
    expect(html).toContain('>更换<');
    expect(html).not.toContain('邮箱验证码');
    expect(html).not.toContain('管理员应急登录');
    expect(html).not.toContain('WebAuthn');
    expect(html).not.toContain('target="_blank"');
  });

  it('无密码时按钮是「设置」;开启邮箱验证码登录时多一行', () => {
    const html = render({ email_code_login: true }, { has_password: false, email: 'alice@example.test' });
    expect(html).toContain('>设置<');
    expect(html).toContain('邮箱验证码');
    expect(html).not.toContain('>修改<');
  });

  it('站点关闭密码登录时普通用户没有密码行,root 仍有', () => {
    expect(render({ password_login: false }, { role: 1 })).not.toContain('>密码<');
    expect(render({ password_login: false }, { role: 100 })).toContain('>密码<');
  });

  it('开启社交登录时有「第三方账号」小节,按关联状态给关联 / 取消关联', () => {
    const html = render({ github_oauth: true, github_client_id: 'x', wechat_login: true }, { github_id: '42' });
    expect(html).toContain('第三方账号');
    expect(html).toContain('GitHub');
    expect(html).toContain('>取消关联<');
    expect(html).toContain('微信');
    expect(html).toContain('>关联<');
  });

  it('第三方账号小节在「账号标识」卡里,不在「登录验证」卡里', () => {
    const html = render({ github_oauth: true, github_client_id: 'x' }, { github_id: '42', email: 'alice@example.test' });
    // 两张卡的顺序固定「登录验证」→「账号标识」,小节落在后一张卡的标题之后就说明搬对了。
    expect(html.indexOf('第三方账号')).toBeGreaterThan(html.indexOf('账号标识'));
    expect(html.indexOf('账号标识')).toBeGreaterThan(html.indexOf('登录验证'));
    // 通行密钥是「登录验证」卡的最后一行,第三方账号必须排在它后面。
    expect(html.indexOf('第三方账号')).toBeGreaterThan(html.indexOf('通行密钥'));
  });

  it('邮箱角标按后端记下的来源走:验证过的显示「已验证」,管理员代填的显示「未验证」', () => {
    // 角标不是「有邮箱就已验证」,而是 users.email_verified —— 管理员在后台代填的邮箱没经过验证码。
    expect(render({ smtp_configured: true }, { email: 'alice@example.test', email_verified: true })).toContain(
      'alice@example.test · 已验证'
    );
    const typed = render({ smtp_configured: true }, { email: 'typed-by-admin@example.test', email_verified: false });
    expect(typed).toContain('typed-by-admin@example.test · 未验证');
    expect(typed).not.toContain('· 已验证');
  });

  it('通行密钥关闭时没有该行;SMTP 未配置时邮箱不可更换', () => {
    const html = render({ passkey_login: false, smtp_configured: false }, { email: 'a@b.c' });
    expect(html).not.toContain('通行密钥');
    expect(html).not.toContain('>更换<');
  });
});

describe('Security — 外部身份提供方', () => {
  it('同一套行,动作是跳到登录服务设置页的外链;不出现供应商名', () => {
    const html = render(external, {
      role: 1,
      email: 'alice@example.test',
      email_verified: true,
      phone_number: '+8613800000000',
      oidc_identities: [{ provider_slug: 'authany', ...links }]
    });
    expect(html).toContain('登录验证');
    expect(html).not.toContain('登录方式');
    expect(html).toContain('两步验证');
    expect(html).toContain('href="https://auth.example.com/settings/change_password"');
    expect(html).toContain('href="https://auth.example.com/settings/mfa"');
    expect(html).toContain('href="https://auth.example.com/settings/passkey"');
    expect(html).toContain('href="https://auth.example.com/settings/identity"');
    // 第三方账号行已搬进「账号标识」卡:在卡标题之后,且排在手机号之后。
    expect(html.indexOf('第三方账号')).toBeGreaterThan(html.indexOf('账号标识'));
    expect(html.indexOf('第三方账号')).toBeGreaterThan(html.indexOf('>手机号<'));
    // 「登录验证」卡只剩密码 / 两步验证 / 通行密钥三行。
    expect(html.indexOf('第三方账号')).toBeGreaterThan(html.indexOf('>通行密钥<'));
    expect(html).toContain('target="_blank"');
    expect(html).toContain('修改后会在你的所有设备上生效');
    expect(html).toContain('账号标识');
    expect(html).toContain('来自登录服务，每次登录时更新。用其中任何一个登录，进的都是同一个账号。');
    expect(html).toContain('+8613800000000 · 已验证');
    expect(html).not.toContain('Authany');
    expect(html).not.toContain('OIDC');
    expect(html).not.toContain('Modeltaps 账号');
    expect(html).not.toContain('管理员应急登录');
  });

  it('账号标识为空时动作是「添加」而不是「更换」,仍指向身份页深链', () => {
    const html = render(external, { role: 1, oidc_identities: [{ provider_slug: 'authany', ...links }] });
    expect(html).toContain('账号标识');
    expect(html).toContain('>手机号<');
    expect(html).toContain('未设置');
    expect(html).toContain('>添加<');
    expect(html).not.toContain('>更换<');
    expect(html).toContain('href="https://auth.example.com/settings/identity"');
    expect(html).toContain('第三方账号');
  });

  it('后台只填了账号设置页时只剩一行「账号设置」', () => {
    const onlySettings = {
      ...external,
      oidc_providers: [{ slug: 'authany', first_party: true, account_settings_url: 'https://auth.example.com/settings' }]
    };
    const html = render(onlySettings, { role: 1 });
    expect(html).toContain('账号设置');
    expect(html).toContain('href="https://auth.example.com/settings"');
    expect(html).not.toContain('href="https://auth.example.com/settings/mfa"');
    // 没有身份页深链时第三方账号行退回账号设置页。
    expect(html).toContain('第三方账号');
    expect(html).toContain('登录凭据由登录服务保管');
    expect(html).not.toContain('修改后会在你的所有设备上生效');
  });

  it('深链全空但有 issuer 时退到登录服务站点根地址', () => {
    const onlyIssuer = {
      ...external,
      oidc_providers: [{ slug: 'authany', first_party: true, issuer: 'https://auth.example.com/realms/x' }]
    };
    const html = render(onlyIssuer, { role: 1 });
    expect(html).toContain('账号设置');
    expect(html).toContain('href="https://auth.example.com"');
    expect(html).toContain('前往登录服务的账号页管理');
    expect(html).toContain('登录凭据由登录服务保管');
    expect(html).not.toContain('由登录服务统一管理');
  });

  it('深链与 issuer 都没有时只有一句说明', () => {
    const none = render({ ...external, oidc_providers: [{ slug: 'authany', first_party: true }] }, { role: 1 });
    expect(none).toContain('由登录服务统一管理');
    expect(none).not.toContain('第三方账号');
    expect(none).toContain('登录凭据由登录服务保管');
    expect(none).not.toContain('target="_blank"');
  });

  it('issuer 不是 http(s) 地址时不渲染兜底行', () => {
    const bad = render({ ...external, oidc_providers: [{ slug: 'authany', first_party: true, issuer: 'not-a-url' }] }, { role: 1 });
    expect(bad).toContain('由登录服务统一管理');
  });

  it('没有身份行时(如 root 用密码登录)退回站点下发的提供方深链', () => {
    const html = render(external, { role: 1 });
    expect(html).toContain('href="https://auth.example.com/settings/change_password"');
  });

  it('root 多一块「管理员应急登录」,含 /login/admin 地址、应急密码与应急通行密钥;关闭应急登录时没有', () => {
    const html = render(external, { role: 100, has_password: true });
    expect(html).toContain('管理员应急登录');
    expect(html).toContain('https://modeltaps.test/login/admin');
    expect(html).toContain('应急密码');
    expect(html).toContain('>轮换<');
    expect(html).toContain('应急通行密钥');
    expect(render({ ...external, admin_login_enabled: false }, { role: 100 })).not.toContain('管理员应急登录');
  });
});
