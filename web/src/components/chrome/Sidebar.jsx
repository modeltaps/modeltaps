import { useRef } from 'react';
import { Link, NavLink, useLocation, useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowUpRight, Bell, Settings, ShieldUser, Wallet } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useOrg } from 'contexts/OrgContext';
import { useIsAdmin } from 'utils/common';
import { saveUserReturnPath } from 'utils/adminPlane';
import { formatFullAmount } from 'utils/compactAmount';
import { useNotice } from 'ui-component/notice';
import { filterUserSections, navLabel } from './nav-config';
import { LogoMark } from './Logo';
import MenuCard from './MenuCard';
import MoreMenu from './MoreMenu';
import { SidebarTopBar } from './SidebarControls';

// ==============================|| CHROME — SIDEBAR CONTENT (user plane) ||============================== //
// Shared by the desktop rail and the mobile drawer. `collapsed` switches the
// desktop rail to an icon-only mode. Active items get a subtle background fill
// only (no left indicator bar). Navigation is a single flat list of labelled
// sections rendered uniformly (muted section title + spacing). Admin sections
// live in the dedicated AdminSidebar (T19); the admin plane is entered from a
// trailing 「管理」 nav section (admins only), not from the footer.
// 页脚照 x.ai 控制台只有一行:左侧额度只放钱包图标 + 完整金额(点击充值,文字标签只在
// tooltip / aria-label 里),右侧依次是「通知」「更多」(面板与页脚同宽、向上展开)与「设置」。
// 用户信息不在页脚,在顶部的账号行(MenuCard)。第一行是 logo + 搜索 + 收起(SidebarTopBar)。

// One-time cleanup of the retired collapsible-group state.
try {
  localStorage.removeItem('sidebar_groups');
} catch {
  // ignore storage errors (e.g. storage disabled)
}

