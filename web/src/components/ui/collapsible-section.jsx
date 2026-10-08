import * as React from 'react';
import { ChevronRight } from 'lucide-react';

import { cn } from '@/lib/utils';

// Progressive-disclosure section: a labelled toggle that hides advanced/optional
// fields behind a click, keeping forms focused on what's required. Collapsed by
// default; pass `defaultOpen` to start expanded (e.g. when editing a record that
// already has advanced values set, so nothing configured stays hidden). This is
// the form rule: required fields first, advanced ones collapsed.
// `plain` renders just the toggle + content with no surrounding box — use it when
// the children are themselves cards/sections (avoids a box-in-box look).
export function CollapsibleSection({ title, description, defaultOpen = false, plain = false, children, className }) {
  const [open, setOpen] = React.useState(defaultOpen);
  const contentId = React.useId();

  return (
    <div className={cn(!plain && 'rounded-lg border border-border', className)}>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={contentId}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          'flex w-full items-center gap-2 text-left font-medium text-foreground transition-colors',
          plain ? 'py-1 text-sm hover:text-foreground/80' : 'px-3 py-2.5 text-sm hover:bg-muted/50'
        )}
      >
        <ChevronRight className={cn('size-4 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
        <span className="flex-1">{title}</span>
        {description && <span className="truncate text-xs font-normal text-muted-foreground">{description}</span>}
      </button>
      {open && (
        <div id={contentId} className={cn(plain ? 'space-y-3 pt-3' : 'space-y-4 border-t border-border p-3')}>
          {children}
        </div>
      )}
    </div>
  );
}
