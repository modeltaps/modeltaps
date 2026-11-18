import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Check, CircleCheck, RefreshCw } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Alert as Notice, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusSegments } from '@/components/list-page';
import useRouteTab from 'hooks/useRouteTab';
import { API } from 'utils/api';
import { relativeTime } from 'utils/relativeTime';
import { timestamp2string } from 'utils/common';
import { alertSubject, alertTarget, describeAlertItem } from './alertItems';

// ==============================|| PANEL — ALERTS (admin) ||============================== //
// Conditions that reach users and need an admin: served models the upstream renamed or dropped,
// price changes that lasted, failing credentials, catalog gaps (`/api/alert`). Each is told once
// by a notice and stays here while it lasts; it closes by itself when the next check no longer
// finds it. Closing one by hand keeps it quiet until something new about it turns up.

const TABS = ['open', 'closed'];
const ITEMS_SHOWN = 8;

export default function Alerts() {
  const { t, i18n } = useTranslation();
  const rtf = useMemo(() => new Intl.RelativeTimeFormat((i18n.language || 'en').replace('_', '-'), { numeric: 'auto' }), [i18n.language]);
  const { tab, setTab, redirecting } = useRouteTab({ tabs: TABS });

  const [alerts, setAlerts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [expanded, setExpanded] = useState(() => new Set());

  const load = useCallback(async () => {
    setRefreshing(true);
    try {
      const res = await API.get('/api/alert/', { params: { state: tab } });
      if (res.data.success) {
        setAlerts(res.data.data || []);
        setError('');
      } else setError(res.data.message);
    } catch (err) {
      setError(err?.response?.data?.message || err.message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [tab]);

  useEffect(() => {
    if (!redirecting) load();
  }, [load, redirecting]);

  const dismiss = async (id) => {
    try {
      const res = await API.post(`/api/alert/${id}/dismiss`);
      if (!res.data.success) {
        setError(res.data.message);
        return;
      }
      setAlerts((list) => list.filter((a) => a.id !== id));
    } catch (err) {
      setError(err?.response?.data?.message || err.message);
    }
  };

  if (redirecting) return null;

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-center gap-3 border-b border-border p-4">
        <StatusSegments
          label={t('alerts.stateLabel')}
          value={tab}
          onChange={setTab}
          options={TABS.map((value) => ({ value, label: t(`alerts.state.${value}`) }))}
        />
        <span className="text-xs text-muted-foreground">{t(`alerts.stateHint.${tab}`)}</span>
        <Button
          variant="outline"
          size="sm"
          className="ml-auto w-8 px-0"
          onClick={load}
          disabled={refreshing}
          title={t('alerts.refresh')}
          aria-label={t('alerts.refresh')}
        >
          <RefreshCw className={cn('size-4', refreshing && 'animate-spin')} strokeWidth={1.5} />
        </Button>
      </div>

      {error && (
        <div className="border-b border-border p-4">
          <Notice variant="error">
            <AlertDescription>{error}</AlertDescription>
          </Notice>
        </div>
      )}

      {loading ? (
        <div className="space-y-3 p-4">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      ) : alerts.length === 0 ? (
        <EmptyState
          icon={CircleCheck}
          title={t(`alerts.empty.${tab}`)}
          description={tab === 'open' ? t('alerts.emptyHint') : undefined}
          className="py-20"
        />
      ) : (
        <ul className="divide-y divide-border">
          {alerts.map((a) => {
            const items = (a.items || []).map((item) => describeAlertItem(t, a.kind, item));
            const warn = items.some((x) => x.tone === 'warn');
            const all = expanded.has(a.id);
            const shown = all ? items : items.slice(0, ITEMS_SHOWN);
            const target = alertTarget(a);
            const closedAt = a.resolved_at || a.dismissed_at;
            return (
              <li key={a.id} className="flex items-start gap-3 px-4 py-3">
                <span
                  className={cn(
                    'mt-[7px] size-2 shrink-0 rounded-full',
                    tab === 'closed' ? 'bg-muted-foreground/40' : warn ? 'bg-warning' : 'bg-info'
                  )}
                />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline gap-x-2 text-sm">
                    <span className="font-medium text-foreground">{t(`alerts.kind.${a.kind}`)}</span>
                    {target ? (
                      <Link to={target} className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline">
                        {alertSubject(t, a)}
                      </Link>
                    ) : (
                      <span className="text-muted-foreground">{alertSubject(t, a)}</span>
                    )}
                  </div>
                  {shown.length > 0 && (
                    <ul className="mt-1 space-y-0.5 text-[13px]">
                      {shown.map((x, i) => (
                        <li key={i} className={cn('break-words', x.tone === 'warn' ? 'text-foreground' : 'text-muted-foreground')}>
                          {x.text}
                        </li>
                      ))}
                    </ul>
                  )}
                  {items.length > ITEMS_SHOWN && (
                    <button
                      type="button"
                      onClick={() =>
                        setExpanded((cur) => {
                          const next = new Set(cur);
                          if (all) next.delete(a.id);
                          else next.add(a.id);
                          return next;
                        })
                      }
                      className="mt-1 text-xs text-info underline-offset-2 hover:underline"
                    >
                      {all ? t('alerts.showLess') : t('alerts.showAll', { count: items.length })}
                    </button>
                  )}
                  <p className="mt-1 text-xs text-muted-foreground">
                    {tab === 'open' ? (
                      <span title={timestamp2string(a.opened_at)}>{t('alerts.openedAt', { time: relativeTime(rtf, a.opened_at) })}</span>
                    ) : (
                      <span title={timestamp2string(closedAt)}>
                        {t(a.dismissed_at ? 'alerts.dismissedAt' : 'alerts.resolvedAt', { time: relativeTime(rtf, closedAt) })}
                      </span>
                    )}
                  </p>
                </div>
                {tab === 'open' && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="shrink-0 gap-1.5 text-muted-foreground"
                    onClick={() => dismiss(a.id)}
                    title={t('alerts.dismissHint')}
                  >
                    <Check className="size-4" strokeWidth={1.5} />
                    {t('alerts.dismiss')}
                  </Button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
