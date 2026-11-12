import { useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Dot } from '@/components/objects';
import { channelSeries, metricsByChannel } from './channelMetrics';
import { openIssues } from './channelGroups';
import { ProviderTile, useProviderName } from './channelParts';

// Requests over the range as a small line, one point per bucket.
function Sparkline({ values }) {
  const w = 160;
  const h = 26;
  const max = Math.max(1, ...values);
  const d = values
    .map((v, i) => `${i ? 'L' : 'M'}${((i / Math.max(1, values.length - 1)) * w).toFixed(1)},${(h - 2 - (v / max) * (h - 4)).toFixed(1)}`)
    .join('');
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      className="mt-2 block h-[26px] w-full text-muted-foreground"
      aria-hidden="true"
    >
      {values.length > 1 && <path d={d} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />}
    </svg>
  );
}

Sparkline.propTypes = { values: PropTypes.arrayOf(PropTypes.number).isRequired };

// Row of provider cards above the channel list: every provider keeps its own traffic in view
// (channels, requests in the range and a small line), and a card filters the list to that
// provider. "All" clears it. The search covers every channel; `/` focuses it.
export default function ProviderStrip({ groups, selected, onSelect, dismissed, metrics, bucket, query, onQueryChange, rangeLabel }) {
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

  const perChannel = useMemo(() => metricsByChannel(metrics.rows), [metrics.rows]);
  const cards = useMemo(
    () =>
      groups.map((g) => {
        const ids = g.channels.map((c) => c.id);
        const points = metrics.end ? channelSeries(metrics.rows, ids, metrics.start, metrics.end, bucket) : [];
        return {
          ...g,
          requests: ids.reduce((sum, id) => sum + (perChannel.get(id)?.requests || 0), 0),
          values: points.map((p) => Object.values(p.values).reduce((a, b) => a + b, 0)),
          enabled: g.channels.filter((c) => c.status === 1).length,
          attention: openIssues(g.channels, dismissed).length > 0
        };
      }),
    [groups, metrics, bucket, perChannel, dismissed]
  );
  const total = cards.reduce((sum, c) => sum + c.channels.length, 0);

  const card = (key, active, onClick, children) => (
    <button
      key={key}
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        'w-52 shrink-0 rounded-xl border bg-card px-3 pb-2 pt-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        active ? 'border-foreground/60 ring-1 ring-foreground/60' : 'border-border hover:border-foreground/25'
      )}
    >
      {children}
    </button>
  );

  return (
    <div className="space-y-3">
      <div className="relative max-w-sm">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          ref={inputRef}
          type="search"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t('channels.search.placeholder')}
          aria-label={t('channels.search.placeholder')}
          className="h-9 w-full rounded-lg border border-input bg-field pl-9 pr-3 text-sm outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
        />
      </div>
      <div className="-mx-1 flex gap-2.5 overflow-x-auto px-1 pb-1" role="group" aria-label={t('channels.lane.label')}>
        {card(
          'all',
          !selected,
          () => onSelect(0),
          <>
            <span className="text-sm font-medium">{t('channels.strip.all')}</span>
            <span className="block text-xs text-muted-foreground">{t('channels.strip.allMeta', { count: total })}</span>
          </>
        )}
        {cards.map((c) =>
          card(
            c.type,
            selected === c.type,
            () => onSelect(c.type),
            <>
              <span className="flex min-w-0 items-center gap-2">
                <ProviderTile type={c.type} />
                <span className="truncate text-sm font-medium">{providerName(c.type)}</span>
                {c.attention && <Dot tone="warn" />}
              </span>
              <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                {t(c.enabled === c.channels.length ? 'channels.strip.meta' : 'channels.strip.metaOff', {
                  count: c.channels.length,
                  enabled: c.enabled,
                  requests: c.requests.toLocaleString(),
                  range: rangeLabel
                })}
              </span>
              <Sparkline values={c.values} />
            </>
          )
        )}
      </div>
    </div>
  );
}

ProviderStrip.propTypes = {
  groups: PropTypes.array.isRequired,
  selected: PropTypes.number,
  onSelect: PropTypes.func.isRequired,
  dismissed: PropTypes.instanceOf(Set).isRequired,
  metrics: PropTypes.object.isRequired,
  bucket: PropTypes.number.isRequired,
  query: PropTypes.string,
  onQueryChange: PropTypes.func.isRequired,
  rangeLabel: PropTypes.string
};
