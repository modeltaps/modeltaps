import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { Menu, PanelLeft, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

// ==============================|| CHROME — SIDEBAR CONTROLS ||============================== //
// The panel has no top bar: the search and collapse buttons sit at the end of the sidebar's first
// row (brand row, or the title back row in platform admin and 设置). When the rail is collapsed (desktop) or hidden (mobile),
// the expand / drawer buttons sit left of the page title (SidebarToggle).

// Menu rows of the sidebars (the workbench / admin menu and the 设置 tree). Active: a subtle
// raised surface with a hairline, no fill colour or indicator bar.
export const sidebarRow = {
  base: 'group flex w-full items-center rounded-lg text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
  active: 'bg-sidebar-accent font-medium text-foreground shadow-[inset_0_0_0_1px_var(--border)]',
  idle: 'text-foreground/85 hover:bg-sidebar-accent/70 hover:text-foreground',
  // Collapsed rail: a 36px square icon button centred in the column, its name in a tooltip.
  collapsed: 'mx-auto size-9 w-9 justify-center',
  // Collapsed rail: a short centred rule between menu sections instead of their headings.
  divider: 'mx-auto my-2 h-px w-5 bg-border'
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

// A row back to where the user came from (platform admin and 设置 show it where the workbench has
// its context switcher, at the same height). Its arrow and text use the columns of the menu rows
// below, so they line up. Collapsed: the arrow only, with the label as tooltip.
// The back chevron of back rows, drawn like the iOS one (tall, narrow, semibold stroke). Its viewBox hugs the stroke, so at 1em tall the stroke spans the
// glyph height of the text next to it (lucide's chevron fills only the middle half of its box).
export function BackChevron({ className }) {
  return (
    <svg
      viewBox="0 0 9 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={cn('h-[1em] w-[0.5625em] shrink-0', className)}
    >
      <path d="M7.5 1.25 1.5 8l6 6.75" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

BackChevron.propTypes = { className: PropTypes.string };

// The title row that takes the brand's place in platform admin ("‹ 平台管理") and 设置 ("‹ 设置"):
// the space's name as the heading, and the whole row leads back to the workbench. Collapsed, the
// chevron alone remains.
export function TitleBackRow({ to, title, label, collapsed = false, onClick }) {
  const link = (
    <Link
      to={to}
      onClick={onClick}
      aria-label={label}
      className={cn(
        'group flex items-center rounded-lg transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        collapsed ? 'size-9 justify-center text-lg' : 'h-9 w-full gap-2.5 pr-2 text-base'
      )}
    >
      <span
        className={cn(
          'flex shrink-0 justify-center text-muted-foreground transition-colors group-hover:text-foreground',
          !collapsed && 'w-7'
        )}
      >
        <BackChevron />
      </span>
      {!collapsed && <span className="min-w-0 truncate font-semibold text-foreground">{title}</span>}
    </Link>
  );
  return (
    <Tooltip>
      <TooltipTrigger asChild>{link}</TooltipTrigger>
      <TooltipContent side={collapsed ? 'right' : 'bottom'}>{label}</TooltipContent>
    </Tooltip>
  );
}

TitleBackRow.propTypes = {
  to: PropTypes.string.isRequired,
  title: PropTypes.string.isRequired,
  label: PropTypes.string.isRequired,
  collapsed: PropTypes.bool,
  onClick: PropTypes.func
};

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
