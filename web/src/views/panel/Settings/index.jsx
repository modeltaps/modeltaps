import { useEffect } from 'react';
import { Navigate, useLocation, useParams } from 'react-router';
import { useTranslation } from 'react-i18next';

import { useOrg } from 'contexts/OrgContext';
import SettingsLayout from '@/components/chrome/SettingsLayout';
import { isOrgAdminRole } from '../Organization';
import OrgSectionHost from './org/OrgSectionHost';
import AccountSection from './account';
import {
  ORG_SECTIONS,
  ACCOUNT_SECTIONS,
  DEFAULT_ACCOUNT_SECTION,
  orgSectionIds,
  accountSectionIds,
  orgSettingsUrl,
  accountSettingsUrl
} from './sections';

// ==============================|| PANEL — SETTINGS (personal / organization) ||============================== //
// Both render inside the sidebar shell with an in-page subnav of their sections.
// - Personal settings (`/panel/settings/account/:section`) open from the account menu.
// - Organization settings (`/panel/settings/org/:orgId/:section`) are the org context's
//   组织 items. The context switcher picks the organization: opening a deep link to another
//   organization the user manages switches the active context to it, so the sidebar and the
//   page always talk about the same organization.

export default function Settings() {
  const { t } = useTranslation();
  const { search } = useLocation();
  const { orgId: orgIdParam, section } = useParams();
  const { organizations, orgsLoaded, currentOrgId, switchOrg } = useOrg();

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

  if (!section && !orgScope) return <Navigate to={`${accountDefault}${search}`} replace />;
  // Unknown sections, or organizations the user does not manage, fall back to personal settings.
  if (orgScope && !(managedOrg && orgSectionIds.includes(section))) return <Navigate to={accountDefault} replace />;
  if (!orgScope && !accountSectionIds.includes(section)) return <Navigate to={accountDefault} replace />;

  const sections = orgScope ? ORG_SECTIONS : ACCOUNT_SECTIONS;
  const active = sections.find(({ id }) => id === section);
  const items = sections.map((item) => ({
    id: item.id,
    label: t(item.labelKey, { defaultValue: item.fallback }),
    to: orgScope ? orgSettingsUrl(orgId, item.id) : accountSettingsUrl(item.id)
  }));

  return (
    <SettingsLayout label={t('nav.subnav.label')} items={items} activeId={section} className="max-w-6xl">
      <div className="max-w-4xl">
        {orgScope ? (
          <OrgSectionHost orgId={orgId} role={managedOrg.role} component={active.component} />
        ) : (
          <AccountSection section={section} />
        )}
      </div>
    </SettingsLayout>
  );
}
