import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { LongListMore, LongListSearch, useLongList } from '@/components/objects';
import { fetchChannelCacheStat, fetchChannelMetrics } from './channelApi';
import { cacheSummary, metricsByModel, metricsSeries, summarizeMetrics } from './channelMetrics';
import { RANGES } from './ProviderPane';

// Read-only Performance tab of the channel panel: how the channel served requests in a range it
// picks itself (the list's range to begin with). The Models tab only maintains the list and
// flags unused or failing models; the numbers live here.

const pct = (v) => (v === null || v === undefined ? '—' : `${(v * 100).toFixed(1)}%`);
const sec = (v) => (v === null || v === undefined ? '—' : `${(v / 1000).toFixed(2)} s`);
const speed = (v) => (v === null || v === undefined ? '—' : `${v.toFixed(1)} tok/s`);
const shortNumber = (v) => {
  const n = Number(v) || 0;
  if (n >= 1e6) return `${Number((n / 1e6).toFixed(1))}M`;
  if (n >= 1e3) return `${Number((n / 1e3).toFixed(1))}K`;
  return `${n}`;
};

function Stat({ label, value }) {
  return (
    <div className="min-w-0 rounded-lg border border-border px-3 py-2.5">
      <div className="truncate text-xs text-muted-foreground">{label}</div>
      <div className="mt-0.5 truncate text-[15px] font-medium tabular-nums">{value}</div>
    </div>
  );
}

Stat.propTypes = { label: PropTypes.node.isRequired, value: PropTypes.node.isRequired };

function ChartTooltip({ active, payload, label, t }) {
  if (!active || !payload?.length) return null;
  const p = payload[0].payload;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-md">
      <p className="mb-1 font-medium text-muted-foreground">{label}</p>
      <p className="tabular-nums">{t('channels.performance.chartRequests', { count: p.requests })}</p>
      {p.failures > 0 && <p className="tabular-nums text-destructive">{t('channels.performance.chartFailures', { count: p.failures })}</p>}
    </div>
  );
}

ChartTooltip.propTypes = { active: PropTypes.bool, payload: PropTypes.array, label: PropTypes.string, t: PropTypes.func.isRequired };

