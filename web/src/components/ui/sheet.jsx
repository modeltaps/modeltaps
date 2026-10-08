import * as React from 'react';
import { X } from 'lucide-react';

import { cn } from '@/lib/utils';

// Dependency-free Sheet (no Radix). A right-side slide-in panel used for long
// forms. Controlled via `open` + `onOpenChange`.
function Sheet({ open, onOpenChange, children }) {
  React.useEffect(() => {
    if (!open) return undefined;
    const onKey = (e) => {
      if (e.key === 'Escape') onOpenChange?.(false);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open, onOpenChange]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[1300]">
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm"
        onClick={() => onOpenChange?.(false)}
        aria-hidden="true"
      />
      {children}
    </div>
  );
}

// `side` anchors the panel: 'right' (default) slides in from the right for long
// forms; 'bottom' rises from the bottom as a rounded sheet for mobile detail views.
const SheetContent = React.forwardRef(({ className, children, onClose, side = 'right', ...props }, ref) => (
  <div
    ref={ref}
    role="dialog"
    aria-modal="true"
    className={cn(
      'absolute flex flex-col bg-card shadow-xl',
      side === 'bottom'
        ? 'inset-x-0 bottom-0 max-h-[90vh] w-full rounded-t-xl border-t border-border'
        : 'inset-y-0 right-0 w-full max-w-xl border-l border-border',
      className
    )}
    {...props}
  >
    <button
      type="button"
      aria-label="Close"
      onClick={onClose}
      className="absolute right-4 top-4 rounded-md text-muted-foreground transition-colors hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <X className="size-5" />
    </button>
    {children}
  </div>
));
SheetContent.displayName = 'SheetContent';

function SheetHeader({ className, ...props }) {
  return <div className={cn('shrink-0 border-b border-border px-6 py-4', className)} {...props} />;
}

function SheetTitle({ className, children, ...props }) {
  return (
    <h2 className={cn('text-lg font-semibold text-foreground', className)} {...props}>
      {children}
    </h2>
  );
}

function SheetDescription({ className, ...props }) {
  return <p className={cn('text-sm text-muted-foreground', className)} {...props} />;
}

function SheetBody({ className, ...props }) {
  return <div className={cn('min-h-0 flex-1 overflow-y-auto px-6 py-4', className)} {...props} />;
}

function SheetFooter({ className, ...props }) {
  return (
    <div
      className={cn('shrink-0 border-t border-border px-6 py-4 flex items-center justify-end gap-2', className)}
      {...props}
    />
  );
}

export { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter };
