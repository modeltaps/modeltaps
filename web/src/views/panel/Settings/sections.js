import { Database, ListChecks, ShieldCheck, SlidersHorizontal, TriangleAlert, User, UsersRound } from 'lucide-react';

// ==============================|| PANEL — SETTINGS GROUP REGISTRY ||============================== //
// Groups of the personal and the organization settings pages (both render with SettingsPage), and
// the old section ids that now redirect. The members page (成员与预算) shares the organization route
// but is a page of its own, not a settings group. The organization's routing policies moved to the
// workbench (路由策略, /panel/routing); their old section redirects there.

export const ORG_GROUPS = [
  { id: 'general', labelKey: 'settingsUi.org.groups.general', icon: SlidersHorizontal },
  { id: 'danger', labelKey: 'settingsUi.org.groups.danger', icon: TriangleAlert }
];

export const ORG_MEMBERS_SECTION = 'members';
export const ORG_ROUTING_SECTION = 'routing';
export const ORG_MODELS_SECTION = 'model-access';
export const ROUTING_POLICIES_URL = '/panel/routing';
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

export const orgSettingsUrl = (orgId, section) => `/panel/settings/org/${orgId}/${section}`;
export const accountSettingsUrl = (section) => `/panel/settings/account/${section}`;

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

// `/panel/settings/org/:orgId/:section` → { orgId, section }; `/panel/settings/account/:section`
// → { orgId: null, section }.
export function parseSettingsPath(pathname) {
  const rest = pathname
    .replace(/^\/panel\/settings\/?/, '')
    .split('/')
    .filter(Boolean);
  if (rest[0] === 'org') return { orgId: Number(rest[1]) || null, section: rest[2] || null };
  if (rest[0] === 'account') return { orgId: null, section: rest[1] || null };
  return { orgId: null, section: null };
}

// The organization the 组织设置 part shows, among the organizations the user manages: the URL's
// organization, otherwise the active one, otherwise the first. Null when the user manages none.
export function settingsOrg(managedOrgs, { orgId = null, currentOrgId = null } = {}) {
  if (!managedOrgs?.length) return null;
  const byId = (id) => (id == null ? null : managedOrgs.find((org) => org.id === id) || null);
  return byId(orgId) || byId(currentOrgId) || managedOrgs[0];
}

// Where 设置 opens: the first item of its tree.
export function settingsLanding({ organizations = [], currentOrgId = null } = {}) {
  const org = settingsOrg(organizations.filter(managesOrg), { currentOrgId });
  return org ? orgSettingsUrl(org.id, ORG_TREE[0].id) : accountSettingsUrl(DEFAULT_ACCOUNT_SECTION);
}

// The tree for a settings path: both parts with their links and the active item.
export function settingsTree({ pathname, organizations = [], currentOrgId = null }) {
  const { orgId, section } = parseSettingsPath(pathname);
  const managedOrgs = organizations.filter(managesOrg);
  const org = settingsOrg(managedOrgs, { orgId, currentOrgId });
  const onOrg = orgId !== null && org?.id === orgId;
  const orgItems = org
    ? ORG_TREE.map((item) => ({
        ...item,
        to: orgSettingsUrl(org.id, item.id),
        active: onOrg && item.id === section
      }))
    : [];
  const personalItems = ACCOUNT_GROUPS.map((group) => ({
    ...group,
    to: accountSettingsUrl(group.id),
    active: orgId === null && group.id === section
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
export function resolveOrgSection(orgId, section) {
  if (section === ORG_ROUTING_SECTION) return ROUTING_POLICIES_URL;
  if (orgGroupIds.includes(section) || ORG_PAGES.includes(section)) return null;
  const legacy = LEGACY_ORG_SECTIONS[section];
  if (legacy) return `${orgSettingsUrl(orgId, legacy.group)}?highlight=${legacy.highlight}`;
  return orgSettingsUrl(orgId, ORG_GROUPS[0].id);
}
