import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { formatSpendAmount } from 'utils/common';

// ==============================|| DASHBOARD — 7-DAY USAGE CHART ||============================== //
// recharts area chart over the same 7-day grouped data the v1 dashboard derives
// from `/api/user/dashboard`. A metric selector mirrors v1's three stat cards.

const METRICS = [
  { key: 'requests', i18n: 'today_requests', format: (v) => v.toLocaleString() },
  { key: 'quota', i18n: 'today_consumption', format: (v) => formatSpendAmount(v) },
  { key: 'tokens', i18n: 'today_tokens', format: (v) => v.toLocaleString() }
];

const RANGES = [
  { key: '7d', i18n: 'range_7d' },
  { key: '30d', i18n: 'range_30d' },
  { key: '90d', i18n: 'range_90d' }
];

function ChartTooltip({ active, payload, label, format }) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-md">
      <p className="mb-1 font-medium text-muted-foreground">{label}</p>
      <p className="font-mono font-semibold tabular-nums text-foreground">{format(payload[0].value)}</p>
    </div>
  );
}

export default function UsageChart({ data, loading = false, range = '7d', onRangeChange }) {
  const { t } = useTranslation();
  const [metric, setMetric] = useState('requests');
  const active = METRICS.find((m) => m.key === metric) || METRICS[0];

  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-center justify-between gap-2 space-y-0">
        <div className="flex items-center gap-2">
          <CardTitle className="text-base">{t('dashboard_index.usage_trend')}</CardTitle>
          {onRangeChange && (
            <div className="flex gap-1">
              {RANGES.map((r) => (
                <Button
                  key={r.key}
                  size="sm"
                  variant={r.key === range ? 'secondary' : 'ghost'}
                  className={cn('h-7 px-2.5 text-xs', r.key === range && 'text-foreground')}
                  onClick={() => onRangeChange(r.key)}
                >
                  {t(`dashboard_index.${r.i18n}`)}
                </Button>
              ))}
            </div>
          )}
        </div>
        <div className="flex gap-1">
          {METRICS.map((m) => (
            <Button
              key={m.key}
              size="sm"
              variant={m.key === metric ? 'secondary' : 'ghost'}
              className={cn('h-7 px-2.5 text-xs', m.key === metric && 'text-foreground')}
              onClick={() => setMetric(m.key)}
            >
              {t(`dashboard_index.${m.i18n}`)}
            </Button>
          ))}
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="h-72 w-full animate-pulse rounded-md bg-muted" />
        ) : (
          <ResponsiveContainer width="100%" height={288}>
            <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="rf-usage-fill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--primary)" stopOpacity={0.35} />
                  <stop offset="100%" stopColor="var(--primary)" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
              <XAxis
                dataKey="label"
                tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }}
                axisLine={{ stroke: 'var(--border)' }}
                tickLine={false}
              />
              <YAxis width={48} tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }} axisLine={false} tickLine={false} />
              <Tooltip content={<ChartTooltip format={active.format} />} cursor={{ stroke: 'var(--border)' }} />
              <Area
                type="monotone"
                dataKey={active.key}
                stroke="var(--primary)"
                strokeWidth={2}
                fill="url(#rf-usage-fill)"
                dot={false}
                activeDot={{ r: 4, fill: 'var(--primary)' }}
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
