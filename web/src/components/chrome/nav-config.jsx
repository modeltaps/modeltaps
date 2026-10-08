import {
  LayoutDashboard,
  BarChart3,
  Building2,
  ChartColumnBig,
  MessageSquare,
  Activity,
  AudioLines,
  Clapperboard,
  Columns2,
  User,
  Network,
  Users,
  ReceiptText,
  Boxes,
  Ticket,
  Wallet,
  KeyRound,
  Settings,
  CreditCard,
  PackageOpen,
  FileText,
  BookOpen,
  Image
} from 'lucide-react';

import { THEME_META } from 'views/panel/Setting/themeMeta';

// ==============================|| CHROME NAV CONFIG ||============================== //
// The single source of truth for navigation: a flat list of labelled sections
// (`nav_sec_*`). Admin-only items carry `isAdmin: true`;
// labels are resolved at render time via navLabel (item.labelKey, else t(id)).
// Non-admins see no admin sections at all (filterSections drops them).

const url = (id) => `/panel/${id}`;

// 系统设置的 7 个主题项直接由 THEME_META 生成(同一份 id / 顺序 / 标签 / root 限定),
// 避免侧边栏与设置页各自维护一份主题清单。
const themeItems = THEME_META.map((theme) => ({
  id: `setting_${theme.id}`,
  url: `/panel/setting/${theme.id}`,
  icon: theme.icon,
  labelKey: theme.labelKey,
  fallback: theme.fallback,
  isAdmin: true,
  ...(theme.rootOnly ? { isRoot: true } : {})
}));

export const navSections = [
  // N3: Grok-style layout. The build group (no section label) holds the five core
  // entries plus `billing` and `organization` (the latter org context only); the
  // API group lists the four modality consoles plus the API docs. The remaining account
  // entries (top-up / settings) have no sidebar entry — top-up is a tab of the
  // billing page, settings lives in the user menu — and keep their page titles
  // via `hiddenNavItems`.
  {
    id: 'nav_sec_build',
    items: [
      { id: 'dashboard', url: url('dashboard'), icon: LayoutDashboard },
      { id: 'token', url: url('token'), icon: KeyRound },
      { id: 'model_price', url: url('model_price'), icon: PackageOpen },
      { id: 'usage', url: url('usage'), icon: BarChart3 },
      { id: 'log', url: url('log'), icon: FileText },
      { id: 'billing', url: url('billing'), icon: ReceiptText, orgAdminOnly: true },
      { id: 'organization', url: url('organization'), icon: Building2, orgOnly: true }
    ]
  },
  {
    // 五个模态项是控制台页(`/panel/api/:modality`),接口文档单独一项收在组末尾。
    // 对比页没有对应的接口文档页。
    id: 'nav_sec_api',
    items: [
      { id: 'api_chat', url: url('api/chat'), icon: MessageSquare },
      { id: 'api_image', url: url('api/image'), icon: Image },
      { id: 'api_audio', url: url('api/speech'), icon: AudioLines },
      { id: 'api_compare', url: url('api/compare'), icon: Columns2 },
      { id: 'api_video', url: url('api/video'), icon: Clapperboard },
      { id: 'api_docs', url: url('api/docs/chat'), icon: BookOpen }
    ]
  },
  // 管理后台按管理对象分 5 组:概览 / 用户与组织 / 网关 / 财务 / 系统设置(最后)。
  // Telegram、模型归属、模型详情不再有侧栏入口(前者并入集成主题页,后两者并入模型页 Tab)。
  {
    id: 'nav_sec_overview',
    items: [
      { id: 'analytics', url: url('analytics'), icon: BarChart3, isAdmin: true },
      { id: 'multi_user_stats', url: url('multi_user_stats'), icon: ChartColumnBig, isAdmin: true },
      { id: 'systemInfo', url: url('system_info'), icon: Activity, isAdmin: true, isRoot: true }
    ]
  },
  {
    id: 'nav_sec_people',
    items: [
      { id: 'user', url: url('user'), icon: User, isAdmin: true },
      { id: 'user_group', url: url('user_group'), icon: Users, isAdmin: true },
      { id: 'org_admin', url: url('org_admin'), icon: Building2, isAdmin: true, isRoot: true }
    ]
  },
  {
    id: 'nav_sec_gateway',
    items: [
      { id: 'channel', url: url('channel'), icon: Network, isAdmin: true },
      { id: 'pricing', url: url('pricing'), icon: Boxes, isAdmin: true }
    ]
  },
  {
    id: 'nav_sec_finance',
    items: [
      { id: 'payment', url: url('payment'), icon: Wallet, isAdmin: true },
      { id: 'redemption', url: url('redemption'), icon: Ticket, isAdmin: true }
    ]
  },
  {
    id: 'nav_sec_system',
    items: themeItems
  }
];

