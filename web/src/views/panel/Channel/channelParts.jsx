import { useId, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, ExternalLink, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { useProviderLabel } from './ProviderPicker';
import { consoleUrl } from './providerDefaults';

// Small building blocks shared by the provider page, the channel panel and the new channel
// sheet: fields with top-aligned labels, option lists (one bordered list, a row per option),
// segmented controls, chip inputs and provider tiles.

export function useProviderName() {
  const labelOf = useProviderLabel();
  const { t } = useTranslation();
  return (type) => (CHANNEL_OPTIONS[type] ? labelOf(CHANNEL_OPTIONS[type]) : t('channels.unknownProvider', { type }));
}

export function ProviderTile({ type, size = 'sm' }) {
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center border border-border bg-muted',
        size === 'lg' ? 'size-9 rounded-lg' : 'size-[22px] rounded-md'
      )}
    >
      <ProviderIcon type={type} className={size === 'lg' ? 'size-5' : 'size-3.5'} />
    </span>
  );
}

ProviderTile.propTypes = { type: PropTypes.number, size: PropTypes.oneOf(['sm', 'lg']) };

const DOT = { live: 'bg-success', warn: 'bg-warning', off: 'bg-muted-foreground/45', bad: 'bg-destructive' };

export function Dot({ tone = 'live', className, title }) {
  return <span title={title} className={cn('inline-block size-2 shrink-0 rounded-full', DOT[tone], className)} />;
}

Dot.propTypes = { tone: PropTypes.oneOf(['live', 'warn', 'off', 'bad']), className: PropTypes.string, title: PropTypes.string };

