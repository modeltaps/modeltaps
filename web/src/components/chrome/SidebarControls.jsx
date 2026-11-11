import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { Menu, PanelLeft, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

// ==============================|| CHROME — SIDEBAR CONTROLS ||============================== //
// The panel has no top bar: the search and collapse buttons sit at the end of the sidebar's first
// row (brand row, or the back row on 设置). When the rail is collapsed (desktop) or hidden (mobile),
// the expand / drawer buttons sit left of the page title (SidebarToggle).

// Menu rows of the sidebars (the workbench / admin menu and the 设置 tree). Active: a subtle
// raised surface with a hairline, no fill colour or indicator bar.
export const sidebarRow = {
  base: 'group flex w-full items-center rounded-lg text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
  active: 'bg-sidebar-accent font-medium text-foreground shadow-[inset_0_0_0_1px_var(--border)]',
  idle: 'text-foreground/85 hover:bg-sidebar-accent/70 hover:text-foreground'
};

const iconButtonClass =
  'flex size-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

// The page an icon link points to is open: the same raised surface as an active menu row.
const iconButtonActiveClass = 'bg-sidebar-accent text-foreground shadow-[inset_0_0_0_1px_var(--border)]';

// An icon with a tooltip. With `to` it is a link to a page (and `active` marks that page as the
// current one); without, a button.
export function SidebarIconButton({ label, tooltip, side = 'bottom', onClick, to, active = false, className, children }) {
  const classes = cn(iconButtonClass, active && iconButtonActiveClass, className);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {to ? (
          <Link to={to} aria-label={label} aria-current={active ? 'page' : undefined} onClick={onClick} className={classes}>
            {children}
          </Link>
        ) : (
          <button type="button" aria-label={label} onClick={onClick} className={classes}>
            {children}
          </button>
        )}
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
  to: PropTypes.string,
  active: PropTypes.bool,
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

// Search: an icon that opens the panel's search palette (CommandMenu). It sits at the end of the
// sidebar's top row (after the brand or the back row) and under the switcher in the collapsed rail.
export function SidebarSearch({ collapsed = false, onClick }) {
  const { t } = useTranslation();
  const label = t('common.search', { defaultValue: 'Search' });
  const kbd = <kbd className="rounded border border-border bg-sidebar px-1 font-mono text-[0.6875rem] text-muted-foreground">⌘K</kbd>;
  return (
    <SidebarIconButton
      label={label}
      side={collapsed ? 'right' : 'bottom'}
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
