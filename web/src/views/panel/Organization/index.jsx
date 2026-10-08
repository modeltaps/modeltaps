import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, useSearchParams } from 'react-router';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { API } from 'utils/api';
import { showWarning } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import { orgSettingsUrl } from '../Settings/sections';
import MyInvitations from './MyInvitations';
import OverviewTab from './OverviewTab';
import UsageTab from './UsageTab';
import AuditTab from './AuditTab';

// ==============================|| ORGANIZATION — MANAGEMENT CONSOLE (T9) ||============================== //
// Personal context (no active org): shows "my invitations" + invite-link redeem.
// Org context: tabbed console — overview / usage / audit (S5:成员与设置已迁到
// 统一设置页 `/panel/settings/org/:orgId/...`,旧 `?tab=` 深链在此重定向)。
// Admin-only tabs are hidden for members; the backend 403 remains the final guard.

export const isOrgAdminRole = (role) => role === 'owner' || role === 'admin';

// Deep-linkable tabs kept in the workspace.
const DEEP_LINK_TABS = ['overview', 'usage', 'audit'];
// 迁走的 Tab → 统一设置页的 section(非管理员没有设置页权限,回落到概览)。
const MOVED_TABS = { settings: 'general', members: 'members' };

export default function Organization() {
  const { t } = useTranslation();
  const { orgEnabled, currentOrgId, orgRole } = useOrg();
  const [searchParams] = useSearchParams();
  const [tab, setTab] = useState('overview');
  const [detail, setDetail] = useState(null);
  const isAdmin = isOrgAdminRole(orgRole);

  const fetchDetail = useCallback(async () => {
    if (!currentOrgId) return;
    try {
      const res = await API.get(`/api/org/${currentOrgId}/`);
      const { success, data } = res.data;
      if (success) setDetail(data);
    } catch (error) {
      console.error(error);
    }
  }, [currentOrgId]);

  useEffect(() => {
    const requested = searchParams.get('tab');
    setTab(DEEP_LINK_TABS.includes(requested) ? requested : 'overview');
    setDetail(null);
    fetchDetail();
  }, [fetchDetail, searchParams]);

  // 角色降级(或切换组织)后离开管理员专属 tab,并提示用户(UX-6)
  useEffect(() => {
    if (!isAdmin && tab === 'audit') {
      setTab('overview');
      showWarning(t('orgPage.members.roleDowngraded'));
    }
  }, [isAdmin, tab, t]);

  // 个人上下文,或携带邀请链接参数:展示"我的邀请"视图
  if (!orgEnabled || !currentOrgId || searchParams.get('invite')) {
    return (
      <>
        <MyInvitations />
      </>
    );
  }

  // 旧深链 `?tab=settings|members` 收口到统一设置页,保留其余 query(如 highlight);
  // 普通成员没有设置页入口,回落到概览。
  const movedSection = MOVED_TABS[searchParams.get('tab')];
  if (movedSection) {
    if (!isAdmin) return <Navigate to="/panel/organization" replace />;
    const rest = new URLSearchParams(searchParams);
    rest.delete('tab');
    const query = rest.toString();
    return <Navigate to={`${orgSettingsUrl(currentOrgId, movedSection)}${query ? `?${query}` : ''}`} replace />;
  }

  return (
    <>
      <div className="space-y-6">
        <Tabs value={tab} onValueChange={setTab}>
          <TabsList>
            <TabsTrigger value="overview">{t('orgPage.tabs.overview')}</TabsTrigger>
            <TabsTrigger value="usage">{t('orgPage.tabs.usage')}</TabsTrigger>
            {isAdmin && <TabsTrigger value="audit">{t('orgPage.tabs.audit')}</TabsTrigger>}
          </TabsList>
        </Tabs>

        {tab === 'overview' && (
          <OverviewTab orgId={currentOrgId} detail={detail} isAdmin={isAdmin} role={orgRole} onRefresh={fetchDetail} />
        )}
        {tab === 'usage' && <UsageTab orgId={currentOrgId} isAdmin={isAdmin} detail={detail} />}
        {tab === 'audit' && isAdmin && <AuditTab orgId={currentOrgId} />}
      </div>
    </>
  );
}
