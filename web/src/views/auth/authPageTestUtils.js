import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { Provider } from 'react-redux';
import { MemoryRouter } from 'react-router';
import { legacy_createStore as createStore } from 'redux';
import { afterAll, beforeAll } from 'vitest';

import reducer from 'store/reducer';
import { SET_SITE_INFO } from 'store/actions';
import 'i18n/i18n';

// 仅测试用:静态渲染认证页(Login / Register)的最小壳 —— 真实 redux reducer + 真实 i18n(zh_CN)+ MemoryRouter。
// AuthShell 依赖浏览器全局(localStorage / matchMedia / document),由各测试文件自行 vi.mock 成透传壳。

// siteInfo 省略(传 undefined)= /api/status 还没回来,store 停在初值(isLoading:true),
// 用来测「状态未知」那一瞬间的行为。
export const renderAuthPage = (Component, path, siteInfo) => {
  const store = createStore(reducer);
  if (siteInfo) store.dispatch({ type: SET_SITE_INFO, payload: siteInfo });
  return renderToStaticMarkup(
    createElement(Provider, { store }, createElement(MemoryRouter, { initialEntries: [path] }, createElement(Component)))
  );
};

// react-router 的 MemoryRouter / Link 在服务端渲染时会打 useLayoutEffect 警告,与被测行为无关;
// 只过滤这一条,其它 console.error 照常输出。
export const silenceRouterLayoutEffectWarning = () => {
  let original;
  beforeAll(() => {
    original = console.error;
    console.error = (...args) => {
      if (typeof args[0] === 'string' && args[0].includes('useLayoutEffect does nothing on the server')) return;
      original(...args);
    };
  });
  afterAll(() => {
    console.error = original;
  });
};
