import * as React from 'react';
import { Check, ChevronDown } from 'lucide-react';

import { cn } from '@/lib/utils';

// Dependency-free Select (no Radix). Compound API mirrors shadcn:
// <Select value onValueChange><SelectTrigger><SelectValue/></SelectTrigger>
// <SelectContent><SelectItem value>…</SelectItem></SelectContent></Select>
const SelectContext = React.createContext(null);

function Select({ value, onValueChange, children }) {
  const [open, setOpen] = React.useState(false);
  const rootRef = React.useRef(null);
  const [labels, setLabels] = React.useState({});

  React.useEffect(() => {
    if (!open) return;
    const onDocClick = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const registerLabel = React.useCallback((val, label) => {
    setLabels((prev) => (prev[val] === label ? prev : { ...prev, [val]: label }));
  }, []);

  return (
    <SelectContext.Provider value={{ value, onValueChange, open, setOpen, labels, registerLabel }}>
      <div ref={rootRef} className="relative">
        {children}
      </div>
    </SelectContext.Provider>
  );
}

const SelectTrigger = React.forwardRef(({ className, children, ...props }, ref) => {
  const ctx = React.useContext(SelectContext);
  return (
    <button
      ref={ref}
      type="button"
      aria-haspopup="listbox"
      aria-expanded={ctx.open}
      onClick={() => ctx.setOpen((o) => !o)}
      className={cn(
        'flex h-9 w-full items-center justify-between rounded-lg border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        className
      )}
      {...props}
    >
      {children}
      <ChevronDown className="size-4 opacity-50" />
    </button>
  );
});
SelectTrigger.displayName = 'SelectTrigger';

function SelectValue({ placeholder, className, children }) {
  const ctx = React.useContext(SelectContext);
  const hasValue = ctx.value !== undefined && ctx.value !== '';
  const resolved = hasValue ? ctx.labels[ctx.value] ?? ctx.value : '';
  const display = children != null && hasValue ? children : resolved;
  return <span className={cn('truncate', !display && 'text-muted-foreground', className)}>{display || placeholder}</span>;
}

function SelectContent({ className, children }) {
  const ctx = React.useContext(SelectContext);
  // Stay mounted when closed (hidden via CSS) so SelectItem label registration
  // always runs and SelectValue can resolve translated labels on first render.
  // `hidden` (display:none) also keeps the option buttons out of the tab order.
  return (
    <div
      className={cn(
        'absolute z-50 mt-1 max-h-72 w-full min-w-[8rem] overflow-y-auto rounded-lg border border-border bg-card p-1 text-card-foreground shadow-md',
        !ctx.open && 'hidden',
        className
      )}
      role="listbox"
      aria-hidden={!ctx.open}
    >
      {children}
    </div>
  );
}

const SelectItem = React.forwardRef(({ className, value, children, ...props }, ref) => {
  const ctx = React.useContext(SelectContext);
  const selected = ctx.value === value;
  const label = typeof children === 'string' ? children : value;

  React.useEffect(() => {
    ctx.registerLabel(value, label);
  }, [value, label]);

  return (
    <button
      ref={ref}
      type="button"
      role="option"
      aria-selected={selected}
      onClick={() => {
        ctx.onValueChange?.(value);
        ctx.setOpen(false);
      }}
      className={cn(
        'relative flex w-full cursor-pointer select-none items-center rounded-md py-1.5 pl-8 pr-2 text-left text-sm outline-none hover:bg-muted focus:bg-muted',
        className
      )}
      {...props}
    >
      {selected && (
        <span className="absolute left-2 flex size-4 items-center justify-center">
          <Check className="size-4" />
        </span>
      )}
      {children}
    </button>
  );
});
SelectItem.displayName = 'SelectItem';

export { Select, SelectTrigger, SelectValue, SelectContent, SelectItem };
