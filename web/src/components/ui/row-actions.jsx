import { createContext, useContext } from 'react';
import PropTypes from 'prop-types';
import { MoreHorizontal, ChevronDown } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel
} from '@/components/ui/dropdown-menu';

// Rendering surface for a row's actions. Tables leave this at the default
// ('table') and get the responsive three-tier collapse below. DataCards wraps
// its action area with a value='card' provider so every action expands into a
// labelled button. It carries structural intent (which surface is rendering),
// not a JS media query — there is no resize listener.
export const RowActionsSurfaceContext = createContext('table');

// Shared list-row action column. Unifies alignment (always right) and the
// "show common actions, collapse the rest under …" rule across every table.
//
// actions: Array<{
//   key?: string,          // unique key (falls back to label)
//   label: string,         // tooltip text / menu item / button text
//   icon: LucideIcon,      // required
//   onClick?: () => void,  // omitted for a menu action (use items instead)
//   disabled?: boolean,
//   destructive?: boolean, // red styling (e.g. delete)
//   primary?: boolean,     // pin inline in every tier when others collapse
//   overflow?: boolean,    // always collapse into … (table surface only)
//   items?: Array<{ key?, label, icon?, onClick, disabled? }> // menu action:
//     // renders as a dropdown button inline / on a card, and as a labelled
//     // group inside the … menu (this custom dropdown has no Radix submenu).
// }>
//
// Table surface tiers (pure Tailwind lg: classes, no matchMedia):
//   - total <= maxInline and no `overflow` flag: every action stays inline in
//     every width (identical to the pre-responsive behaviour — backward compat).
//   - total > maxInline (or any `overflow` flag): `primary` actions stay inline
//     in every tier; at md~lg the rest sit in the … menu; at >= lg the leading
//     `normal` actions (declaration order) are promoted to inline icon buttons up
//     to maxInlineLg (primary counts toward it), the remainder plus any `overflow`
//     item stay in …, and the … trigger hides (lg:hidden) when it would be empty.
// Card surface: mirrors the table's "few inline + rest under …" rule so the
//   action row stays on a single line at any width. Inline shows the `primary`
//   actions as outline icon+label buttons (or the leading 2 when none are
//   flagged); every other action collapses into a … menu (icon+label items,
//   destructive in red behind a separator) — menu actions become labelled groups.
export function RowActions({ actions, maxInline = 3, maxInlineLg = 6 }) {
  const { t } = useTranslation();
  const surface = useContext(RowActionsSurfaceContext);
  const items = (actions || []).filter(Boolean);
  if (items.length === 0) return null;

  // --- Card surface: inline the primary actions, collapse the rest into …. ---
  if (surface === 'card') {
    const hasPrimary = items.some((a) => a.primary);
    const inlineItems = hasPrimary ? items.filter((a) => a.primary) : items.slice(0, 2);
    const inlineSet = new Set(inlineItems);
    const menuItems = items.filter((a) => !inlineSet.has(a));

    const renderCardMenuItem = (a, i) => {
      const prevDestructive = i > 0 && menuItems[i - 1].destructive;
      const separator = a.destructive && i > 0 && !prevDestructive ? <DropdownMenuSeparator /> : null;
      if (a.items) {
        return (
          <div key={a.key || a.label}>
            {separator}
            <DropdownMenuLabel>{a.label}</DropdownMenuLabel>
            {a.items.map((it) => {
              const ItIcon = it.icon;
              return (
                <DropdownMenuItem key={it.key || it.label} disabled={it.disabled} onClick={it.onClick}>
                  {ItIcon ? <ItIcon /> : null} {it.label}
                </DropdownMenuItem>
              );
            })}
          </div>
        );
      }
      const Icon = a.icon;
      return (
        <div key={a.key || a.label}>
          {separator}
          <DropdownMenuItem disabled={a.disabled} className={a.destructive ? 'text-destructive' : undefined} onClick={a.onClick}>
            <Icon /> {a.label}
          </DropdownMenuItem>
        </div>
      );
    };

    return (
      <div className="flex flex-nowrap items-center gap-1.5">
        {inlineItems.map((a) => {
          const Icon = a.icon;
          if (a.items) {
            return (
              <DropdownMenu key={a.key || a.label}>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" disabled={a.disabled}>
                    <Icon /> {a.label} <ChevronDown className="opacity-60" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  {a.items.map((it) => {
                    const ItIcon = it.icon;
                    return (
                      <DropdownMenuItem key={it.key || it.label} disabled={it.disabled} onClick={it.onClick}>
                        {ItIcon ? <ItIcon /> : null} {it.label}
                      </DropdownMenuItem>
                    );
                  })}
                </DropdownMenuContent>
              </DropdownMenu>
            );
          }
          return (
            <Button
              key={a.key || a.label}
              variant="outline"
              size="sm"
              className={cn(a.destructive && 'text-destructive hover:text-destructive')}
              disabled={a.disabled}
              onClick={a.onClick}
            >
              <Icon /> {a.label}
            </Button>
          );
        })}
        {menuItems.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm" className="px-2" aria-label={t('common.actions')}>
                <MoreHorizontal />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">{menuItems.map(renderCardMenuItem)}</DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    );
  }

  // --- Table surface: static bucketing computed once per render. ---
  const collapse = items.length > maxInline || items.some((a) => a.overflow);
  const primaryItems = collapse ? items.filter((a) => a.primary) : [];
  const rest = collapse ? items.filter((a) => !a.primary) : [];
  const normal = rest.filter((a) => !a.overflow);
  const promoteCount = Math.max(0, maxInlineLg - primaryItems.length);
  const promoted = new Set(normal.slice(0, promoteCount));
  const emptyAtLg = collapse && rest.length > 0 && rest.every((a) => promoted.has(a));

  // Inline button for a plain action; responsiveClass hides it outside its tier.
  const renderInline = (a, responsiveClass) => {
    const Icon = a.icon;
    if (a.items) {
      return (
        <span key={a.key || a.label} className={cn('inline-flex', responsiveClass)}>
          <DropdownMenu>
            <Tooltip>
              <TooltipTrigger asChild>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" className="h-8 gap-1 px-1.5" aria-label={a.label} disabled={a.disabled}>
                    <Icon className="size-4" />
                    <ChevronDown className="size-3 opacity-60" />
                  </Button>
                </DropdownMenuTrigger>
              </TooltipTrigger>
              <TooltipContent>{a.label}</TooltipContent>
            </Tooltip>
            <DropdownMenuContent align="end">
              {a.items.map((it) => {
                const ItIcon = it.icon;
                return (
                  <DropdownMenuItem key={it.key || it.label} disabled={it.disabled} onClick={it.onClick}>
                    {ItIcon ? <ItIcon /> : null} {it.label}
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuContent>
          </DropdownMenu>
        </span>
      );
    }
    return (
      <Tooltip key={a.key || a.label}>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className={cn('size-8', a.destructive && 'text-destructive hover:text-destructive', responsiveClass)}
            aria-label={a.label}
            disabled={a.disabled}
            onClick={a.onClick}
          >
            <Icon className="size-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>{a.label}</TooltipContent>
      </Tooltip>
    );
  };

  // One … menu entry: a plain item, or a menu action as a labelled group. Items
  // promoted inline at lg get lg:hidden so they do not duplicate there.
  const renderMenuItem = (a, i) => {
    const responsiveClass = promoted.has(a) ? 'lg:hidden' : undefined;
    const prevDestructive = i > 0 && rest[i - 1].destructive;
    const separator = a.destructive && i > 0 && !prevDestructive ? <DropdownMenuSeparator /> : null;
    if (a.items) {
      return (
        <div key={a.key || a.label} className={responsiveClass}>
          {separator}
          <DropdownMenuLabel>{a.label}</DropdownMenuLabel>
          {a.items.map((it) => {
            const ItIcon = it.icon;
            return (
              <DropdownMenuItem key={it.key || it.label} disabled={it.disabled} onClick={it.onClick}>
                {ItIcon ? <ItIcon /> : null} {it.label}
              </DropdownMenuItem>
            );
          })}
        </div>
      );
    }
    const Icon = a.icon;
    return (
      <div key={a.key || a.label} className={responsiveClass}>
        {separator}
        <DropdownMenuItem disabled={a.disabled} className={a.destructive ? 'text-destructive' : undefined} onClick={a.onClick}>
          <Icon /> {a.label}
        </DropdownMenuItem>
      </div>
    );
  };

  return (
    <TooltipProvider delayDuration={150}>
      <div className="flex items-center justify-end gap-0.5">
        {items.map((a) => {
          if (!collapse) return renderInline(a);
          if (a.primary) return renderInline(a);
          if (promoted.has(a)) return renderInline(a, 'hidden lg:inline-flex');
          return null;
        })}
        {collapse && rest.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className={cn('size-8', emptyAtLg && 'lg:hidden')} aria-label={t('common.actions')}>
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">{rest.map(renderMenuItem)}</DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </TooltipProvider>
  );
}

const actionShape = PropTypes.shape({
  key: PropTypes.string,
  label: PropTypes.node,
  icon: PropTypes.elementType,
  onClick: PropTypes.func,
  disabled: PropTypes.bool,
  destructive: PropTypes.bool,
  primary: PropTypes.bool,
  overflow: PropTypes.bool,
  items: PropTypes.arrayOf(
    PropTypes.shape({
      key: PropTypes.string,
      label: PropTypes.node,
      icon: PropTypes.elementType,
      onClick: PropTypes.func,
      disabled: PropTypes.bool
    })
  )
});

RowActions.propTypes = {
  actions: PropTypes.arrayOf(actionShape),
  maxInline: PropTypes.number,
  maxInlineLg: PropTypes.number
};

export default RowActions;
