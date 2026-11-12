import { useId, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ExternalLink, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import { OptionList, OptionRow } from '@/components/objects';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { useProviderLabel } from './ProviderPicker';
import { consoleUrl } from './providerDefaults';
import { ALL_GROUPS, isAllGroups } from './channelGroups';

// Channel-specific building blocks shared by the provider page, the channel panel and the new
// channel sheet: provider names and tiles, the key link, group and usage option lists and chip
// inputs. Generic fields, option lists and segmented controls are in components/objects.

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

// Multi-select list of user groups, led by "All groups" (stored as "*", which also covers groups
// created later). While all groups is on the single groups are locked; turning it off selects
// every current group so nothing narrows until one is unticked. At least one row stays selected.
export function GroupOptions({ groups, value, onChange }) {
  const { t } = useTranslation();
  const list = groups.length ? groups : [{ symbol: 'default' }];
  const all = isAllGroups(value);
  const known = new Set(list.map((g) => g.symbol));
  // Groups the channel carries that no longer exist still show, so they can be removed.
  const rows = [...list, ...value.filter((g) => g !== ALL_GROUPS && !known.has(g)).map((symbol) => ({ symbol, missing: true }))];
  const toggle = (g) => {
    if (value.includes(g)) {
      if (value.length > 1) onChange(value.filter((x) => x !== g));
    } else onChange([...value, g]);
  };
  const toggleAll = () => onChange(all ? list.map((g) => g.symbol) : [ALL_GROUPS]);
  return (
    <OptionList label={t('channels.groups')}>
      <OptionRow selected={all} title={t('channels.allGroups')} description={t('channels.allGroupsDesc')} onClick={toggleAll} />
      {rows.map((g) => (
        <OptionRow
          key={g.symbol}
          mono
          disabled={all}
          selected={!all && value.includes(g.symbol)}
          title={g.symbol}
          description={g.missing ? t('channels.groupMissing') : g.description || g.name || ''}
          meta={g.ratio != null ? t('channels.groupRatio', { ratio: Number(g.ratio).toFixed(1) }) : ''}
          onClick={() => toggle(g.symbol)}
        />
      ))}
    </OptionList>
  );
}

// A channel's groups for display: "All groups" for "*", the symbols otherwise.
export function useGroupLabel() {
  const { t } = useTranslation();
  return (groups, sep = ', ') => (isAllGroups(groups) ? t('channels.allGroups') : groups.join(sep));
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
          'flex min-h-9 cursor-text flex-wrap items-center gap-1.5 rounded-lg border border-input bg-field px-2 py-1',
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
