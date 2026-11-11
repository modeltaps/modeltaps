import PropTypes from 'prop-types';
import { Check, ChevronDown } from 'lucide-react';

import { cn } from '@/lib/utils';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

// Page size picker for list footers, replacing the native <select>: a compact button with the
// current size and a centered chevron, opening a short menu with a check on the current option.
// `format` renders an option's label (e.g. "10 / page"); the default is the bare number.
export default function PageSizeSelect({ value, options, onChange, format = (n) => String(n), label, className }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={label}
          className={cn(
            'inline-flex h-7 items-center gap-1 rounded-md border border-input bg-background pl-2 pr-1.5 text-xs tabular-nums text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            className
          )}
        >
          {format(value)}
          <ChevronDown className="size-3.5 text-muted-foreground" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-24">
        {options.map((n) => (
          <DropdownMenuItem key={n} onClick={() => onChange(n)} className="justify-between gap-3 text-xs tabular-nums">
            {format(n)}
            {n === value ? <Check className="size-3.5" /> : <span className="size-3.5" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

PageSizeSelect.propTypes = {
  value: PropTypes.number.isRequired,
  options: PropTypes.arrayOf(PropTypes.number).isRequired,
  onChange: PropTypes.func.isRequired,
  format: PropTypes.func,
  label: PropTypes.string,
  className: PropTypes.string
};
