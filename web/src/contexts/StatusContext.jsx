import { useEffect, useCallback, createContext } from 'react';
import { API } from 'utils/api';
import { showNotice } from 'utils/common';
import { brandName } from 'utils/brand';
import { setDocumentTitleBrand } from 'utils/documentTitle';
import { SET_SITE_INFO, SET_MODEL_OWNEDBY } from 'store/actions';
import { useDispatch } from 'react-redux';
import { useTranslation } from 'react-i18next';

export const LoadStatusContext = createContext();

// eslint-disable-next-line
const StatusProvider = ({ children }) => {
  const { t } = useTranslation();
  const dispatch = useDispatch();

  // /api/status 拿不到时退回本地缓存;没有缓存也要派发空对象,否则 siteInfo.isLoading 永远为 true,
  // 等它的页面(AdminLogin / OAuthCallback / Login)会一直转圈,而不是退回内置默认。
  const dispatchCachedSiteInfo = useCallback(() => {
    const backupSiteInfo = localStorage.getItem('siteInfo');
    let data = {};
    if (backupSiteInfo) {
      try {
        data = JSON.parse(backupSiteInfo) || {};
      } catch (error) {
        data = {};
      }
    }
    dispatch({ type: SET_SITE_INFO, payload: data });
    return data.system_name || '';
  }, [dispatch]);

  const loadStatus = useCallback(async () => {
    let system_name = '';
    let analytics_code = '';
    try {
      const res = await API.get('/api/status');
      const { success, data } = res.data;
      if (success) {
        if (!data.chat_link) {
          delete data.chat_link;
        }
        localStorage.setItem('siteInfo', JSON.stringify(data));
        localStorage.setItem('quota_per_unit', data.quota_per_unit);
        localStorage.setItem('display_in_currency', data.display_in_currency);
        dispatch({ type: SET_SITE_INFO, payload: data });
        const localVer = import.meta.env.VITE_APP_VERSION;
        if (data.version && localVer && data.version !== localVer && data.version !== 'v0.0.0') {
          showNotice(t('common.unableServerTip', { version: data.version }));
        }
        if (data.system_name) {
          system_name = data.system_name;
        }
        if (data.analytics_code) {
          analytics_code = data.analytics_code;
        }
      } else {
        system_name = dispatchCachedSiteInfo() || system_name;
      }
    } catch (error) {
      // The API response interceptor already surfaces the failure toast.
      system_name = dispatchCachedSiteInfo() || system_name;
    }

    setDocumentTitleBrand(brandName(system_name));

    if (analytics_code) {
      // Check if the script is already injected
      if (!document.getElementById('analytics-code')) {
        const range = document.createRange();
        const fragment = range.createContextualFragment(analytics_code);
        // Add an ID to the first child to prevent duplicate injection
        if (fragment.firstElementChild) {
          fragment.firstElementChild.id = 'analytics-code';
          document.head.appendChild(fragment);
        }
      }
    }
    // eslint-disable-next-line
  }, [dispatch, dispatchCachedSiteInfo]);

  const loadOwnedby = useCallback(async () => {
    try {
      const res = await API.get('/api/model_ownedby');
      const { success, data } = res.data;
      if (success) {
        dispatch({ type: SET_MODEL_OWNEDBY, payload: data });
      }
    } catch (error) {
      // The API response interceptor already surfaces the failure toast.
    }
  }, [dispatch]);

  useEffect(() => {
    loadStatus().then();
    loadOwnedby();
  }, [loadStatus, loadOwnedby]);

  return <LoadStatusContext.Provider value={loadStatus}> {children} </LoadStatusContext.Provider>;
};

export default StatusProvider;
