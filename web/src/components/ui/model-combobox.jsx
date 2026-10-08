import * as React from 'react';
import PropTypes from 'prop-types';
import { cn } from '@/lib/utils';

// Creatable combobox: the text input IS the value (free text allowed). The
// dropdown offers `options` as filtered suggestions, matched case-insensitively.
// When the current input doesn't case-insensitively match any option, a "create"
// row lets you commit the typed value as-is. Used by ModelMappingInput so an
// alias / upstream model can be either picked from the channel's model list or
// typed by hand. Case-insensitivity lives only here (UI suggestions + dedup);
// the serialized model_mapping and runtime matching stay exact.
//
// options: string[]   createLabel: (input) => node
export function ModelCombobox({ value, onChange, options = [], placeholder, createLabel, disabled, className, id }) {
  const [open, setOpen] = React.useState(false);
  const [active, setActive] = React.useState(0);
  const rootRef = React.useRef(null);
  const listboxId = React.useId();

  React.useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const q = (value || '').trim();
  const ql = q.toLowerCase();
  const filtered = options.filter((o) => o && o.toLowerCase().includes(ql));
  const hasExact = options.some((o) => o && o.toLowerCase() === ql);
  const showCreate = q !== '' && !hasExact;

  const rows = [
    ...(showCreate ? [{ type: 'create', value: q }] : []),
    ...filtered.map((o) => ({ type: 'option', value: o }))
  ];

  React.useEffect(() => {
    setActive(0);
  }, [value, open]);

  const commit = (v) => {
    onChange(v);
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
      }
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  };

  return (
    <div ref={rootRef} className={cn('relative', className)}>
      <input
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
          if (!open) setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown}
        className="flex h-9 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
      />
      {open && rows.length > 0 && (
        <div
          id={listboxId}
          role="listbox"
          className="absolute z-50 mt-1 max-h-60 w-full overflow-y-auto rounded-lg border border-border bg-card p-1 text-card-foreground shadow-md"
        >
          {rows.map((row, i) => (
            <button
              key={row.type + ':' + row.value}
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
              {row.type === 'create' ? (createLabel ? createLabel(row.value) : `"${row.value}"`) : row.value}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

ModelCombobox.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  options: PropTypes.arrayOf(PropTypes.string),
  placeholder: PropTypes.string,
  createLabel: PropTypes.func,
  disabled: PropTypes.bool,
  className: PropTypes.string,
  id: PropTypes.string
};
