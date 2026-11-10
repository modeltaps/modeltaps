import { Database, ShieldCheck, SlidersHorizontal, TriangleAlert, User } from 'lucide-react';

// ==============================|| PANEL — SETTINGS GROUP REGISTRY ||============================== //
// Groups of the personal and the organization settings pages (both render with SettingsPage), and
// the old section ids that now redirect. The members page (成员与预算) and the routing policies
// page (自动路由) share the organization route but are pages of their own, not settings groups.

export const ORG_GROUPS = [
  { id: 'general', labelKey: 'settingsUi.org.groups.general', icon: SlidersHorizontal },
  { id: 'danger', labelKey: 'settingsUi.org.groups.danger', icon: TriangleAlert }
];

export const ORG_MEMBERS_SECTION = 'members';
export const ORG_ROUTING_SECTION = 'routing';
const ORG_PAGES = [ORG_MEMBERS_SECTION, ORG_ROUTING_SECTION];

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

// Redirect target for an account section id, or null when it is a group.
export function resolveAccountSection(section) {
  if (accountGroupIds.includes(section)) return null;
  const legacy = LEGACY_ACCOUNT_SECTIONS[section];
  if (legacy) return `${accountSettingsUrl(legacy.group)}?highlight=${legacy.highlight}`;
  return accountSettingsUrl(DEFAULT_ACCOUNT_SECTION);
}

// Redirect target for an organization section id, or null when it is a group or one of the
// organization pages. The routing page redirects to the first group while the site does not let
// organizations manage routing policies (`routing`: OrgContext's orgRoutingPolicyEnabled).
export function resolveOrgSection(orgId, section, { routing = false } = {}) {
  if (section === ORG_ROUTING_SECTION && !routing) return orgSettingsUrl(orgId, ORG_GROUPS[0].id);
  if (orgGroupIds.includes(section) || ORG_PAGES.includes(section)) return null;
  const legacy = LEGACY_ORG_SECTIONS[section];
  if (legacy) return `${orgSettingsUrl(orgId, legacy.group)}?highlight=${legacy.highlight}`;
  return orgSettingsUrl(orgId, ORG_GROUPS[0].id);
}
