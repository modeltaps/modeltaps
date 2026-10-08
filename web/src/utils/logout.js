import config from 'config';
import { API } from 'utils/api';
import { uiLocalesQuery } from 'i18n/uiLocale';

// 退出登录的落地页：公开路由，后端也把它登记为 IdP 的 post_logout_redirect_uri。
export const SIGNED_OUT_PATH = '/signed-out';

// 整页跳转用的落地地址：带上路由 basename（basename 为 '/' 时即 /signed-out）。
export const signedOutUrl = () => `${(config.basename || '').replace(/\/+$/, '')}${SIGNED_OUT_PATH}`;

// 调后端登出接口，返回要整页跳转过去的 IdP 结束会话地址。
// 后端在 data.redirect_url 下发：站点没有 IdP 会话（如密码登录站点）时为空串，此时前端自己落 /signed-out。
// 接口失败也必须让用户退出本站，所以异常一律按「无外跳」处理。
// 带上当前界面语言（ui_locales），让 IdP 侧若出现退出确认页与本站语言一致。
export const requestLogout = async () => {
  try {
    const res = await API.get('/api/user/logout' + uiLocalesQuery());
    const url = res?.data?.data?.redirect_url;
    return typeof url === 'string' ? url : '';
  } catch {
    return '';
  }
};
