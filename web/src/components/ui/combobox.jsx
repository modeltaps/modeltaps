import * as React from 'react';
import { Command } from 'cmdk';
import { Check, ChevronsUpDown, Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { FloatingPanel } from '@/components/ui/floating';

// Searchable single-select (combobox). Dependency-light: cmdk drives filtering +
// keyboard nav, wrapped in the same popover shell / styling as ui/select so it
// reads as a sibling control. Filters by the visible label (cmdk matches each
// item's `value`, which we set to the label). Use when an option list is long
// enough that a plain Select becomes hard to scan (e.g. channel types).
//
// options: [{ value: string, label: string, icon?: node, hint?: node }]; `icon` renders before the
// label (also in the trigger), `hint` as muted text at the end of the row.
// groups (optional, replaces options): [{ key: string, heading?: node, options: [...] }]. Groups
// render in order with a separator between them; cmdk filters every group while searching and
// hides the separators and any group left without a match.

// Tallest a searchable dropdown grows before its list scrolls; below that it takes the room
// the window has (FloatingPanel caps it), with the search box pinned above the list.
export const SEARCH_PANEL_MAX_HEIGHT = 560;

// A printable key pressed on a closed trigger opens the dropdown with that key as the query,
// so typing on a focused trigger searches straight away.
export function typeaheadKey(e) {
  if (e.ctrlKey || e.metaKey || e.altKey || e.nativeEvent?.isComposing) return null;
  return e.key.length === 1 && e.key !== ' ' ? e.key : null;
}

// Search box for a cmdk dropdown: a leading search icon sized and spaced like the item icons,
// so the placeholder lines up with the item labels below. Focuses itself once the panel is
// shown: FloatingPanel renders the first frame hidden to measure it, which makes autoFocus miss.
export function CommandSearch({ value, onValueChange, placeholder, className }) {
  const ref = React.useRef(null);
  React.useEffect(() => {
    let frame;
    let tries = 0;
    const focus = () => {
      const el = ref.current;
      if (!el) return;
      el.focus({ preventScroll: true });
      if (document.activeElement !== el && tries++ < 10) frame = requestAnimationFrame(focus);
    };
    focus();
    return () => cancelAnimationFrame(frame);
  }, []);
  return (
    <div className={cn('flex items-center gap-2 border-b border-border px-3', className)}>
      <Search className="size-4 shrink-0 text-muted-foreground" aria-hidden />
      <Command.Input
        ref={ref}
        value={value}
        onValueChange={onValueChange}
        placeholder={placeholder}
        className="h-10 w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
      />
    </div>
  );
}

export function Combobox({
  value,
  onValueChange,
  options,
  groups,
  placeholder,
  searchPlaceholder,
  emptyText,
  id,
  className,
  contentClassName,
  disabled
}) {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState('');
  const rootRef = React.useRef(null);
  const triggerRef = React.useRef(null);
  const panelRef = React.useRef(null);
  const listId = React.useId();

  React.useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      // The list floats outside the root (see FloatingPanel), so clicks in it count as inside.
      if (rootRef.current?.contains(e.target) || panelRef.current?.contains(e.target)) return;
      setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const allOptions = groups ? groups.flatMap((g) => g.options) : options;
  const selected = allOptions.find((o) => o.value === value);

  const renderItem = (o) => (
    <Command.Item
      key={o.value}
      value={o.label}
      onSelect={() => {
        onValueChange?.(o.value);
        setOpen(false);
      }}
      className="relative flex w-full cursor-pointer select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm outline-none data-[selected=true]:bg-muted"
    >
      {o.value === value && (
        <span className="absolute left-2 flex size-4 items-center justify-center">
          <Check className="size-4" />
        </span>
      )}
      {o.icon && <span className="mr-2 flex size-4 shrink-0 items-center justify-center">{o.icon}</span>}
      <span className="truncate">{o.label}</span>
      {o.hint && <span className="ml-auto shrink-0 pl-3 text-xs text-muted-foreground">{o.hint}</span>}
    </Command.Item>
  );

  return (
    <div ref={rootRef} className="relative">
      <button
        ref={triggerRef}
        id={id}
        type="button"
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-haspopup="listbox"
        disabled={disabled}
        onClick={() => {
          setQuery('');
          setOpen((o) => !o);
        }}
        onKeyDown={(e) => {
          const key = open ? null : typeaheadKey(e);
          if (!key) return;
          e.preventDefault();
          setQuery(key);
          setOpen(true);
        }}
        className={cn(
          'flex h-9 w-full items-center justify-between rounded-lg border border-input bg-field px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
          className
        )}
      >
        <span className={cn('flex min-w-0 items-center gap-2', !selected && 'text-muted-foreground')}>
          {selected?.icon && <span className="flex size-4 shrink-0 items-center justify-center">{selected.icon}</span>}
          <span className="truncate">{selected ? selected.label : placeholder}</span>
        </span>
        <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
      </button>

      <FloatingPanel
        ref={panelRef}
        open={open}
        anchorRef={triggerRef}
        matchWidth
        maxHeight={SEARCH_PANEL_MAX_HEIGHT}
        className="flex flex-col overflow-hidden"
      >
        <Command
          onKeyDown={(e) => {
            // Close only the dropdown; preventDefault keeps a surrounding Sheet / Dialog open.
            if (e.key === 'Escape') {
              e.preventDefault();
              setOpen(false);
              triggerRef.current?.focus();
            }
          }}
          className={cn(
            'flex min-h-0 min-w-[8rem] flex-1 flex-col overflow-hidden rounded-lg border border-border bg-card text-card-foreground shadow-md',
            contentClassName
          )}
        >
          <CommandSearch value={query} onValueChange={setQuery} placeholder={searchPlaceholder} />
          <Command.List id={listId} className="min-h-0 flex-1 overflow-y-auto p-1">
            {emptyText && <Command.Empty className="px-2 py-3 text-center text-sm text-muted-foreground">{emptyText}</Command.Empty>}
            {groups
              ? groups.map((g, i) => (
                  <React.Fragment key={g.key}>
                    {i > 0 && <Command.Separator className="-mx-1 my-1 h-px bg-border" />}
                    <Command.Group
                      heading={g.heading}
                      className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-muted-foreground"
                    >
                      {g.options.map(renderItem)}
                    </Command.Group>
                  </React.Fragment>
                ))
              : options.map(renderItem)}
          </Command.List>
        </Command>
      </FloatingPanel>
    </div>
  );
}
