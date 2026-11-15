import { PLATFORM_GROUPS, SETTINGS_BASE, cardTitleKey, groupDescriptionKey, groupLabelKey, settingsGroupUrl } from 'views/panel/Setting/registry';
import { ACCOUNT_GROUPS, ORG_TREE, accountSettingsUrl, orgSettingsUrl } from 'views/panel/Settings/sections';
import { LEGACY_ADMIN_PATHS } from 'routes/legacyPaths';
import {
  ORG_SETTINGS_PREFIX,
  PANEL,
  SETTINGS,
  admin,
  invitations,
  isAdminPath as isAdminRoute,
  isOrgPath,
  isSettingsPath as isSettingsRoute,
  rescope,
  settings as settingsPaths,
  unscope,
  workbench
} from 'routes/paths';
import {
  BarChart3,
  BookOpen,
  Boxes,
  Building2,
  CreditCard,
  Globe,
  Landmark,
  LogIn,
  Bell,
  Mail,
  MessagesSquare,
  ShieldCheck,
  FileText,
  MailPlus,
  KeyRound,
  LayoutDashboard,
  ListChecks,
  Network,
  PackageOpen,
  Play,
  ReceiptText,
  Route,
  ScrollText,
  Settings,
  ShieldUser,
  SlidersHorizontal,
  Waypoints,
  Ticket,
  User,
  Users,
  UsersRound,
  Wallet
} from 'lucide-react';

// ==============================|| CHROME — NAV CONFIG ||============================== //
// Single source of truth for the panel navigation. One sidebar shell serves three contexts:
//   personal  the workbench of the signed-in user
//   org       the same workbench scoped to the active organization (plus the 组织 group)
//   admin     platform administration (admins only)
// The context is derived from the route (admin pages) and the org context (personal vs org),
// The context is the URL's: `/panel/admin/...` is platform admin, `/panel/orgs/:orgSlug/...` an
// organization's workbench, other workbench pages the personal one (routes/paths.js). Workbench
// items hold their personal URL; in an organization they resolve under its prefix. Labels resolve through `labelKey` (our own locale files, `nav.*`);
// page subtitles keep resolving through `nav_subtitles.<item id>`.
//
// Item fields:
//   url          link target (personal form for workbench items); `:orgSlug` is replaced with the
//                active organization's slug
//   match        extra paths (segment prefixes) or RegExps that make the item active
//   orgAdmin     org context, owner/admin only
//   root         root admins only
//   requires     a site feature that must be on (`features` of the context, e.g. routingPolicy)
//   pageIn       contexts where the page works even though the item is not in the menu
//                (used when switching context to decide whether to stay on the page)

export const DASHBOARD_URL = workbench.dashboard();
// 设置 (`/panel/settings/*`): its own sidebar (SettingsSidebar) with 组织设置 first, headed by a
// picker of the organizations the user manages, then 个人设置. The account menu's 设置 and the
// settings button next to the account row open SETTINGS_ROOT, which lands on the first item.
// Platform settings belong to 平台管理.
export const SETTINGS_ROOT = SETTINGS;
export const isSettingsPath = isSettingsRoute;
export const ORG_PARAM = ':orgSlug';
// An organization settings URL with the slug left as a parameter.
const orgSettingsPattern = (section) => `${ORG_SETTINGS_PREFIX}/${ORG_PARAM}/${section}`;

// ---------------------------------------------------------------- workbench (personal / org)

export const workbenchSections = [
  {
    id: 'main',
    items: [
      { id: 'dashboard', url: workbench.dashboard(), icon: LayoutDashboard, labelKey: 'nav.items.dashboard' },
      { id: 'usage', url: workbench.usage(), icon: BarChart3, labelKey: 'nav.items.usage' },
      { id: 'log', url: workbench.logs(), icon: FileText, labelKey: 'nav.items.log' }
    ]
  },
  {
    id: 'develop',
    labelKey: 'nav.sections.develop',
    items: [
      { id: 'token', url: workbench.keys(), icon: KeyRound, labelKey: 'nav.items.token' },
      { id: 'model_price', url: workbench.models(), icon: PackageOpen, labelKey: 'nav.items.models' },
      // Routing policies the context's keys call: personal ones, or the organization's.
      { id: 'routing', url: workbench.routingPolicies(), icon: Route, labelKey: 'nav.items.routingPolicies', requires: 'routingPolicy' },
      // One item for the five API consoles (`/panel/playground/:modality`); the console pages
      // switch modality in-page.
      { id: 'playground', url: workbench.playground(), match: [`${PANEL}/playground`], icon: Play, labelKey: 'nav.items.playground' },
      // The docs (the API overview and every modality page) share one title.
      { id: 'api_docs', url: workbench.docs('chat'), match: [workbench.docs()], icon: BookOpen, labelKey: 'nav.items.docs' }
    ]
  },
  {
    id: 'organization',
    labelKey: 'nav.sections.organization',
    orgOnly: true,
    items: [
      // Org console (overview / usage / audit, leaving the organization). Visible to every
      // member, so members keep a way to reach it. Org-only: its url is the personal form of
      // `/panel/orgs/:orgSlug/overview`, like every workbench item.
      { id: 'organization', url: `${PANEL}/overview`, icon: Building2, labelKey: 'nav.items.orgOverview' },
      {
        id: 'org_members',
        url: orgSettingsPattern('members'),
        match: [/^\/panel\/settings\/orgs\/[^/]+\/members(\/|$)/],
        icon: UsersRound,
        labelKey: 'nav.items.orgMembers',
        orgAdmin: true
      },
      // Model access (ORG-15): approved models, the organization default and named lists.
      {
        id: 'org_models',
        url: orgSettingsPattern('model-access'),
        match: [/^\/panel\/settings\/orgs\/[^/]+\/model-access(\/|$)/],
        icon: ListChecks,
        labelKey: 'nav.items.orgModelAccess',
        orgAdmin: true
      },
      { id: 'billing', url: workbench.billing(), icon: ReceiptText, labelKey: 'nav.items.billing', orgAdmin: true, pageIn: ['personal'] }
    ]
  }
];

