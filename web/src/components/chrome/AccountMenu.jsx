import PropTypes from 'prop-types';
import { useLocation, useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import {
  ArrowUpRight,
  BookOpen,
  Check,
  Globe,
  LogOut,
  Megaphone,
  Monitor,
  Moon,
  Palette,
  Settings,
  ShieldUser,
  Sun,
  Wallet
} from 'lucide-react';

import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import useLogin from 'hooks/useLogin';
import useTheme from 'hooks/useTheme';
import useI18n from 'hooks/useI18n';
import { setAppLanguage } from 'utils/userSetting';
import i18nList from 'i18n/i18nList';
import { useIsAdmin, useIsRoot } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import { useNotice } from 'ui-component/notice';
import { SETTINGS_ROOT, contextSwitchTarget } from './nav-config';
import UserAvatar, { accountDisplayName } from './avatar';

// ==============================|| CHROME — ACCOUNT MENU ||============================== //
// The account row at the bottom of the sidebar is the only place that shows who is signed in
// (avatar and name; the email sits at the top of its menu). Its menu: the balance (links to
// billing), 设置 (organization and personal settings; the settings button next to the account row
// is a shortcut to the same page), appearance, language, platform admin (admins only), the site
// announcement (while the site has one), docs & help (when the site has a docs link) and log out.
// In the collapsed rail the row shrinks to the avatar and the menu opens to the right.

const THEME_OPTIONS = [
  { value: 'system', Icon: Monitor, labelKey: 'theme.auto' },
  { value: 'light', Icon: Sun, labelKey: 'theme.light' },
  { value: 'dark', Icon: Moon, labelKey: 'theme.dark' }
];

export default function AccountMenu({ collapsed = false, anchorRef = null, context = 'personal', balance = null, onNavigate }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const user = useSelector((state) => state.account.user);
  const siteInfo = useSelector((state) => state.siteInfo);
  const { theme, setTheme } = useTheme();
  const i18n = useI18n();
  const { logout } = useLogin();
  const { pathname } = useLocation();
  const isAdmin = useIsAdmin();
  const isRoot = useIsRoot();
  const { orgRoutingPolicyEnabled } = useOrg();
  const { notice, openNotice } = useNotice();

  const name = accountDisplayName(user);
  // Shown under the name at the top of the menu (the sidebar row itself is one line): the email
  // when it differs from the shown name, else @username.
  const secondary = user?.email && user.email !== name ? user.email : user?.display_name && user?.username ? `@${user.username}` : '';
  const currentLang = i18n.language || i18nList[0].lng;
  const currentLangName = i18nList.find((item) => item.lng === currentLang)?.name || currentLang;
  const docsLink = siteInfo?.docs_link;

  const go = (to) => {
    navigate(to);
    onNavigate?.();
  };
  const showNotice = () => {
    openNotice();
    onNavigate?.();
  };
  const openAdmin = () => {
    const target = contextSwitchTarget({
      pathname,
      to: { kind: 'admin' },
      isRoot,
      features: { orgRoutingPolicy: orgRoutingPolicyEnabled }
    });
    if (target) go(target);
  };

  const trigger = collapsed ? (
    <button
      type="button"
      aria-label={`${t('nav.account.menu')}: ${name}`}
      className="flex size-9 items-center justify-center rounded-full transition-opacity hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <UserAvatar user={user} className="size-6" textClass="text-[10px]" />
    </button>
  ) : (
    <button
      type="button"
      aria-label={`${t('nav.account.menu')}: ${name}`}
      className="flex h-9 w-full min-w-0 items-center gap-2 rounded-lg px-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <UserAvatar user={user} className="size-6" textClass="text-[10px]" />
      {/* Optical nudge: Latin caps sit ~0.6 px above the line box centre, so the name looked high
          against the round avatar (measured at 1x: ink 1 px above the avatar centre). */}
      <span className="min-w-0 flex-1 translate-y-[0.5px] truncate text-[13px] font-medium text-foreground">{name}</span>
    </button>
  );

  return (
    <DropdownMenu className={cn(!collapsed && 'min-w-0 flex-1')}>
      {collapsed ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
          </TooltipTrigger>
          <TooltipContent side="right">{name}</TooltipContent>
        </Tooltip>
      ) : (
        <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
      )}
      <DropdownMenuContent
        side={collapsed ? 'right' : 'top'}
        align={collapsed ? 'end' : 'start'}
        anchorRef={collapsed ? null : anchorRef}
        matchTriggerWidth={!collapsed}
        className={cn(collapsed && 'w-56')}
      >
        <DropdownMenuLabel className="font-normal">
          <span className="block truncate text-[13px] font-medium text-foreground">{name}</span>
          {secondary && <span className="block truncate">{secondary}</span>}
        </DropdownMenuLabel>
        {balance && (
          <DropdownMenuItem disabled={!balance.href} onClick={() => balance.href && go(balance.href)} className="disabled:opacity-100">
            <Wallet />
            <span className="flex-1">{balance.label}</span>
            <span className="font-medium tabular-nums text-foreground">{balance.value}</span>
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => go(SETTINGS_ROOT)}>
          <Settings />
          <span className="flex-1">{t('nav.account.settings')}</span>
        </DropdownMenuItem>
        {/* Appearance: an inline segmented switch, so the menu stays open while trying themes. */}
        <div className="flex items-center gap-2 px-2 py-1.5 text-sm [&>svg]:size-4 [&>svg]:shrink-0">
          <Palette />
          <span className="flex-1">{t('nav.account.appearance')}</span>
          <div role="radiogroup" aria-label={t('nav.account.appearance')} className="flex items-center gap-0.5 rounded-md bg-control p-0.5">
            {THEME_OPTIONS.map(({ value, Icon, labelKey }) => (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={theme === value}
                aria-label={t(labelKey)}
                title={t(labelKey)}
                onClick={() => setTheme(value)}
                className={cn(
                  'flex h-6 w-7 items-center justify-center rounded-[5px] transition-colors',
                  theme === value ? 'bg-control-thumb text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                )}
              >
                <Icon className="size-3.5" />
              </button>
            ))}
          </div>
        </div>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Globe />
            <span className="flex-1">{t('nav.account.language')}</span>
            <span className="text-xs text-muted-foreground">{currentLangName}</span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="min-w-40">
            <DropdownMenuRadioGroup value={currentLang} onValueChange={setAppLanguage}>
              {i18nList.map((item) => (
                <DropdownMenuRadioItem key={item.lng} value={item.lng}>
                  {item.name}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        {isAdmin && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={openAdmin}>
              <ShieldUser />
              <span className="flex-1">{t('nav.context.admin')}</span>
              {context === 'admin' && <Check className="opacity-70" />}
            </DropdownMenuItem>
          </>
        )}
        <DropdownMenuSeparator />
        {/* The site announcement, to read it again after its banner is gone. */}
        {notice && (
          <DropdownMenuItem onClick={showNotice}>
            <Megaphone />
            <span className="flex-1">{t('notice.announcement')}</span>
          </DropdownMenuItem>
        )}
        {docsLink && (
          <DropdownMenuItem onClick={() => window.open(docsLink, '_blank', 'noopener,noreferrer')}>
            <BookOpen />
            <span className="flex-1">{t('nav.account.docs')}</span>
            <ArrowUpRight className="opacity-60" />
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={logout}>
          <LogOut />
          {t('nav.account.logout')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

AccountMenu.propTypes = {
  collapsed: PropTypes.bool,
  anchorRef: PropTypes.shape({ current: PropTypes.any }),
  context: PropTypes.oneOf(['personal', 'org', 'admin']),
  balance: PropTypes.shape({ label: PropTypes.string, value: PropTypes.string, href: PropTypes.string }),
  onNavigate: PropTypes.func
};
