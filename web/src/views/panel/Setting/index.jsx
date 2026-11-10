import { Navigate, useParams, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';

import SettingsLayout from '@/components/chrome/SettingsLayout';

import { useIsAdmin, useIsRoot } from 'utils/common';
import useHighlightTarget from 'hooks/useHighlightTarget';
import { useOptionSettings } from './parts';
import { THEME_META } from './themeMeta';
import SiteTheme from './themes/SiteTheme';
import AuthTheme from './themes/AuthTheme';
import BillingTheme from './themes/BillingTheme';
import RelayTheme from './themes/RelayTheme';
import PrivacyTheme from './themes/PrivacyTheme';
import IntegrationsTheme from './themes/IntegrationsTheme';
import OrganizationTheme from './themes/OrganizationTheme';

// ==============================|| PANEL — SYSTEM SETTINGS (admin) ||============================== //
// Seven theme pages, one per `/panel/setting/:section` route, each a single column of
// cards. The sidebar has one 平台设置 item; the themes are this page's in-page subnav.
// Every option key maps 1:1 to the `/api/option/` API for functional parity.
// The 13 legacy section ids and the older `?tab=` values redirect to their theme
// with `?highlight=<card>` so old deep links land on the same card.

const THEME_COMPONENTS = {
  site: SiteTheme,
  auth: AuthTheme,
  billing: BillingTheme,
  relay: RelayTheme,
  privacy: PrivacyTheme,
  integrations: IntegrationsTheme,
  organization: OrganizationTheme
};

// Theme list (id / order / label / root-only) comes from `./themeMeta`.
export const THEMES = THEME_META.map((theme) => ({ ...theme, Component: THEME_COMPONENTS[theme.id] }));

const DEFAULT_THEME = THEMES[0].id;

// 旧 section id → 新主题 + 落点卡片。
const LEGACY_SECTION_MAP = {
  general: { theme: 'site', highlight: 'server-address' },
  branding: { theme: 'site', highlight: 'branding' },
  email: { theme: 'site', highlight: 'smtp' },
  oauth: { theme: 'auth', highlight: 'login-methods' },
  login: { theme: 'auth', highlight: 'login-methods' },
  invite: { theme: 'auth', highlight: 'invite-code' },
  payment: { theme: 'billing', highlight: 'payment' },
  gateways: { theme: 'billing', highlight: 'payment-gateways' },
  model: { theme: 'relay', highlight: 'native-protocol' },
  image: { theme: 'relay', highlight: 'image-proxy' },
  ops: { theme: 'relay', highlight: 'channel-auto' },
  safety: { theme: 'privacy', highlight: 'safety' },
  chat: { theme: 'integrations', highlight: 'chat' },
  org: { theme: 'organization', highlight: 'org-policy' }
};

// 更老的 `/panel/setting?tab=xxx`:先折叠到旧 section id,再走上面的映射。
const LEGACY_TAB_MAP = { other: 'branding' };

// 旧链接自带的 `?highlight=`(如日志详情的留存开关)决定落到哪个主题。
const HIGHLIGHT_THEME_MAP = { 'log-io': 'privacy' };

// 旧 section(可选叠加旧 tab)解析成新主题路径;未知值落到默认主题。
const resolveLegacyTarget = (section, highlight) => {
  const legacy = LEGACY_SECTION_MAP[section];
  const theme = HIGHLIGHT_THEME_MAP[highlight] || legacy?.theme || DEFAULT_THEME;
  const anchor = highlight || legacy?.highlight;
  return anchor ? `/panel/setting/${theme}?highlight=${anchor}` : `/panel/setting/${theme}`;
};

export default function Setting() {
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const ctx = useOptionSettings();
  const { section } = useParams();
  const [searchParams] = useSearchParams();
  useHighlightTarget();

  if (!isAdmin) return <Navigate to="/panel/dashboard" replace />;

  // Root-only themes (组织策略) are hidden from non-root admins entirely.
  const themes = THEMES.filter((s) => !s.rootOnly || isRoot);
  const themeIds = themes.map(({ id }) => id);
  const highlight = searchParams.get('highlight');

  // Bare `/panel/setting`: honor a legacy `?tab=` link, otherwise land on the
  // first theme.
  if (!section) {
    const legacyTab = searchParams.get('tab');
    if (!legacyTab && !highlight) return <Navigate to={`/panel/setting/${DEFAULT_THEME}`} replace />;
    return <Navigate to={resolveLegacyTarget(LEGACY_TAB_MAP[legacyTab] || legacyTab, highlight)} replace />;
  }

  // Legacy / unknown / root-gated section id redirects to the owning theme.
  if (!themeIds.includes(section)) {
    return <Navigate to={resolveLegacyTarget(section, highlight)} replace />;
  }

  const Active = themes.find(({ id }) => id === section).Component;
  const items = themes.map((theme) => ({
    id: theme.id,
    label: t(theme.labelKey, { defaultValue: theme.fallback }),
    to: `/panel/setting/${theme.id}`
  }));

  return (
    <SettingsLayout label={t('nav.subnav.label')} items={items} activeId={section} className="max-w-7xl">
      <div className="max-w-5xl">
        <Active ctx={ctx} />
      </div>
    </SettingsLayout>
  );
}
