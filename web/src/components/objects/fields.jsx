import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Switch } from '@/components/ui/switch';

// Form and display rows shared by object panels and new-object sheets: fields with the label on
// top, switch rows, option lists (one bordered list, a row per option), segmented controls,
// status dots, section headings and read-only label/value rows. Rows are separated by a
// hairline; groups of rows by a section heading.

const DOT = { live: 'bg-success', warn: 'bg-warning', off: 'bg-muted-foreground/45', bad: 'bg-destructive' };

export function Dot({ tone = 'live', className, title }) {
  return <span title={title} className={cn('inline-block size-2 shrink-0 rounded-full', DOT[tone], className)} />;
}

Dot.propTypes = { tone: PropTypes.oneOf(['live', 'warn', 'off', 'bad']), className: PropTypes.string, title: PropTypes.string };

// A form field: label on top (with an optional marker, a help toggle and something on the right
// such as a link), the control, and a one-line hint. Consecutive fields are separated by a
// hairline. `help` is a longer rule shown only when its toggle is opened.
export function Field({ label, htmlFor, optional, extra, hint, help, error, children, className }) {
  const { t } = useTranslation();
  const [helpOpen, setHelpOpen] = useState(false);
  return (
    <div className={cn('py-3 [&+&]:border-t [&+&]:border-border/60', className)}>
      {(label || extra) && (
        <div className="mb-1.5 flex items-baseline justify-between gap-3 text-sm font-medium">
          {label && (
            <span className="inline-flex items-baseline gap-1.5">
              <label htmlFor={htmlFor}>
                {label}
                {optional && <span className="ml-1.5 text-xs font-normal text-muted-foreground">{t('objects.optional')}</span>}
              </label>
              {help && (
                <button
                  type="button"
                  aria-expanded={helpOpen}
                  aria-label={t('objects.help')}
                  onClick={() => setHelpOpen((o) => !o)}
                  className="flex size-4 shrink-0 items-center justify-center self-center rounded-full border border-border text-[10px] font-medium leading-none text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
                >
                  ?
                </button>
              )}
            </span>
          )}
          {extra && <span className="text-xs font-normal text-muted-foreground">{extra}</span>}
        </div>
      )}
      {help && helpOpen && (
        <div className="mb-2 rounded-lg bg-muted/60 px-3 py-2 text-xs leading-relaxed text-muted-foreground">{help}</div>
      )}
      {children}
      {hint && !error && <p className="mt-1.5 text-xs text-muted-foreground">{hint}</p>}
      {error && <p className="mt-1.5 text-xs text-destructive">{error}</p>}
    </div>
  );
}

Field.propTypes = {
  label: PropTypes.node,
  htmlFor: PropTypes.string,
  optional: PropTypes.bool,
  extra: PropTypes.node,
  hint: PropTypes.node,
  help: PropTypes.node,
  error: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};

// A switch row: label and hint on the left, the switch on the right.
export function ToggleField({ id, label, hint, checked, onChange, disabled }) {
  return (
    <div className="flex items-start justify-between gap-4 py-3 [&+&]:border-t [&+&]:border-border/60">
      <div className="min-w-0">
        <p className="text-sm font-medium">{id ? <label htmlFor={id}>{label}</label> : label}</p>
        {hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}
      </div>
      <Switch id={id} checked={!!checked} disabled={disabled} onCheckedChange={onChange} className="mt-0.5 shrink-0" />
    </div>
  );
}

ToggleField.propTypes = {
  id: PropTypes.string,
  label: PropTypes.node,
  hint: PropTypes.node,
  checked: PropTypes.bool,
  onChange: PropTypes.func,
  disabled: PropTypes.bool
};

export function OptionList({ children, label }) {
  return (
    <div role="group" aria-label={label} className="overflow-hidden rounded-lg border border-border">
      {children}
    </div>
  );
}

OptionList.propTypes = { children: PropTypes.node, label: PropTypes.string };

