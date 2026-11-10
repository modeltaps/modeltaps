import { useRef } from 'react';
import PropTypes from 'prop-types';
import { Link, useLocation } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Bell } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useOrg } from 'contexts/OrgContext';
import { useIsRoot } from 'utils/common';
import { brandWordmark } from 'utils/brand';
import { formatFullAmount } from 'utils/compactAmount';
import { useNotice } from 'ui-component/notice';
import { DASHBOARD_URL, findActiveItem, isOrgAdminRole, navLabel, navSectionsFor } from './nav-config';
import { LogoMark } from './Logo';
import ContextSwitcher from './ContextSwitcher';
import AccountMenu from './AccountMenu';
import { CollapseButton, SidebarIconButton, SidebarSearch } from './SidebarControls';

// ==============================|| CHROME — SIDEBAR (one shell, three contexts) ||============================== //
// Every panel page uses this sidebar. Top: brand row, context switcher (personal / org /
// platform admin), search. Middle: the menu of the current context (nav-config). Bottom:
// the account row (its menu carries the balance and the platform admin entry) and the bell.
// Shared by the desktop rail and the mobile drawer; `collapsed` switches the desktop rail to
// an icon column.

// One-time cleanup of the retired collapsible-group state.
try {
  localStorage.removeItem('sidebar_groups');
} catch {
  // ignore storage errors (e.g. storage disabled)
}

const rowBase =
  'group flex w-full items-center rounded-lg text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';
// Active: a subtle raised surface with a hairline, no fill colour or indicator bar.
const rowActive = 'bg-sidebar-accent font-medium text-foreground shadow-[inset_0_0_0_1px_var(--border)]';
const rowIdle = 'text-foreground/85 hover:bg-sidebar-accent/70 hover:text-foreground';

