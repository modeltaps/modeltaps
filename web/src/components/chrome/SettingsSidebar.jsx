import { useState } from 'react';
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
import { CollapseButton, SidebarIconButton, sidebarRow } from './SidebarControls';

// ==============================|| CHROME — SETTINGS SIDEBAR (设置) ||============================== //
// On 设置 the whole sidebar is the settings tree. Top row: a back row to the page the user came
// from (工作台, or 平台管理 when they came from there) and, at its end, a search button (while the
// page has enough groups to search) that turns the row into the settings search box. Then the
// organization part for owners and admins of an organization: a picker of the organizations they
// manage (it names the part, so there is no heading) and 常规 / 成员与预算 / 危险操作; then 个人设置.
// The collapse button shows only where it is needed: on narrow screens, where the rail collapses
// by itself and can be expanded from the page title, and in the phone drawer (to close it).
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

export default function SettingsSidebar({ collapsed = false, showCollapse = false, onItemClick, onToggleCollapse }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { organizations, currentOrgId } = useOrg();
  const plane = useSettingsPlane();

  const tree = settingsTree({ pathname, organizations, currentOrgId });
  const backTo = settingsReturnPath();
  const backToAdmin = isAdminPath(backTo.split('?')[0]);
  const backName = backToAdmin ? t('nav.context.admin') : t('admin_console.workspace');
  const backLabel = backToAdmin ? t('settingsUi.sections.backToAdmin') : t('admin_console.back');

  const query = plane?.query ?? '';
  const searching = query.trim() !== '';
  const unsaved = t('settingsUi.unsaved');
  const clearSearch = () => plane?.setQuery('');
  // The search box replaces the back row while it is open or holds a query.
  const [searchOpen, setSearchOpen] = useState(false);
  const canSearch = !collapsed && Boolean(plane?.searchable);
  const searchShown = canSearch && (searchOpen || searching);
  const closeSearch = () => {
    clearSearch();
    setSearchOpen(false);
  };
  const searchLabel = t('settingsUi.search.placeholder');
  const onPick = () => {
    clearSearch();
    onItemClick?.();
  };

  // Picking another organization keeps the current item.
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
        {/* Top row: back link (or the search box), search button, collapse button where needed. */}
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
          <div className="flex h-16 shrink-0 items-center gap-1 px-2">
            {searchShown ? (
              <div className="relative min-w-0 flex-1">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="search"
                  autoFocus
                  value={query}
                  onChange={(e) => plane.setQuery(e.target.value)}
                  onKeyDown={(e) => e.key === 'Escape' && closeSearch()}
                  onBlur={() => !searching && setSearchOpen(false)}
                  placeholder={searchLabel}
                  aria-label={searchLabel}
                  className="h-9 bg-background pl-9 pr-9 [&::-webkit-search-cancel-button]:hidden"
                />
                <button
                  type="button"
                  onClick={closeSearch}
                  aria-label={t('settingsUi.search.clear')}
                  className="absolute right-1.5 top-1/2 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:text-foreground"
                >
                  <X className="size-3.5" />
                </button>
              </div>
            ) : (
              <>
                <Link
                  to={backTo}
                  onClick={onItemClick}
                  aria-label={backLabel}
                  className="group flex h-9 min-w-0 flex-1 items-center gap-2 rounded-lg px-2 text-sm font-medium text-foreground/85 transition-colors hover:bg-muted hover:text-foreground"
                >
                  <ArrowLeft className="size-[1.125rem] shrink-0 text-muted-foreground group-hover:text-foreground" />
                  <span className="truncate">{backName}</span>
                </Link>
                {canSearch && (
                  <SidebarIconButton label={searchLabel} onClick={() => setSearchOpen(true)} className="size-9">
                    <Search className="size-[1.125rem]" />
                  </SidebarIconButton>
                )}
              </>
            )}
            {showCollapse && onToggleCollapse && <CollapseButton onClick={onToggleCollapse} />}
          </div>
        )}

        <nav
          aria-label={t('settingsUi.sections.label')}
          className={cn('flex-1 overflow-y-auto py-3', collapsed ? 'px-1' : 'px-2')}
          style={collapsed ? { scrollbarGutter: 'stable both-edges' } : undefined}
        >
          {tree.org && (
            <div role="group" aria-label={t('settingsUi.sections.org')} className="space-y-0.5">
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
  // Show the collapse button (narrow screens and the phone drawer).
  showCollapse: PropTypes.bool,
  onItemClick: PropTypes.func,
  onToggleCollapse: PropTypes.func
};
