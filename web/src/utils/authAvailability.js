// 登录相关入口的可用性判定,全部以 /api/status 下发的「有效值」为准。
// 账号体系二选一(account_system):
//   builtin  —— 本站表单(邮箱 / 用户名 + 密码、邮箱验证码、通行密钥)+ 社交登录;
//   external —— /login 直达身份提供方,本站不渲染任何登录表单,只保留 /login/admin 给 root 应急。
// 旧后端没有 account_system 时按 builtin 处理,password_login / password_register 缺失视为开启。

// /api/status 回来之前所有开关都是初值,据此跳转会把逃生路径判错(见 AdminLogin / OAuthCallback);
// SET_SITE_INFO 写入 isLoading:false,就用它当「已加载」。
export const siteInfoLoaded = (siteInfo) => siteInfo?.isLoading === false;

export const accountSystem = (siteInfo) => (siteInfo?.account_system === 'external' ? 'external' : 'builtin');

export const isExternalAccountSystem = (siteInfo) => accountSystem(siteInfo) === 'external';

// 外部模式下承担登录的那一个提供方(后端只会下发这一个);没有配好时返回 null。
export const externalProvider = (siteInfo) => {
  if (!isExternalAccountSystem(siteInfo)) return null;
  const providers = Array.isArray(siteInfo?.oidc_providers) ? siteInfo.oidc_providers : [];
  return providers.find((provider) => provider?.slug) || null;
};

export const passwordLoginEnabled = (siteInfo) => !isExternalAccountSystem(siteInfo) && siteInfo?.password_login !== false;

export const passwordRegisterEnabled = (siteInfo) => !isExternalAccountSystem(siteInfo) && siteInfo?.password_register !== false;

// 邮箱验证码登录是新增能力,后端明确下发 true 才显示入口。
export const emailCodeLoginEnabled = (siteInfo) => !isExternalAccountSystem(siteInfo) && siteInfo?.email_code_login === true;

export const passkeyLoginEnabled = (siteInfo) => !isExternalAccountSystem(siteInfo) && siteInfo?.passkey_login !== false;

// /login/admin 只在外部模式下存在。
export const adminLoginEnabled = (siteInfo) => isExternalAccountSystem(siteInfo) && siteInfo?.admin_login_enabled !== false;

// 社交登录(GitHub / 微信 / 飞书 / LinuxDo)只在内置模式下由本站渲染。
export const socialLoginEnabled = (siteInfo) =>
  !isExternalAccountSystem(siteInfo) &&
  Boolean(siteInfo?.github_oauth || siteInfo?.wechat_login || siteInfo?.lark_login || siteInfo?.linuxDo_oauth);

// 登录直达:外部模式且提供方配好时返回要跳去的 slug,否则返回空串(登录页照常渲染)。
export const oidcAutoRedirectTarget = (siteInfo) => externalProvider(siteInfo)?.slug || '';
