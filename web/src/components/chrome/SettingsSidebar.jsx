import PropTypes from 'prop-types';
import { Link, useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ArrowLeft } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useSettingsPlane } from '@/components/settings/SettingsPlane';
import { useOrg } from 'contexts/OrgContext';
import { settingsReturnPath } from 'utils/settingsReturn';
import OrgPicker from 'views/panel/Settings/org/OrgPicker';
import { ORG_TREE, orgSettingsUrl, parseSettingsPath, settingsTree } from 'views/panel/Settings/sections';
import { isAdminPath } from './nav-config';
import { CollapseButton, SidebarSearch, sidebarRow } from './SidebarControls';

// ==============================|| CHROME — SETTINGS SIDEBAR (设置) ||============================== //
// On 设置 the whole sidebar is the settings tree, laid out like the workbench sidebar so nothing
// moves between them:
//   first row  a back row to the page the user came from (工作台, or 平台管理 when they came from
//              there) where the brand is, then the search button (the panel's search palette)
//              and the collapse button where it is needed;
//   switcher   where the workbench has its context switcher: the picker of the organizations the
//              user manages (owners and admins only; it names the organization part, no heading);
//   menu       常规 / 成员与预算 / 危险操作 of that organization, then 个人设置.
// The collapse button shows on narrow screens, where the rail collapses by itself and can be
// expanded from the page title, and in the phone drawer (to close it).
// No footer: balance, account menu and the settings button are back in the workbench.

function TreeItem({ item, label, collapsed, dirty, onClick }) {
  const Icon = item.icon;
  const row = (
    <Link
      to={item.to}
      onClick={onClick}
      aria-current={item.active ? 'page' : undefined}
      className={cn(
        sidebarRow.base,
        collapsed ? 'min-h-[52px] flex-col justify-center gap-1 px-1 py-1.5 text-center' : 'h-[34px] gap-2.5 px-3',
        item.active ? sidebarRow.active : sidebarRow.idle
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
      {!collapsed && dirty && (
        <span className="size-1.5 shrink-0 rounded-full bg-orange-500" title={dirty}>
          <span className="sr-only">{dirty}</span>
        </span>
      )}
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
  onClick: PropTypes.func
};

export default function SettingsSidebar({ collapsed = false, showCollapse = false, onItemClick, onToggleCollapse, onOpenCommand }) {
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
  const unsaved = t('settingsUi.unsaved');

  // Picking another organization keeps the current item.
  const { section } = parseSettingsPath(pathname);
  const openOrg = (orgId) => {
    const keep = tree.orgItems.some((item) => item.active) ? section : ORG_TREE[0].id;
    navigate(orgSettingsUrl(orgId, keep));
    onItemClick?.();
  };

  const renderItem = (item) => (
    <TreeItem
      key={item.id}
      item={item}
      label={t(item.labelKey)}
      collapsed={collapsed}
      dirty={plane?.dirtyGroups.includes(item.id) ? unsaved : undefined}
      onClick={onItemClick}
    />
  );

  return (
    <TooltipProvider delayDuration={150}>
      <div className="flex h-full flex-col bg-sidebar">
        {/* First row: back link, then search and (where needed) collapse. */}
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
            {onOpenCommand && <SidebarSearch onClick={onOpenCommand} />}
            {showCollapse && onToggleCollapse && <CollapseButton onClick={onToggleCollapse} />}
          </div>
        )}

        {/* Where the workbench has its context switcher (same box, same spacing). */}
        <div className={cn('flex shrink-0 flex-col pb-1', collapsed ? 'items-center gap-1 px-1' : 'gap-2 px-2')}>
          {tree.org && <OrgPicker orgs={tree.managedOrgs} value={tree.org.id} onChange={openOrg} collapsed={collapsed} />}
          {collapsed && onOpenCommand && <SidebarSearch collapsed onClick={onOpenCommand} />}
        </div>

        <nav
          aria-label={t('settingsUi.sections.label')}
          className={cn('flex-1 overflow-y-auto py-3', collapsed ? 'px-1' : 'px-2')}
          style={collapsed ? { scrollbarGutter: 'stable both-edges' } : undefined}
        >
          {tree.org && (
            <div role="group" aria-label={t('settingsUi.sections.org')} className="space-y-0.5">
              {tree.orgItems.map(renderItem)}
            </div>
          )}
          <div
            role="group"
            aria-label={t('settingsUi.sections.personal')}
            className={cn('space-y-0.5', tree.org && (collapsed ? 'mt-2 border-t border-border/60 pt-2' : 'mt-6'))}
          >
            {!collapsed && <p className="px-3 pb-1.5 text-[13px] font-medium text-muted-foreground">{t('settingsUi.sections.personal')}</p>}
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
  onToggleCollapse: PropTypes.func,
  // Opens the panel's search palette.
  onOpenCommand: PropTypes.func
};
