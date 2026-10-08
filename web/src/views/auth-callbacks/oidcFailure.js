// 回调失败的两个判断:后端错误码 → 前端文案 key;以及失败后回本站登录页还有没有意义(UX-35)。

import { isExternalAccountSystem } from 'utils/authAvailability';

// 后端 controller/oidc.go 的错误码契约:`<CODE>: 原文`。冒号后的原文可能来自旧版本、带供应商名,
// 一律不展示,只用前端文案。
const FAILURE_KEYS = {
  'OIDC_LINK_CONFLICT:': 'login.oidcLinkConflict',
  'OIDC_REGISTER_DISABLED:': 'login.oidcRegisterDisabled',
  'OIDC_REGISTER_CLOSED:': 'login.oidcRegisterClosed'
};

export const oidcFailureKey = (message) => {
  if (typeof message !== 'string') return '';
  const prefix = Object.keys(FAILURE_KEYS).find((item) => message.startsWith(item));
  return prefix ? FAILURE_KEYS[prefix] : '';
};

// 外部账号体系下 /login 直接送回身份提供方,回落过去就是死循环;回调页必须自己给出口。
// slug 参数保留给旧调用方,判定只看账号体系。
export const localLoginIsDeadEnd = (siteInfo) => isExternalAccountSystem(siteInfo);

// 失败终态的出口:死路时停在回调页给「重新登录 / 返回首页」('retry'),否则回本站登录页('login')。
// 授权失败与用户取消邀请码走同一判定,避免只有一处收口。
export const oidcFailureExit = (siteInfo, slug) => (localLoginIsDeadEnd(siteInfo, slug) ? 'retry' : 'login');
