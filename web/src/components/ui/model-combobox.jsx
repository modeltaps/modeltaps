import * as React from 'react';
import PropTypes from 'prop-types';
import { Check, ChevronDown } from 'lucide-react';

import { cn } from '@/lib/utils';
import { FloatingPanel } from '@/components/ui/floating';
import { matchesSearch } from 'utils/textSearch';

// Creatable combobox: the text input IS the value (free text allowed). The
// dropdown offers `options` as filtered suggestions, matched case-insensitively.
// When the current input doesn't case-insensitively match any option, a "create"
// row lets you commit the typed value as-is. Used by ModelMappingInput so an
// alias / upstream model can be either picked from the channel's model list or
// typed by hand. Case-insensitivity lives only here (UI suggestions + dedup);
// the serialized model_mapping and runtime matching stay exact.
//
// options: string[]   createLabel: (input) => node
//
// Picker mode (onSelect given): the input is only a query. Picking a row, or pressing Enter on
// a non-empty query, calls onSelect(value) instead of onChange, and Enter never submits the
// surrounding form. allowCreate=false hides the "create" row (e.g. the typed value is already
// taken); inputClassName adds classes to the input (e.g. room for a leading icon).
//
// groups: [{ label, options }] in place of options lists the suggestions under headings, in the
// given order; a group with no match is left out.
export function ModelCombobox({
  value,
  onChange,
  onSelect,
  options: flatOptions = [],
  groups,
  placeholder,
  createLabel,
  allowCreate = true,
  disabled,
  className,
  inputClassName,
  id
}) {
  const [open, setOpen] = React.useState(false);
  const [active, setActive] = React.useState(0);
  // The value only filters the list once the user types: opening a filled picker lists every option.
  const [typed, setTyped] = React.useState(false);
  const rootRef = React.useRef(null);
  const inputRef = React.useRef(null);
  const panelRef = React.useRef(null);
  const listboxId = React.useId();

  React.useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      if (rootRef.current?.contains(e.target) || panelRef.current?.contains(e.target)) return;
      setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const q = (value || '').trim();
  const ql = q.toLowerCase();
  const filter = typed ? q : '';
  const sections = groups || [{ label: null, options: flatOptions }];
  const options = sections.flatMap((g) => g.options);
  const hasExact = options.some((o) => o && o.toLowerCase() === ql);
  const showCreate = allowCreate && typed && q !== '' && !hasExact;

  // Headings are drawn above a group's first row; only option and create rows are selectable.
  const rows = [...(showCreate ? [{ type: 'create', value: q }] : [])];
  for (const group of sections) {
    const matched = group.options.filter((o) => o && matchesSearch(filter, o));
    matched.forEach((o, i) => rows.push({ type: 'option', value: o, heading: i === 0 ? group.label : null }));
  }

  // An untyped list starts on the current value; a typed one on its best match.
  const selectedIndex = typed ? -1 : rows.findIndex((r) => r.type === 'option' && r.value === value);
  React.useEffect(() => {
    setActive(selectedIndex > 0 ? selectedIndex : 0);
  }, [value, open, selectedIndex]);

  React.useEffect(() => {
    if (!open) setTyped(false);
  }, [open]);

  const commit = (v) => {
    if (onSelect) onSelect(v);
    else onChange(v);
    setOpen(false);
  };

  const onKeyDown = (e) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (!open) setOpen(true);
      setActive((a) => Math.min(a + 1, Math.max(rows.length - 1, 0)));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === 'Enter') {
      if (open && rows[active]) {
        e.preventDefault();
        commit(rows[active].value);
      } else if (onSelect && q !== '') {
        e.preventDefault();
        if (allowCreate) commit(q);
      }
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  };

  return (
    <div ref={rootRef} className={cn('relative', className)}>
      <input
        ref={inputRef}
        id={id}
        type="text"
        role="combobox"
        aria-expanded={open}
        aria-controls={listboxId}
        autoComplete="off"
        disabled={disabled}
        value={value || ''}
        placeholder={placeholder}
        onChange={(e) => {
          onChange(e.target.value);
          setTyped(true);
          if (!open) setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown}
        className={cn(
          'flex h-9 w-full rounded-lg border border-input bg-field px-3 py-2 text-sm pr-8 placeholder:text-muted-foreground focus:outline-none focus:border-primary disabled:cursor-not-allowed disabled:opacity-50',
          inputClassName
        )}
      />
      {/* The arrow says the field is a picker; it opens the list (focusing the input) or closes it. */}
      <button
        type="button"
        tabIndex={-1}
        aria-hidden="true"
        disabled={disabled}
        onMouseDown={(e) => {
          e.preventDefault();
          if (open) setOpen(false);
          else if (document.activeElement === inputRef.current) setOpen(true);
          else inputRef.current?.focus();
        }}
        className="absolute inset-y-0 right-0 flex w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground disabled:pointer-events-none"
      >
        <ChevronDown className={cn('size-4 transition-transform', open && 'rotate-180')} />
      </button>
      <FloatingPanel
        ref={panelRef}
        open={open && rows.length > 0}
        anchorRef={inputRef}
        matchWidth
        maxHeight={240}
        id={listboxId}
        role="listbox"
        className="rounded-lg border border-border bg-card p-1 text-card-foreground shadow-md"
      >
        {rows.map((row, i) => (
          <React.Fragment key={row.type + ':' + row.value}>
            {row.heading && (
              <div role="presentation" className="px-2 pb-1 pt-2 text-xs text-muted-foreground first:pt-1">
                {row.heading}
              </div>
            )}
            <button
              type="button"
              // onMouseDown (not onClick) so the commit fires before the input's
              // blur closes the dropdown.
              onMouseDown={(e) => {
                e.preventDefault();
                commit(row.value);
              }}
              onMouseEnter={() => setActive(i)}
              className={cn(
                'flex w-full cursor-pointer select-none items-center rounded-md px-2 py-1.5 text-left text-sm outline-none',
                i === active && 'bg-muted'
              )}
            >
              <span className="min-w-0 flex-1 truncate">
                {row.type === 'create' ? (createLabel ? createLabel(row.value) : `"${row.value}"`) : row.value}
              </span>
              {row.type === 'option' && row.value === value && <Check className="ml-2 size-4 shrink-0 text-muted-foreground" />}
            </button>
          </React.Fragment>
        ))}
      </FloatingPanel>
    </div>
  );
}

ModelCombobox.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  onSelect: PropTypes.func,
  options: PropTypes.arrayOf(PropTypes.string),
  groups: PropTypes.arrayOf(PropTypes.shape({ label: PropTypes.node, options: PropTypes.arrayOf(PropTypes.string).isRequired })),
  placeholder: PropTypes.string,
  createLabel: PropTypes.func,
  allowCreate: PropTypes.bool,
  disabled: PropTypes.bool,
  className: PropTypes.string,
  inputClassName: PropTypes.string,
  id: PropTypes.string
};
