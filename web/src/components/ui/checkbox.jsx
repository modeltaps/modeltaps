import * as React from 'react';
import { Check, Minus } from 'lucide-react';

import { cn } from '@/lib/utils';

// Dependency-free checkbox (no Radix). Controlled via `checked` +
// `onCheckedChange`. Supports an `indeterminate` visual state.
const Checkbox = React.forwardRef(({ className, checked = false, indeterminate = false, onCheckedChange, disabled, ...props }, ref) => {
  const toggle = () => {
    if (disabled) return;
    onCheckedChange?.(!checked);
  };

  return (
    <button
      ref={ref}
      type="button"
      role="checkbox"
      aria-checked={indeterminate ? 'mixed' : checked}
      disabled={disabled}
      onClick={toggle}
      className={cn(
        'flex size-4 shrink-0 items-center justify-center rounded-sm border border-input ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        (checked || indeterminate) && 'border-primary bg-primary text-primary-foreground',
        className
      )}
      {...props}
    >
      {indeterminate ? <Minus className="size-3.5" /> : checked ? <Check className="size-3.5" /> : null}
    </button>
  );
});
Checkbox.displayName = 'Checkbox';

export { Checkbox };
