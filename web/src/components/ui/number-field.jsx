import * as React from 'react';
import PropTypes from 'prop-types';
import { Minus, Plus } from 'lucide-react';

import { cn } from '@/lib/utils';
import { NUMBER_INPUT } from './input';

// ==============================|| NUMBER FIELD — A BOUNDED NUMBER WITH STEP BUTTONS ||============================== //
// For whole numbers with a range, such as caps, percentages and durations: the value sits in the
// middle with a minus and a plus button on either side and the unit after it, all in one field
// box. Typing works as in any input; the buttons step by `step` and stop at `min` / `max`.
// `value` is the text of the field (a string, like the option values it edits) and `onChange`
// receives text. Sizes: 'sm' (h-8, for rows and tables) and 'md' (h-9, for forms).

const clamp = (n, min, max) => Math.min(max ?? Infinity, Math.max(min ?? -Infinity, n));

export const NumberField = React.forwardRef(
  (
    {
      id,
      value,
      onChange,
      min,
      max,
      step = 1,
      unit,
      size = 'sm',
      placeholder,
      disabled = false,
      className,
      'aria-label': ariaLabel,
      onBlur,
      onKeyDown
    },
    ref
  ) => {
    const text = value === undefined || value === null ? '' : String(value);
    const number = text.trim() === '' ? null : Number(text);
    const base = Number.isFinite(number) ? number : Number(placeholder) || min || 0;
    const stepTo = (delta) => {
      const next = clamp(Math.round((base + delta) / step) * step, min, max);
      onChange(String(next));
    };
    const atMin = min !== undefined && Number.isFinite(number) && number <= min;
    const atMax = max !== undefined && Number.isFinite(number) && number >= max;
    const button =
      'flex h-full w-7 shrink-0 items-center justify-center text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-40';
    return (
      <div
        className={cn(
          'inline-flex items-stretch overflow-hidden rounded-lg border border-input bg-field text-sm transition-colors focus-within:border-primary',
          size === 'md' ? 'h-9' : 'h-8',
          disabled && 'cursor-not-allowed opacity-50',
          className ?? 'w-28'
        )}
      >
        <button type="button" tabIndex={-1} aria-hidden className={button} disabled={disabled || atMin} onClick={() => stepTo(-step)}>
          <Minus className="size-3.5" />
        </button>
        <span className="flex min-w-0 flex-1 items-center justify-center gap-1">
          <input
            ref={ref}
            id={id}
            type="number"
            inputMode="numeric"
            min={min}
            max={max}
            step={step}
            value={text}
            placeholder={placeholder}
            disabled={disabled}
            aria-label={ariaLabel}
            onChange={(e) => onChange(e.target.value)}
            onBlur={onBlur}
            onKeyDown={(e) => {
              if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
                e.preventDefault();
                stepTo(e.key === 'ArrowUp' ? step : -step);
              }
              onKeyDown?.(e);
            }}
            // As wide as the figure it holds, so a short value and its unit stay centered together.
            style={{ width: `${Math.max(2, (text || placeholder || '').length + 0.5)}ch` }}
            className={cn('min-w-0 bg-transparent text-center outline-none placeholder:text-muted-foreground', NUMBER_INPUT)}
          />
          {unit && <span className="shrink-0 text-xs text-muted-foreground">{unit}</span>}
        </span>
        <button type="button" tabIndex={-1} aria-hidden className={button} disabled={disabled || atMax} onClick={() => stepTo(step)}>
          <Plus className="size-3.5" />
        </button>
      </div>
    );
  }
);
NumberField.displayName = 'NumberField';

NumberField.propTypes = {
  id: PropTypes.string,
  value: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  onChange: PropTypes.func.isRequired,
  min: PropTypes.number,
  max: PropTypes.number,
  step: PropTypes.number,
  unit: PropTypes.node,
  size: PropTypes.oneOf(['sm', 'md']),
  placeholder: PropTypes.string,
  disabled: PropTypes.bool,
  className: PropTypes.string,
  'aria-label': PropTypes.string,
  onBlur: PropTypes.func,
  onKeyDown: PropTypes.func
};
