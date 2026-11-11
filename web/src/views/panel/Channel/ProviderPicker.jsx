import { useCallback, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Combobox } from '@/components/ui/combobox';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { CHANNEL_OPTIONS, channelTypeSections } from 'constants/ChannelConstants';

// ==============================|| CHANNEL — PROVIDER PICKER ||============================== //
// Two forms of the same choice. ProviderGrid is the first step of a new channel: a searchable
// card grid in categories (common, vendors, cloud, gateways, subscription accounts, ...), each
// card showing the logo, the name and how it connects. ProviderSelect is the compact dropdown
// with the same logos and categories, for editing a channel.

// Display name of a channel type: the catch-all custom channel is translated, brands are not.
export function useProviderLabel() {
  const { t } = useTranslation();
  return useCallback((o) => (o.value === 8 ? t('providerPicker.customName') : o.text), [t]);
}

export function ProviderGrid({ value, onPick }) {
  const { t } = useTranslation();
  const labelOf = useProviderLabel();
  const [query, setQuery] = useState('');
  const [current, setCurrent] = useState('common');
  const scrollRef = useRef(null);
  const sections = useMemo(() => channelTypeSections(labelOf), [labelOf]);

  const q = query.trim().toLowerCase();
  // While searching the common section is hidden: its types also sit in their own category.
  const shown = useMemo(() => {
    if (!q) return sections;
    return sections
      .filter((s) => s.key !== 'common')
      .map((s) => ({
        ...s,
        options: s.options.filter((o) => o.label.toLowerCase().includes(q) || CHANNEL_OPTIONS[o.type].text.toLowerCase().includes(q))
      }))
      .filter((s) => s.options.length > 0);
  }, [sections, q]);

  const jump = (key) => {
    setCurrent(key);
    scrollRef.current?.querySelector(`[data-section="${key}"]`)?.scrollIntoView({ block: 'start', behavior: 'smooth' });
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('providerPicker.search')}
          className="h-9 w-full rounded-lg border border-input bg-field pl-9 pr-3 text-sm outline-none transition-colors placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
        />
      </div>

      <div className="flex min-h-0 flex-1 gap-6">
        {!q && (
          <nav className="hidden w-36 shrink-0 flex-col gap-0.5 sm:flex" aria-label={t('providerPicker.title')}>
            {sections.map((s) => (
              <button
                key={s.key}
                type="button"
                onClick={() => jump(s.key)}
                className={cn(
                  'flex h-8 items-center justify-between rounded-md px-2 text-left text-sm transition-colors',
                  current === s.key ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'
                )}
              >
                <span className="truncate">{t(`providerPicker.category.${s.key}`)}</span>
                <span className="font-mono text-xs tabular-nums text-muted-foreground">{s.options.length}</span>
              </button>
            ))}
          </nav>
        )}

        <div ref={scrollRef} className="min-h-0 flex-1 space-y-6 overflow-y-auto pb-2">
          {shown.length === 0 && <p className="py-10 text-center text-sm text-muted-foreground">{t('providerPicker.empty')}</p>}
          {shown.map((s) => (
            <section key={s.key} data-section={s.key} className="space-y-2">
              <h3 className="text-xs text-muted-foreground">{t(`providerPicker.category.${s.key}`)}</h3>
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {s.options.map((o) => {
                  const isOn = String(value) === o.value;
                  return (
                    <button
                      key={o.value}
                      type="button"
                      onClick={() => onPick(o.type)}
                      className={cn(
                        'flex h-14 items-center gap-3 rounded-lg border px-3 text-left transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                        isOn ? 'border-foreground/30 bg-muted' : 'border-border hover:bg-muted/50'
                      )}
                    >
                      <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-background">
                        <ProviderIcon type={o.type} name={o.label} className="size-5" />
                      </span>
                      <span className="min-w-0">
                        <span className="block truncate text-sm">{o.label}</span>
                        <span className="block truncate text-xs text-muted-foreground">{t(`providerPicker.auth.${o.auth}`)}</span>
                      </span>
                    </button>
                  );
                })}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}

ProviderGrid.propTypes = {
  value: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  onPick: PropTypes.func.isRequired
};

export function ProviderSelect({ id, value, onChange, disabled }) {
  const { t } = useTranslation();
  const labelOf = useProviderLabel();
  // The dropdown skips the common section: the same types follow in their categories, and
  // duplicate rows would show up twice in search results.
  const groups = useMemo(
    () =>
      channelTypeSections(labelOf)
        .filter((s) => s.key !== 'common')
        .map((s) => ({
          key: s.key,
          heading: t(`providerPicker.category.${s.key}`),
          options: s.options.map((o) => ({
            ...o,
            icon: <ProviderIcon type={o.type} name={o.label} className="size-4" />,
            hint: t(`providerPicker.auth.${o.auth}`)
          }))
        })),
    [labelOf, t]
  );
  return (
    <Combobox
      id={id}
      value={String(value)}
      onValueChange={(v) => onChange(Number(v))}
      groups={groups}
      disabled={disabled}
      searchPlaceholder={t('providerPicker.search')}
      emptyText={t('providerPicker.empty')}
    />
  );
}

ProviderSelect.propTypes = {
  id: PropTypes.string,
  value: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  onChange: PropTypes.func.isRequired,
  disabled: PropTypes.bool
};
