import PropTypes from 'prop-types';
import { Link, useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useSettingsPlane } from '@/components/settings/SettingsPlane';
import { useOrg } from 'contexts/OrgContext';
import { settingsReturnPath } from 'utils/settingsReturn';
import { adminReturnPath } from 'utils/adminReturn';
import OrgPicker from 'views/panel/Settings/org/OrgPicker';
import { ORG_TREE, orgSettingsUrl, parseSettingsPath, settingsTree } from 'views/panel/Settings/sections';
import { isAdminPath } from './nav-config';
import { SidebarSearch, TitleBackRow, sidebarRow } from './SidebarControls';
import SidebarBrandRow from './SidebarBrandRow';

// ==============================|| CHROME — SETTINGS SIDEBAR (设置) ||============================== //
// On 设置 the whole sidebar is the settings tree, laid out like the workbench and platform admin
// sidebars so nothing moves between them:
//   title row  the same first row (SidebarBrandRow) as platform admin: "‹ 设置" in the brand's
//              place, leading back to the workbench page the user came from (when they came from
//              platform admin, the workbench page before it), then search and collapse;
//   picker     where the workbench has its context switcher, the organizations the user manages
//              (owners and admins only; it
//              names the organization part, no heading);
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
        collapsed ? cn(sidebarRow.collapsed, 'relative') : 'h-[34px] gap-2.5 px-3',
        item.active ? sidebarRow.active : sidebarRow.idle
      )}
    >
      <Icon
        className={cn(
          'shrink-0',
          collapsed ? 'size-[1.125rem]' : 'size-4',
          item.active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground'
        )}
      />
      <span className={collapsed ? 'sr-only' : 'min-w-0 flex-1 truncate text-left'}>{label}</span>
      {dirty && (
        <span className={cn('size-1.5 shrink-0 rounded-full bg-orange-500', collapsed && 'absolute right-1 top-1')} title={dirty}>
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
  const lastPage = settingsReturnPath();
  const backTo = isAdminPath(lastPage.split(/[?#]/)[0]) ? adminReturnPath() : lastPage;
  const unsaved = t('settingsUi.unsaved');

  // Picking another organization keeps the current item.
  const { section } = parseSettingsPath(pathname);
  const openOrg = (orgId) => {
    const keep = tree.orgItems.some((item) => item.active) ? section : ORG_TREE[0].id;
    const org = tree.managedOrgs.find((o) => o.id === orgId);
    if (org) navigate(orgSettingsUrl(org.slug, keep));
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
        <SidebarBrandRow
          collapsed={collapsed}
          lead={
            <TitleBackRow to={backTo} title={t('settings')} label={t('admin_console.back')} collapsed={collapsed} onClick={onItemClick} />
          }
          onItemClick={onItemClick}
          onOpenCommand={onOpenCommand}
          onToggleCollapse={showCollapse ? onToggleCollapse : undefined}
        />

        {/* Where the workbench has its context switcher (same box, same spacing). */}
        <div className={cn('flex shrink-0 flex-col pb-1', collapsed ? 'items-center gap-1 px-1' : 'gap-2 px-2')}>
          {tree.org && <OrgPicker orgs={tree.managedOrgs} value={tree.org.id} onChange={openOrg} collapsed={collapsed} />}
          {collapsed && onOpenCommand && <SidebarSearch collapsed onClick={onOpenCommand} />}
        </div>

        <nav
          aria-label={t('settingsUi.sections.label')}
          className={cn('flex-1 overflow-y-auto pb-3 pt-1', collapsed ? 'px-0 [scrollbar-width:none]' : 'px-2')}
        >
          {tree.org && (
            <div role="group" aria-label={t('settingsUi.sections.org')} className={collapsed ? 'space-y-1' : 'space-y-0.5'}>
              {tree.orgItems.map(renderItem)}
            </div>
          )}
          <div
            role="group"
            aria-label={t('settingsUi.sections.personal')}
            className={cn(collapsed ? 'space-y-1' : 'space-y-0.5', tree.org && !collapsed && 'mt-6')}
          >
            {tree.org && collapsed && <div className={sidebarRow.divider} aria-hidden="true" />}
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