// One option of an option list: a checkbox or radio mark, a short title, a description and an
// optional figure on the right.
export function OptionRow({ kind = 'check', selected, title, description, meta, onClick, mono }) {
  return (
    <button
      type="button"
      role={kind === 'radio' ? 'radio' : 'checkbox'}
      aria-checked={selected}
      onClick={onClick}
      className={cn(
        'grid min-h-12 w-full grid-cols-[16px_minmax(5rem,auto)_minmax(0,1fr)_auto] items-center gap-3 px-3 py-2.5 text-left transition-colors',
        '[&+&]:border-t [&+&]:border-border hover:bg-foreground/[0.04] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring',
        selected && 'bg-accent/55'
      )}
    >
      {kind === 'radio' ? (
        <span
          className={cn(
            'size-4 rounded-full',
            selected
              ? 'shadow-[inset_0_0_0_5px_var(--foreground)]'
              : 'shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--foreground)_35%,transparent)]'
          )}
        />
      ) : (
        <span
          className={cn(
            'flex size-4 items-center justify-center rounded-[5px] border',
            selected ? 'border-primary bg-primary text-primary-foreground' : 'border-foreground/30'
          )}
        >
          {selected && <Check className="size-3" strokeWidth={2.5} />}
        </span>
      )}
      <span className={cn('truncate text-sm font-medium', mono && 'font-mono')}>{title}</span>
      <span className="min-w-0 text-xs text-muted-foreground">{description}</span>
      <span className={cn('whitespace-nowrap text-xs tabular-nums', selected ? 'text-foreground' : 'text-muted-foreground')}>{meta}</span>
    </button>
  );
}

OptionRow.propTypes = {
  kind: PropTypes.oneOf(['check', 'radio']),
  selected: PropTypes.bool,
  title: PropTypes.node,
  description: PropTypes.node,
  meta: PropTypes.node,
  onClick: PropTypes.func,
  mono: PropTypes.bool
};

// Segmented control. `size` mini is for a switch inside a label row; wide stretches the
// segments over the full width.
export function Segmented({ options, value, onChange, label, size = 'md', role = 'radiogroup' }) {
  return (
    <div
      role={role}
      aria-label={label}
      className={cn('inline-flex gap-0.5 rounded-lg bg-muted p-[3px]', size === 'mini' && 'p-0.5', size === 'wide' && 'flex w-full')}
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            'rounded-[7px] px-3 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            size === 'mini' ? 'h-6 px-2.5 text-xs' : size === 'wide' ? 'h-8 flex-1' : 'h-7',
            value === o.value ? 'bg-card text-foreground shadow-[0_0_0_1px_var(--border)]' : 'text-muted-foreground hover:text-foreground'
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

Segmented.propTypes = {
  options: PropTypes.array.isRequired,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  label: PropTypes.string,
  size: PropTypes.oneOf(['mini', 'md', 'wide']),
  role: PropTypes.string
};

// A titled group inside a tab: a plain heading (sentence case, weight 500) and its rows. The
// first section of a tab sits flush with the tab bar.
export function Section({ title, extra, children, className }) {
  return (
    <section className={cn('pt-4 first:pt-3', className)}>
      {(title || extra) && (
        <div className="mb-1 flex items-baseline justify-between gap-3">
          {title && <h3 className="text-sm font-medium">{title}</h3>}
          {extra && <span className="text-xs text-muted-foreground">{extra}</span>}
        </div>
      )}
      {children}
    </section>
  );
}

Section.propTypes = { title: PropTypes.node, extra: PropTypes.node, children: PropTypes.node, className: PropTypes.string };

// Read-only label/value rows (an overview tab, an order's details).
export function InfoRows({ children, className }) {
  return <dl className={cn('text-sm', className)}>{children}</dl>;
}

InfoRows.propTypes = { children: PropTypes.node, className: PropTypes.string };

export function InfoRow({ label, children, mono }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2 [&+&]:border-t [&+&]:border-border/60">
      <dt className="shrink-0 text-muted-foreground">{label}</dt>
      <dd className={cn('min-w-0 break-words text-right', mono && 'font-mono text-[13px]')}>{children}</dd>
    </div>
  );
}

InfoRow.propTypes = { label: PropTypes.node, children: PropTypes.node, mono: PropTypes.bool };

// Figures across the top of an overview tab: label above, value below, no boxes.
export function StatRow({ items }) {
  return (
    <div className="grid grid-cols-2 gap-x-6 gap-y-3 py-3 sm:grid-cols-3">
      {items.map((s) => (
        <div key={s.label} className="min-w-0">
          <div className="text-xs text-muted-foreground">{s.label}</div>
          <div className="mt-0.5 truncate text-lg font-medium tabular-nums">{s.value}</div>
          {s.hint && <div className="truncate text-xs text-muted-foreground">{s.hint}</div>}
        </div>
      ))}
    </div>
  );
}

StatRow.propTypes = { items: PropTypes.array.isRequired };
