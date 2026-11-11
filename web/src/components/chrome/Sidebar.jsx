import { useRef } from 'react';
import PropTypes from 'prop-types';
import { Link, useLocation } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Settings } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useOrg } from 'contexts/OrgContext';
import { useIsRoot } from 'utils/common';
import { brandWordmark, isDefaultBrand } from 'utils/brand';
import { formatFullAmount } from 'utils/compactAmount';
import { adminReturnPath } from 'utils/adminReturn';
import { DASHBOARD_URL, SETTINGS_ROOT, findActiveItem, isOrgAdminRole, navLabel, navSectionsFor } from './nav-config';
import { BrandLockup, LogoMark } from './Logo';
import ContextSwitcher from './ContextSwitcher';
import AccountMenu from './AccountMenu';
import { CollapseButton, SidebarIconButton, SidebarSearch, sidebarRow } from './SidebarControls';

// ==============================|| CHROME — SIDEBAR (one shell, three contexts) ||============================== //
// Every panel page uses this sidebar. Top: brand row, context switcher (personal / org; in
// platform admin a back row to the workbench instead), search. Middle: the menu of the current context (nav-config). Bottom:
// the account row (its menu carries the balance, 设置 and the platform admin entry) and the
// settings button, a shortcut to the same 设置 page in every context. Shared by the desktop rail
// and the mobile drawer; `collapsed` switches the desktop rail to an icon column.

// One-time cleanup of the retired collapsible-group state.
try {
  localStorage.removeItem('sidebar_groups');
} catch {
  // ignore storage errors (e.g. storage disabled)
}

const { base: rowBase, active: rowActive, idle: rowIdle } = sidebarRow;

// Platform admin has no space switcher: its own menu only, and one row back to the workbench page
// the user came from (organization context is kept while in platform admin). Same height as the
// switcher row it replaces.
export function AdminBackRow({ collapsed, onClick }) {
  const { t } = useTranslation();
  const to = adminReturnPath();
  const label = t('admin_console.back');
  if (collapsed) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <Link
            to={to}
            onClick={onClick}
            aria-label={label}
            className="flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ArrowLeft className="size-[1.125rem]" />
          </Link>
        </TooltipTrigger>
        <TooltipContent side="right">{label}</TooltipContent>
      </Tooltip>
    );
  }
  return (
    <Link
      to={to}
      onClick={onClick}
      aria-label={label}
      className="group flex h-9 w-full items-center gap-2.5 rounded-lg px-2 text-sm font-medium text-foreground/85 transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <span className="flex size-6 shrink-0 items-center justify-center">
        <ArrowLeft className="size-[1.125rem] text-muted-foreground group-hover:text-foreground" />
      </span>
      <span className="min-w-0 flex-1 truncate">{t('admin_console.workspace')}</span>
    </Link>
  );
}

AdminBackRow.propTypes = {
  collapsed: PropTypes.bool,
  onClick: PropTypes.func
};

export default function Sidebar({ context, collapsed = false, onItemClick, onToggleCollapse, onOpenCommand }) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const { currentOrgId, orgRole, orgDetail, routingPolicyEnabled } = useOrg();
  const siteInfo = useSelector((state) => state.siteInfo);
  const user = useSelector((state) => state.account.user);
  const isRoot = useIsRoot();
  const footerRef = useRef(null);

  const sections = navSectionsFor({
    context,
    orgRole,
    orgId: currentOrgId,
    isRoot,
    features: { routingPolicy: routingPolicyEnabled }
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

  const settingsButton = (
    <SidebarIconButton
      label={t('nav.items.settings')}
      side={collapsed ? 'right' : 'top'}
      to={SETTINGS_ROOT}
      onClick={onItemClick}
      className="size-9"
    >
      <Settings className="size-[1.125rem]" />
    </SidebarIconButton>
  );

  return (
    <TooltipProvider delayDuration={150}>
      <div className="flex h-full flex-col bg-sidebar">
        {/* Brand row; the collapse button sits at its end (the drawer uses it to close). */}
        {collapsed ? (
          <div className="flex h-16 shrink-0 items-center justify-center">
            <Link to={DASHBOARD_URL} onClick={onItemClick} aria-label={brand} className="text-foreground hover:opacity-90">
              <LogoMark size={20} />
            </Link>
          </div>
        ) : (
          <div className="flex h-16 shrink-0 items-center gap-1 pl-4 pr-2">
            <Link
              to={DASHBOARD_URL}
              onClick={onItemClick}
              className="flex min-w-0 flex-1 items-center gap-2.5 text-foreground transition-opacity hover:opacity-90"
            >
              <BrandLockup text={brand} wordmark={isDefaultBrand(siteInfo?.system_name)} truncate />
            </Link>
            {onToggleCollapse && <CollapseButton onClick={onToggleCollapse} />}
          </div>
        )}

        <div className={cn('flex shrink-0 flex-col pb-1', collapsed ? 'items-center gap-1 px-1' : 'gap-2 px-2')}>
          {context === 'admin' ? (
            <AdminBackRow collapsed={collapsed} onClick={onItemClick} />
          ) : (
            <ContextSwitcher context={context} collapsed={collapsed} onNavigate={onItemClick} />
          )}
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
              {settingsButton}
            </>
          ) : (
            <div ref={footerRef} className="flex items-center gap-1">
              <AccountMenu anchorRef={footerRef} context={context} balance={balance} onNavigate={onItemClick} />
              {settingsButton}
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
