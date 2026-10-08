import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { Provider } from 'react-redux';
import { MemoryRouter } from 'react-router';
import { legacy_createStore as createStore } from 'redux';
import { describe, expect, it } from 'vitest';

import reducer from 'store/reducer';
import { LOGIN, SET_SITE_INFO } from 'store/actions';
import { UserContext } from 'contexts/UserContext';
import 'i18n/i18n';

import { silenceRouterLayoutEffectWarning } from './authPageTestUtils';
import SignedOut from './SignedOut';

silenceRouterLayoutEffectWarning();

// 已登出页:退出登录(以及 IdP 结束会话后回跳)的落点。会话已清 → 「你已退出登录」+「重新登录」;
// 用回退键回到本页而会话仍在 → 「你仍在登录状态」+「进入控制台」。全程不出现 OIDC / 供应商名。

const render = ({ siteInfo = {}, user = null, isUserLoaded = true } = {}) => {
  const store = createStore(reducer);
  store.dispatch({ type: SET_SITE_INFO, payload: siteInfo });
  if (user) store.dispatch({ type: LOGIN, payload: user });
  return renderToStaticMarkup(
    createElement(
      Provider,
      { store },
      createElement(
        MemoryRouter,
        { initialEntries: ['/signed-out'] },
        createElement(UserContext.Provider, { value: { isUserLoaded } }, createElement(SignedOut))
      )
    )
  );
};

describe('SignedOut — 已退出登录', () => {
  it('渲染「你已退出登录」与「重新登录」,不提进入控制台', () => {
    const html = render();
    expect(html).toContain('你已退出登录');
    expect(html).toContain('重新登录');
    expect(html).not.toContain('你仍在登录状态');
    expect(html).not.toContain('进入控制台');
  });

  it('说明文案带站点名,并给「返回首页」次级出口', () => {
    const html = render({ siteInfo: { system_name: 'Modeltaps' } });
    expect(html).toContain('你已安全退出 Modeltaps');
    expect(html).toContain('href="/"');
    expect(html).toContain('返回首页');
  });

  it('未配置站点 logo 时用内置站点图标', () => {
    expect(render()).toContain('viewBox="48 32 160 192"');
  });

  it('配置了站点 logo 时用该 logo', () => {
    expect(render({ siteInfo: { logo: '/brand/logo.png' } })).toContain('src="/brand/logo.png"');
  });

  it('不出现 OIDC 与供应商名', () => {
    const html = render({ siteInfo: { oidc_providers: [{ slug: 'authany', display_name: 'Authany', first_party: true }] } });
    expect(html).not.toContain('OIDC');
    expect(html).not.toContain('Authany');
  });
});

describe('SignedOut — 会话仍在', () => {
  const user = { id: 1, username: 'alice' };

  it('渲染「你仍在登录状态」与「进入控制台」,不提重新登录', () => {
    const html = render({ user });
    expect(html).toContain('你仍在登录状态');
    expect(html).toContain('进入控制台');
    expect(html).not.toContain('你已退出登录');
  });

  it('用户信息尚未加载完时仍按已退出渲染(不闪「仍在登录」)', () => {
    const html = render({ user, isUserLoaded: false });
    expect(html).toContain('你已退出登录');
    expect(html).not.toContain('你仍在登录状态');
  });
});
