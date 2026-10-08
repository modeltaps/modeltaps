import { Database, KeyRound, ShieldCheck, SlidersHorizontal, User, Users, Wallet } from 'lucide-react';

import MembersTab from '../Organization/MembersTab';
import GeneralSection from './org/GeneralSection';
import BudgetSection from './org/BudgetSection';

// ==============================|| PANEL — SETTINGS SECTION REGISTRY ||============================== //
// Single source of truth for the two settings groups:组织分组的内容组件登记在 component 上,
// 个人账号分组由 `./account` 按 section id 分发。`icon` 供整页设置面的侧边栏折叠态使用。

export const ORG_SECTIONS = [
  { id: 'general', labelKey: 'settingsPage.sections.general', fallback: 'General', icon: SlidersHorizontal, component: GeneralSection },
  { id: 'members', labelKey: 'settingsPage.sections.members', fallback: 'Members', icon: Users, component: MembersTab },
  { id: 'budget', labelKey: 'settingsPage.sections.budget', fallback: 'Budget', icon: Wallet, component: BudgetSection }
];

export const ACCOUNT_SECTIONS = [
  { id: 'profile', labelKey: 'settingsPage.sections.profile', fallback: 'Profile', icon: User },
  { id: 'security', labelKey: 'settingsPage.sections.security', fallback: 'Sign-in & security', icon: ShieldCheck },
  { id: 'tokens', labelKey: 'settingsPage.sections.tokens', fallback: 'API Tokens', icon: KeyRound },
  { id: 'privacy', labelKey: 'settingsPage.sections.privacy', fallback: 'Data & privacy', icon: Database },
  { id: 'preferences', labelKey: 'settingsPage.sections.preferences', fallback: 'Preferences', icon: SlidersHorizontal }
];

export const DEFAULT_ORG_SECTION = ORG_SECTIONS[0].id;
export const DEFAULT_ACCOUNT_SECTION = ACCOUNT_SECTIONS[0].id;

export const orgSectionIds = ORG_SECTIONS.map(({ id }) => id);
export const accountSectionIds = ACCOUNT_SECTIONS.map(({ id }) => id);

export const orgSettingsUrl = (orgId, section) => `/panel/settings/org/${orgId}/${section}`;
export const accountSettingsUrl = (section) => `/panel/settings/account/${section}`;

export const SETTINGS_ROOT = '/panel/settings';

// 整页设置面:布局按路径切换 chrome(侧边栏 + 标题),所以路径解析放在 URL 的单一事实源这里。
export const isSettingsPath = (pathname) => pathname === SETTINGS_ROOT || pathname.startsWith(`${SETTINGS_ROOT}/`);

// `/panel/settings/org/:orgId/:section` → { orgScope: true, orgId, section }
// `/panel/settings/account/:section`    → { orgScope: false, orgId: null, section }
export function parseSettingsPath(pathname) {
  const rest = pathname.slice(SETTINGS_ROOT.length).split('/').filter(Boolean);
  if (rest[0] === 'org') return { orgScope: true, orgId: Number(rest[1]), section: rest[2] || null };
  if (rest[0] === 'account') return { orgScope: false, orgId: null, section: rest[1] || null };
  return { orgScope: false, orgId: null, section: null };
}
