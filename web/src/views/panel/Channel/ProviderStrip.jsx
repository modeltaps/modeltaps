import { useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Dot } from '@/components/objects';
import { openIssues } from './channelGroups';
import { ProviderTile, useProviderName } from './channelParts';

// Search and provider filters above the channel list: one chip per provider with its channel
// count (and a dot when one of its channels needs attention); a chip filters the list to that
// provider, "All" clears it. The search covers every channel; `/` focuses it. Traffic is shown per
// channel in the list itself.
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

  const chip = (key, active, onClick, label, children) => (
    <button
      key={key}
      type="button"
      onClick={onClick}
      aria-pressed={active}
      aria-label={label}
      className={cn(
        'inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-3 text-[13px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        active ? 'border-foreground bg-foreground text-background' : 'border-border bg-field hover:border-foreground/30'
      )}
    >
      {children}
    </button>
  );
  const count = (n, active) => <span className={cn('tabular-nums', active ? 'opacity-70' : 'text-muted-foreground')}>{n}</span>;

  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="relative w-64 shrink-0">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          ref={inputRef}
          type="search"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t('channels.search.placeholder')}
          aria-label={t('channels.search.placeholder')}
          className="h-8 w-full rounded-lg border border-input bg-field pl-9 pr-3 text-[13px] outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
        />
      </div>
      <div className="flex min-w-0 flex-wrap gap-1.5" role="group" aria-label={t('channels.lane.label')}>
        {chip(
          'all',
          !selected,
          () => onSelect(0),
          t('channels.strip.all'),
          <>
            {t('channels.strip.all')}
            {count(total, !selected)}
          </>
        )}
        {chips.map((c) => {
          const active = selected === c.type;
          const name = providerName(c.type);
          return chip(
            c.type,
            active,
            () => onSelect(c.type),
            name,
            <>
              <ProviderTile type={c.type} />
              {name}
              {count(c.enabled === c.channels.length ? c.channels.length : `${c.enabled}/${c.channels.length}`, active)}
              {c.attention && <Dot tone="warn" />}
            </>
          );
        })}
      </div>
    </div>
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
