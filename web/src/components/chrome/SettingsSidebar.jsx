import PropTypes from 'prop-types';
import { Link, useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Search, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useSettingsPlane } from '@/components/settings/SettingsPlane';
import { useOrg } from 'contexts/OrgContext';
import { settingsReturnPath } from 'utils/settingsReturn';
import OrgPicker from 'views/panel/Settings/org/OrgPicker';
import { ORG_TREE, orgSettingsUrl, parseSettingsPath, settingsTree } from 'views/panel/Settings/sections';
import { isAdminPath } from './nav-config';
import { CollapseButton, sidebarRow } from './SidebarControls';

// ==============================|| CHROME — SETTINGS SIDEBAR (设置) ||============================== //
// On 设置 the whole sidebar is the settings tree. Top: a back row to the page the user came from
// (工作台, or 平台管理 when they came from there) and the collapse button. Then the settings search
// (while the page has enough groups), 组织设置 for owners and admins of an organization (a picker
// of the organizations they manage, then 常规 / 成员与预算 / 自动路由 / 危险操作) and 个人设置.
// No footer: balance, account menu and the settings button are back in the workbench.

function TreeItem({ item, label, collapsed, dirty, count, dimmed, onClick }) {
  const Icon = item.icon;
  const row = (
    <Link
      to={item.to}
      onClick={onClick}
      aria-current={item.active ? 'page' : undefined}
      className={cn(
        sidebarRow.base,
        collapsed ? 'min-h-[52px] flex-col justify-center gap-1 px-1 py-1.5 text-center' : 'h-[34px] gap-2.5 px-3',
        item.active ? sidebarRow.active : sidebarRow.idle,
        dimmed && 'opacity-40'
      )}
    >
      <Icon
        className={cn(
          'shrink-0',
          collapsed ? 'size-5' : 'size-4',
          item.active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground'
        )}
      />
      <span className={cn('min-w-0 truncate', collapsed ? 'w-full text-[11px] font-normal leading-tight' : 'flex-1 text-left')}>
        {label}
      </span>
      {!collapsed &&
        (dirty ? (
          <span className="size-1.5 shrink-0 rounded-full bg-orange-500" title={dirty}>
            <span className="sr-only">{dirty}</span>
          </span>
        ) : (
          count != null && <span className="text-xs tabular-nums text-muted-foreground">{count}</span>
        ))}
    </Link>
  );
  if (!collapsed) return row;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{row}</TooltipTrigger>
      <TooltipContent side="right">{label}</TooltipContent>
    </Tooltip>
  );
}

TreeItem.propTypes = {
  item: PropTypes.object.isRequired,
  label: PropTypes.string.isRequired,
  collapsed: PropTypes.bool,
  dirty: PropTypes.string,
  count: PropTypes.number,
  dimmed: PropTypes.bool,
  onClick: PropTypes.func
};

