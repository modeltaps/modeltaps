import * as React from 'react';

import { cn } from '@/lib/utils';

// Lightweight, dependency-free switch (no Radix). Controlled via `checked` +
// `onCheckedChange`; falls back to `onChange(checked)` for convenience.
// size 'sm' is the compact variant for dense table rows.
const Switch = React.forwardRef(({ className, checked = false, onCheckedChange, onChange, disabled, size, ...props }, ref) => {
  const sm = size === 'sm';
  const toggle = () => {
    if (disabled) return;
    onCheckedChange?.(!checked);
    onChange?.(!checked);
  };

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      data-state={checked ? 'checked' : 'unchecked'}
      ref={ref}
      disabled={disabled}
      onClick={toggle}
      className={cn(
        sm ? 'h-[18px] w-8' : 'h-6 w-11',
        'peer inline-flex shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50',
        checked ? 'bg-primary' : 'bg-control-off',
        className
      )}
      {...props}
    >
      <span
        className={cn(
          'pointer-events-none block rounded-full shadow ring-0 transition-transform',
          sm ? 'size-3.5' : 'h-5 w-5',
          checked ? cn(sm ? 'translate-x-3.5' : 'translate-x-5', 'bg-primary-foreground') : 'translate-x-0 bg-white dark:bg-muted-foreground'
        )}
      />
    </button>
  );
});
Switch.displayName = 'Switch';

export { Switch };