// A form field: label on top (with an optional marker and something on the right such as a
// link), the control, and a one-line hint. Consecutive fields are separated by a hairline.
export function Field({ label, htmlFor, optional, extra, hint, error, children, className }) {
  const { t } = useTranslation();
  return (
    <div className={cn('py-3 [&+&]:border-t [&+&]:border-border/60', className)}>
      {(label || extra) && (
        <div className="mb-1.5 flex items-baseline justify-between gap-3 text-sm font-medium">
          {label && (
            <label htmlFor={htmlFor}>
              {label}
              {optional && <span className="ml-1.5 text-xs font-normal text-muted-foreground">{t('channels.optional')}</span>}
            </label>
          )}
          {extra && <span className="text-xs font-normal text-muted-foreground">{extra}</span>}
        </div>
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
  error: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};

// "Get a key" link to the provider's console, when one is known.
export function KeyLink({ type }) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const url = consoleUrl(type);
  if (!url) return null;
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-1 text-xs font-normal text-info underline-offset-2 hover:underline"
    >
      {t('channels.getKey', { provider: providerName(type) })}
      <ExternalLink className="size-3" />
    </a>
  );
}

KeyLink.propTypes = { type: PropTypes.number };

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

// Multi-select list of user groups. At least one group stays selected.
export function GroupOptions({ groups, value, onChange }) {
  const { t } = useTranslation();
  const list = groups.length ? groups : [{ symbol: 'default' }];
  const known = new Set(list.map((g) => g.symbol));
  // Groups the channel carries that no longer exist still show, so they can be removed.
  const rows = [...list, ...value.filter((g) => !known.has(g)).map((symbol) => ({ symbol, missing: true }))];
  const toggle = (g) => {
    if (value.includes(g)) {
      if (value.length > 1) onChange(value.filter((x) => x !== g));
    } else onChange([...value, g]);
  };
  return (
    <OptionList label={t('channels.groups')}>
      {rows.map((g) => (
        <OptionRow
          key={g.symbol}
          mono
          selected={value.includes(g.symbol)}
          title={g.symbol}
          description={g.missing ? t('channels.groupMissing') : g.description || g.name || ''}
          meta={g.ratio != null ? t('channels.groupRatio', { ratio: Number(g.ratio).toFixed(1) }) : ''}
          onClick={() => toggle(g.symbol)}
        />
      ))}
    </OptionList>
  );
}

GroupOptions.propTypes = { groups: PropTypes.array.isRequired, value: PropTypes.array.isRequired, onChange: PropTypes.func.isRequired };

// Radio list of usage options (see channelUsage.js) with the numbers each one saves. `custom`
// shows two number inputs underneath.
export function UsageOptions({ usage, value, onChange, custom, onCustomChange }) {
  const { t } = useTranslation();
  const name = usage.top?.name || '';
  const describe = (o) => {
    switch (o.key) {
      case 'only':
        return t('channels.usage.onlyDesc');
      case 'backup':
        return t('channels.usage.backupDesc', { name });
      case 'share':
        return t('channels.usage.shareDesc', { name, share: o.share });
      case 'first':
        return t('channels.usage.firstDesc', { name });
      default:
        return t('channels.usage.customDesc');
    }
  };
  return (
    <div className="space-y-2">
      <OptionList label={t('channels.usage.label')}>
        {usage.options.map((o) => {
          const selected = value === o.key;
          const nums = o.key === 'custom' ? custom : o;
          return (
            <OptionRow
              key={o.key}
              kind="radio"
              selected={selected}
              title={t(`channels.usage.${o.key}`)}
              description={describe(o)}
              meta={selected && o.key !== 'custom' ? t('channels.usage.numbers', { priority: nums.priority, weight: nums.weight }) : ''}
              onClick={() => onChange(o.key)}
            />
          );
        })}
      </OptionList>
      {value === 'custom' && (
        <div className="grid grid-cols-2 gap-3">
          <label className="space-y-1.5 text-xs text-muted-foreground">
            <span>{t('channels.usage.priority')}</span>
            <Input
              type="number"
              value={custom.priority}
              onChange={(e) => onCustomChange({ ...custom, priority: e.target.value })}
              className="tabular-nums"
            />
          </label>
          <label className="space-y-1.5 text-xs text-muted-foreground">
            <span>{t('channels.usage.weight')}</span>
            <Input
              type="number"
              min="1"
              value={custom.weight}
              onChange={(e) => onCustomChange({ ...custom, weight: e.target.value })}
              className="tabular-nums"
            />
          </label>
          <p className="col-span-2 text-xs text-muted-foreground">{t('channels.usage.customHint')}</p>
        </div>
      )}
    </div>
  );
}

UsageOptions.propTypes = {
  usage: PropTypes.object.isRequired,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  custom: PropTypes.object.isRequired,
  onCustomChange: PropTypes.func.isRequired
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

// Chip input: Enter adds the typed value (through `transform`, which may return '' to refuse
// it), Backspace on an empty input removes the last chip. Suggestions show while focused.
export function ChipInput({ id, values, onChange, placeholder, suggestions = [], transform = (v) => v, chipLabel, onReject }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const [focused, setFocused] = useState(false);
  const autoId = useId();
  const inputId = id || autoId;
  const add = (raw) => {
    const v = transform(String(raw || '').trim());
    if (!v) {
      if (String(raw || '').trim()) onReject?.(raw);
      return;
    }
    if (!values.includes(v)) onChange([...values, v]);
    setQuery('');
  };
  const q = query.trim().toLowerCase();
  const shown = suggestions.filter((s) => !values.includes(s) && (!q || s.toLowerCase().includes(q))).slice(0, 8);
  return (
    <div
      onFocus={() => setFocused(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget)) setFocused(false);
      }}
    >
      <label
        htmlFor={inputId}
        className={cn(
          'flex min-h-9 cursor-text flex-wrap items-center gap-1.5 rounded-lg border border-input bg-background px-2 py-1',
          focused && 'border-foreground/40'
        )}
      >
        {values.map((v) => (
          <span key={v} className="inline-flex h-6 items-center gap-1 rounded-full bg-muted pl-2.5 pr-1 font-mono text-xs">
            {chipLabel ? chipLabel(v) : v}
            <button
              type="button"
              aria-label={t('channels.removeChip', { value: v })}
              onClick={() => onChange(values.filter((x) => x !== v))}
              className="flex size-[18px] items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              <X className="size-3" />
            </button>
          </span>
        ))}
        <input
          id={inputId}
          value={query}
          placeholder={values.length ? '' : placeholder}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              add(query);
            } else if (e.key === 'Backspace' && !query && values.length) {
              onChange(values.slice(0, -1));
            }
          }}
          className="h-[26px] min-w-40 flex-1 bg-transparent text-[13px] outline-none placeholder:text-muted-foreground"
        />
      </label>
      {focused && shown.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {shown.map((s) => (
            <button
              key={s}
              type="button"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => add(s)}
              className="h-[26px] rounded-full border border-dashed border-border px-2.5 font-mono text-xs text-muted-foreground hover:border-solid hover:text-foreground"
            >
              {s}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

ChipInput.propTypes = {
  id: PropTypes.string,
  values: PropTypes.array.isRequired,
  onChange: PropTypes.func.isRequired,
  placeholder: PropTypes.string,
  suggestions: PropTypes.array,
  transform: PropTypes.func,
  chipLabel: PropTypes.func,
  onReject: PropTypes.func
};
