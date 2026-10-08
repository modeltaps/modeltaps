// contexts/OrgContext.jsx
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { useSelector } from 'react-redux';
import { API } from 'utils/api';
import { readStoredOrgId, setActiveOrgId } from 'utils/orgScope';
import { UserContext } from 'contexts/UserContext';

// ==============================|| ORG CONTEXT ||============================== //
// Holds the user's organization list and the active organization selection.
// Invisible / zero-impact when organization_enabled is off or the user has no
// organizations: activeOrgId stays null so the API layer never rewrites paths.

export const OrgContext = createContext({
  orgEnabled: false,
  organizations: [],
  orgsLoaded: false,
  currentOrgId: null,
  currentOrg: null,
  orgRole: null,
  orgDetail: null,
  switchOrg: () => {},
  refreshOrganizations: () => {},
  refreshOrgDetail: () => {}
});

export const useOrg = () => useContext(OrgContext);

// Restore the persisted selection only when the cached siteInfo says the
// feature is enabled, so a disabled deployment never rewrites requests.
function initialOrgId() {
  try {
    const cached = JSON.parse(localStorage.getItem('siteInfo') || '{}');
    if (cached.organization_enabled !== true) return null;
  } catch {
    return null;
  }
  return readStoredOrgId();
}

// eslint-disable-next-line
const OrgProvider = ({ children }) => {
  const account = useSelector((state) => state.account);
  const siteInfo = useSelector((state) => state.siteInfo);
  const { isUserLoaded } = useContext(UserContext);
  const orgEnabled = siteInfo?.organization_enabled === true;
  // SET_SITE_INFO 写入 isLoading:false;加载完成前 organization_enabled 不可信
  const siteInfoLoaded = siteInfo?.isLoading === false;
  const userId = account?.user?.id;

  const [organizations, setOrganizations] = useState([]);
  // 组织列表是否已给出结论(含"功能关闭 / 未登录"两种确定态),供页面在
  // 依赖列表内容做重定向前等待,避免刷新瞬间按空列表下判断。
  const [orgsFetched, setOrgsFetched] = useState(false);
  const [currentOrgId, setCurrentOrgIdState] = useState(() => {
    const id = initialOrgId();
    setActiveOrgId(id);
    return id;
  });
  const [orgDetail, setOrgDetail] = useState(null);

  const switchOrg = useCallback((orgId) => {
    const id = Number.isInteger(orgId) && orgId > 0 ? orgId : null;
    setActiveOrgId(id);
    setCurrentOrgIdState(id);
    setOrgDetail(null);
  }, []);

  const refreshOrganizations = useCallback(async () => {
    if (!orgEnabled || !userId) {
      setOrganizations([]);
      setOrgsFetched(true);
      return [];
    }
    try {
      const res = await API.get('/api/org/');
      const { success, data } = res.data;
      if (success) {
        const list = Array.isArray(data) ? data : [];
        setOrganizations(list);
        return list;
      }
    } catch {
      // 全局响应拦截器已提示错误,这里静默保持现状
    } finally {
      setOrgsFetched(true);
    }
    return null;
  }, [orgEnabled, userId]);

  // 登出(或功能确认关闭)时回到个人上下文并清空列表。
  // siteInfo 尚未从服务端加载时不清理,避免刷新瞬间误清持久化的 active_org_id。
  useEffect(() => {
    if ((isUserLoaded && !userId) || (siteInfoLoaded && !orgEnabled && currentOrgId)) {
      switchOrg(null);
      setOrganizations([]);
    }
  }, [isUserLoaded, userId, siteInfoLoaded, orgEnabled, currentOrgId, switchOrg]);

  // 登录且功能开启时加载组织列表,并校验持久化的选择是否仍然有效
  useEffect(() => {
    if (!orgEnabled || !userId) return;
    let cancelled = false;
    (async () => {
      const list = await refreshOrganizations();
      if (cancelled || list === null) return;
      setCurrentOrgIdState((prev) => {
        if (prev && !list.some((o) => o.id === prev)) {
          setActiveOrgId(null);
          setOrgDetail(null);
          return null;
        }
        return prev;
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [orgEnabled, userId, refreshOrganizations]);

  // 当前组织详情(role/member_count;quota 仅 Owner/Admin 返回)。
  // 序号自增丢弃过期响应,切换组织或并发刷新时只认最后一次请求的结果。
  const detailSeqRef = useRef(0);
  const refreshOrgDetail = useCallback(async () => {
    if (!orgEnabled || !userId || !currentOrgId) return;
    const seq = ++detailSeqRef.current;
    try {
      const res = await API.get(`/api/org/${currentOrgId}/`);
      const { success, data } = res.data;
      if (success && seq === detailSeqRef.current) setOrgDetail(data);
    } catch {
      // 列表校验已兜底,这里静默
    }
  }, [orgEnabled, userId, currentOrgId]);

  useEffect(() => {
    if (!currentOrgId) return;
    if (!organizations.some((o) => o.id === currentOrgId)) return;
    refreshOrgDetail();
  }, [currentOrgId, organizations, refreshOrgDetail]);

  const currentOrg = useMemo(
    () => (currentOrgId ? organizations.find((o) => o.id === currentOrgId) || null : null),
    [organizations, currentOrgId]
  );

  const value = useMemo(
    () => ({
      orgEnabled,
      organizations,
      orgsLoaded: isUserLoaded && siteInfoLoaded && (!orgEnabled || !userId || orgsFetched),
      currentOrgId: currentOrg ? currentOrgId : null,
      currentOrg,
      orgRole: currentOrg?.role || null,
      orgDetail: currentOrg ? orgDetail : null,
      switchOrg,
      refreshOrganizations,
      refreshOrgDetail
    }),
    [
      orgEnabled,
      organizations,
      isUserLoaded,
      siteInfoLoaded,
      userId,
      orgsFetched,
      currentOrgId,
      currentOrg,
      orgDetail,
      switchOrg,
      refreshOrganizations,
      refreshOrgDetail
    ]
  );

  return <OrgContext.Provider value={value}>{children}</OrgContext.Provider>;
};

export default OrgProvider;