const bucketLabel = (ts, bucket) => {
  const d = new Date(ts * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  const day = `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
  return bucket >= 24 * 3600 ? day : `${day} ${pad(d.getHours())}:00`;
};

const modelText = (row) => row.model;

function PerformanceBody({ object: channel, ctx }) {
  const { t } = useTranslation();
  const [rangeKey, setRangeKey] = useState(ctx.rangeKey || RANGES[0].key);
  const range = RANGES.find((r) => r.key === rangeKey) || RANGES[0];
  // Loaded figures and the range they are for; a range switch shows the skeleton until the new
  // figures arrive.
  const [loaded, setLoaded] = useState(null);
  const data = loaded?.key === range.key ? loaded : null;

  useEffect(() => {
    let alive = true;
    const end = Math.floor(Date.now() / 1000);
    const start = end - range.seconds;
    Promise.all([
      fetchChannelMetrics(channel.id, start, end).catch(() => []),
      fetchChannelCacheStat(channel.id, start, end).catch(() => null)
    ]).then(([rows, cache]) => alive && setLoaded({ key: range.key, rows, cache, start, end }));
    return () => {
      alive = false;
    };
  }, [channel.id, range.key, range.seconds]);

  const summary = useMemo(() => summarizeMetrics(data?.rows), [data]);
  const cache = useMemo(() => cacheSummary(data?.cache), [data]);
  const models = useMemo(() => metricsByModel(data?.rows).filter((m) => m.requests > 0), [data]);
  const series = useMemo(
    () =>
      data
        ? metricsSeries(data.rows, data.start, data.end, range.bucket).map((b) => ({
            ...b,
            label: bucketLabel(b.ts, range.bucket),
            ok: b.requests - b.failures
          }))
        : [],
    [data, range.bucket]
  );
  const list = useLongList(models, { text: modelText });

  return (
    <div className="space-y-4 py-3">
      <div className="flex items-center gap-2">
        <span className="text-[13px] text-muted-foreground">{t('channels.performance.intro')}</span>
        <span className="flex-1" />
        <Select value={range.key} onValueChange={setRangeKey}>
          <SelectTrigger className="h-8 w-[92px] rounded-lg text-[13px]" aria-label={t('channels.range.label')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {RANGES.map((r) => (
              <SelectItem key={r.key} value={r.key}>
                {t(`channels.range.${r.key}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {data === null ? (
        <div className="space-y-3" aria-busy="true">
          <div className="grid grid-cols-3 gap-2">
            {[0, 1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className="h-[58px] rounded-lg" />
            ))}
          </div>
          <Skeleton className="h-[150px] rounded-lg" />
        </div>
      ) : summary.requests === 0 ? (
        <p className="rounded-lg border border-dashed border-border py-10 text-center text-sm text-muted-foreground">
          {t('channels.performance.none', { range: t(`channels.range.short.${range.key}`) })}
        </p>
      ) : (
        <>
          <div className="grid grid-cols-3 gap-2">
            <Stat label={t('channels.performance.requests')} value={summary.requests.toLocaleString()} />
            <Stat label={t('channels.table.successRate')} value={pct(summary.successRate)} />
            <Stat label={t('channels.table.avgLatency')} value={sec(summary.avgLatencyMs)} />
            <Stat label={t('channels.models.ttft')} value={sec(summary.avgTtftMs)} />
            <Stat label={t('channels.performance.speed')} value={speed(summary.tokensPerSec)} />
            <Stat label={t('channels.performance.cacheHit')} value={pct(cache.hitRate)} />
          </div>

          <div className="rounded-lg border border-border px-2 pb-1 pt-3">
            <div className="h-[150px]">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={series} margin={{ top: 4, right: 8, bottom: 0, left: -12 }}>
                  <CartesianGrid vertical={false} stroke="var(--border)" />
                  <XAxis
                    dataKey="label"
                    tickLine={false}
                    axisLine={false}
                    minTickGap={24}
                    tick={{ fontSize: 11, fill: 'var(--muted-foreground)' }}
                  />
                  <YAxis
                    allowDecimals={false}
                    tickLine={false}
                    axisLine={false}
                    width={44}
                    tickFormatter={shortNumber}
                    tick={{ fontSize: 11, fill: 'var(--muted-foreground)' }}
                  />
                  <Tooltip cursor={{ fill: 'var(--muted)' }} content={<ChartTooltip t={t} />} />
                  <Bar dataKey="ok" stackId="r" fill="var(--chart-1)" isAnimationActive={false} />
                  <Bar dataKey="failures" stackId="r" fill="var(--destructive)" isAnimationActive={false} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>

          <div>
            <LongListSearch list={list} className="pb-2" />
            <div className="overflow-hidden rounded-lg border border-border">
              <table className="w-full table-fixed border-collapse text-[13px]">
                <colgroup>
                  <col />
                  <col className="w-[4.5rem]" />
                  <col className="w-[5.5rem]" />
                  <col className="w-[5rem]" />
                  <col className="w-[4.5rem]" />
                </colgroup>
                <thead>
                  <tr className="text-left text-xs text-muted-foreground [&>th]:h-9 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
                    <th title={t('channels.performance.modelHint')}>{t('channels.performance.model')}</th>
                    <th className="text-right">{t('channels.performance.requests')}</th>
                    <th className="text-right">{t('channels.table.successRate')}</th>
                    <th className="text-right">{t('channels.table.avgLatency')}</th>
                    <th className="text-right">{t('channels.models.ttft')}</th>
                  </tr>
                </thead>
                <tbody>
                  {list.visible.map((m) => (
                    <tr
                      key={m.model}
                      className="[&>td]:h-11 [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0"
                    >
                      <td className="font-mono" title={m.model}>
                        {m.model}
                      </td>
                      <td className="text-right tabular-nums">{m.requests.toLocaleString()}</td>
                      <td className="text-right tabular-nums">{pct(m.successRate)}</td>
                      <td className="text-right tabular-nums">{sec(m.avgLatencyMs)}</td>
                      <td className="text-right tabular-nums">{sec(m.avgTtftMs)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <LongListMore list={list} />
            </div>
          </div>
        </>
      )}
    </div>
  );
}

PerformanceBody.propTypes = {
  object: PropTypes.object.isRequired,
  draft: PropTypes.object,
  setDraft: PropTypes.func,
  ctx: PropTypes.object
};

// No `patch`: read-only, so the panel shows no save bar for it.
export const performanceTab = { key: 'performance', Body: PerformanceBody };
