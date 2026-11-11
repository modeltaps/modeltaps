import { useEffect } from 'react';
import PropTypes from 'prop-types';
import { Navigate, useLocation, useParams } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { useOrg } from 'contexts/OrgContext';
import useHighlightTarget from 'hooks/useHighlightTarget';
import SettingsPage from '@/components/settings/SettingsPage';
import { isOrgAdminRole } from '../Organization';
import MembersTab from '../Organization/MembersTab';
import OrgSectionHost, { useOrgDetail } from './org/OrgSectionHost';
import GeneralSection from './org/GeneralSection';
import BudgetSection from './org/BudgetSection';
import DangerSection from './org/DangerSection';
import RoutingPage from './org/RoutingPage';
import ModelAccessPage from './org/ModelAccessPage';
import useSelfUser from './account/useSelfUser';
import { accountGroupCards } from './account';
import {
  ACCOUNT_GROUPS,
  DEFAULT_ACCOUNT_SECTION,
  ORG_GROUPS,
  ORG_MEMBERS_SECTION,
  ORG_MODELS_SECTION,
  ORG_ROUTING_SECTION,
  accountSettingsUrl,
  orgSettingsUrl,
  resolveAccountSection,
  resolveOrgSection
} from './sections';

// ==============================|| PANEL — SETTINGS (personal / organization) ||============================== //
// Both are a SettingsPage inside the sidebar shell.
// - Personal settings (`/panel/settings/account/:group`) open from the account menu.
// - Organization settings (`/panel/settings/org/:orgId/:group`) are the org context's 组织设置;
//   `/panel/settings/org/:orgId/members` on the same route is the 成员与预算 page and
//   `/panel/settings/org/:orgId/routing` the organization's 自动路由 page. The context
//   switcher picks the organization: opening a deep link to another organization the user manages
//   switches the active context to it, so the sidebar and the page talk about the same one.

function AccountSettings({ section }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const self = useSelfUser();
  const cards = accountGroupCards({ siteInfo, self });
  const groups = ACCOUNT_GROUPS.map((group) => ({
    id: group.id,
    icon: group.icon,
    to: accountSettingsUrl(group.id),
    label: t(group.labelKey),
    description: t(`${group.labelKey}Description`, { defaultValue: '' }),
    cards: cards[group.id]
  }));
  return <SettingsPage label={t('settingsUi.personal.label')} groups={groups} activeGroupId={section} className="max-w-6xl" />;
}

AccountSettings.propTypes = { section: PropTypes.string.isRequired };

function OrgSettings({ orgId, role, section }) {
  const { t } = useTranslation();
  const { detail, refresh } = useOrgDetail(orgId);
  const props = { orgId, role, detail, onRefresh: refresh };
  const cards = {
    general: [
      { id: 'profile', render: () => <GeneralSection {...props} /> },
      { id: 'team-budget', render: () => <BudgetSection {...props} /> }
    ],
    danger: [{ id: 'danger', render: () => <DangerSection {...props} /> }]
  };
  const groups = ORG_GROUPS.map((group) => ({
    id: group.id,
    icon: group.icon,
    to: orgSettingsUrl(orgId, group.id),
    label: t(group.labelKey),
    description: t(`${group.labelKey}Description`, { defaultValue: '' }),
    cards: cards[group.id]
  }));
  return <SettingsPage key={orgId} label={t('settingsUi.org.label')} groups={groups} activeGroupId={section} className="max-w-6xl" />;
}

OrgSettings.propTypes = { orgId: PropTypes.number.isRequired, role: PropTypes.string, section: PropTypes.string.isRequired };

export default function Settings() {
  const { search } = useLocation();
  const { orgId: orgIdParam, section } = useParams();
  const { organizations, orgsLoaded, currentOrgId, switchOrg, orgRoutingPolicyEnabled } = useOrg();
  useHighlightTarget();

  const orgScope = orgIdParam !== undefined;
  const orgId = orgScope ? Number(orgIdParam) : null;
  const managedOrg = orgScope ? organizations.find((org) => org.id === orgId && isOrgAdminRole(org.role)) || null : null;
  const accountDefault = accountSettingsUrl(DEFAULT_ACCOUNT_SECTION);

  // Follow the URL's organization once per deep link (not on every context change, so that
  // switching context from this page is not undone before the switcher navigates away).
  const isManaged = Boolean(managedOrg);
  useEffect(() => {
    if (isManaged && orgId !== currentOrgId) switchOrg(orgId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgId, isManaged]);

  // Wait for the organization list before deciding, so a reload does not bounce an org admin.
  if (!orgsLoaded) return null;

  if (!orgScope) {
    if (!section) return <Navigate to={`${accountDefault}${search}`} replace />;
    const target = resolveAccountSection(section);
    if (target) return <Navigate to={target} replace />;
    return <AccountSettings section={section} />;
  }

  // Organizations the user does not manage fall back to personal settings.
  if (!managedOrg) return <Navigate to={accountDefault} replace />;
  const target = resolveOrgSection(orgId, section, { routing: orgRoutingPolicyEnabled });
  if (target) return <Navigate to={target} replace />;
  if (section === ORG_MEMBERS_SECTION) {
    return (
      <div className="max-w-6xl">
        <OrgSectionHost orgId={orgId} role={managedOrg.role} component={MembersTab} />
      </div>
    );
  }
  if (section === ORG_ROUTING_SECTION) return <RoutingPage orgId={orgId} />;
  if (section === ORG_MODELS_SECTION) return <ModelAccessPage key={orgId} orgId={orgId} />;
  return <OrgSettings orgId={orgId} role={managedOrg.role} section={section} />;
}
