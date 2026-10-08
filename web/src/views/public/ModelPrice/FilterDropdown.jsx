import { useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { ChevronDown } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';

// Single-select filter popover for the ModelPrice page.
// Trigger shows "label" when neutral (value === all/first) and
// "label: selected" with active styling once a non-neutral option is picked.
// Clicking an option commits immediately and closes the panel.
export default function FilterDropdown({
  icon: Icon,
  label,
  value,
  options,
  onSelect,
  active,
  searchable = false,
  searchPlaceholder,
  emptyText,
  align = 'left'
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const rootRef = useRef(null);

  useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) {
        setOpen(false);
        setSearch('');
      }
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const selectedOption = useMemo(() => options.find((o) => o.value === value), [options, value]);

  const filtered = useMemo(() => {
    if (!searchable || !search) return options;
    const lower = search.toLowerCase();
    return options.filter((opt) => (opt.label ?? '').toLowerCase().includes(lower));
  }, [options, search, searchable]);

  const handleSelect = (opt) => {
    onSelect(opt.value);
    setOpen(false);
    setSearch('');
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className={cn(
          'inline-flex h-9 items-center gap-1.5 rounded-md border px-2.5 text-[0.8125rem] font-medium transition-colors',
          active
            ? 'border-foreground/40 bg-muted text-foreground shadow-sm'
            : 'border-input bg-background text-foreground hover:border-foreground/30'
        )}
      >
        {Icon && <Icon className="size-4 shrink-0" />}
        <span className="truncate max-w-[200px]">{active && selectedOption ? `${label}: ${selectedOption.label}` : label}</span>
        <ChevronDown className="size-4 shrink-0 opacity-50" />
      </button>
      {open && (
        <div
          className={cn(
            'absolute top-full z-50 mt-1 w-64 rounded-md border border-border bg-card text-card-foreground shadow-md',
            align === 'right' ? 'right-0' : 'left-0'
          )}
        >
          {searchable && (
            <div className="p-2">
              <Input
                type="text"
                placeholder={searchPlaceholder}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="h-8"
                onClick={(e) => e.stopPropagation()}
              />
            </div>
          )}
          <div className="max-h-64 overflow-y-auto p-1">
            {filtered.length === 0 ? (
              <div className="py-6 text-center text-sm text-muted-foreground">{emptyText}</div>
            ) : (
              filtered.map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => handleSelect(opt)}
                  className={cn(
                    'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted',
                    value === opt.value && 'bg-muted font-medium'
                  )}
                >
                  {opt.icon}
                  <span className="truncate">{opt.label}</span>
                  {opt.badge}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

FilterDropdown.propTypes = {
  icon: PropTypes.elementType,
  label: PropTypes.string.isRequired,
  value: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  options: PropTypes.arrayOf(
    PropTypes.shape({
      value: PropTypes.oneOfType([PropTypes.string, PropTypes.number]).isRequired,
      label: PropTypes.string,
      icon: PropTypes.node,
      badge: PropTypes.node
    })
  ).isRequired,
  onSelect: PropTypes.func.isRequired,
  active: PropTypes.bool,
  searchable: PropTypes.bool,
  searchPlaceholder: PropTypes.string,
  emptyText: PropTypes.string,
  align: PropTypes.string
};
