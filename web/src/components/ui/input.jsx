import * as React from 'react';

import { cn } from '@/lib/utils';

// Number fields show figures in tabular digits and without the browser's spin arrows: a value
// is typed, or stepped with a NumberField's own buttons.
export const NUMBER_INPUT =
  'tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none';

const Input = React.forwardRef(({ className, type, ...props }, ref) => {
  return (
    <input
      type={type}
      className={cn(
        'flex h-9 w-full rounded-lg border border-input bg-field px-3 py-2 text-sm file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:border-primary disabled:cursor-not-allowed disabled:opacity-50',
        type === 'number' && NUMBER_INPUT,
        className
      )}
      ref={ref}
      {...props}
    />
  );
});
Input.displayName = 'Input';

export { Input };
