import { showError } from './common';
import axios from 'axios';
import { store } from 'store/index';
import { LOGIN } from 'store/actions';
import { rewriteOrgUrl } from './orgScope';
import { clearChatSession } from 'views/panel/Playground/chat/chatPersistence';

export const API = axios.create({
  // ... 其他代码 ...

  baseURL: import.meta.env.VITE_APP_SERVER || '/'
});

// 组织上下文激活时,把个人维度请求自动改写为 /api/org/:id/*(见 utils/orgScope.js)
API.interceptors.request.use((config) => {
  config.url = rewriteOrgUrl(config.url);
  return config;
});

API.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('user');
      clearChatSession();
      store.dispatch({ type: LOGIN, payload: null });
      // 仅在已登录区域(/panel)内会话失效时跳登录页并带回跳目标；
      // 登录页与公开页自身的 401 不跳转，避免跳转循环与匿名浏览被打断。
      const { pathname, search, hash } = window.location;
      if (pathname.startsWith('/panel')) {
        window.location.href = `/login?redirect=${encodeURIComponent(pathname + search + hash)}`;
      }
    }

    if (error.response?.data?.message) {
      error.message = error.response.data.message;
    }

    showError(error);

    return Promise.reject(error);
  }
);

export const LoginCheckAPI = axios.create({
  // ... 其他代码 ...

  baseURL: import.meta.env.VITE_APP_SERVER || '/'
});
