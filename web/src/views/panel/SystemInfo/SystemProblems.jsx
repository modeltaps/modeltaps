import { Fragment, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Check, ChevronRight, CircleCheck, RefreshCw } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusSegments } from '@/components/list-page';
import { API } from 'utils/api';
import { relativeTime } from 'utils/relativeTime';
import LogContextDialog from './LogContextDialog';
import { LogRow } from './LogRows';
import { formatFullTimestamp, processLogEntry } from './logHelpers';

// ==============================|| SYSTEM LOGS — PROBLEMS ||============================== //
// Warnings and errors since the last start, folded by message template
// (`/api/system_info/problems`): one row per problem with its count and when it last happened.
// A row opens its latest occurrences; "Mark as handled" drops it until it happens again.

const REFRESH_MS = 30000;
const LEVEL_DOT = { error: 'bg-destructive', warn: 'bg-warning' };

export default function SystemProblems() {
  const { t, i18n } = useTranslation();
  const rtf = useMemo(() => new Intl.RelativeTimeFormat((i18n.language || 'en').replace('_', '-'), { numeric: 'auto' }), [i18n.language]);

  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [level, setLevel] = useState('all');
  const [open, setOpen] = useState(() => new Set());
  const [contextLog, setContextLog] = useState(null);

  const load = useCallback(async ({ silent = false } = {}) => {
    if (!silent) setRefreshing(true);
    try {
      const res = await API.get('/api/system_info/problems');
      if (res.data.success) {
        setGroups(res.data.data || []);
        setError('');
      } else {
        setError(res.data.message);
      }
    } catch (err) {
      setError(err?.response?.data?.message || err.message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(() => load({ silent: true }), REFRESH_MS);
    return () => clearInterval(id);
  }, [load]);

  const resolve = async (keys) => {
    try {
      const res = await API.post('/api/system_info/problems/resolve', { keys });
      if (!res.data.success) {
        setError(res.data.message);
        return;
      }
      setGroups((list) => (keys.length ? list.filter((g) => !keys.includes(g.key)) : []));
    } catch (err) {
      setError(err?.response?.data?.message || err.message);
    }
  };

  const toggle = (key) =>
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const counts = { all: groups.length, error: 0, warn: 0 };
  groups.forEach((g) => (counts[g.level] = (counts[g.level] || 0) + 1));
  const shown = level === 'all' ? groups : groups.filter((g) => g.level === level);
  const occurrences = shown.reduce((sum, g) => sum + g.count, 0);

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-center gap-3 border-b border-border p-4">
        <StatusSegments
          label={t('systemLogs.levelLabel')}
          value={level}
          onChange={setLevel}
          options={['all', 'error', 'warn'].map((value) => ({ value, label: t(`systemLogs.${value}`) }))}
          counts={counts}
        />
        <span className="text-xs text-muted-foreground">{t('systemLogs.sinceStart')}</span>
        <div className="ml-auto flex items-center gap-2">
          {groups.length > 0 && (
            <Button variant="outline" size="sm" onClick={() => resolve([])}>
              {t('systemLogs.resolveAll')}
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            className="w-8 px-0"
            onClick={() => load()}
            disabled={refreshing}
            title={t('systemLogs.refresh')}
            aria-label={t('systemLogs.refresh')}
          >
            <RefreshCw className={cn('size-4', refreshing && 'animate-spin')} strokeWidth={1.5} />
          </Button>
        </div>
      </div>

      {error && (
        <div className="border-b border-border p-4">
          <Alert variant="error">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </div>
      )}

      {loading ? (
        <div className="space-y-3 p-4">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      ) : shown.length === 0 ? (
        <EmptyState icon={CircleCheck} title={t('systemLogs.noProblems')} description={t('systemLogs.noProblemsHint')} className="py-20" />
      ) : (
        <div className="divide-y divide-border">
          {shown.map((g) => {
            const expanded = open.has(g.key);
            const last = Math.floor(new Date(g.last_seen).getTime() / 1000);
            return (
              <Fragment key={g.key}>
                <div className="group flex items-start gap-3 px-4 py-3 transition-colors hover:bg-muted/40">
                  <button
                    type="button"
                    onClick={() => toggle(g.key)}
                    aria-expanded={expanded}
                    className="flex min-w-0 flex-1 items-start gap-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <ChevronRight
                      className={cn('mt-0.5 size-4 shrink-0 text-muted-foreground transition-transform', expanded && 'rotate-90')}
                      strokeWidth={1.5}
                    />
                    <span className={cn('mt-[7px] size-2 shrink-0 rounded-full', LEVEL_DOT[g.level])} title={t(`systemLogs.${g.level}`)} />
                    <span className="min-w-0 flex-1">
                      <span className={cn('block font-mono text-xs leading-5 text-foreground', !expanded && 'line-clamp-2')}>
                        {g.template}
                      </span>
                      <span className="mt-1 flex flex-wrap items-center gap-x-3 text-xs text-muted-foreground">
                        {g.source && <span className="font-mono">{g.source === 'request' ? t('systemLogs.requests') : g.source}</span>}
                        <span title={formatFullTimestamp(g.first_seen)}>
                          {t('systemLogs.firstSeen', { time: formatFullTimestamp(g.first_seen) })}
                        </span>
                      </span>
                    </span>
                  </button>
                  <div className="flex shrink-0 items-center gap-4 pt-0.5">
                    <span className="w-16 text-right text-sm tabular-nums text-foreground">
                      {t('systemLogs.times', { count: g.count })}
                    </span>
                    <span className="w-24 text-right text-xs text-muted-foreground" title={formatFullTimestamp(g.last_seen)}>
                      {relativeTime(rtf, last)}
                    </span>
                    <button
                      type="button"
                      onClick={() => resolve([g.key])}
                      title={t('systemLogs.resolve')}
                      aria-label={t('systemLogs.resolve')}
                      className="flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <Check className="size-4" strokeWidth={1.5} />
                    </button>
                  </div>
                </div>
                {expanded && (
                  <div className="border-t border-border/60 bg-muted/30 py-1 pl-8">
                    <p className="px-3 py-1.5 text-xs text-muted-foreground">
                      {t('systemLogs.latestSamples', { count: g.samples.length })}
                    </p>
                    <div className="divide-y divide-border/60">
                      {[...g.samples].reverse().map((s, i) => (
                        <LogRow key={`${s.Timestamp}-${i}`} log={processLogEntry(s)} onContext={setContextLog} />
                      ))}
                    </div>
                  </div>
                )}
              </Fragment>
            );
          })}
        </div>
      )}

      {!loading && shown.length > 0 && (
        <div className="flex h-11 items-center border-t border-border px-4 text-xs tabular-nums text-muted-foreground">
          {t('systemLogs.problemSummary', { groups: shown.length, count: occurrences })}
        </div>
      )}

      {contextLog && <LogContextDialog log={contextLog} onClose={() => setContextLog(null)} />}
    </Card>
  );
}
