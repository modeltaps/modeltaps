import * as React from 'react';
import { Command } from 'cmdk';
import { Check, ChevronsUpDown } from 'lucide-react';

import { cn } from '@/lib/utils';

// Searchable single-select (combobox). Dependency-light: cmdk drives filtering +
// keyboard nav, wrapped in the same popover shell / styling as ui/select so it
// reads as a sibling control. Filters by the visible label (cmdk matches each
// item's `value`, which we set to the label). Use when an option list is long
// enough that a plain Select becomes hard to scan (e.g. channel types).
//
// options: [{ value: string, label: string }]
export function Combobox({
  value,
  onValueChange,
  options,
  placeholder,
  searchPlaceholder,
  emptyText,
  id,
  className,
  contentClassName,
  disabled
}) {
  const [open, setOpen] = React.useState(false);
  const rootRef = React.useRef(null);
  const listId = React.useId();

  React.useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const selected = options.find((o) => o.value === value);

  return (
    <div ref={rootRef} className="relative">
      <button
        id={id}
        type="button"
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-haspopup="listbox"
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className={cn(
          'flex h-9 w-full items-center justify-between rounded-lg border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
          className
        )}
      >
        <span className={cn('truncate', !selected && 'text-muted-foreground')}>{selected ? selected.label : placeholder}</span>
        <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
      </button>

      {open && (
        <Command
          className={cn(
            'absolute z-50 mt-1 w-full min-w-[8rem] overflow-hidden rounded-lg border border-border bg-card text-card-foreground shadow-md',
            contentClassName
          )}
        >
          <div className="border-b border-border px-2">
            <Command.Input
              autoFocus
              placeholder={searchPlaceholder}
              className="h-9 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
          </div>
          <Command.List id={listId} className="max-h-60 overflow-y-auto p-1">
            {emptyText && <Command.Empty className="px-2 py-3 text-center text-sm text-muted-foreground">{emptyText}</Command.Empty>}
            {options.map((o) => (
              <Command.Item
                key={o.value}
                value={o.label}
                onSelect={() => {
                  onValueChange?.(o.value);
                  setOpen(false);
                }}
                className="relative flex w-full cursor-pointer select-none items-center rounded-md py-1.5 pl-8 pr-2 text-sm outline-none data-[selected=true]:bg-muted"
              >
                {o.value === value && (
                  <span className="absolute left-2 flex size-4 items-center justify-center">
                    <Check className="size-4" />
                  </span>
                )}
                {o.label}
              </Command.Item>
            ))}
          </Command.List>
        </Command>
      )}
    </div>
  );
}