// Pages that are reachable (footer balance row, user menu, direct URL) but have
// no sidebar entry. They take part in `findActiveItem` only, so the PageHeader
// still resolves a title/subtitle for them.
export const hiddenNavItems = [
  { id: 'topup', url: url('topup'), icon: CreditCard, hidden: true },
  { id: 'settings', url: url('settings'), icon: Settings, hidden: true },
  // 侧栏的文档项只指向对话文档;其余模态文档页靠这条前缀解析出同一个标题。
  { id: 'api_docs', url: url('api/docs'), icon: BookOpen, hidden: true }
];

// Flatten navSections (sections → items) into a list of routable items, then
// match the current pathname against the longest `item.url` (exact or prefix).
// Shared by the top Header and the in-body PageHeader so both resolve the same
// active menu item from a path.
const flatNavItems = [...navSections.flatMap((section) => section.items), ...hiddenNavItems];

export function findActiveItem(pathname) {
  let match = null;
  for (const item of flatNavItems) {
    if (pathname === item.url || pathname.startsWith(item.url)) {
      if (!match || item.url.length > match.url.length) match = item;
    }
  }
  return match;
}

// Mirror of v1 MenuList filtering: hide admin-only items for non-admins; gate
// `requiresInvoice` items behind `siteInfo.UserInvoiceMonth` and `requiresChat`
// items behind `siteInfo.builtin_chat_enabled` (callers pass the resolved flags).
// Sections whose items are all filtered out are dropped (so their labels never
// render).
export function filterSections({ isAdmin, isRoot = false, orgActive = false, invoiceEnabled = false, chatEnabled = true }) {
  const visible = (item) =>
    (!item.isAdmin || isAdmin) &&
    (!item.isRoot || isRoot) &&
    !(item.orgOnly && !orgActive) &&
    !(item.requiresInvoice && !invoiceEnabled) &&
    !(item.requiresChat && !chatEnabled);
  return navSections.map((section) => ({ ...section, items: section.items.filter(visible) })).filter((section) => section.items.length > 0);
}

// T19 split: the admin sections (`isAdmin: true` groups) render in a dedicated
// admin sidebar; the user sidebar drops them plus the `settings` item (the user
// menu already links to settings). Routes/urls are untouched.
// 系统设置是管理侧栏的最后一组(7 个主题平铺,root 限定项由 filterAdminSections 过滤)。
export const adminNavSections = navSections
  .map((section) => ({ ...section, items: section.items.filter((item) => item.isAdmin) }))
  .filter((section) => section.items.length > 0);

// Admin-sidebar sections with root-only items (`isRoot: true`) dropped for
// non-root admins, mirroring the backend RootAuth gating. `adminNavSections`
// (and `isAdminPath`) keep every admin url so the admin plane still resolves
// for root users.
export function filterAdminSections({ isRoot = false }) {
  const visible = (item) => !item.isRoot || isRoot;
  return adminNavSections
    .map((section) => ({ ...section, items: section.items.filter(visible) }))
    .filter((section) => section.items.length > 0);
}

// 已从侧边栏移除但仍在册的管理面路径:裸 `/panel/setting`(跳首个主题)与三条重定向旧
// 路由。重定向发生在路由层,这一瞬间也要用管理面 chrome,否则会闪一下用户侧栏。
const LEGACY_ADMIN_URLS = ['/panel/setting', '/panel/telegram', '/panel/model_ownedby', '/panel/model_info'];

const adminUrls = [...adminNavSections.flatMap((section) => section.items.map((item) => item.url)), ...LEGACY_ADMIN_URLS];

// Whether a pathname belongs to the admin plane (exact match or sub-path).
export function isAdminPath(pathname) {
  return adminUrls.some((u) => pathname === u || pathname.startsWith(`${u}/`));
}

// 条目标签:主题项带 `labelKey`(复用设置页的 sectionNav 文案),其余沿用 t(item.id)。
export function navLabel(t, item) {
  return item.labelKey ? t(item.labelKey, { defaultValue: item.fallback }) : t(item.id);
}

// User-sidebar sections: no admin items. `orgOnly` items only show in org
// context; the remaining gates (`hideInOrganization`, `orgAdminOnly`,
// `requiresInvoice`, `requiresChat`) stay available for future entries.
// The admin plane is entered from a pinned row at the end of the sidebar nav
// (rendered by Sidebar.jsx, admins only), so no admin group here.
export function filterUserSections({ orgActive = false, orgRole = null, invoiceEnabled = false, chatEnabled = true }) {
  const isOrgAdmin = orgRole === 'owner' || orgRole === 'admin';
  const visible = (item) =>
    !item.isAdmin &&
    !(item.orgOnly && !orgActive) &&
    !(item.hideInOrganization && orgActive) &&
    !(item.orgAdminOnly && orgActive && !isOrgAdmin) &&
    !(item.requiresInvoice && !invoiceEnabled) &&
    !(item.requiresChat && !chatEnabled);
  return navSections.map((section) => ({ ...section, items: section.items.filter(visible) })).filter((section) => section.items.length > 0);
}
