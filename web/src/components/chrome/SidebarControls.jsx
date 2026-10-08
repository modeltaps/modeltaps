import PropTypes from 'prop-types';
import { Menu, PanelLeft, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

// ==============================|| CHROME — SIDEBAR CONTROLS ||============================== //
// 面板没有顶部栏:搜索与收起按钮放在侧栏第一行(四种侧栏共用 SidebarTopBar),
// 收起后的展开按钮与手机端的抽屉菜单按钮放在页面标题左侧(SidebarToggle)。

const iconButtonClass =
  'flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

function IconButton({ label, tooltip, side = 'bottom', onClick, className, children }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" aria-label={label} onClick={onClick} className={cn(iconButtonClass, className)}>
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side={side}>{tooltip ?? label}</TooltipContent>
    </Tooltip>
  );
}

IconButton.propTypes = {
  label: PropTypes.string.isRequired,
  tooltip: PropTypes.node,
  side: PropTypes.string,
  onClick: PropTypes.func,
  className: PropTypes.string,
  children: PropTypes.node
};

export function SidebarSearchButton({ collapsed = false, onClick }) {
  const { t } = useTranslation();
  const label = t('common.search', { defaultValue: 'Search' });
  return (
    <IconButton
      label={label}
      side={collapsed ? 'right' : 'bottom'}
      onClick={onClick}
      tooltip={
        <span className="flex items-center gap-2">
          {label}
          <kbd className="rounded border border-border/60 px-1 font-mono text-[0.625rem]">⌘K</kbd>
        </span>
      }
    >
      <Search className="size-[1.125rem]" />
    </IconButton>
  );
}

SidebarSearchButton.propTypes = { collapsed: PropTypes.bool, onClick: PropTypes.func };

// 侧栏第一行(h-14):左侧是各侧栏原有内容(logo / 返回行),右端依次是搜索、收起。
// 收起态只剩搜索图标;各侧栏原有的收起态内容由调用方接在下面。
export function SidebarTopBar({ collapsed = false, onOpenCommand, onToggleCollapse, children }) {
  const { t } = useTranslation();
  if (collapsed) {
    return (
      <div className="flex h-14 shrink-0 items-center justify-center">
        <SidebarSearchButton collapsed onClick={onOpenCommand} />
      </div>
    );
  }
  return (
    <div className="flex h-14 shrink-0 items-center gap-1 pl-4 pr-3">
      <div className="flex min-w-0 flex-1 items-center">{children}</div>
      <SidebarSearchButton onClick={onOpenCommand} />
      {onToggleCollapse && (
        <IconButton label={t('chrome.collapseSidebar', { defaultValue: 'Collapse sidebar' })} onClick={onToggleCollapse}>
          <PanelLeft className="size-[1.125rem]" />
        </IconButton>
      )}
    </div>
  );
}

SidebarTopBar.propTypes = {
  collapsed: PropTypes.bool,
  onOpenCommand: PropTypes.func,
  onToggleCollapse: PropTypes.func,
  children: PropTypes.node
};

// 页面标题左侧的按钮:手机端(md 以下)打开侧栏抽屉;电脑端仅在侧栏收起时出现,用于展开。
// 两个按钮都不显示时不占位(display:none 不参与 flex 间距)。
export function SidebarToggle({ collapsed = false, onOpenMobile, onExpand }) {
  const { t } = useTranslation();
  return (
    <TooltipProvider delayDuration={150}>
      <IconButton label={t('chrome.menu', { defaultValue: 'Menu' })} onClick={onOpenMobile} className="md:hidden">
        <Menu className="size-5" />
      </IconButton>
      {collapsed && (
        <IconButton label={t('chrome.expandSidebar', { defaultValue: 'Expand sidebar' })} onClick={onExpand} className="hidden md:flex">
          <PanelLeft className="size-5" />
        </IconButton>
      )}
    </TooltipProvider>
  );
}

SidebarToggle.propTypes = {
  collapsed: PropTypes.bool,
  onOpenMobile: PropTypes.func,
  onExpand: PropTypes.func
};