// ---------------------------------------------------------------- platform admin

// The platform settings groups are menu items of their own, each filed under the section it
// configures; every group is a route of the one settings page (`/panel/admin/settings/:group`), whose
// header shows the group's description.
const settingItem = (groupId, icon, labelKey, extra = {}) => ({
  id: `setting_${groupId}`,
  url: settingsGroupUrl(groupId),
  icon,
  labelKey,
  subtitleKey: groupDescriptionKey(groupId),
  ...extra
});

export const adminSections = [
  {
    id: 'admin_main',
    items: [
      // 统计 comes first: entering platform admin lands on the first visible item.
      // Analytics and per-user stats share one item; the pages switch with in-page tabs.
      { id: 'analytics', url: admin.stats(), icon: BarChart3, labelKey: 'nav.items.stats' },
      { id: 'systemInfo', url: admin.systemLogs(), icon: ScrollText, labelKey: 'nav.items.systemLogs', root: true }
    ]
  },
  {
    id: 'gateway',
    labelKey: 'nav.sections.gateway',
    items: [
      { id: 'channel', url: admin.providers(), icon: Network, labelKey: 'nav.items.channels' },
      { id: 'pricing', url: admin.models(), icon: Boxes, labelKey: 'nav.items.catalog' },
      { id: 'routing_policy', url: admin.routing(), icon: Route, labelKey: 'nav.items.routing' },
      settingItem('gateway', Waypoints, 'nav.items.settingGateway')
    ]
  },
  {
    id: 'people',
    labelKey: 'nav.sections.people',
    items: [
      { id: 'user', url: admin.users(), icon: User, labelKey: 'nav.items.users' },
      { id: 'user_group', url: admin.userGroups(), icon: Users, labelKey: 'nav.items.userGroups' },
      { id: 'org_admin', url: admin.orgs(), icon: Building2, labelKey: 'nav.items.orgs', root: true },
      settingItem('auth', LogIn, 'nav.items.settingAuth'),
      settingItem('orgpolicy', Landmark, 'nav.items.settingOrgPolicy', { root: true })
    ]
  },
  {
    id: 'finance',
    labelKey: 'nav.sections.finance',
    items: [
      { id: 'payment', url: admin.orders(), icon: Wallet, labelKey: 'nav.items.orders' },
      { id: 'redemption', url: admin.redemptionCodes(), icon: Ticket, labelKey: 'nav.items.redemption' },
      { id: 'invite_code', url: admin.inviteCodes(), icon: MailPlus, labelKey: 'nav.items.inviteCodes' },
      settingItem('billing', ReceiptText, 'nav.items.settingBilling'),
      settingItem('payment', CreditCard, 'nav.items.settingPayment')
    ]
  },
  {
    id: 'system',
    labelKey: 'nav.sections.system',
    items: [
      // Bare `/panel/admin/settings` redirects to 站点, so it resolves to that item.
      settingItem('site', Globe, 'nav.items.settingSite', { match: [SETTINGS_BASE] }),
      settingItem('entries', MessagesSquare, 'nav.items.settingEntries'),
      settingItem('privacy', ShieldCheck, 'nav.items.settingPrivacy'),
      settingItem('mail', Mail, 'nav.items.settingMail'),
      settingItem('notify', Bell, 'nav.items.settingNotify')
    ]
  }
];

