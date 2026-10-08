import { Link, NavLink, useLocation } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ArrowLeft } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useOrg } from 'contexts/OrgContext';
import { getReturnPath } from 'utils/adminPlane';
import { isAdminPath } from './nav-config';
import { SidebarTopBar } from './SidebarControls';
import { isOrgAdminRole } from 'views/panel/Organization';
import OrgSelector from 'views/panel/Settings/OrgSelector';
import {
  ACCOUNT_SECTIONS,
  DEFAULT_ORG_SECTION,
  ORG_SECTIONS,
  accountSettingsUrl,
  orgSettingsUrl,
  parseSettingsPath
} from 'views/panel/Settings/sections';

// ==============================|| CHROME — SETTINGS SIDEBAR (settings plane) ||============================== //
// 整页设置面的侧边栏,结构对齐 AdminSidebar:无品牌行,顶部「返回」项、分组导航、底部 UserMenu。
// 导航 = 「组织」分组(仅当我管理 ≥1 个组织,含组织选择器)+「个人账号」分组。
// 返回路径由 MainLayout 在进入设置面时记录(settings key),刷新/直达回用户面首页。

export default function SettingsSidebar({ collapsed = false, onItemClick, onToggleCollapse, onOpenCommand }) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const { organizations, currentOrgId } = useOrg();

  const { orgScope, orgId, section } = parseSettingsPath(pathname);
  const managedOrgs = organizations.filter((org) => isOrgAdminRole(org.role));
  // 账号分组下组织项不高亮,但仍要有个落点:沿用设置页的默认组织规则。
  const defaultOrg = managedOrgs.find((org) => org.id === currentOrgId) || managedOrgs[0] || null;
  const activeOrgId = orgScope ? orgId : (defaultOrg?.id ?? null);
  const backTo = getReturnPath('settings');
  // 返回行写去向:可见文案是目标面名称,aria-label / tooltip 写「返回 X」(与管理面返回行同规则)。
  const backToAdmin = isAdminPath(backTo);
  const backName = backToAdmin ? t('admin_console.enter') : t('admin_console.workspace');
  const backLabel = backToAdmin ? t('settingsPage.backToAdmin', { defaultValue: 'Back to admin console' }) : t('admin_console.back');

  const renderLeaf = ({ id, labelKey, fallback, icon: Icon }, to, active) => {
    const label = t(labelKey, { defaultValue: fallback });
    if (collapsed) {
      return (
        <Tooltip key={id}>
          <TooltipTrigger asChild>
            <NavLink
              to={to}
              onClick={onItemClick}
              className={cn(
                'group flex min-h-[56px] flex-col items-center justify-center gap-1 rounded-md px-1 py-1.5 text-center transition-colors',
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
        key={id}
        to={to}
        onClick={onItemClick}
        className={cn(
          'group flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
          active ? 'bg-muted text-foreground' : 'text-foreground/80 hover:bg-muted hover:text-foreground'
        )}
      >
        <Icon
          className={cn('size-[1.125rem] shrink-0', active ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground')}
        />
        <span className="truncate">{label}</span>
      </NavLink>
    );
  };

  return (
    <TooltipProvider delayDuration={150}>
      <div className="flex h-full flex-col bg-card">
        <SidebarTopBar collapsed={collapsed} onOpenCommand={onOpenCommand} onToggleCollapse={onToggleCollapse}>
          <Link
            to={backTo}
            onClick={onItemClick}
            aria-label={backLabel}
            className="group flex min-w-0 items-center gap-2 text-sm font-medium text-foreground/80 transition-colors hover:text-foreground"
          >
            <ArrowLeft className="size-[1.125rem] shrink-0 text-muted-foreground group-hover:text-foreground" />
            <span className="truncate">{backName}</span>
          </Link>
        </SidebarTopBar>
        {collapsed && (
          <div className="shrink-0 px-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Link
                  to={backTo}
                  onClick={onItemClick}
                  aria-label={backLabel}
                  className="flex min-h-[44px] flex-col items-center justify-center rounded-md px-1 py-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                >
                  <ArrowLeft className="size-[1.375rem] shrink-0" />
                </Link>
              </TooltipTrigger>
              <TooltipContent side="right">{backLabel}</TooltipContent>
            </Tooltip>
          </div>
        )}

        <nav
          className={cn('flex-1 overflow-y-auto py-4', collapsed ? 'px-1' : 'space-y-8 px-2')}
          style={collapsed ? { scrollbarGutter: 'stable both-edges' } : undefined}
        >
          {managedOrgs.length > 0 && (
            <div className="space-y-0.5">
              {!collapsed && (
                <>
                  <div className="pb-2">
                    <OrgSelector orgs={managedOrgs} activeOrgId={activeOrgId} section={orgScope ? section : DEFAULT_ORG_SECTION} />
                  </div>
                  <p className="px-3 pb-1.5 text-sm font-medium text-muted-foreground">
                    {t('settingsPage.groups.organization', { defaultValue: 'Organization' })}
                  </p>
                </>
              )}
              {ORG_SECTIONS.map((item) => renderLeaf(item, orgSettingsUrl(activeOrgId, item.id), orgScope && item.id === section))}
            </div>
          )}
          <div className={cn('space-y-0.5', collapsed && managedOrgs.length > 0 && 'mt-4')}>
            {!collapsed && (
              <p className="px-3 pb-1.5 text-sm font-medium text-muted-foreground">
                {t('settingsPage.groups.account', { defaultValue: 'Account' })}
              </p>
            )}
            {ACCOUNT_SECTIONS.map((item) => renderLeaf(item, accountSettingsUrl(item.id), !orgScope && item.id === section))}
          </div>
        </nav>

        {/* 设置面没有页脚(照 x.ai):额度、更多、设置都不在这里,回到工作台再用 */}
      </div>
    </TooltipProvider>
  );
}
