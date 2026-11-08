import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { Loader2, RefreshCw } from 'lucide-react';

import { renderSpend, timestamp2string } from 'utils/common';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { cacheSummary, formatBalance, metricsByModel, metricsSeries, summarizeMetrics } from './channelMetrics';
import { fetchChannelCacheStat, fetchChannelMetrics } from './channelApi';
import { ResponseTime, countModels } from './channelColumns';

const RANGES = [
  { key: '24h', seconds: 24 * 3600, bucket: 3600, i18n: 'channelDetail.range24h' },
  { key: '7d', seconds: 7 * 24 * 3600, bucket: 6 * 3600, i18n: 'channelDetail.range7d' }
];

const pct = (v) => (v === null || v === undefined ? '-' : `${(v * 100).toFixed(v >= 0.999 || v === 0 ? 0 : 1)}%`);
const ms = (v) => (v === null || v === undefined ? '-' : v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${Math.round(v)}ms`);
const tps = (v) => (v === null || v === undefined ? '-' : `${v.toFixed(1)} tok/s`);
const num = (v) => (v || 0).toLocaleString();

function Panel({ title, action, className, children }) {
  return (
    <section className={cn('rounded-xl border border-border bg-card', className)}>
      {(title || action) && (
        <div className="flex min-h-12 items-center justify-between gap-2 px-5 pt-4">
          <h3 className="text-sm font-medium">{title}</h3>
          {action}
        </div>
      )}
      <div className="px-5 pb-5 pt-3">{children}</div>
    </section>
  );
}

function Stat({ label, value, caption, tone }) {
  return (
    <div className="rounded-xl border border-border bg-card px-5 py-4">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className={cn('mt-2 text-2xl font-medium tabular-nums tracking-tight', tone)}>{value}</div>
      <div className="mt-1 min-h-5 text-xs text-muted-foreground">{caption}</div>
    </div>
  );
}

function Row({ label, children }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-1.5 text-sm">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right tabular-nums">{children}</span>
    </div>
  );
}

function TrafficTooltip({ active, payload, label, t, bucket }) {
  if (!active || !payload?.length) return null;
  const point = payload[0].payload;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-md">
      <p className="mb-1 text-muted-foreground">{bucketLabel(label, bucket, true)}</p>
      <p className="tabular-nums">
        {t('channelDetail.requests')} {num(point.requests)}
      </p>
      {point.failures > 0 && (
        <p className="tabular-nums text-destructive">
          {t('channelDetail.failures')} {num(point.failures)}
        </p>
      )}
    </div>
  );
}

function bucketLabel(ts, bucket, long = false) {
  const d = new Date(ts * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  if (!long && bucket <= 3600) return `${hh}:00`;
  return `${d.getMonth() + 1}/${d.getDate()} ${hh}:00`;
}

// Read-only overview of one channel: spend, upstream balance, traffic and cache over the
// chosen range, per-model health, quota, drift and routing. Every action calls back into the
// detail page, which owns the shared channel actions.
export default function ChannelOverview({ channel, actions, busy, onEditSection }) {
  const { t } = useTranslation();
  const [rangeKey, setRangeKey] = useState('24h');
  const range = RANGES.find((r) => r.key === rangeKey) || RANGES[0];
  const [state, setState] = useState({ loading: true, rows: [], cache: null, start: 0, end: 0 });

  useEffect(() => {
    let alive = true;
    const end = Math.floor(Date.now() / 1000);
    const start = end - range.seconds;
    setState((s) => ({ ...s, loading: true }));
    Promise.all([
      fetchChannelMetrics(channel.id, start, end).catch(() => []),
      fetchChannelCacheStat(channel.id, start, end).catch(() => null)
    ]).then(([rows, cache]) => alive && setState({ loading: false, rows, cache, start, end }));
    return () => {
      alive = false;
    };
  }, [channel.id, range.seconds]);

  const summary = useMemo(() => summarizeMetrics(state.rows), [state.rows]);
  const models = useMemo(() => metricsByModel(state.rows), [state.rows]);
  const series = useMemo(
    () => (state.end ? metricsSeries(state.rows, state.start, state.end, range.bucket) : []),
    [state.rows, state.start, state.end, range.bucket]
  );
  const cache = useMemo(() => cacheSummary(state.cache), [state.cache]);

  const windows = channel.usage_windows?.windows || [];
  const snapshot = channel.quota_snapshot;
  const drift = channel.model_drift;
  const groups = (channel.group || '').split(',').filter(Boolean);
  const rangeLabel = t(range.i18n);

  let balanceValue;
  let balanceCaption;
  if (windows.length) {
    const top = windows.reduce((a, b) => (b.used_percent > a.used_percent ? b : a), windows[0]);
    balanceValue = `${Math.round(top.used_percent)}%`;
    balanceCaption = t('channelDetail.windowUsed', { name: top.name });
  } else if (channel.balance_updated_time) {
    balanceValue = formatBalance(channel.type, channel.balance);
    balanceCaption = t('channelDetail.balanceUpdated', { time: timestamp2string(channel.balance_updated_time) });
  } else {
    balanceValue = <span className="text-muted-foreground">-</span>;
    balanceCaption = t('channelDetail.balanceNever');
  }
  const lowBalance =
    !windows.length && channel.balance_threshold > 0 && channel.balance_updated_time && channel.balance < channel.balance_threshold;

  const rangeSwitch = (
    <div className="flex gap-1" role="group" aria-label={t('channelDetail.range')}>
      {RANGES.map((r) => (
        <Button
          key={r.key}
          size="sm"
          variant={r.key === rangeKey ? 'secondary' : 'ghost'}
          className={cn('h-7 px-2.5 text-xs', r.key === rangeKey && 'text-foreground')}
          onClick={() => setRangeKey(r.key)}
        >
          {t(r.i18n)}
        </Button>
      ))}
    </div>
  );

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat label={t('channelDetail.used')} value={renderSpend(channel.used_quota)} caption={t('channelDetail.usedCaption')} />
        <Stat
          label={windows.length ? t('channelDetail.subscription') : t('channelDetail.balance')}
          value={balanceValue}
          tone={lowBalance ? 'text-destructive' : undefined}
          caption={balanceCaption}
        />
        <Stat
          label={t('channelDetail.requestsIn', { range: rangeLabel })}
          value={state.loading ? '-' : num(summary.requests)}
          caption={summary.requests ? t('channelDetail.successRate', { rate: pct(summary.successRate) }) : t('channelDetail.noTraffic')}
          tone={summary.successRate !== null && summary.successRate < 0.9 ? 'text-destructive' : undefined}
        />
        <Stat
          label={t('channelDetail.cacheHit')}
          value={state.loading ? '-' : pct(cache.hitRate)}
          caption={
            cache.affinityRate !== null ? t('channelDetail.affinity', { rate: pct(cache.affinityRate) }) : t('channelDetail.noTraffic')
          }
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Panel title={t('channelDetail.trafficTitle')} action={rangeSwitch} className="lg:col-span-2">
          {state.loading ? (
            <div className="flex h-64 items-center justify-center text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
            </div>
          ) : summary.requests === 0 ? (
            <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">{t('channelDetail.metricsEmpty')}</div>
          ) : (
            <ResponsiveContainer width="100%" height={256}>
              <BarChart data={series} margin={{ top: 4, right: 4, left: 0, bottom: 0 }} barCategoryGap={2}>
                <CartesianGrid stroke="var(--border)" vertical={false} />
                <XAxis
                  dataKey="ts"
                  tickFormatter={(v) => bucketLabel(v, range.bucket)}
                  tick={{ fill: 'var(--muted-foreground)', fontSize: 11, fontFamily: 'var(--font-mono, monospace)' }}
                  axisLine={false}
                  tickLine={false}
                  minTickGap={24}
                />
                <YAxis
                  width={40}
                  allowDecimals={false}
                  tick={{ fill: 'var(--muted-foreground)', fontSize: 11, fontFamily: 'var(--font-mono, monospace)' }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip content={<TrafficTooltip t={t} bucket={range.bucket} />} cursor={{ fill: 'var(--muted)' }} />
                <Bar
                  dataKey={(d) => d.requests - d.failures}
                  stackId="r"
                  fill="var(--foreground)"
                  fillOpacity={0.7}
                  isAnimationActive={false}
                />
                <Bar dataKey="failures" stackId="r" fill="var(--destructive)" radius={[2, 2, 0, 0]} isAnimationActive={false} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </Panel>

        <Panel title={t('channelDetail.health')}>
          <Row label={t('channelDetail.lastTest')}>
            {channel.test_time ? (
              <span className="inline-flex items-center gap-2">
                <ResponseTime ms={channel.response_time} />
                <span className="text-xs text-muted-foreground">{timestamp2string(channel.test_time)}</span>
              </span>
            ) : (
              <span className="text-muted-foreground">{t('channelDetail.neverTested')}</span>
            )}
          </Row>
          <Row label={t('channelDetail.avgLatency')}>{ms(summary.avgLatencyMs)}</Row>
          <Row label={t('channelDetail.ttft')}>{ms(summary.avgTtftMs)}</Row>
          <Row label={t('channelDetail.speed')}>{tps(summary.tokensPerSec)}</Row>
          <Row label={t('channelDetail.failures')}>
            <span className={summary.failures ? 'text-destructive' : undefined}>{num(summary.failures)}</span>
          </Row>
          {cache.mismatches > 0 && (
            <Row label={t('channelDetail.mismatches')}>
              <span className="text-amber-600 dark:text-amber-400">{num(cache.mismatches)}</span>
            </Row>
          )}
          <Button variant="outline" size="sm" className="mt-3 w-full" disabled={busy.testing} onClick={() => actions.onTest(channel)}>
            {busy.testing && <Loader2 className="size-4 animate-spin" />}
            {t('channelDetail.runTest')}
          </Button>
        </Panel>
      </div>

      <div className="grid items-start gap-4 lg:grid-cols-3">
        <Panel title={t('channelDetail.byModel')} className="lg:col-span-2">
          {models.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">{t('channelDetail.metricsEmpty')}</p>
          ) : (
            <div className="-mx-5 overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs text-muted-foreground">
                    <th className="px-5 pb-2 font-normal">{t('channelDetail.colModel')}</th>
                    <th className="px-3 pb-2 text-right font-normal">{t('channelDetail.requests')}</th>
                    <th className="px-3 pb-2 text-right font-normal">{t('channelDetail.colSuccess')}</th>
                    <th className="px-3 pb-2 text-right font-normal">{t('channelDetail.avgLatency')}</th>
                    <th className="px-3 pb-2 text-right font-normal">{t('channelDetail.ttft')}</th>
                    <th className="px-5 pb-2 text-right font-normal">{t('channelDetail.speed')}</th>
                  </tr>
                </thead>
                <tbody>
                  {models.slice(0, 12).map((m) => (
                    <tr key={m.model} className="border-b border-border last:border-0">
                      <td className="max-w-[16rem] truncate px-5 py-2 font-mono text-xs" title={m.model}>
                        {m.model || '-'}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">{num(m.requests)}</td>
                      <td
                        className={cn(
                          'px-3 py-2 text-right tabular-nums',
                          m.successRate !== null && m.successRate < 0.9 && 'text-destructive'
                        )}
                      >
                        {pct(m.successRate)}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">{ms(m.avgLatencyMs)}</td>
                      <td className="px-3 py-2 text-right tabular-nums">{ms(m.avgTtftMs)}</td>
                      <td className="px-5 py-2 text-right tabular-nums">{tps(m.tokensPerSec)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        <div className="space-y-4">
          <Panel
            title={windows.length ? t('channelDetail.subscription') : t('channelDetail.balance')}
            action={
              <Button
                variant="ghost"
                size="sm"
                className="h-7 px-2 text-xs"
                disabled={busy.refreshing}
                onClick={() => actions.onRefreshBalance(channel)}
              >
                {busy.refreshing ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
                {t('channelDetail.refresh')}
              </Button>
            }
          >
            {windows.length > 0 ? (
              <div className="space-y-3">
                {windows.map((w, i) => (
                  <div key={w.name || i}>
                    <div className="flex items-baseline justify-between text-sm">
                      <span>{w.name}</span>
                      <span className="tabular-nums text-muted-foreground">{Math.round(w.used_percent)}%</span>
                    </div>
                    <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
                      <div
                        className={cn('h-full rounded-full', w.used_percent >= 90 ? 'bg-destructive' : 'bg-foreground/70')}
                        style={{ width: `${Math.min(100, Math.max(0, w.used_percent))}%` }}
                      />
                    </div>
                    {w.reset_at ? (
                      <p className="mt-1 text-xs text-muted-foreground">
                        {t('channelDetail.resetsAt', { time: timestamp2string(w.reset_at) })}
                      </p>
                    ) : null}
                  </div>
                ))}
              </div>
            ) : snapshot ? (
              <QuotaSnapshot
                t={t}
                snapshot={snapshot}
                threshold={channel.balance_threshold > 0 ? formatBalance(channel.type, channel.balance_threshold) : null}
              />
            ) : (
              <>
                <Row label={t('channelDetail.balance')}>
                  {channel.balance_updated_time ? formatBalance(channel.type, channel.balance) : t('channelDetail.balanceNever')}
                </Row>
                <p className="mt-1 text-xs text-muted-foreground">{t('channelDetail.balanceHint')}</p>
              </>
            )}
            {!windows.length && !snapshot && channel.balance_threshold > 0 && (
              <Row label={t('channelDetail.threshold')}>{formatBalance(channel.type, channel.balance_threshold)}</Row>
            )}
          </Panel>

          <Panel
            title={t('channelDetail.driftTitle')}
            action={
              <Button
                variant="ghost"
                size="sm"
                className="h-7 px-2 text-xs"
                disabled={busy.drift}
                onClick={() => actions.onCheckDrift(channel)}
              >
                {busy.drift ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
                {t('channelDetail.driftCheck')}
              </Button>
            }
          >
            <DriftSummary t={t} drift={drift} busy={busy.dismissing} onDismiss={() => actions.onDismissNewModels(channel)} />
          </Panel>

          <Panel
            title={t('channelDetail.routingTitle')}
            action={
              <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => onEditSection('advanced')}>
                {t('common.edit')}
              </Button>
            }
          >
            <Row label={t('channel_index.group')}>
              <span className="inline-flex flex-wrap justify-end gap-1">
                {groups.map((g) => (
                  <Badge key={g} variant="secondary">
                    {g}
                  </Badge>
                ))}
              </span>
            </Row>
            <Row label={t('channel_index.priority')}>{channel.priority ?? 0}</Row>
            <Row label={t('channel_index.weight')}>{channel.weight ?? 1}</Row>
            {channel.tag && <Row label={t('channelDetail.tag')}>{channel.tag}</Row>}
            <Row label={t('channelDetail.models')}>
              <button
                type="button"
                className="underline-offset-4 hover:underline focus-visible:underline focus-visible:outline-none"
                onClick={() => onEditSection('model')}
              >
                {t('channelDetail.modelCount', { count: countModels(channel.models) })}
              </button>
            </Row>
            <Row label={t('channel_index.testModel')}>
              <span className="font-mono text-xs">{channel.test_model || '-'}</span>
            </Row>
          </Panel>
        </div>
      </div>
    </div>
  );
}

ChannelOverview.propTypes = {
  channel: PropTypes.object.isRequired,
  actions: PropTypes.object.isRequired,
  busy: PropTypes.object.isRequired,
  onEditSection: PropTypes.func.isRequired
};

function QuotaSnapshot({ t, snapshot, threshold }) {
  const money = (v) => (v === null || v === undefined ? '-' : `$${Number(v).toFixed(2)}`);
  return (
    <>
      <Row label={t('channelDetail.quotaRemaining')}>
        {snapshot.limit === null || snapshot.limit === undefined ? t('channelDetail.unlimited') : money(snapshot.remaining)}
      </Row>
      {snapshot.limit !== null && snapshot.limit !== undefined && <Row label={t('channelDetail.quotaLimit')}>{money(snapshot.limit)}</Row>}
      <Row label={t('channelDetail.quotaReset')}>{t(`channelDetail.reset.${snapshot.reset_interval || 'none'}`)}</Row>
      <Row label={t('channelDetail.usageDaily')}>{money(snapshot.usage_daily)}</Row>
      <Row label={t('channelDetail.usageWeekly')}>{money(snapshot.usage_weekly)}</Row>
      <Row label={t('channelDetail.usageMonthly')}>{money(snapshot.usage_monthly)}</Row>
      {threshold && <Row label={t('channelDetail.threshold')}>{threshold}</Row>}
      {snapshot.is_free_tier && <p className="mt-1 text-xs text-muted-foreground">{t('channelDetail.freeTier')}</p>}
      {snapshot.fetched_at ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {t('channelDetail.balanceUpdated', { time: timestamp2string(snapshot.fetched_at) })}
        </p>
      ) : null}
    </>
  );
}

QuotaSnapshot.propTypes = { t: PropTypes.func, snapshot: PropTypes.object, threshold: PropTypes.string };

function ModelList({ models }) {
  return <p className="mt-1 max-h-24 overflow-y-auto break-words font-mono text-xs leading-5 text-muted-foreground">{models.join(', ')}</p>;
}

ModelList.propTypes = { models: PropTypes.array };

function DriftSummary({ t, drift, busy, onDismiss }) {
  if (!drift?.checked_at) return <p className="text-sm text-muted-foreground">{t('channelDetail.driftNever')}</p>;
  const missing = drift.missing_models || [];
  const added = drift.new_models || [];
  return (
    <div className="space-y-3 text-sm">
      {missing.length === 0 && added.length === 0 && <p>{t('channelDetail.driftOk')}</p>}
      {missing.length > 0 && (
        <div>
          <p className="text-amber-600 dark:text-amber-400">{t('channelDetail.driftMissing', { count: missing.length })}</p>
          <ModelList models={missing} />
        </div>
      )}
      {added.length > 0 && (
        <div>
          <div className="flex items-center justify-between gap-2">
            <p>{t('channelDetail.driftNew', { count: added.length })}</p>
            <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" disabled={busy} onClick={onDismiss}>
              {t('channelDetail.driftDismiss')}
            </Button>
          </div>
          <ModelList models={added} />
        </div>
      )}
      <p className="text-xs text-muted-foreground">{t('channelDetail.driftCheckedAt', { time: timestamp2string(drift.checked_at) })}</p>
    </div>
  );
}

DriftSummary.propTypes = { t: PropTypes.func, drift: PropTypes.object, busy: PropTypes.bool, onDismiss: PropTypes.func };
