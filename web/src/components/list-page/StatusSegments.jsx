import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { Segmented } from '@/components/objects';

// ==============================|| LIST PAGE — STATUS SEGMENTS ||============================== //
// The list's top-level split (All / Enabled / Disabled …) with a count per segment. In the table
// layout it is a segmented control; in the phone list (`chips`) a row of pill chips that scrolls
// sideways instead of wrapping. `counts` is { [value]: number } or null while unknown.

export default function StatusSegments({ value, onChange, options, counts = null, label, chips = false }) {
  const { t } = useTranslation();
  const name = label ?? t('listPage.segments');
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
  chips: PropTypes.bool
};
