import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Plus, Search, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { openIssues } from './channelGroups';
import { Dot } from '@/components/objects';
import { ProviderTile, useProviderName } from './channelParts';

// Left lane: a search over every provider's channels, then one row per provider type that has
// channels, with a warning dot when one of its channels needs attention (or a faded dot when all
// are disabled) and the channel count. While searching, only providers with matches are listed
// and the count is the number of matching channels. `/` focuses the search.
export default function ProviderLane({ groups, selected, onSelect, dismissed, onNewOther, query, onQueryChange }) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const inputRef = useRef(null);
  // The input keeps its own text and writes it to the URL shortly after typing stops, so fast
  // typing never races the URL update; an outside change (clear, back) flows back in.
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
  const setNow = (value) => {
    setText(value);
    onQueryChange(value);
  };

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

  return (
    <aside
      aria-label={t('channels.lane.label')}
      className="flex flex-col rounded-2xl bg-[color-mix(in_srgb,var(--muted)_70%,var(--background))] p-3 sticky top-4 max-h-[calc(100vh-140px)]"
    >
      <div className="relative mb-3">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          ref={inputRef}
          type="search"
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape' && text) {
              e.preventDefault();
              setNow('');
            }
          }}
          placeholder={t('channels.search.placeholder')}
          aria-label={t('channels.search.label')}
          className="h-9 w-full rounded-lg border border-input bg-card pl-9 pr-9 text-[13px] outline-none placeholder:text-muted-foreground focus-visible:border-foreground/40 [&::-webkit-search-cancel-button]:hidden"
        />
        {text ? (
          <button
            type="button"
            aria-label={t('channels.search.clear')}
            onClick={() => {
              setNow('');
              inputRef.current?.focus();
            }}
            className="absolute right-2 top-1/2 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <X className="size-3.5" />
          </button>
        ) : (
          <kbd className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 rounded-[5px] border border-border bg-muted px-1.5 font-mono text-[11px] text-muted-foreground">
            /
          </kbd>
        )}
      </div>
      <div className="px-1.5 pb-2 pt-0.5 text-xs font-medium text-muted-foreground">
        {query ? t('channels.search.results') : t('channels.lane.label')}
      </div>
      {query && groups.length === 0 && <p className="px-1.5 py-3 text-[13px] text-muted-foreground">{t('channels.search.none')}</p>}
      <div className="flex min-h-0 flex-col gap-1 overflow-y-auto">
        {groups.map((g) => {
          const on = g.type === selected;
          const warn = openIssues(g.channels, dismissed).length > 0;
          const allOff = g.channels.every((c) => c.status !== 1);
          return (
            <button
              key={g.type}
              type="button"
              aria-current={on ? 'true' : undefined}
              onClick={() => onSelect(g.type)}
              className={cn(
                'flex h-10 w-full shrink-0 items-center gap-2.5 rounded-lg border border-transparent px-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                on ? 'border-border bg-card' : 'hover:bg-foreground/[0.05]'
              )}
            >
              <ProviderTile type={g.type} />
              <span className={cn('min-w-0 flex-1 truncate text-[13px]', on && 'font-medium')}>{providerName(g.type)}</span>
              {warn ? (
                <Dot tone="warn" title={t('channels.lane.attention')} />
              ) : allOff ? (
                <Dot tone="off" title={t('channels.lane.allOff')} />
              ) : null}
              <span className="font-mono text-xs text-muted-foreground">{g.channels.length}</span>
            </button>
          );
        })}
      </div>
      <div className="mt-2 border-t border-border pt-2">
        <Button variant="ghost" size="sm" className="w-full justify-start rounded-lg" onClick={onNewOther}>
          <Plus />
          {t('channels.lane.newOther')}
        </Button>
      </div>
    </aside>
  );
}

ProviderLane.propTypes = {
  groups: PropTypes.array.isRequired,
  selected: PropTypes.number,
  onSelect: PropTypes.func.isRequired,
  dismissed: PropTypes.instanceOf(Set).isRequired,
  onNewOther: PropTypes.func.isRequired,
  query: PropTypes.string,
  onQueryChange: PropTypes.func.isRequired
};
