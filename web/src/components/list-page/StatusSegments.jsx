import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { ChevronDown } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Segmented } from '@/components/objects';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

// ==============================|| LIST PAGE — STATUS SEGMENTS ||============================== //
// The list's top-level split (All / Enabled / Disabled …) with a count per segment. In the table
// layout it is a segmented control; in the phone list (`chips`) a row of pill chips that scrolls
// sideways instead of wrapping. `counts` is { [value]: number } or null while unknown.
// `more` lists values that are folded into one dropdown segment named `moreLabel` (in the table
// layout) and left out while their count is 0 and they are not selected (in both layouts).

const SEGMENT =
  'h-7 shrink-0 whitespace-nowrap rounded-[7px] px-3 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';
const SEGMENT_ON = 'bg-control-thumb text-foreground shadow-sm';
const SEGMENT_OFF = 'text-muted-foreground hover:text-foreground';

function CountBadge({ value }) {
  return <span className="min-w-[1.25rem] rounded-[5px] bg-foreground/[0.07] px-1 text-[11px] tabular-nums leading-[18px]">{value}</span>;
}

CountBadge.propTypes = { value: PropTypes.number };

export default function StatusSegments({ value, onChange, options, counts = null, label, chips = false, more = [], moreLabel }) {
  const { t } = useTranslation();
  const name = label ?? t('listPage.segments');
  const folded = new Set(more);
  // A folded value is shown only when it has rows (or is the current one); unknown counts show all.
  options = options.filter((o) => !folded.has(o.value) || o.value === value || !counts || (counts[o.value] ?? 0) > 0);
  if (chips) {
    return (
      <div role="radiogroup" aria-label={name} className="-mx-4 flex gap-1.5 overflow-x-auto px-4 no-scrollbar">
        {options.map((o) => {
          const on = value === o.value;
          return (
            <button
              key={o.value}
              type="button"
              role="radio"
              aria-checked={on}
              onClick={() => onChange(o.value)}
              className={cn(
                'inline-flex h-8 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border px-3 text-[13px] transition-colors',
                on ? 'border-foreground bg-foreground text-background' : 'border-input bg-field text-foreground hover:bg-muted'
              )}
            >
              {o.label}
              {counts && (
                <span className={cn('tabular-nums', on ? 'text-background/70' : 'text-muted-foreground')}>{counts[o.value] ?? 0}</span>
              )}
            </button>
          );
        })}
      </div>
    );
  }
  if (folded.size > 0) {
    const primary = options.filter((o) => !folded.has(o.value));
    const extra = options.filter((o) => folded.has(o.value));
    const current = extra.find((o) => o.value === value);
    return (
      <div role="radiogroup" aria-label={name} className="inline-flex gap-0.5 rounded-lg bg-control p-[3px]">
        {primary.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={value === o.value}
            onClick={() => onChange(o.value)}
            className={cn(SEGMENT, value === o.value ? SEGMENT_ON : SEGMENT_OFF)}
          >
            <span className="inline-flex items-center gap-1.5">
              {o.label}
              {counts && <CountBadge value={counts[o.value] ?? 0} />}
            </span>
          </button>
        ))}
        {extra.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                role="radio"
                aria-checked={Boolean(current)}
                className={cn(SEGMENT, 'pr-2', current ? SEGMENT_ON : SEGMENT_OFF)}
              >
                <span className="inline-flex items-center gap-1.5">
                  {current ? current.label : moreLabel}
                  {current && counts && <CountBadge value={counts[current.value] ?? 0} />}
                  <ChevronDown className="size-3.5 opacity-60" />
                </span>
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="min-w-40">
              {extra.map((o) => (
                <DropdownMenuItem
                  key={o.value}
                  onClick={() => onChange(o.value)}
                  className={cn('justify-between gap-6', o.value === value && 'font-medium')}
                >
                  {o.label}
                  {counts && <span className="text-xs tabular-nums text-muted-foreground">{counts[o.value] ?? 0}</span>}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    );
  }
  return (
    <Segmented
      label={name}
      value={value}
      onChange={onChange}
      options={options.map((o) => ({
        value: o.value,
        label: (
          <span className="inline-flex items-center gap-1.5">
            {o.label}
            {counts && (
              <span className="min-w-[1.25rem] rounded-[5px] bg-foreground/[0.07] px-1 text-[11px] tabular-nums leading-[18px]">
                {counts[o.value] ?? 0}
              </span>
            )}
          </span>
        )
      }))}
    />
  );
}

StatusSegments.propTypes = {
  value: PropTypes.string.isRequired,
  onChange: PropTypes.func.isRequired,
  options: PropTypes.arrayOf(PropTypes.shape({ value: PropTypes.string.isRequired, label: PropTypes.node.isRequired })).isRequired,
  counts: PropTypes.object,
  label: PropTypes.string,
  chips: PropTypes.bool,
  more: PropTypes.arrayOf(PropTypes.string),
  moreLabel: PropTypes.node
};