export default function Sidebar({ collapsed = false, onItemClick, onToggleCollapse, onOpenCommand }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { openNotice } = useNotice();
  const { currentOrgId, orgRole, orgDetail } = useOrg();
  const siteInfo = useSelector((state) => state.siteInfo);
  const user = useSelector((state) => state.account.user);
  const isAdmin = useIsAdmin();

  const orgActive = Boolean(currentOrgId);
  const invoiceEnabled = Boolean(siteInfo.UserInvoiceMonth);
  const chatEnabled = siteInfo.builtin_chat_enabled !== false;
  const sections = filterUserSections({ orgActive, orgRole, invoiceEnabled, chatEnabled });
  const footerRef = useRef(null);

  // 页脚额度:个人态 = 个人额度 + 充值;组织态 Owner/Admin = 池额度 + 充值;Member = 我的额度、无充值
  const isOrgAdmin = orgRole === 'owner' || orgRole === 'admin';
  const quotaPerUnit = Number(localStorage.getItem('quota_per_unit')) || 500000;
  const personalBalance = (user?.quota || 0) / quotaPerUnit;
  const orgQuota = orgDetail?.quota;
  const showPool = orgActive && isOrgAdmin;
  const canTopup = !orgActive || isOrgAdmin;
  const balanceLabel = showPool ? t('org.poolBalance') : orgActive ? t('org.myBalance') : t('sidebar.remainingBalance');
  // 页脚里放完整金额,文字标签只放 tooltip / aria-label
  const balanceAmount = showPool ? (typeof orgQuota === 'number' ? orgQuota / quotaPerUnit : null) : personalBalance;
  const balanceValue = balanceAmount === null ? '—' : formatFullAmount(balanceAmount);
  const balanceText = `${balanceLabel} ${balanceValue}`;
  // 页脚额度按钮落到账单页本身(不自动弹充值层),UserMenu 的「充值」才带 ?topup=1
  const goBilling = () => {
    onItemClick?.();
    navigate('/panel/billing');
  };

  // 管理后台入口:记住来路,进入管理面的第一项(与旧用户菜单里的入口行为一致)
  const enterAdminConsole = () => {
    onItemClick?.();
    saveUserReturnPath(pathname);
    navigate('/panel/analytics');
  };

  const renderBalance = () => {
    if (collapsed) {
      const icon = <Wallet className="size-[1.125rem]" />;
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            {canTopup ? (
              <button
                type="button"
                onClick={goBilling}
                aria-label={`${balanceText} · ${t('billing')}`}
                className="flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              >
                {icon}
              </button>
            ) : (
              <span aria-label={balanceText} className="flex size-8 items-center justify-center rounded-md text-muted-foreground">
                {icon}
              </span>
            )}
          </TooltipTrigger>
          <TooltipContent side="right">{canTopup ? `${balanceText} · ${t('billing')}` : balanceText}</TooltipContent>
        </Tooltip>
      );
    }
    const content = (
      <>
        <Wallet className="size-[1.125rem] shrink-0" />
        <span className="min-w-0 flex-1 truncate text-left font-semibold text-foreground tabular-nums whitespace-nowrap">
          {balanceValue}
        </span>
      </>
    );
    if (!canTopup) {
      return (
        <div title={balanceText} className="flex h-8 min-w-0 flex-1 items-center gap-3 rounded-md px-3 text-sm text-muted-foreground">
          {content}
        </div>
      );
    }
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            onClick={goBilling}
            className="flex h-8 min-w-0 flex-1 items-center gap-3 rounded-md px-3 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {content}
          </button>
        </TooltipTrigger>
        <TooltipContent side="top">{`${balanceText} · ${t('billing')}`}</TooltipContent>
      </Tooltip>
    );
  };

  const renderNoticeButton = () => (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={() => {
            onItemClick?.();
            openNotice();
          }}
          aria-label={t('notice.title', { defaultValue: 'Notifications' })}
          className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Bell className="size-[1.125rem]" />
        </button>
      </TooltipTrigger>
      <TooltipContent side={collapsed ? 'right' : 'top'}>{t('notice.title', { defaultValue: 'Notifications' })}</TooltipContent>
    </Tooltip>
  );

  const renderSettingsButton = () => (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          to="/panel/settings"
          onClick={onItemClick}
          aria-label={t('userMenu.settings', { defaultValue: 'Settings' })}
          className="flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Settings className="size-[1.125rem]" />
        </Link>
      </TooltipTrigger>
      <TooltipContent side={collapsed ? 'right' : 'top'}>{t('userMenu.settings', { defaultValue: 'Settings' })}</TooltipContent>
    </Tooltip>
  );

  // 管理后台入口(仅管理员):作为导航最后一个分组「管理」的唯一一项,与普通导航项同形、
  // 尾部箭头暗示切到管理面。不经 nav-config:filterUserSections 会丢弃 isAdmin 项且有测试锁定。
  const renderAdminEntry = () => {
    const label = t('admin_console.enter');
    if (collapsed) {
      return (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={enterAdminConsole}
              className="group flex min-h-[56px] w-full flex-col items-center justify-center gap-1 rounded-md px-1 py-1.5 text-center text-foreground/80 transition-colors hover:bg-muted hover:text-foreground"
            >
              <ShieldUser className="size-[1.375rem] shrink-0 text-muted-foreground group-hover:text-foreground" />
              <span className="w-full truncate text-[11px] leading-tight">{label}</span>
            </button>
          </TooltipTrigger>
          <TooltipContent side="right">{label}</TooltipContent>
        </Tooltip>
      );
    }
    return (
      <button
        type="button"
        onClick={enterAdminConsole}
        className="group flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-foreground/80 transition-colors hover:bg-muted hover:text-foreground"
      >
        <ShieldUser className="size-[1.125rem] shrink-0 text-muted-foreground group-hover:text-foreground" />
        <span className="min-w-0 flex-1 truncate text-left">{label}</span>
        <ArrowUpRight className="size-4 shrink-0 text-muted-foreground/60" />
      </button>
    );
  };

  const renderLeaf = (item) => {
    const Icon = item.icon;
    const label = navLabel(t, item);
    const active = pathname === item.url || pathname.startsWith(`${item.url}/`);
    if (collapsed) {
      return (
        <Tooltip key={item.id}>
          <TooltipTrigger asChild>
            <NavLink
              to={item.url}
              onClick={onItemClick}
              className={cn(
                'group flex min-h-[56px] w-full flex-col items-center justify-center gap-1 rounded-md px-1 py-1.5 text-center transition-colors',
                active ? 'bg-muted text-foreground' : 'text-foreground/80 hover:bg-muted hover:text-foreground'
              )}
            >
              <Icon
                className={cn('size-[1.375rem] shrink-0', active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground')}
              />
              <span className="w-full truncate text-[11px] leading-tight">{label}</span>
            </NavLink>
          </TooltipTrigger>
          <TooltipContent side="right">{label}</TooltipContent>
        </Tooltip>
      );
    }
    return (
      <NavLink
        key={item.id}
        to={item.url}
        onClick={onItemClick}
        className={cn(
          'group flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
          active ? 'bg-muted text-foreground' : 'text-foreground/80 hover:bg-muted hover:text-foreground'
        )}
      >
        <Icon
          className={cn('size-[1.125rem] shrink-0', active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground')}
        />
        <span className="min-w-0 flex-1 truncate text-left">{label}</span>
      </NavLink>
    );
  };

  return (
    <TooltipProvider delayDuration={150}>
      <div className="flex h-full flex-col bg-card">
        <SidebarTopBar collapsed={collapsed} onOpenCommand={onOpenCommand} onToggleCollapse={onToggleCollapse}>
          <Link
            to="/panel/dashboard"
            onClick={onItemClick}
            aria-label="Modeltaps"
            className="flex items-center gap-2 text-foreground transition-opacity hover:opacity-90"
          >
            <LogoMark className="h-5 w-auto shrink-0" />
            <span className="text-[1.15rem] font-bold leading-none tracking-tight">Modeltaps</span>
          </Link>
        </SidebarTopBar>

        <MenuCard collapsed={collapsed} />

        <nav
          className={cn('flex-1 overflow-y-auto py-4', collapsed ? 'px-1' : 'space-y-6 px-2')}
          style={collapsed ? { scrollbarGutter: 'stable both-edges' } : undefined}
        >
          {sections.map((section, sectionIdx) => (
            <div key={section.id} className={cn('space-y-0.5', collapsed && sectionIdx > 0 && 'mt-2 border-t border-border/50 pt-2')}>
              {!collapsed && t(section.id) && <p className="px-3 pb-1.5 text-sm font-medium text-muted-foreground">{t(section.id)}</p>}
              {section.items.map(renderLeaf)}
            </div>
          ))}
          {/* 导航最后一个分组「管理」(仅管理员),与业务分组同样式、随导航一起滚动 */}
          {isAdmin && (
            <div className={cn('space-y-0.5', collapsed && sections.length > 0 && 'mt-2 border-t border-border/50 pt-2')}>
              {!collapsed && <p className="px-3 pb-1.5 text-sm font-medium text-muted-foreground">{t('nav_sec_admin')}</p>}
              {renderAdminEntry()}
            </div>
          )}
        </nav>

        {/* 页脚一行:额度(充值) · 通知 · 更多 · 设置;窄栏时竖排图标。
            内层 wrapper 不带内边距、只负责排布,「更多」面板以它为锚,于是左右各内缩 8px 落在侧栏内 */}
        <div className="border-t border-border/60 px-2 py-2">
          <div ref={footerRef} className={cn('flex w-full items-center gap-1', collapsed && 'flex-col')}>
            {renderBalance()}
            {renderNoticeButton()}
            <MoreMenu anchorRef={footerRef} collapsed={collapsed} />
            {renderSettingsButton()}
          </div>
        </div>
      </div>
    </TooltipProvider>
  );
}
