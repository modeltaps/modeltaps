import * as React from 'react';

import { cn } from '@/lib/utils';
import { Label } from '@/components/ui/label';

// Shared field group primitive: Label + control + hint/error with the
// standardized 8px (space-y-2) label↔control spacing. Field↔field spacing
// (space-y-5) is the responsibility of the surrounding form container.
//
// `row` switches to a settings-row layout (label+hint on the left, control on a
// fixed-width column on the right) — use for short-value controls (selects, small
// numbers) so they don't stretch across a wide panel. Keep the default stacked
// layout for free-text / long fields (textareas, model lists). `controlClassName`
// overrides the right-column width in row mode.
//
// `hint` is a short (≤1 line) inline note. `help` is a long, rule-style explanation
// (e.g. wildcard syntax) — it does NOT render inline; instead a `?` toggle next to
// the label discloses it on demand, keeping help-heavy forms from ballooning. Rule:
// "what it is" → hint or nothing; "the rule for filling it" → help.
const FormField = React.forwardRef(
  ({ id, htmlFor, label, required, hint, help, error, row = false, controlClassName, className, children, ...props }, ref) => {
    const labelFor = htmlFor ?? id;
    const errorMessage = error && typeof error === 'object' ? error.message : error;
    const [helpOpen, setHelpOpen] = React.useState(false);

    const labelEl = (label || help) && (
      <div className="flex items-center gap-1.5">
        {label && (
          <Label htmlFor={labelFor} className="w-fit gap-0">
            {label}
            {required && <span className="ml-0.5 text-destructive">*</span>}
          </Label>
        )}
        {help && (
          <button
            type="button"
            aria-expanded={helpOpen}
            aria-label="Help"
            onClick={() => setHelpOpen((o) => !o)}
            className="flex size-4 shrink-0 items-center justify-center rounded-full border border-border text-[10px] font-semibold leading-none text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
          >
            ?
          </button>
        )}
      </div>
    );

    const helpEl = help && helpOpen && (
      <div className="rounded-md bg-muted/50 px-3 py-2 text-xs leading-relaxed text-muted-foreground">{help}</div>
    );

    if (row) {
      return (
        <div ref={ref} className={cn('space-y-2', className)} {...props}>
          <div className={cn('flex justify-between gap-4', hint ? 'items-start' : 'items-center')}>
            <div className="min-w-0 space-y-1">
              {labelEl}
              {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
              {errorMessage && <p className="text-sm text-destructive">{errorMessage}</p>}
            </div>
            <div className={cn('w-56 shrink-0', controlClassName)}>{children}</div>
          </div>
          {helpEl}
        </div>
      );
    }

    return (
      <div ref={ref} className={cn('space-y-2', className)} {...props}>
        {labelEl}
        {children}
        {hint && <p className="text-sm text-muted-foreground">{hint}</p>}
        {helpEl}
        {errorMessage && <p className="text-sm text-destructive">{errorMessage}</p>}
      </div>
    );
  }
);
FormField.displayName = 'FormField';

export { FormField };
