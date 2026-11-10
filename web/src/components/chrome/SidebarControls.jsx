import PropTypes from 'prop-types';
import { Menu, PanelLeft, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

// ==============================|| CHROME — SIDEBAR CONTROLS ||============================== //
// The panel has no top bar: the collapse button sits on the sidebar's brand row and the search
// row sits under the context switcher. When the rail is collapsed (desktop) or hidden (mobile),
// the expand / drawer buttons sit left of the page title (SidebarToggle).

const iconButtonClass =
  'flex size-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

export function SidebarIconButton({ label, tooltip, side = 'bottom', onClick, className, children }) {
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

SidebarIconButton.propTypes = {
  label: PropTypes.string.isRequired,
  tooltip: PropTypes.node,
  side: PropTypes.string,
  onClick: PropTypes.func,
  className: PropTypes.string,
  children: PropTypes.node
};

export function CollapseButton({ onClick }) {
  const { t } = useTranslation();
  return (
    <SidebarIconButton label={t('chrome.collapseSidebar', { defaultValue: 'Collapse sidebar' })} onClick={onClick}>
      <PanelLeft className="size-[1.125rem]" />
    </SidebarIconButton>
  );
}

CollapseButton.propTypes = { onClick: PropTypes.func };

// Search: a full row under the context switcher, an icon in the collapsed rail.
export function SidebarSearch({ collapsed = false, onClick }) {
  const { t } = useTranslation();
  const label = t('common.search', { defaultValue: 'Search' });
  const kbd = <kbd className="rounded border border-border bg-sidebar px-1 font-mono text-[0.6875rem] text-muted-foreground">⌘K</kbd>;
  if (collapsed) {
    return (
      <SidebarIconButton
        label={label}
        side="right"
        onClick={onClick}
        tooltip={
          <span className="flex items-center gap-2">
            {label}
            {kbd}
          </span>
        }
        className="size-9"
      >
        <Search className="size-[1.125rem]" />
      </SidebarIconButton>
    );
  }
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-[34px] w-full items-center gap-2.5 rounded-lg border border-border bg-background px-3 text-sm text-muted-foreground transition-colors hover:border-foreground/20 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <Search className="size-4 shrink-0" />
      <span className="flex-1 text-left">{label}</span>
      {kbd}
    </button>
  );
}

SidebarSearch.propTypes = { collapsed: PropTypes.bool, onClick: PropTypes.func };

// Left of the page title: opens the drawer below md; on desktop it only shows while the rail
// is collapsed, to expand it. Neither takes space when hidden (display:none).
export function SidebarToggle({ collapsed = false, onOpenMobile, onExpand }) {
  const { t } = useTranslation();
  return (
    <TooltipProvider delayDuration={150}>
      <SidebarIconButton label={t('chrome.menu', { defaultValue: 'Menu' })} onClick={onOpenMobile} className="md:hidden">
        <Menu className="size-5" />
      </SidebarIconButton>
      {collapsed && (
        <SidebarIconButton
          label={t('chrome.expandSidebar', { defaultValue: 'Expand sidebar' })}
          onClick={onExpand}
          className="hidden md:flex"
        >
          <PanelLeft className="size-5" />
        </SidebarIconButton>
      )}
    </TooltipProvider>
  );
}

SidebarToggle.propTypes = {
  collapsed: PropTypes.bool,
  onOpenMobile: PropTypes.func,
  onExpand: PropTypes.func
};
