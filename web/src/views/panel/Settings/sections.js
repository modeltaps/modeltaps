import { Database, ListChecks, ShieldCheck, SlidersHorizontal, TriangleAlert, User, UsersRound } from 'lucide-react';

import { ORG_SETTINGS_PREFIX, SETTINGS, settings, workbench } from 'routes/paths';

// ==============================|| PANEL — SETTINGS GROUP REGISTRY ||============================== //
// Groups of the personal and the organization settings pages (both render with SettingsPage), and
// the old section ids that now redirect. The members page (成员与预算) shares the organization route
// but is a page of its own, not a settings group. The organization's routing policies moved to the
// workbench (路由策略, `/panel/orgs/:orgSlug/routing-policies`); their old section redirects there.

export const ORG_GROUPS = [
  { id: 'general', labelKey: 'settingsUi.org.groups.general', icon: SlidersHorizontal },
  { id: 'danger', labelKey: 'settingsUi.org.groups.danger', icon: TriangleAlert }
];

export const ORG_MEMBERS_SECTION = 'members';
export const ORG_ROUTING_SECTION = 'routing';
export const ORG_MODELS_SECTION = 'model-access';
const ORG_PAGES = [ORG_MEMBERS_SECTION, ORG_MODELS_SECTION];

export const ACCOUNT_GROUPS = [
  { id: 'profile', labelKey: 'settingsUi.personal.groups.profile', icon: User },
  { id: 'security', labelKey: 'settingsUi.personal.groups.security', icon: ShieldCheck },
  { id: 'privacy', labelKey: 'settingsUi.personal.groups.privacy', icon: Database },
  { id: 'preferences', labelKey: 'settingsUi.personal.groups.preferences', icon: SlidersHorizontal }
];

export const DEFAULT_ACCOUNT_SECTION = ACCOUNT_GROUPS[0].id;

export const orgGroupIds = ORG_GROUPS.map(({ id }) => id);
export const accountGroupIds = ACCOUNT_GROUPS.map(({ id }) => id);

// Old section ids → { group, highlight }.
export const LEGACY_ACCOUNT_SECTIONS = { tokens: { group: 'security', highlight: 'access-token' } };
export const LEGACY_ORG_SECTIONS = { budget: { group: 'general', highlight: 'team-budget' } };

// `/panel/settings/orgs/:orgSlug/:section` and `/panel/settings/:section`.
export const orgSettingsUrl = settings.org;
export const accountSettingsUrl = settings.account;

// ---------------------------------------------------------------- the 设置 tree
// 设置 has its own sidebar (SettingsSidebar): 组织设置 first, for owners and admins of at least one
// organization, headed by a picker of the organizations they manage; then 个人设置. The members
// page is in the tree too (the org menu keeps a shortcut to it). The organization is the one in
// the URL; picking another one only changes the URL, never the active context.

export const ORG_TREE = [
  { id: 'general', labelKey: 'settingsUi.org.groups.general', icon: SlidersHorizontal },
  { id: ORG_MEMBERS_SECTION, labelKey: 'nav.items.orgMembers', icon: UsersRound },
  { id: ORG_MODELS_SECTION, labelKey: 'nav.items.orgModelAccess', icon: ListChecks },
  { id: 'danger', labelKey: 'settingsUi.org.groups.danger', icon: TriangleAlert }
];

const managesOrg = (org) => org.role === 'owner' || org.role === 'admin';

// `/panel/settings/orgs/:orgSlug/:section` → { orgSlug, section }; `/panel/settings/:section`
// → { orgSlug: null, section }.
export function parseSettingsPath(pathname) {
  if (pathname.startsWith(`${ORG_SETTINGS_PREFIX}/`)) {
    const [slug, section] = pathname.slice(ORG_SETTINGS_PREFIX.length + 1).split('/');
    let orgSlug = slug || null;
    try {
      orgSlug = orgSlug && decodeURIComponent(orgSlug);
    } catch {
      // keep the raw segment
    }
    return { orgSlug, section: section || null };
  }
  const [section] = pathname
    .replace(new RegExp(`^${SETTINGS}/?`), '')
    .split('/')
    .filter(Boolean);
  return { orgSlug: null, section: section || null };
}

// The organization the 组织设置 part shows, among the organizations the user manages: the URL's
// organization, otherwise the active one, otherwise the first. Null when the user manages none.
export function settingsOrg(managedOrgs, { orgSlug = null, currentOrgId = null } = {}) {
  if (!managedOrgs?.length) return null;
  const bySlug = orgSlug == null ? null : managedOrgs.find((org) => org.slug === orgSlug) || null;
  const byId = currentOrgId == null ? null : managedOrgs.find((org) => org.id === currentOrgId) || null;
  return bySlug || byId || managedOrgs[0];
}

// Where 设置 opens: the first item of its tree.
export function settingsLanding({ organizations = [], currentOrgId = null } = {}) {
  const org = settingsOrg(organizations.filter(managesOrg), { currentOrgId });
  return org ? orgSettingsUrl(org.slug, ORG_TREE[0].id) : accountSettingsUrl(DEFAULT_ACCOUNT_SECTION);
}

// The tree for a settings path: both parts with their links and the active item.
export function settingsTree({ pathname, organizations = [], currentOrgId = null }) {
  const { orgSlug, section } = parseSettingsPath(pathname);
  const managedOrgs = organizations.filter(managesOrg);
  const org = settingsOrg(managedOrgs, { orgSlug, currentOrgId });
  const onOrg = orgSlug !== null && org?.slug === orgSlug;
  const orgItems = org
    ? ORG_TREE.map((item) => ({
        ...item,
        to: orgSettingsUrl(org.slug, item.id),
        active: onOrg && item.id === section
      }))
    : [];
  const personalItems = ACCOUNT_GROUPS.map((group) => ({
    ...group,
    to: accountSettingsUrl(group.id),
    active: orgSlug === null && group.id === section
  }));
  const activeOrgItem = orgItems.find((item) => item.active) || null;
  const active = activeOrgItem || personalItems.find((item) => item.active) || null;
  return { managedOrgs, org, orgItems, personalItems, active, activeScope: active ? (activeOrgItem ? 'org' : 'personal') : null };
}

// Redirect target for an account section id, or null when it is a group.
export function resolveAccountSection(section) {
  if (accountGroupIds.includes(section)) return null;
  const legacy = LEGACY_ACCOUNT_SECTIONS[section];
  if (legacy) return `${accountSettingsUrl(legacy.group)}?highlight=${legacy.highlight}`;
  return accountSettingsUrl(DEFAULT_ACCOUNT_SECTION);
}

// Redirect target for an organization section id, or null when it is a group or one of the
// organization pages.
export function resolveOrgSection(orgSlug, section) {
  if (section === ORG_ROUTING_SECTION) return workbench.routingPolicies(orgSlug);
  if (orgGroupIds.includes(section) || ORG_PAGES.includes(section)) return null;
  const legacy = LEGACY_ORG_SECTIONS[section];
  if (legacy) return `${orgSettingsUrl(orgSlug, legacy.group)}?highlight=${legacy.highlight}`;
  return orgSettingsUrl(orgSlug, ORG_GROUPS[0].id);
}