// Pages without a menu item: the two parts of 设置 (opened from the account menu and the
// settings button, never from the menu). They match settings paths for the command palette and
// for context switching; 设置 itself is titled from its own tree (settings-tree.js).
const orgSettingsItem = {
  id: 'org_settings',
  url: orgSettingsPattern('general'),
  match: [/^\/panel\/settings\/orgs\/[^/]+\/(general|danger|budget)(\/|$)/],
  icon: Settings,
  labelKey: 'nav.items.orgSettings',
  orgAdmin: true
};
const personalSettingsItem = {
  id: 'personal_settings',
  url: settingsPaths.root(),
  match: ACCOUNT_GROUPS.map((group) => settingsPaths.account(group.id)),
  icon: Settings,
  labelKey: 'nav.items.personalSettings',
  pageIn: ['personal', 'org']
};
// The personal invitations page (reached from the context switcher's organizations and invite
// links), titled like the organization console it leads to.
const invitationsItem = { id: 'invitations', url: invitations(), icon: Building2, labelKey: 'nav.items.orgOverview', pageIn: ['personal'] };
export const hiddenNavItems = [orgSettingsItem, personalSettingsItem, invitationsItem];

const workbenchItems = workbenchSections.flatMap((section) => section.items);
const adminItems = adminSections.flatMap((section) => section.items);
const adminItemIds = new Set(adminItems.map((item) => item.id));
const allItems = [...workbenchItems, ...adminItems, ...hiddenNavItems];

// ---------------------------------------------------------------- matching

