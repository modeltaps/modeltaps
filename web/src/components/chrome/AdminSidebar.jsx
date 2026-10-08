import { Link, NavLink, useLocation } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ArrowLeft } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useIsRoot } from 'utils/common';
import { getUserReturnPath } from 'utils/adminPlane';
import { filterAdminSections, navLabel } from './nav-config';
import { SidebarTopBar } from './SidebarControls';

// ==============================|| CHROME — ADMIN SIDEBAR (admin plane, T19) ||============================== //
// Dedicated sidebar for the admin plane: no brand row, a "‹ 工作台" back row
// as the very first row (return path a), then the admin sections with system
// settings last. No MenuCard here.

// Return paths land back on where the user was before entering the admin plane
// (UX-2, via getUserReturnPath; falls back to the user-plane home).

export default function AdminSidebar({ collapsed = false, onItemClick, onToggleCollapse, onOpenCommand }) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const isRoot = useIsRoot();
  const sections = filterAdminSections({ isRoot });
  const backTo = getUserReturnPath();
  const backLabel = t('admin_console.back');

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
        key={item.id}
        to={item.url}
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
        {/* 第一行「‹ 工作台」:写去向,与个人设置面返回行同一样式;右端搜索 + 收起 */}
        <SidebarTopBar collapsed={collapsed} onOpenCommand={onOpenCommand} onToggleCollapse={onToggleCollapse}>
          <Link
            to={backTo}
            onClick={onItemClick}
            aria-label={backLabel}
            className="group flex min-w-0 items-center gap-2 text-sm font-medium text-foreground/80 transition-colors hover:text-foreground"
          >
            <ArrowLeft className="size-[1.125rem] shrink-0 text-muted-foreground group-hover:text-foreground" />
            <span className="truncate">{t('admin_console.workspace')}</span>
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
          className={cn('flex-1 overflow-y-auto py-4', collapsed ? 'px-1' : 'space-y-6 px-2')}
          style={collapsed ? { scrollbarGutter: 'stable both-edges' } : undefined}
        >
          {sections.map((section, sectionIdx) => (
            <div key={section.id} className={cn('space-y-0.5', collapsed && sectionIdx > 0 && 'mt-2 border-t border-border/50 pt-2')}>
              {!collapsed && <p className="px-3 pb-1.5 text-sm font-medium text-muted-foreground">{t(section.id)}</p>}
              {section.items.map(renderLeaf)}
            </div>
          ))}
        </nav>
      </div>
    </TooltipProvider>
  );
}
