import PropTypes from 'prop-types';
import { useEffect, useMemo, useRef, useState } from 'react';
import dayjs from 'dayjs';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { ChevronDown, ChevronUp } from 'lucide-react';

import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import { toSearchParams } from '@/components/filter-bar';
import { API } from 'utils/api';
import { renderNumber, renderSpend } from 'utils/common';
import { SERIES_COLORS } from '../Analytics/chartUtils';

// ==============================|| PANEL — LOG REQUEST HISTOGRAM ||============================== //
// 表格上方的按时段请求量柱状图。数据由后端单条聚合端点(/api/log[/self]/histogram,组织上下文经
// orgScope 改写到 /api/org/:id/logs/histogram)返回,桶粒度(hour/day)由后端按窗口自适应。
// 筛选参数与列表完全一致,随筛选联动;折叠状态记忆到 localStorage。

const COLLAPSE_STORAGE_KEY = 'log-histogram-collapsed';

const axis = {
  tick: { fill: 'var(--muted-foreground)', fontSize: 11 },
  axisLine: { stroke: 'var(--border)' },
  tickLine: false
};

const tooltipStyle = {
  contentStyle: { background: 'var(--card)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 },
  labelStyle: { color: 'var(--muted-foreground)' }
};

// 后端桶标签:hour="YYYY-MM-DD HH:00"、day="YYYY-MM-DD"。压缩为紧凑显示标签。
const formatLabel = (bucket, granularity) => {
  const d = dayjs(bucket);
  if (!d.isValid()) return bucket;
  return granularity === 'hour' ? d.format('MM-DD HH:00') : d.format('MM-DD');
};

// LOGX-3 指标切换:请求数/花费/Tokens 共用同一份桶数据,切换仅改 dataKey 与格式化,不重发请求。
const METRICS = ['requests', 'spend', 'tokens'];
const METRIC_DATA_KEY = { requests: 'count', spend: 'quota', tokens: 'tokens' };

// 轴/提示格式化:花费走 renderSpend(尊重货币/点数模式),请求数与 Tokens 用 K/M/B 缩写。
const formatMetricValue = (metric, value) => (metric === 'spend' ? renderSpend(value) : renderNumber(value));

export default function LogHistogram({ filterQuery, effectiveAdmin, t }) {
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(COLLAPSE_STORAGE_KEY) === '1';
    } catch {
      return false;
    }
  });
  const [rows, setRows] = useState([]);
  const [granularity, setGranularity] = useState('day');
  const [metric, setMetric] = useState('requests');
  const [loading, setLoading] = useState(false);
  const reqIdRef = useRef(0);

  const queryKey = useMemo(() => JSON.stringify(filterQuery), [filterQuery]);

  useEffect(() => {
    if (collapsed) return undefined;
    const reqId = ++reqIdRef.current;
    setLoading(true);
    (async () => {
      try {
        const url = effectiveAdmin ? '/api/log/histogram' : '/api/log/self/histogram';
        const res = await API.get(url, { params: toSearchParams(filterQuery) });
        if (reqId !== reqIdRef.current) return;
        const { success, data } = res.data;
        if (success && data) {
          setGranularity(data.bucket || 'day');
          setRows(Array.isArray(data.buckets) ? data.buckets : []);
        } else {
          setRows([]);
        }
      } catch (error) {
        if (reqId === reqIdRef.current) setRows([]);
      } finally {
        if (reqId === reqIdRef.current) setLoading(false);
      }
    })();
    return undefined;
    // queryKey 覆盖 filterQuery 的内容变化;collapsed 展开时才拉取。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queryKey, effectiveAdmin, collapsed]);

  const chartData = useMemo(
    () =>
      rows.map((r) => ({
        label: formatLabel(r.bucket, granularity),
        count: Number(r.count) || 0,
        quota: Number(r.quota) || 0,
        tokens: Number(r.tokens) || 0
      })),
    [rows, granularity]
  );
  const totals = useMemo(
    () =>
      chartData.reduce(
        (acc, r) => ({ count: acc.count + r.count, quota: acc.quota + r.quota, tokens: acc.tokens + r.tokens }),
        { count: 0, quota: 0, tokens: 0 }
      ),
    [chartData]
  );
  const summary = useMemo(
    () => [
      { label: t('logPage.histogram.totalRequests'), value: totals.count.toLocaleString() },
      { label: t('logPage.histogram.totalSpend'), value: renderSpend(totals.quota) },
      { label: t('logPage.histogram.totalTokens'), value: renderNumber(totals.tokens) }
    ],
    [totals, t]
  );

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(COLLAPSE_STORAGE_KEY, next ? '1' : '0');
      } catch {
        /* storage unavailable -> keep in-memory only */
      }
      return next;
    });
  };

  return (
    <Card>
      <CardContent className="p-3">
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs font-medium text-foreground">{t('logPage.histogram.title')}</span>
          <div className="flex items-center gap-2">
            {!collapsed && (
              <div className="inline-flex overflow-hidden rounded-md border border-border">
                {METRICS.map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setMetric(m)}
                    className={cn(
                      'px-2 py-0.5 text-xs font-medium transition-colors',
                      metric === m ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted'
                    )}
                  >
                    {t(`logPage.histogram.${m}`)}
                  </button>
                ))}
              </div>
            )}
            <button
              type="button"
              onClick={toggleCollapsed}
              aria-expanded={!collapsed}
              aria-label={collapsed ? t('logPage.histogram.expand') : t('logPage.histogram.collapse')}
              className="flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            >
              {collapsed ? <ChevronDown className="size-4" /> : <ChevronUp className="size-4" />}
            </button>
          </div>
        </div>
        {!collapsed && (
          <>
            <div className="mt-1.5 flex flex-wrap items-baseline gap-x-4 gap-y-1">
              {summary.map((s) => (
                <span key={s.label} className="flex items-baseline gap-1">
                  <span className="text-xs text-muted-foreground">{s.label}</span>
                  <span className="font-mono text-xs font-medium tabular-nums text-foreground">{s.value}</span>
                </span>
              ))}
            </div>
            <div className="mt-2">
              {loading ? (
                <div className="h-[90px] w-full animate-pulse rounded-md bg-muted" />
              ) : chartData.length === 0 ? (
                <div className="flex h-[90px] items-center justify-center text-sm text-muted-foreground">{t('logPage.histogram.empty')}</div>
              ) : (
                <ResponsiveContainer width="100%" height={90}>
                  <BarChart data={chartData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }} barCategoryGap="12%">
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
                    <XAxis dataKey="label" {...axis} minTickGap={16} />
                    <YAxis
                      width={44}
                      allowDecimals={false}
                      tickFormatter={(v) => formatMetricValue(metric, v)}
                      {...axis}
                      axisLine={false}
                    />
                    <Tooltip
                      cursor={{ fill: 'var(--muted)', opacity: 0.3 }}
                      formatter={(v) => [formatMetricValue(metric, v), t(`logPage.histogram.${metric}`)]}
                      {...tooltipStyle}
                    />
                    <Bar
                      dataKey={METRIC_DATA_KEY[metric]}
                      name={t(`logPage.histogram.${metric}`)}
                      fill={SERIES_COLORS[0]}
                      radius={[3, 3, 0, 0]}
                      maxBarSize={56}
                    />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

LogHistogram.propTypes = {
  filterQuery: PropTypes.object.isRequired,
  effectiveAdmin: PropTypes.bool,
  t: PropTypes.func.isRequired
};
