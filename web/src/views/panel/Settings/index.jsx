import { Navigate, useLocation, useParams } from 'react-router';

import { useOrg } from 'contexts/OrgContext';
import { isOrgAdminRole } from '../Organization';
import OrgSectionHost from './org/OrgSectionHost';
import AccountSection from './account';
import {
  ORG_SECTIONS,
  ACCOUNT_SECTIONS,
  DEFAULT_ORG_SECTION,
  DEFAULT_ACCOUNT_SECTION,
  orgSectionIds,
  accountSectionIds,
  orgSettingsUrl,
  accountSettingsUrl
} from './sections';

// ==============================|| PANEL — UNIFIED SETTINGS ||============================== //
// 整页设置面:导航(两个分组 + 组织选择器)由 chrome 的 SettingsSidebar 承担,这里只做
// 重定向/校验与内容渲染。设置页的组织由 URL 显式携带,与左上角全局上下文解耦
// (切换不调用 switchOrg)。组织分组的内容组件登记在 sections.js 上,经 OrgSectionHost 取数后渲染。

export default function Settings() {
  const { search } = useLocation();
  const { orgId: orgIdParam, section } = useParams();
  const { organizations, orgsLoaded, currentOrgId } = useOrg();

  const managedOrgs = organizations.filter((org) => isOrgAdminRole(org.role));
  // 默认落点:当前全局上下文是我管理的组织 → 该组织;否则第一个我管理的组织;都没有 → 个人资料。
  const defaultOrg = managedOrgs.find((org) => org.id === currentOrgId) || managedOrgs[0] || null;
  const defaultTarget = defaultOrg ? orgSettingsUrl(defaultOrg.id, DEFAULT_ORG_SECTION) : accountSettingsUrl(DEFAULT_ACCOUNT_SECTION);

  const orgScope = orgIdParam !== undefined;
  const orgId = orgScope ? Number(orgIdParam) : null;

  // 组织列表未就绪时不做判断,避免刷新瞬间把组织管理员误导到个人分组。
  if (!orgsLoaded) return null;

  if (!section) return <Navigate to={`${defaultTarget}${search}`} replace />;
  // 未知 section 或不在我管理列表中的 orgId 回退到默认落点。
  if (orgScope && !(managedOrgs.some((org) => org.id === orgId) && orgSectionIds.includes(section))) {
    return <Navigate to={defaultTarget} replace />;
  }
  if (!orgScope && !accountSectionIds.includes(section)) {
    return <Navigate to={defaultTarget} replace />;
  }

  const activeOrgId = orgScope ? orgId : (defaultOrg?.id ?? null);
  const activeOrgRole = managedOrgs.find((org) => org.id === activeOrgId)?.role;
  const activeSection = orgScope ? ORG_SECTIONS.find(({ id }) => id === section) : ACCOUNT_SECTIONS.find(({ id }) => id === section);

  return (
    <div className="mx-auto max-w-4xl">
      <div className="min-w-0 flex-1">
        {orgScope ? (
          <OrgSectionHost orgId={activeOrgId} role={activeOrgRole} component={activeSection.component} />
        ) : (
          <AccountSection section={section} />
        )}
      </div>
    </div>
  );
}
