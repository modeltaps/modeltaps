import { useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Columns3, Minus } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';

// ==============================|| LIST PAGE — COLUMN PICKER ||============================== //
// "Columns n/m": which table columns show, kept in localStorage. A column registry entry is
// { id, labelKey, fixed?, defaultHidden? }: fixed columns (selection, name, actions) always show and
// stay out of the menu. `available` lists the ids that exist for this user / context (org-only
// columns and the like); only those are counted and offered.

const buildDefaults = (columns) => Object.fromEntries(columns.map((c) => [c.id, !c.defaultHidden]));

// Bump `storageKey` when the defaults change so a stale saved state cannot hide a new default
// column; list the old keys in `legacyKeys` so they are cleaned up.
export function useColumnVisibility({ storageKey, legacyKeys = [], columns, available }) {
  const [visibility, setVisibility] = useState(() => {
    const defaults = buildDefaults(columns);
    try {
      legacyKeys.forEach((k) => localStorage.removeItem(k));
      const saved = JSON.parse(localStorage.getItem(storageKey));
      if (!saved || typeof saved !== 'object') return defaults;
      for (const c of columns) {
        if (!c.fixed && typeof saved[c.id] === 'boolean') defaults[c.id] = saved[c.id];
      }
    } catch {
      /* corrupted storage -> defaults */
    }
    return defaults;
  });

  useEffect(() => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(visibility));
    } catch {
      /* storage unavailable -> keep in-memory only */
    }
  }, [storageKey, visibility]);

  const menuColumns = useMemo(() => columns.filter((c) => !c.fixed && (!available || available.includes(c.id))), [columns, available]);
  const allVisible = menuColumns.every((c) => visibility[c.id]);
  const someVisible = menuColumns.some((c) => visibility[c.id]);

  return {
    visibility,
    menuColumns,
    allVisible,
    someVisible,
    visibleCount: menuColumns.filter((c) => visibility[c.id]).length,
    toggle: (id) => setVisibility((v) => ({ ...v, [id]: !v[id] })),
    toggleAll: () =>
      setVisibility((v) => {
        const next = { ...v };
        menuColumns.forEach((c) => {
          next[c.id] = !allVisible;
        });
        return next;
      }),
    reset: () => setVisibility(buildDefaults(columns))
  };
}

// Menu row with a checkbox glyph. A plain DropdownMenuItem closes the menu on click, which breaks
// multi-toggle; the glyph is a span (not the Checkbox component) to avoid a button in a button.
function ColumnToggleItem({ checked, indeterminate = false, label, onToggle }) {
  return (
    <button
      type="button"
      role="menuitemcheckbox"
      aria-checked={indeterminate ? 'mixed' : checked}
      onClick={onToggle}
      className="flex w-full cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted focus:bg-muted"
    >
      <span
        className={cn(
          'flex size-4 shrink-0 items-center justify-center rounded-sm border border-input',
          (checked || indeterminate) && 'border-primary bg-primary text-primary-foreground'
        )}
      >
        {indeterminate ? <Minus className="size-3.5" /> : checked ? <Check className="size-3.5" /> : null}
      </span>
      <span className="truncate">{label}</span>
    </button>
  );
}

ColumnToggleItem.propTypes = {
  checked: PropTypes.bool,
  indeterminate: PropTypes.bool,
  label: PropTypes.node,
  onToggle: PropTypes.func.isRequired
};

// ↑/↓/Home/End roving focus for the column menu items.
const focusItem = (root, key) => {
  const items = Array.from(root?.querySelectorAll('[role="menuitemcheckbox"]') ?? []);
  if (!items.length) return;
  const idx = items.indexOf(document.activeElement);
  let next;
  if (key === 'Home') next = 0;
  else if (key === 'End') next = items.length - 1;
  else if (key === 'ArrowDown') next = idx < 0 ? 0 : (idx + 1) % items.length;
  else next = idx < 0 ? items.length - 1 : (idx - 1 + items.length) % items.length;
  items[next].focus();
};

// The "Columns n/m" button and its menu; pass the object from useColumnVisibility as `columns`.
export default function ColumnPicker({ columns }) {
  const { t } = useTranslation();
  const menuRef = useRef(null);
  const { visibility, menuColumns, allVisible, someVisible, visibleCount, toggle, toggleAll, reset } = columns;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="h-8 gap-1.5"
          onKeyDown={(e) => {
            if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && menuRef.current) {
              e.preventDefault();
              focusItem(menuRef.current, e.key);
            }
          }}
        >
          <Columns3 className="size-4" />
          {t('listPage.columns')}
          <span className="text-xs tabular-nums text-muted-foreground">
            {visibleCount}/{menuColumns.length}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        ref={menuRef}
        align="end"
        className="max-h-96 w-52 overflow-y-auto"
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Home' || e.key === 'End') {
            e.preventDefault();
            focusItem(e.currentTarget, e.key);
          }
        }}
      >
        <DropdownMenuLabel>{t('listPage.selectColumns')}</DropdownMenuLabel>
        <ColumnToggleItem
          checked={allVisible}
          indeterminate={!allVisible && someVisible}
          label={t('listPage.columnsSelectAll')}
          onToggle={toggleAll}
        />
        <DropdownMenuSeparator />
        {menuColumns.map((c) => (
          <ColumnToggleItem key={c.id} checked={!!visibility[c.id]} label={t(c.labelKey)} onToggle={() => toggle(c.id)} />
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={reset}>{t('listPage.resetColumns')}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

ColumnPicker.propTypes = { columns: PropTypes.object.isRequired };