const segmentPrefix = (pathname, prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`);

// Length of the best match of `item` against `pathname`, or 0. Longer matches win, so a docs
// page resolves to 文档 rather than 演练场. Workbench items match in every scope: an organization's
// pages are compared in their personal form.
function matchLength(item, pathname) {
  let best = 0;
  const path = unscope(pathname);
  const candidates = [item.url.includes(ORG_PARAM) ? null : item.url, ...(item.match || [])].filter(Boolean);
  for (const candidate of candidates) {
    if (candidate instanceof RegExp) {
      const m = path.match(candidate);
      if (m && m.index === 0) best = Math.max(best, m[0].length);
    } else if (segmentPrefix(path, candidate)) {
      best = Math.max(best, candidate.length);
    }
  }
  return best;
}

export function findActiveItem(pathname) {
  let match = null;
  let length = 0;
  for (const item of allItems) {
    const l = matchLength(item, pathname);
    if (l > length) {
      match = item;
      length = l;
    }
  }
  return match;
}

export const isItemActive = (item, pathname) => findActiveItem(pathname)?.id === item.id;

// Whether a pathname belongs to platform admin (`/panel/admin/...`, or an old admin URL that is
// about to redirect there).
export function isAdminPath(pathname) {
  return isAdminRoute(pathname) || LEGACY_ADMIN_PATHS.some((u) => segmentPrefix(pathname, u));
}

// ---------------------------------------------------------------- contexts and filtering

export const isOrgAdminRole = (role) => role === 'owner' || role === 'admin';

// The shell context of a page, from its URL. Non-admins on admin routes keep the workbench chrome
// (the route guard renders its 404 in place). 设置 is not a workbench scope; it keeps the active
// organization's context for its back row and palette.
export function deriveContext({ pathname, isAdmin = false, orgId = null }) {
  if (isAdmin && isAdminPath(pathname)) return 'admin';
  if (isOrgPath(pathname)) return 'org';
  if (isSettingsPath(pathname)) return orgId ? 'org' : 'personal';
  return 'personal';
}

const workbenchItemIds = new Set(workbenchItems.map((item) => item.id));

// An item's link in a context: `:orgSlug` filled in, workbench items under the organization's
// prefix in an organization.
export function itemUrl(item, orgSlug = null, context = orgSlug ? 'org' : 'personal') {
  if (item.url.includes(ORG_PARAM)) return item.url.replace(ORG_PARAM, encodeURIComponent(orgSlug ?? ''));
  if (context === 'org' && orgSlug && workbenchItemIds.has(item.id)) return rescope(item.url, orgSlug);
  return item.url;
}

// Whether an item shows in the menu of `context`.
export function isItemVisible(item, { context, orgRole = null, isRoot = false, features = {} }) {
  if (item.requires && !features?.[item.requires]) return false;
  if (adminItemIds.has(item.id)) return context === 'admin' && (!item.root || isRoot);
  if (context === 'admin') return false;
  if (item.orgAdmin) return context === 'org' && isOrgAdminRole(orgRole);
  if (hiddenNavItems.includes(item)) return false;
  return true;
}

const isOrgOnlyItem = (item) => workbenchSections.some((section) => section.orgOnly && section.items.includes(item));

function itemShows(item, ctx) {
  if (isOrgOnlyItem(item) && ctx.context !== 'org') return false;
  return isItemVisible(item, ctx);
}

// Whether the page of `item` works in `context` (menu item or `pageIn`).
export function isPageAvailable(item, ctx) {
  return itemShows(item, ctx) || Boolean(item.pageIn?.includes(ctx.context));
}

// Menu sections of a context with urls resolved; empty sections are dropped.
export function navSectionsFor({ context, orgRole = null, orgSlug = null, isRoot = false, features = {} }) {
  const sections = context === 'admin' ? adminSections : workbenchSections;
  const ctx = { context, orgRole, isRoot, features };
  return sections
    .filter((section) => !section.orgOnly || context === 'org')
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => itemShows(item, ctx)).map((item) => ({ ...item, href: itemUrl(item, orgSlug, context) }))
    }))
    .filter((section) => section.items.length > 0);
}

export function firstAdminUrl({ isRoot = false } = {}) {
  return navSectionsFor({ context: 'admin', isRoot })[0].items[0].href;
}

// Where to go after switching context.
//   to: { kind: 'personal' } | { kind: 'org', orgSlug, role } | { kind: 'admin' }
// Entering admin lands on its first visible page; leaving admin lands on the target workbench's
// dashboard. 设置 does not depend on the workbench, so it stays. Between workbenches the page stays
// (same path, other scope prefix) when it exists in the target context, otherwise the dashboard.
export function contextSwitchTarget({ pathname, to, isRoot = false, features = {} }) {
  if (to.kind === 'admin') return firstAdminUrl({ isRoot });
  const orgSlug = to.kind === 'org' ? to.orgSlug : null;
  const dashboard = workbench.dashboard(orgSlug);
  if (isSettingsPath(pathname)) return pathname;
  if (isAdminPath(pathname)) return dashboard;
  const item = findActiveItem(pathname);
  const context = orgSlug ? 'org' : 'personal';
  if (!item || !isPageAvailable(item, { context, orgRole: to.role ?? null, isRoot, features })) return dashboard;
  return rescope(pathname, orgSlug);
}

// Command palette: the current workbench plus, for admins, every platform admin page under one
// heading (with an entry for platform admin itself).
export function commandSections({ isAdmin = false, isRoot = false, orgSlug = null, orgRole = null, features = {} }) {
  const context = orgSlug ? 'org' : 'personal';
  const workbenchNav = navSectionsFor({ context, orgSlug, orgRole, isRoot, features });
  // The two parts of 设置: the active organization's settings (owners and admins) and personal,
  // plus every item of their tree while typing.
  const orgAdminHere = context === 'org' && isOrgAdminRole(orgRole);
  const treeItem = (part, group, href) => ({
    id: `settings_${part}_${group.id}`,
    icon: group.icon,
    labelKey: group.labelKey,
    prefixKey: `settingsUi.sections.${part}`,
    href,
    searchOnly: true
  });
  const extras = [
    ...(orgAdminHere ? [{ ...orgSettingsItem, href: itemUrl(orgSettingsItem, orgSlug) }] : []),
    { ...personalSettingsItem, href: personalSettingsItem.url },
    ...(orgAdminHere ? ORG_TREE.map((group) => treeItem('org', group, orgSettingsUrl(orgSlug, group.id))) : []),
    ...ACCOUNT_GROUPS.map((group) => treeItem('personal', group, accountSettingsUrl(group.id)))
  ];
  const sections = [...workbenchNav, { id: 'account', items: extras }];
  if (!isAdmin) return sections;
  const admin = navSectionsFor({ context: 'admin', isRoot }).flatMap((section) => section.items);
  const home = { id: 'admin_home', icon: ShieldUser, labelKey: 'nav.context.admin', href: firstAdminUrl({ isRoot }) };
  return [
    ...sections,
    { id: 'admin', admin: true, labelKey: 'nav.context.admin', items: [home, ...admin] },
    { id: 'platform_settings', admin: true, labelKey: 'nav.items.platformSettings', items: settingsCommandItems({ isRoot }) }
  ];
}

// Command palette entries for the platform settings cards. They only show while typing
// (`searchOnly`), so the palette indexes the cards without listing ~40 of them; the groups are
// admin menu items and already listed.
export function settingsCommandItems({ isRoot = false } = {}) {
  const groups = PLATFORM_GROUPS.filter((group) => !group.rootOnly || isRoot);
  return groups.flatMap((group) =>
    group.cards.map((card) => ({
      id: `setting_card_${card.id}`,
      icon: SlidersHorizontal,
      labelKey: cardTitleKey(card.id),
      prefixKey: groupLabelKey(group.id),
      searchOnly: true,
      href: `${settingsGroupUrl(group.id)}?highlight=${card.id}`
    }))
  );
}

export function navLabel(t, item) {
  const label = t(item.labelKey);
  return item.prefixKey ? `${t(item.prefixKey)} · ${label}` : label;
}