export default function SettingsSidebar({ collapsed = false, onItemClick, onToggleCollapse }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { organizations, currentOrgId, orgRoutingPolicyEnabled } = useOrg();
  const plane = useSettingsPlane();

  const tree = settingsTree({ pathname, organizations, currentOrgId, routing: orgRoutingPolicyEnabled });
  const backTo = settingsReturnPath();
  const backToAdmin = isAdminPath(backTo.split('?')[0]);
  const backName = backToAdmin ? t('nav.context.admin') : t('admin_console.workspace');
  const backLabel = backToAdmin ? t('settingsUi.sections.backToAdmin') : t('admin_console.back');

  const query = plane?.query ?? '';
  const searching = query.trim() !== '';
  const unsaved = t('settingsUi.unsaved');
  const clearSearch = () => plane?.setQuery('');
  const onPick = () => {
    clearSearch();
    onItemClick?.();
  };

  // Picking another organization keeps the item when it exists for it (自动路由 may not).
  const { section } = parseSettingsPath(pathname);
  const openOrg = (orgId) => {
    const keep = tree.orgItems.some((item) => item.active) ? section : ORG_TREE[0].id;
    navigate(orgSettingsUrl(orgId, keep));
    onItemClick?.();
  };

  const renderItem = (item) => {
    const count = searching && plane?.counts ? plane.counts[item.id] || 0 : null;
    return (
      <TreeItem
        key={item.id}
        item={item}
        label={t(item.labelKey)}
        collapsed={collapsed}
        dirty={plane?.dirtyGroups.includes(item.id) ? unsaved : undefined}
        count={count}
        dimmed={searching && count === 0}
        onClick={onPick}
      />
    );
  };

  const heading = (text) => !collapsed && <p className="px-3 pb-1.5 text-[13px] font-medium text-muted-foreground">{text}</p>;

  return (
    <TooltipProvider delayDuration={150}>
      <div className="flex h-full flex-col bg-sidebar">
        {/* Back row; the collapse button sits at its end (the drawer uses it to close). */}
        {collapsed ? (
          <div className="flex h-16 shrink-0 items-center justify-center">
            <Tooltip>
              <TooltipTrigger asChild>
                <Link
                  to={backTo}
                  onClick={onItemClick}
                  aria-label={backLabel}
                  className="flex size-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                >
                  <ArrowLeft className="size-[1.125rem]" />
                </Link>
              </TooltipTrigger>
              <TooltipContent side="right">{backLabel}</TooltipContent>
            </Tooltip>
          </div>
        ) : (
          <div className="flex h-16 shrink-0 items-center gap-1 pl-2 pr-2">
            <Link
              to={backTo}
              onClick={onItemClick}
              aria-label={backLabel}
              className="group flex h-9 min-w-0 flex-1 items-center gap-2 rounded-lg px-2 text-sm font-medium text-foreground/85 transition-colors hover:bg-muted hover:text-foreground"
            >
              <ArrowLeft className="size-[1.125rem] shrink-0 text-muted-foreground group-hover:text-foreground" />
              <span className="truncate">{backName}</span>
            </Link>
            {onToggleCollapse && <CollapseButton onClick={onToggleCollapse} />}
          </div>
        )}

        {!collapsed && plane?.searchable && (
          <div className="relative shrink-0 px-2 pb-1">
            <Search className="pointer-events-none absolute left-5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              value={query}
              onChange={(e) => plane.setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Escape' && clearSearch()}
              placeholder={t('settingsUi.search.placeholder')}
              aria-label={t('settingsUi.search.placeholder')}
              className="h-[34px] bg-background pl-9 [&::-webkit-search-cancel-button]:hidden"
            />
            {searching && (
              <button
                type="button"
                onClick={clearSearch}
                aria-label={t('settingsUi.search.clear')}
                className="absolute right-4 top-1/2 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:text-foreground"
              >
                <X className="size-3.5" />
              </button>
            )}
          </div>
        )}

        <nav
          aria-label={t('settingsUi.sections.label')}
          className={cn('flex-1 overflow-y-auto py-3', collapsed ? 'px-1' : 'px-2')}
          style={collapsed ? { scrollbarGutter: 'stable both-edges' } : undefined}
        >
          {tree.org && (
            <div role="group" aria-label={t('settingsUi.sections.org')} className="space-y-0.5">
              {heading(t('settingsUi.sections.org'))}
              {!collapsed && (
                <div className="pb-1">
                  <OrgPicker orgs={tree.managedOrgs} value={tree.org.id} onChange={openOrg} />
                </div>
              )}
              {tree.orgItems.map(renderItem)}
            </div>
          )}
          <div
            role="group"
            aria-label={t('settingsUi.sections.personal')}
            className={cn('space-y-0.5', tree.org && (collapsed ? 'mt-2 border-t border-border/60 pt-2' : 'mt-6'))}
          >
            {heading(t('settingsUi.sections.personal'))}
            {tree.personalItems.map(renderItem)}
          </div>
        </nav>
      </div>
    </TooltipProvider>
  );
}

SettingsSidebar.propTypes = {
  collapsed: PropTypes.bool,
  onItemClick: PropTypes.func,
  onToggleCollapse: PropTypes.func
};
