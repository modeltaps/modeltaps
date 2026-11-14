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
import { formatFullAmount } from 'utils/compactAmount';
import { adminReturnPath } from 'utils/adminReturn';
import { SETTINGS_ROOT, findActiveItem, isOrgAdminRole, navLabel, navSectionsFor } from './nav-config';
import ContextSwitcher, { ContextMark } from './ContextSwitcher';
import AccountMenu from './AccountMenu';
import { SidebarIconButton, SidebarSearch, sidebarRow } from './SidebarControls';
import SidebarBrandRow from './SidebarBrandRow';

// ==============================|| CHROME — SIDEBAR (one shell, three contexts) ||============================== //
// Every panel page uses this sidebar (设置 swaps it for SettingsSidebar, laid out the same way).
// Top: the brand row (SidebarBrandRow, with the search button and, on narrow screens and in the
// phone drawer, the collapse button at its end), then the
// context switcher (personal / org; in platform admin a 平台管理 row that leads back to the workbench).
// Middle: the menu of the current context (nav-config). Bottom:
// the account row (its menu carries the balance, 设置 and the platform admin entry) and the
// settings button, a shortcut to the same 设置 page in every context. Shared by the desktop rail
// and the mobile drawer; `collapsed` switches the desktop rail to an icon column.

// One-time cleanup of the retired collapsible-group state.
try {
  localStorage.removeItem('sidebar_groups');
} catch {
  // ignore storage errors (e.g. storage disabled)
}

const { base: rowBase, active: rowActive, idle: rowIdle, collapsed: rowCollapsed, divider } = sidebarRow;

// Platform admin has no space switcher. Its slot holds the same kind of row as the switcher (the
// context mark and name, so the sidebar says where the user is), and the whole row leads back to
// the workbench page the user came from (organization context is kept while in platform admin).
export function AdminBackRow({ collapsed, onClick }) {
  const { t } = useTranslation();
  const name = t('nav.context.admin');
  const label = t('admin_console.back');
  const to = adminReturnPath();
  const rowClass =
    'group flex items-center rounded-lg transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';
  if (collapsed) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <Link to={to} onClick={onClick} aria-label={label} className={cn(rowClass, 'size-9 justify-center')}>
            <ContextMark kind="admin" className="size-7" />
          </Link>
        </TooltipTrigger>
        <TooltipContent side="right">
          {name} · {label}
        </TooltipContent>
      </Tooltip>
    );
  }
  // gap-1.5 after the 24px mark puts the name on the text column of the menu rows, as in the switcher.
  return (
    <Link to={to} onClick={onClick} aria-label={label} className={cn(rowClass, 'h-9 w-full gap-1.5 px-2 text-left')}>
      <ContextMark kind="admin" className="size-6" />
      <span className="min-w-0 flex-1 truncate text-sm font-medium text-foreground">{name}</span>
      <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground transition-colors group-hover:text-foreground">
        <ArrowLeft className="size-3.5" />
        {t('admin_console.workspace')}
      </span>
    </Link>
  );
}

AdminBackRow.propTypes = {
  collapsed: PropTypes.bool,
  onClick: PropTypes.func
};

export default function Sidebar({ context, collapsed = false, showCollapse = false, onItemClick, onToggleCollapse, onOpenCommand }) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const { currentOrgId, orgRole, orgDetail, routingPolicyEnabled } = useOrg();
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
              aria-label={label}
              className={cn(rowBase, rowCollapsed, active ? rowActive : rowIdle)}
            >
              <Icon
                className={cn('size-[1.125rem] shrink-0', active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground')}
              />
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
        <SidebarBrandRow
          collapsed={collapsed}
          onItemClick={onItemClick}
          onOpenCommand={onOpenCommand}
          onToggleCollapse={showCollapse ? onToggleCollapse : undefined}
        />

        <div className={cn('flex shrink-0 flex-col pb-1', collapsed ? 'items-center gap-1 px-1' : 'gap-2 px-2')}>
          {context === 'admin' ? (
            <AdminBackRow collapsed={collapsed} onClick={onItemClick} />
          ) : (
            <ContextSwitcher context={context} collapsed={collapsed} onNavigate={onItemClick} />
          )}
          {collapsed && <SidebarSearch collapsed onClick={onOpenCommand} />}
        </div>

        <nav
          aria-label={t('nav.mainLabel')}
          className={cn('flex-1 overflow-y-auto pb-3 pt-1', collapsed ? 'px-0 [scrollbar-width:none]' : 'px-2')}
        >
          {sections.map((section, idx) => (
            <div key={section.id} className={cn(collapsed ? 'space-y-1' : 'space-y-0.5', idx > 0 && !collapsed && 'mt-6')}>
              {idx > 0 && collapsed && <div className={divider} aria-hidden="true" />}
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
  // Show the collapse button: on narrow screens, where the rail collapses by itself, and in the
  // phone drawer (to close it). Wide screens keep the rail expanded.
  showCollapse: PropTypes.bool,
  onItemClick: PropTypes.func,
  onToggleCollapse: PropTypes.func,
  onOpenCommand: PropTypes.func
};
