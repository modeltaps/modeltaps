import { PLATFORM_GROUPS, cardTitleKey, groupLabelKey, settingsGroupUrl } from 'views/panel/Setting/registry';
import {
  Activity,
  BarChart3,
  BookOpen,
  Boxes,
  Building2,
  FileText,
  MailPlus,
  KeyRound,
  LayoutDashboard,
  Network,
  PackageOpen,
  Play,
  ReceiptText,
  Route,
  Settings,
  ShieldUser,
  SlidersHorizontal,
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
// so URLs never change. Labels resolve through `labelKey` (our own locale files, `nav.*`);
// page subtitles keep resolving through `nav_subtitles.<item id>`.
//
// Item fields:
//   url          link target; `:orgId` is replaced with the active organization id
//   match        extra paths (segment prefixes) or RegExps that make the item active
//   orgAdmin     org context, owner/admin only
//   root         root admins only
//   requires     a site feature that must be on (`features` of the context, e.g. routingPolicy)
//   pageIn       contexts where the page works even though the item is not in the menu
//                (used when switching context to decide whether to stay on the page)

export const DASHBOARD_URL = '/panel/dashboard';
// 设置 (`/panel/settings/*`): its own sidebar (SettingsSidebar) with 组织设置 first, headed by a
// picker of the organizations the user manages, then 个人设置. The account menu's 设置 and the
// settings button next to the account row open SETTINGS_ROOT, which lands on the first item.
// Platform settings belong to 平台管理.
export const SETTINGS_ROOT = '/panel/settings';
export const isSettingsPath = (pathname) => pathname === SETTINGS_ROOT || pathname.startsWith(`${SETTINGS_ROOT}/`);
export const ORG_PARAM = ':orgId';

const url = (id) => `/panel/${id}`;
const ORG_SETTINGS_PREFIX = /^\/panel\/settings\/org\/[^/]+/;

// ---------------------------------------------------------------- workbench (personal / org)

export const workbenchSections = [
  {
    id: 'main',
    items: [
      { id: 'dashboard', url: url('dashboard'), icon: LayoutDashboard, labelKey: 'nav.items.dashboard' },
      { id: 'usage', url: url('usage'), icon: BarChart3, labelKey: 'nav.items.usage' },
      { id: 'log', url: url('log'), icon: FileText, labelKey: 'nav.items.log' }
    ]
  },
  {
    id: 'develop',
    labelKey: 'nav.sections.develop',
    items: [
      { id: 'token', url: url('token'), icon: KeyRound, labelKey: 'nav.items.token' },
      { id: 'model_price', url: url('model_price'), icon: PackageOpen, labelKey: 'nav.items.models' },
      // Routing policies the context's keys call: personal ones, or the organization's.
      { id: 'routing', url: url('routing'), icon: Route, labelKey: 'nav.items.routingPolicies', requires: 'routingPolicy' },
      // One item for the five API consoles (`/panel/api/:modality`); the console pages switch
      // modality in-page. `/panel/api` itself (the API overview) is not part of it.
      {
        id: 'playground',
        url: url('api/chat'),
        match: [/^\/panel\/api\/(?!docs(\/|$))[^/]+/],
        icon: Play,
        labelKey: 'nav.items.playground'
      },
      // The docs share one title for every modality page.
      { id: 'api_docs', url: url('api/docs/chat'), match: [url('api/docs')], icon: BookOpen, labelKey: 'nav.items.docs' }
    ]
  },
  {
    id: 'organization',
    labelKey: 'nav.sections.organization',
    orgOnly: true,
    items: [
      // Org console (overview / usage / audit, leaving the organization). Visible to every
      // member, so members keep a way to reach it; in personal context the same route shows
      // the user's pending invitations.
      {
        id: 'organization',
        url: url('organization'),
        icon: Building2,
        labelKey: 'nav.items.orgOverview',
        pageIn: ['personal']
      },
      {
        id: 'org_members',
        url: `/panel/settings/org/${ORG_PARAM}/members`,
        match: [/^\/panel\/settings\/org\/[^/]+\/members(\/|$)/],
        icon: UsersRound,
        labelKey: 'nav.items.orgMembers',
        orgAdmin: true
      },
      { id: 'billing', url: url('billing'), icon: ReceiptText, labelKey: 'nav.items.billing', orgAdmin: true, pageIn: ['personal'] }
    ]
  }
];

// ---------------------------------------------------------------- platform admin

export const adminSections = [
  {
    id: 'admin_main',
    items: [
      { id: 'systemInfo', url: url('system_info'), icon: Activity, labelKey: 'nav.items.health', root: true },
      // Analytics and per-user stats share one item; the pages switch with in-page tabs.
      { id: 'analytics', url: url('analytics'), match: [url('multi_user_stats')], icon: BarChart3, labelKey: 'nav.items.stats' }
    ]
  },
  {
    id: 'gateway',
    labelKey: 'nav.sections.gateway',
    items: [
      { id: 'channel', url: url('channel'), icon: Network, labelKey: 'nav.items.channels' },
      { id: 'pricing', url: url('pricing'), icon: Boxes, labelKey: 'nav.items.catalog' },
      { id: 'routing_policy', url: url('routing_policy'), icon: Route, labelKey: 'nav.items.routing' }
    ]
  },
  {
    id: 'people',
    labelKey: 'nav.sections.people',
    items: [
      { id: 'user', url: url('user'), icon: User, labelKey: 'nav.items.users' },
      { id: 'user_group', url: url('user_group'), icon: Users, labelKey: 'nav.items.userGroups' },
      { id: 'org_admin', url: url('org_admin'), icon: Building2, labelKey: 'nav.items.orgs', root: true }
    ]
  },
  {
    id: 'finance',
    labelKey: 'nav.sections.finance',
    items: [
      { id: 'payment', url: url('payment'), icon: Wallet, labelKey: 'nav.items.orders' },
      { id: 'redemption', url: url('redemption'), icon: Ticket, labelKey: 'nav.items.redemption' },
      { id: 'invite_code', url: url('invite_code'), icon: MailPlus, labelKey: 'nav.items.inviteCodes' }
    ]
  },
  {
    id: 'admin_settings',
    labelKey: 'nav.sections.settings',
    items: [
      // One item for the platform settings; its groups are an in-page subnav.
      { id: 'platform_settings', url: url('setting'), icon: SlidersHorizontal, labelKey: 'nav.items.platformSettings' }
    ]
  }
];

// Pages without a menu item: the two parts of 设置 (opened from the account menu and the
// settings button, never from the menu). They match settings paths for the command palette and
// for context switching; 设置 itself is titled from its own tree (settings-tree.js).
const orgSettingsItem = {
  id: 'org_settings',
  url: `/panel/settings/org/${ORG_PARAM}/general`,
  match: [/^\/panel\/settings\/org\/[^/]+\/(general|danger|budget)(\/|$)/],
  icon: Settings,
  labelKey: 'nav.items.orgSettings',
  orgAdmin: true
};
const personalSettingsItem = {
  id: 'personal_settings',
  url: url('settings'),
  match: [url('settings/account')],
  icon: Settings,
  labelKey: 'nav.items.personalSettings',
  pageIn: ['personal', 'org']
};
export const hiddenNavItems = [orgSettingsItem, personalSettingsItem];

// Legacy admin routes that redirect into admin pages; they keep the admin chrome during the
// redirect so the sidebar does not flash the workbench.
const LEGACY_ADMIN_URLS = [url('telegram'), url('model_ownedby'), url('model_info')];

const workbenchItems = workbenchSections.flatMap((section) => section.items);
const adminItems = adminSections.flatMap((section) => section.items);
const adminItemIds = new Set(adminItems.map((item) => item.id));
const allItems = [...workbenchItems, ...adminItems, ...hiddenNavItems];

// ---------------------------------------------------------------- matching

const segmentPrefix = (pathname, prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`);

// Length of the best match of `item` against `pathname`, or 0. Longer matches win, so a docs
// page resolves to 文档 rather than 演练场.
function matchLength(item, pathname) {
  let best = 0;
  const candidates = [item.url.includes(ORG_PARAM) ? null : item.url, ...(item.match || [])].filter(Boolean);
  for (const candidate of candidates) {
    if (candidate instanceof RegExp) {
      const m = pathname.match(candidate);
      if (m && m.index === 0) best = Math.max(best, m[0].length);
    } else if (segmentPrefix(pathname, candidate)) {
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

// Whether a pathname belongs to platform admin (exact match or sub-path).
export function isAdminPath(pathname) {
  if (LEGACY_ADMIN_URLS.some((u) => segmentPrefix(pathname, u))) return true;
  const item = findActiveItem(pathname);
  return Boolean(item && adminItemIds.has(item.id));
}

// ---------------------------------------------------------------- contexts and filtering

export const isOrgAdminRole = (role) => role === 'owner' || role === 'admin';

// The shell context of a page. Non-admins on admin routes keep the workbench chrome (the
// route guard renders its 404 in place).
export function deriveContext({ pathname, isAdmin = false, orgId = null }) {
  if (isAdmin && isAdminPath(pathname)) return 'admin';
  return orgId ? 'org' : 'personal';
}

export function itemUrl(item, orgId = null) {
  return item.url.includes(ORG_PARAM) ? item.url.replace(ORG_PARAM, String(orgId ?? '')) : item.url;
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
export function navSectionsFor({ context, orgRole = null, orgId = null, isRoot = false, features = {} }) {
  const sections = context === 'admin' ? adminSections : workbenchSections;
  const ctx = { context, orgRole, isRoot, features };
  return sections
    .filter((section) => !section.orgOnly || context === 'org')
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => itemShows(item, ctx)).map((item) => ({ ...item, href: itemUrl(item, orgId) }))
    }))
    .filter((section) => section.items.length > 0);
}

export function firstAdminUrl({ isRoot = false } = {}) {
  return navSectionsFor({ context: 'admin', isRoot })[0].items[0].href;
}

// Where to go after switching context, or null to stay on the current page.
//   to: { kind: 'personal' } | { kind: 'org', orgId, role } | { kind: 'admin' }
// Entering admin lands on its first visible page; leaving admin lands on the dashboard.
// Between workbench contexts the page stays when it exists in the target context (org-scoped
// pages are retargeted to the new organization), otherwise the dashboard.
export function contextSwitchTarget({ pathname, to, isRoot = false, features = {} }) {
  if (to.kind === 'admin') return firstAdminUrl({ isRoot });
  if (isAdminPath(pathname)) return DASHBOARD_URL;
  const item = findActiveItem(pathname);
  if (!item) return null;
  const context = to.kind === 'org' ? 'org' : 'personal';
  if (!isPageAvailable(item, { context, orgRole: to.role ?? null, isRoot, features })) return DASHBOARD_URL;
  if (ORG_SETTINGS_PREFIX.test(pathname)) return pathname.replace(ORG_SETTINGS_PREFIX, `/panel/settings/org/${to.orgId}`);
  return null;
}

// Command palette: the current workbench plus, for admins, every platform admin page under one
// heading (with an entry for platform admin itself).
export function commandSections({ isAdmin = false, isRoot = false, orgId = null, orgRole = null, features = {} }) {
  const context = orgId ? 'org' : 'personal';
  const workbench = navSectionsFor({ context, orgId, orgRole, isRoot, features });
  // The two parts of 设置: the active organization's settings (owners and admins) and personal.
  const extras = [
    ...(context === 'org' && isOrgAdminRole(orgRole) ? [{ ...orgSettingsItem, href: itemUrl(orgSettingsItem, orgId) }] : []),
    { ...personalSettingsItem, href: personalSettingsItem.url }
  ];
  const sections = [...workbench, { id: 'account', items: extras }];
  if (!isAdmin) return sections;
  const admin = navSectionsFor({ context: 'admin', isRoot }).flatMap((section) => section.items);
  const home = { id: 'admin_home', icon: ShieldUser, labelKey: 'nav.context.admin', href: firstAdminUrl({ isRoot }) };
  return [
    ...sections,
    { id: 'admin', admin: true, labelKey: 'nav.context.admin', items: [home, ...admin] },
    { id: 'platform_settings', admin: true, labelKey: 'nav.items.platformSettings', items: settingsCommandItems({ isRoot }) }
  ];
}

// Command palette entries for the platform settings: one per group, and one per card that only
// shows while typing (`searchOnly`), so the palette indexes the cards without listing ~40 of them.
export function settingsCommandItems({ isRoot = false } = {}) {
  const groups = PLATFORM_GROUPS.filter((group) => !group.rootOnly || isRoot);
  const groupItems = groups.map((group) => ({
    id: `setting_${group.id}`,
    icon: SlidersHorizontal,
    labelKey: groupLabelKey(group.id),
    prefixKey: 'nav.items.platformSettings',
    href: settingsGroupUrl(group.id)
  }));
  const cardItems = groups.flatMap((group) =>
    group.cards.map((card) => ({
      id: `setting_card_${card.id}`,
      icon: SlidersHorizontal,
      labelKey: cardTitleKey(card.id),
      prefixKey: groupLabelKey(group.id),
      searchOnly: true,
      href: `${settingsGroupUrl(group.id)}?highlight=${card.id}`
    }))
  );
  return [...groupItems, ...cardItems];
}

export function navLabel(t, item) {
  const label = t(item.labelKey);
  return item.prefixKey ? `${t(item.prefixKey)} · ${label}` : label;
}
