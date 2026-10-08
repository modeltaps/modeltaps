import { API, LoginCheckAPI } from 'utils/api';
import { syncUserSettingFromServer } from 'utils/userSetting';
import { useCallback } from 'react';
import { useDispatch } from 'react-redux';
import { LOGIN, SET_USER_GROUP } from 'store/actions';
import { useNavigate } from 'react-router';
import { showSuccess } from 'utils/common';
import { requestLogout, signedOutUrl } from 'utils/logout';
import { clearChatSession } from 'views/panel/Playground/chat/chatPersistence';
import { useTranslation } from 'react-i18next';

const useLogin = (navPath = '/panel') => {
  const { t } = useTranslation();
  const dispatch = useDispatch();
  const navigate = useNavigate();
  const login = async (username, password, turnstile = '') => {
    try {
      const res = await API.post(`/api/user/login?turnstile=${turnstile}`, {
        username,
        password
      });
      const { success, message } = res.data;
      if (success) {
        // 等待用户信息加载完成后再跳转
        await loadUser();
        loadUserGroup();
        navigate(navPath);
      }
      return { success, message };
    } catch (err) {
      // 请求失败，返回后端真实错误信息供调用方内联展示/分支判断
      return { success: false, message: err.message || '' };
    }
  };

  // 邮箱验证码登录(内置账号模式):先发码,再凭码建立会话。
  const sendLoginCode = async (email, turnstile = '') => {
    try {
      const res = await API.post(`/api/user/login_code?turnstile=${turnstile}`, { email });
      const { success, message } = res.data;
      return { success, message };
    } catch (err) {
      return { success: false, message: err.message || '' };
    }
  };

  const loginWithCode = async (email, code) => {
    try {
      const res = await API.post('/api/user/login/code', { email, code });
      const { success, message } = res.data;
      if (success) {
        await loadUser();
        loadUserGroup();
        navigate(navPath);
      }
      return { success, message };
    } catch (err) {
      return { success: false, message: err.message || '' };
    }
  };

  const githubLogin = async (code, state) => {
    try {
      const affCode = localStorage.getItem('aff');
      const res = await API.get(`/api/oauth/github?code=${code}&state=${state}&aff=${affCode}`);
      const { success, message } = res.data;
      if (success) {
        if (message === 'bind') {
          showSuccess(t('common.bindOk'));
          navigate(navPath);
        } else {
          // 等待用户信息加载完成后再跳转
          await loadUser();
          loadUserGroup();
          showSuccess(t('common.loginOk'));
          navigate(navPath);
        }
      }
      return { success, message };
    } catch (err) {
      // 请求失败，返回后端真实错误信息供调用方内联展示/分支判断
      return { success: false, message: err.message || '' };
    }
  };

  // slug 为空时走无 slug 的旧回调，后端等价于 slug=oidc。
  const oidcLogin = async (code, state, slug = '') => {
    try {
      const affCode = localStorage.getItem('aff');
      const path = slug ? `/api/oauth/oidc/${encodeURIComponent(slug)}` : '/api/oauth/oidc';
      const res = await API.get(`${path}?code=${code}&state=${state}&aff=${affCode}`);
      const { success, message } = res.data;
      if (success) {
        if (message === 'bind') {
          showSuccess(t('common.bindOk'));
          navigate(navPath);
        } else {
          loadUser();
          loadUserGroup();
          showSuccess(t('common.loginOk'));
          navigate(navPath);
        }
      }
      return { success, message };
    } catch (err) {
      // 请求失败，返回后端真实错误信息供调用方内联展示/分支判断
      return { success: false, message: err.message || '' };
    }
  };

  const larkLogin = async (code, state) => {
    try {
      const affCode = localStorage.getItem('aff');
      const res = await API.get(`/api/oauth/lark?code=${code}&state=${state}&aff=${affCode}`);
      const { success, message } = res.data;
      if (success) {
        if (message === 'bind') {
          showSuccess(t('common.bindOk'));
          navigate(navPath);
        } else {
          // 等待用户信息加载完成后再跳转
          await loadUser();
          showSuccess(t('common.loginOk'));
          navigate(navPath);
        }
      }
      return { success, message };
    } catch (err) {
      // 请求失败，返回后端真实错误信息供调用方内联展示/分支判断
      return { success: false, message: err.message || '' };
    }
  };

  const wechatLogin = async (code) => {
    try {
      const affCode = localStorage.getItem('aff');
      const res = await API.get(`/api/oauth/wechat?code=${code}&aff=${affCode}`);
      const { success, message } = res.data;
      if (success) {
        // 等待用户信息加载完成后再跳转
        await loadUser();
        loadUserGroup();
        showSuccess(t('common.loginOk'));
        navigate(navPath);
      }
      return { success, message };
    } catch (err) {
      // 请求失败，返回后端真实错误信息供调用方内联展示/分支判断
      return { success: false, message: err.message || '' };
    }
  };

  const linuxDoLogin = async (code, state) => {
    try {
      const affCode = localStorage.getItem('aff');
      const res = await API.get(`/api/oauth/linuxdo?code=${code}&state=${state}&aff=${affCode}`);
      const { success, message } = res.data;
      if (success) {
        if (message === 'bind') {
          showSuccess(t('common.bindOk'));
          navigate(navPath);
        } else {
          // 等待用户信息加载完成后再跳转
          await loadUser();
          loadUserGroup();
          showSuccess(t('common.loginOk'));
          navigate(navPath);
        }
      }
      return { success, message };
    } catch (err) {
      // 请求失败，返回后端真实错误信息供调用方内联展示/分支判断
      return { success: false, message: err.message || '' };
    }
  };

  // 退出登录：后端下发 IdP 结束会话地址时整页跳过去（IdP 清掉 SSO 会话后回本站 /signed-out），否则整页落 /signed-out。
  // 两个分支都整页加载：react-router 7 的导航走 startTransition，页内「先导航再清用户状态」已挡不住 AuthGuard 抢先
  // navigate('/login')；整页加载还会丢掉所有内存态（控制台 key、各会话 store），同一标签页里下一个登录的人拿不到上一个人的 key。
  const logout = async () => {
    const redirectUrl = await requestLogout();
    localStorage.removeItem('user');
    clearChatSession();
    window.location.assign(redirectUrl || signedOutUrl());
  };

  const loadUser = useCallback(async () => {
    try {
      const res = await LoginCheckAPI.get('/api/user/self');
      const { success, data } = res.data;
      if (success) {
        dispatch({ type: LOGIN, payload: data });
        // 登录后拉取服务端偏好（theme/language）并应用，失败静默回退 localStorage
        syncUserSettingFromServer();
        return data;
      }
      return null;
    } catch (err) {
      // 只在非401错误时打印错误信息
      if (err.response?.status !== 401) {
        console.error(err);
      }
      return null;
    }
  }, [dispatch]);

  const loadUserGroup = useCallback(() => {
    try {
      API.get('/api/user_group_map').then((res) => {
        const { success, data } = res.data;
        if (success) {
          dispatch({ type: SET_USER_GROUP, payload: data });
        }
      });
    } catch (error) {
      console.error(error);
    }
    return [];
  }, []);

  return {
    login,
    logout,
    sendLoginCode,
    loginWithCode,
    githubLogin,
    wechatLogin,
    larkLogin,
    oidcLogin,
    linuxDoLogin,
    loadUser,
    loadUserGroup
  };
};

export default useLogin;