export default function Sidebar({ context, collapsed = false, onItemClick, onToggleCollapse, onOpenCommand }) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const { openNotice } = useNotice();
  const { currentOrgId, orgRole, orgDetail, orgRoutingPolicyEnabled } = useOrg();
  const siteInfo = useSelector((state) => state.siteInfo);
  const user = useSelector((state) => state.account.user);
  const isRoot = useIsRoot();
  const footerRef = useRef(null);

  const sections = navSectionsFor({
    context,
    orgRole,
    orgId: currentOrgId,
    isRoot,
    features: { orgRoutingPolicy: orgRoutingPolicyEnabled }
  });
  const activeId = findActiveItem(pathname)?.id;
  const brand = brandWordmark(siteInfo?.system_name);

  // Balance: personal balance; in org context the pool for owners/admins, the member's own
  // balance otherwise (the pool is not visible to members). Hidden in platform admin. It is shown
  // in the account menu, not as a sidebar row.
  const orgActive = context === 'org';
  const isOrgAdmin = isOrgAdminRole(orgRole);
  const quotaPerUnit = Number(localStorage.getItem('quota_per_unit')) || 500000;
  const showPool = orgActive && isOrgAdmin;
  const balanceAmount = showPool
    ? typeof orgDetail?.quota === 'number'
      ? orgDetail.quota / quotaPerUnit
      : null
    : (user?.quota || 0) / quotaPerUnit;
  const balance =
    context === 'admin'
      ? null
      : {
          label: showPool ? t('nav.balance.pool') : orgActive ? t('nav.balance.mine') : t('nav.balance.personal'),
          value: balanceAmount === null ? '—' : formatFullAmount(balanceAmount),
          href: !orgActive || isOrgAdmin ? '/panel/billing' : null
        };

  const renderItem = (item) => {
    const Icon = item.icon;
    const label = navLabel(t, item);
    const active = item.id === activeId;
    if (collapsed) {
      return (
        <Tooltip key={item.id}>
          <TooltipTrigger asChild>
            <Link
              to={item.href}
              onClick={onItemClick}
              aria-current={active ? 'page' : undefined}
              className={cn(rowBase, 'min-h-[52px] flex-col justify-center gap-1 px-1 py-1.5 text-center', active ? rowActive : rowIdle)}
            >
              <Icon className={cn('size-5 shrink-0', active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground')} />
              <span className="w-full truncate text-[11px] font-normal leading-tight">{label}</span>
            </Link>
          </TooltipTrigger>
          <TooltipContent side="right">{label}</TooltipContent>
        </Tooltip>
      );
    }
    return (
      <Link
        key={item.id}
        to={item.href}
        onClick={onItemClick}
        aria-current={active ? 'page' : undefined}
        className={cn(rowBase, 'h-[34px] gap-2.5 px-3', active ? rowActive : rowIdle)}
      >
        <Icon className={cn('size-4 shrink-0', active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground')} />
        <span className="min-w-0 flex-1 truncate text-left">{label}</span>
      </Link>
    );
  };

  const noticeLabel = t('notice.title', { defaultValue: 'Notifications' });
  const bell = (
    <SidebarIconButton
      label={noticeLabel}
      side={collapsed ? 'right' : 'top'}
      onClick={() => {
        onItemClick?.();
        openNotice();
      }}
      className="size-9"
    >
      <Bell className="size-[1.125rem]" />
    </SidebarIconButton>
  );

  return (
    <TooltipProvider delayDuration={150}>
      <div className="flex h-full flex-col bg-sidebar">
        {/* Brand row; the collapse button sits at its end (the drawer uses it to close). */}
        {collapsed ? (
          <div className="flex h-12 shrink-0 items-center justify-center">
            <Link to={DASHBOARD_URL} onClick={onItemClick} aria-label={brand} className="text-foreground hover:opacity-90">
              <LogoMark size={20} />
            </Link>
          </div>
        ) : (
          <div className="flex h-12 shrink-0 items-center gap-1 pl-4 pr-2">
            <Link
              to={DASHBOARD_URL}
              onClick={onItemClick}
              className="flex min-w-0 flex-1 items-center gap-2.5 text-foreground transition-opacity hover:opacity-90"
            >
              <LogoMark size={20} />
              <span className="truncate text-[17px] font-semibold leading-none tracking-[0.01em]">{brand}</span>
            </Link>
            {onToggleCollapse && <CollapseButton onClick={onToggleCollapse} />}
          </div>
        )}

        <div className={cn('flex shrink-0 flex-col pb-1', collapsed ? 'items-center gap-1 px-1' : 'gap-2 px-2')}>
          <ContextSwitcher context={context} collapsed={collapsed} onNavigate={onItemClick} />
          <SidebarSearch collapsed={collapsed} onClick={onOpenCommand} />
        </div>

        <nav
          aria-label={t('nav.mainLabel')}
          className={cn('flex-1 overflow-y-auto py-3', collapsed ? 'px-1' : 'px-2')}
          style={collapsed ? { scrollbarGutter: 'stable both-edges' } : undefined}
        >
          {sections.map((section, idx) => (
            <div key={section.id} className={cn('space-y-0.5', idx > 0 && (collapsed ? 'mt-2 border-t border-border/60 pt-2' : 'mt-6'))}>
              {!collapsed && section.labelKey && (
                <p className="px-3 pb-1.5 text-[13px] font-medium text-muted-foreground">{t(section.labelKey)}</p>
              )}
              {section.items.map(renderItem)}
            </div>
          ))}
        </nav>

        <div className={cn('shrink-0 border-t border-border/70 p-2', collapsed && 'flex flex-col items-center gap-1')}>
          {collapsed ? (
            <>
              <AccountMenu collapsed context={context} balance={balance} onNavigate={onItemClick} />
              {bell}
            </>
          ) : (
            <div ref={footerRef} className="flex items-center gap-1">
              <AccountMenu anchorRef={footerRef} context={context} balance={balance} onNavigate={onItemClick} />
              {bell}
            </div>
          )}
        </div>
      </div>
    </TooltipProvider>
  );
}

Sidebar.propTypes = {
  context: PropTypes.oneOf(['personal', 'org', 'admin']).isRequired,
  collapsed: PropTypes.bool,
  onItemClick: PropTypes.func,
  onToggleCollapse: PropTypes.func,
  onOpenCommand: PropTypes.func
};
