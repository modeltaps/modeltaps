import { useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';

import { Dot } from '@/components/objects';
import { openIssues } from './channelGroups';
import { useProviderName } from './channelParts';
import { Combobox } from '@/components/ui/combobox';
import ProviderIcon from '@/components/brand/ProviderIcon';

// Search and provider filter, the first items of the channel list's toolbar. The provider
// dropdown lists each provider with its channel count (and a dot when one of its channels needs
// attention); picking one filters the list. The search covers every channel; `/` focuses it.
export default function ProviderStrip({ groups, selected, onSelect, dismissed, query, onQueryChange }) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const inputRef = useRef(null);
  // The input keeps its own text and writes it to the URL shortly after typing stops.
  const [text, setText] = useState(query);
  const [synced, setSynced] = useState(query);
  if (query !== synced) {
    setSynced(query);
    setText(query);
  }
  useEffect(() => {
    if (text === query) return undefined;
    const timer = setTimeout(() => onQueryChange(text), 200);
    return () => clearTimeout(timer);
  }, [text, query, onQueryChange]);

  useEffect(() => {
    const onKey = (e) => {
      if (e.key !== '/' || e.metaKey || e.ctrlKey || e.altKey) return;
      const el = document.activeElement;
      if (el && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName))) return;
      if (document.querySelector('[role="dialog"]')) return;
      e.preventDefault();
      inputRef.current?.focus();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  const chips = useMemo(
    () =>
      groups.map((g) => ({
        ...g,
        enabled: g.channels.filter((c) => c.status === 1).length,
        attention: openIssues(g.channels, dismissed).length > 0
      })),
    [groups, dismissed]
  );
  const total = chips.reduce((sum, c) => sum + c.channels.length, 0);

  const options = useMemo(
    () => [
      { value: '0', label: t('channels.strip.allProviders'), hint: String(total) },
      ...chips.map((c) => ({
        value: String(c.type),
        label: providerName(c.type),
        icon: <ProviderIcon type={c.type} className="size-4" />,
        hint: (
          <span className="inline-flex items-center gap-1.5">
            {c.attention && <Dot tone="warn" />}
            {c.enabled === c.channels.length ? c.channels.length : `${c.enabled}/${c.channels.length}`}
          </span>
        )
      }))
    ],
    [chips, providerName, t, total]
  );

  return (
    <>
      <div className="relative w-36 shrink-0">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          ref={inputRef}
          type="search"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t('channels.search.short')}
          aria-label={t('channels.search.placeholder')}
          title={t('channels.search.placeholder')}
          className="h-8 w-full rounded-lg border border-input bg-field pl-8 pr-3 text-[13px] outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
        />
      </div>
      <div className="w-36 shrink-0">
        <Combobox
          value={String(selected || 0)}
          onValueChange={(v) => onSelect(Number(v))}
          options={options}
          className="h-8 text-[13px]"
          contentClassName="min-w-[14rem]"
          searchPlaceholder={t('providerPicker.search')}
          emptyText={t('providerPicker.empty')}
        />
      </div>
    </>
  );
}

ProviderStrip.propTypes = {
  groups: PropTypes.array.isRequired,
  selected: PropTypes.number,
  onSelect: PropTypes.func.isRequired,
  dismissed: PropTypes.instanceOf(Set).isRequired,
  query: PropTypes.string,
  onQueryChange: PropTypes.func.isRequired
};
