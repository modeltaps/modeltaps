import * as React from 'react';

import { cn } from '@/lib/utils';

// Lightweight, dependency-free switch (no Radix). Controlled via `checked` +
// `onCheckedChange`; falls back to `onChange(checked)` for convenience.
const Switch = React.forwardRef(({ className, checked = false, onCheckedChange, onChange, disabled, ...props }, ref) => {
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
        'peer inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50',
        checked ? 'bg-primary' : 'bg-muted',
        className
      )}
      {...props}
    >
      <span
        className={cn(
          'pointer-events-none block h-5 w-5 rounded-full bg-background shadow ring-0 transition-transform',
          checked ? 'translate-x-5' : 'translate-x-0'
        )}
      />
    </button>
  );
});
Switch.displayName = 'Switch';

export { Switch };
