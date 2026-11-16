import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Regex, RefreshCw, ScrollText, Search, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusSegments } from '@/components/list-page';
import { API } from 'utils/api';
import LogContextDialog from './LogContextDialog';
import { LogHeader, LogRow } from './LogRows';
import { LEVELS, processLogEntry } from './logHelpers';

// ==============================|| PANEL — SYSTEM LOGS ||============================== //
// The raw in-memory history over `/api/system_info/log/query` (RootAuth): the last 1,000 lines
// plus older warnings and errors, newest first, filtered by level and a search on the server.
// Clicking a source narrows the list to that request or job; a row opens the lines around it.

const COUNTS = [100, 200, 500, 1000];
const LIVE_MS = 5000;
const EMPTY = { logs: [], total: 0, hasMore: false, levelCounts: null };

function useDebounced(value, ms) {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return debounced;
}

export default function SystemLogs() {
  const { t } = useTranslation();

  const [level, setLevel] = useState('all');
  const [search, setSearch] = useState('');
  const [useRegex, setUseRegex] = useState(false);
  const [count, setCount] = useState(200);
  const [live, setLive] = useState(false);
  const [data, setData] = useState(EMPTY);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [contextLog, setContextLog] = useState(null);

  const term = useDebounced(search.trim(), 300);
  const reqRef = useRef(0);

  const fetchLogs = useCallback(
    async ({ silent = false } = {}) => {
      const id = ++reqRef.current;
      if (useRegex && term) {
        try {
          new RegExp(term);
        } catch {
          setError(t('systemLogs.invalidRegex'));
          setLoading(false);
          return;
        }
      }
      if (!silent) setRefreshing(true);
      try {
        const res = await API.post('/api/system_info/log/query', {
          count,
          search_term: term,
          use_regex: useRegex,
          level: level === 'all' ? '' : level
        });
        if (id !== reqRef.current) return;
        if (res.data.success) {
          const result = res.data.data || {};
          setData({
            logs: (result.logs || []).map(processLogEntry),
            total: result.total_count || 0,
            hasMore: Boolean(result.has_more),
            levelCounts: result.level_counts || null
          });
          setError('');
        } else {
          setError(res.data.message);
        }
      } catch (err) {
        if (id === reqRef.current) setError(err?.response?.data?.message || err.message);
      } finally {
        if (id === reqRef.current) {
          setLoading(false);
          setRefreshing(false);
        }
      }
    },
    [count, term, useRegex, level, t]
  );

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  const liveOn = live;
  useEffect(() => {
    if (!liveOn) return undefined;
    const id = setInterval(() => fetchLogs({ silent: true }), LIVE_MS);
    return () => clearInterval(id);
  }, [liveOn, fetchLogs]);

  const filtered = Boolean(term) || level !== 'all';
  const clearFilters = () => {
    setSearch('');
    setLevel('all');
  };

  const counts = data.levelCounts ? { all: LEVELS.reduce((sum, l) => sum + (data.levelCounts[l] || 0), 0), ...data.levelCounts } : null;
  const levelOptions = ['all', ...LEVELS].map((value) => ({ value, label: t(`systemLogs.${value}`) }));

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-col gap-3 border-b border-border p-4">
        <div className="flex flex-wrap items-center gap-3">
          <StatusSegments label={t('systemLogs.levelLabel')} value={level} onChange={setLevel} options={levelOptions} counts={counts} />
          <div className="ml-auto flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              aria-pressed={liveOn}
              onClick={() => setLive((v) => !v)}
              title={t('systemLogs.liveHint')}
              className={cn('gap-2', liveOn && 'border-foreground/20 bg-muted')}
            >
              <span className={cn('size-2 rounded-full', liveOn ? 'bg-success' : 'bg-muted-foreground/45')} />
              {t('systemLogs.live')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="w-8 px-0"
              onClick={() => fetchLogs()}
              disabled={refreshing}
              title={t('systemLogs.refresh')}
              aria-label={t('systemLogs.refresh')}
            >
              <RefreshCw className={cn('size-4', refreshing && 'animate-spin')} strokeWidth={1.5} />
            </Button>
          </div>
        </div>

        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" strokeWidth={1.5} />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('systemLogs.search')}
            aria-label={t('systemLogs.search')}
            className={cn('h-9 pl-9 pr-16', useRegex && 'font-mono')}
          />
          <div className="absolute right-1.5 top-1/2 flex -translate-y-1/2 items-center gap-0.5">
            {search && (
              <button
                type="button"
                onClick={() => setSearch('')}
                aria-label={t('systemLogs.clearFilters')}
                className="flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground"
              >
                <X className="size-4" strokeWidth={1.5} />
              </button>
            )}
            <button
              type="button"
              aria-pressed={useRegex}
              onClick={() => setUseRegex((v) => !v)}
              title={t('systemLogs.regex')}
              aria-label={t('systemLogs.regex')}
              className={cn(
                'flex size-6 items-center justify-center rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                useRegex ? 'bg-foreground text-background' : 'text-muted-foreground hover:bg-muted hover:text-foreground'
              )}
            >
              <Regex className="size-4" strokeWidth={1.5} />
            </button>
          </div>
        </div>
      </div>

      {error && (
        <div className="border-b border-border p-4">
          <Alert variant="error">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </div>
      )}

      <div className="max-h-[calc(100vh-300px)] min-h-[320px] overflow-y-auto">
        {loading ? (
          <div className="space-y-2 p-3">
            {Array.from({ length: 10 }, (_, i) => (
              <Skeleton key={i} className="h-5 w-full" />
            ))}
          </div>
        ) : data.logs.length === 0 ? (
          <EmptyState
            icon={ScrollText}
            title={filtered ? t('systemLogs.noMatch') : t('systemLogs.empty')}
            description={filtered ? null : t('systemLogs.emptyHint')}
            action={
              filtered && (
                <Button variant="outline" size="sm" onClick={clearFilters}>
                  {t('systemLogs.clearFilters')}
                </Button>
              )
            }
            className="py-20"
          />
        ) : (
          <>
            <LogHeader />
            <div className="divide-y divide-border/60">
              {data.logs.map((log, i) => (
                <LogRow
                  key={`${log.timestamp}-${i}`}
                  log={log}
                  onSource={(s) => setSearch(s === 'SYS' ? '[SYS]' : s)}
                  onContext={setContextLog}
                />
              ))}
            </div>
          </>
        )}
      </div>

      <div className="flex h-11 items-center justify-between gap-3 border-t border-border px-4 text-xs text-muted-foreground">
        <span className="tabular-nums">
          {data.hasMore
            ? t('systemLogs.shown', { shown: data.logs.length, total: data.total })
            : t('systemLogs.matched', { count: data.total })}
        </span>
        <div className="flex items-center gap-2">
          <span>{t('systemLogs.showLatest')}</span>
          <Select value={String(count)} onValueChange={(v) => setCount(Number(v))}>
            <SelectTrigger className="h-7 w-20 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {COUNTS.map((n) => (
                <SelectItem key={n} value={String(n)}>
                  {n}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span>{t('systemLogs.entries')}</span>
        </div>
      </div>

      {contextLog && <LogContextDialog log={contextLog} onClose={() => setContextLog(null)} />}
    </Card>
  );
}
