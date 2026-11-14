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
import ModelAccessPage from './org/ModelAccessPage';
import useSelfUser from './account/useSelfUser';
import { accountGroupCards } from './account';
import {
  ACCOUNT_GROUPS,
  DEFAULT_ACCOUNT_SECTION,
  ORG_GROUPS,
  ORG_MEMBERS_SECTION,
  ORG_MODELS_SECTION,
  accountSettingsUrl,
  orgSettingsUrl,
  resolveAccountSection,
  resolveOrgSection,
  settingsLanding,
  settingsOrg
} from './sections';

// ==============================|| PANEL — SETTINGS (设置: organization + personal) ||============================== //
// 设置 opens from the account menu and the settings button next to the account row (no menu item
// for either part). The sidebar becomes its tree (chrome/SettingsSidebar, built from
// sections.js): 组织设置 first for owners and admins of an organization, headed by a picker of
// the organizations they manage, then 个人设置. This page renders the content of the active item:
// the groups of both parts are one SettingsPage (unsaved edits survive switching between them;
// its search, dots and counts show in the sidebar), and 成员与预算 (`.../members`) is a page of its
// own (`.../routing` redirects to the workbench's 路由策略 page). The organization is the one in the URL and stays
// decoupled from the active context of the workbench. Platform settings are part of 平台管理.

function SettingsHub({ section, orgId, managedOrgs }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { currentOrgId } = useOrg();
  const self = useSelfUser();

  const org = settingsOrg(managedOrgs, { orgId, currentOrgId });
  const { detail, refresh } = useOrgDetail(org?.id ?? null);

  const groupOf = (group, sectionLabel, to, cards) => ({
    id: group.id,
    icon: group.icon,
    to,
    label: t(group.labelKey),
    sectionLabel,
    description: t(`${group.labelKey}Description`, { defaultValue: '' }),
    cards
  });

  const groups = [];
  if (org) {
    // Keyed by organization, so picking another one remounts its forms instead of carrying edits over.
    const props = { orgId: org.id, role: org.role, detail, onRefresh: refresh };
    const orgCards = {
      general: [
        { id: 'org-profile', render: () => <GeneralSection key={org.id} {...props} /> },
        { id: 'team-budget', render: () => <BudgetSection key={org.id} {...props} /> }
      ],
      danger: [{ id: 'danger', render: () => <DangerSection key={org.id} {...props} /> }]
    };
    const orgLabel = t('settingsUi.sections.org');
    for (const group of ORG_GROUPS) groups.push(groupOf(group, orgLabel, orgSettingsUrl(org.id, group.id), orgCards[group.id]));
  }
  const accountCards = accountGroupCards({ siteInfo, self });
  const personalLabel = t('settingsUi.sections.personal');
  for (const group of ACCOUNT_GROUPS) groups.push(groupOf(group, personalLabel, accountSettingsUrl(group.id), accountCards[group.id]));

  return <SettingsPage label={t('settingsUi.sections.label')} groups={groups} activeGroupId={section} className="max-w-4xl" />;
}

SettingsHub.propTypes = {
  section: PropTypes.string.isRequired,
  orgId: PropTypes.number,
  managedOrgs: PropTypes.arrayOf(PropTypes.object).isRequired
};

export default function Settings() {
  const { search } = useLocation();
  const { orgId: orgIdParam, section } = useParams();
  const { organizations, orgsLoaded, currentOrgId } = useOrg();
  useHighlightTarget();

  const orgScope = orgIdParam !== undefined;
  const orgId = orgScope ? Number(orgIdParam) : null;
  const managedOrgs = organizations.filter((org) => isOrgAdminRole(org.role));
  const managedOrg = orgScope ? managedOrgs.find((org) => org.id === orgId) || null : null;
  const accountDefault = accountSettingsUrl(DEFAULT_ACCOUNT_SECTION);

  // Wait for the organization list before deciding, so a reload does not bounce an org admin.
  if (!orgsLoaded) return null;

  if (!orgScope) {
    // `/panel/settings` opens the first item of the tree (with a query, the personal settings, as
    // older links expect).
    if (!section) return <Navigate to={search ? `${accountDefault}${search}` : settingsLanding({ organizations, currentOrgId })} replace />;
    const target = resolveAccountSection(section);
    if (target) return <Navigate to={target} replace />;
    return <SettingsHub section={section} orgId={null} managedOrgs={managedOrgs} />;
  }

  // Organizations the user does not manage fall back to the first item of the tree.
  if (!managedOrg) return <Navigate to={settingsLanding({ organizations, currentOrgId })} replace />;
  const target = resolveOrgSection(orgId, section);
  if (target) return <Navigate to={target} replace />;
  if (section === ORG_MEMBERS_SECTION) {
    return (
      <div className="max-w-6xl">
        <OrgSectionHost key={orgId} orgId={orgId} role={managedOrg.role} component={MembersTab} />
      </div>
    );
  }
  if (section === ORG_MODELS_SECTION) {
    return (
      <div className="max-w-6xl">
        <ModelAccessPage key={orgId} orgId={orgId} />
      </div>
    );
  }
  return <SettingsHub section={section} orgId={orgId} managedOrgs={managedOrgs} />;
}
