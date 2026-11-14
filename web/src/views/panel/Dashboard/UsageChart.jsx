import { useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { formatSpendAmount } from 'utils/common';

// ==============================|| DASHBOARD — USAGE TREND ||============================== //
// One bar per day (a smoothed area made a single busy day look like a three-day wave), in the
// accent. A metric selector switches requests / spend / tokens; the full breakdown is one link
// away on the usage page.

// Axis labels: whole numbers abbreviated (1.2K, 3M), spend with two decimals.
const shortNumber = (v) => {
  const n = Number(v) || 0;
  if (Math.abs(n) >= 1e6) return `${Number((n / 1e6).toFixed(1))}M`;
  if (Math.abs(n) >= 1e3) return `${Number((n / 1e3).toFixed(1))}K`;
  return `${n}`;
};

const METRICS = [
  { key: 'requests', i18n: 'today_requests', format: (v) => v.toLocaleString(), axis: shortNumber },
  { key: 'quota', i18n: 'today_consumption', format: (v) => formatSpendAmount(v), axis: (v) => `$${Number(v).toFixed(2)}` },
  { key: 'tokens', i18n: 'today_tokens', format: (v) => v.toLocaleString(), axis: shortNumber }
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
      <p className="font-medium tabular-nums text-foreground">{format(payload[0].value)}</p>
    </div>
  );
}

ChartTooltip.propTypes = { active: PropTypes.bool, payload: PropTypes.array, label: PropTypes.string, format: PropTypes.func };

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
        <div className="flex flex-wrap items-center gap-1">
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
          <Link to="/panel/usage" className="ml-1 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
            {t('dashboardList.viewUsage')}
            <ArrowRight className="size-3" />
          </Link>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="h-72 w-full animate-pulse rounded-md bg-muted" />
        ) : (
          <ResponsiveContainer width="100%" height={288}>
            <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
              <XAxis
                dataKey="label"
                tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }}
                axisLine={{ stroke: 'var(--border)' }}
                tickLine={false}
              />
              <YAxis
                width={52}
                tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }}
                axisLine={false}
                tickLine={false}
                tickFormatter={active.axis}
              />
              <Tooltip content={<ChartTooltip format={active.format} />} cursor={{ fill: 'var(--muted)', opacity: 0.3 }} />
              <Bar dataKey={active.key} fill="var(--chart-1)" radius={[4, 4, 0, 0]} maxBarSize={48} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}

UsageChart.propTypes = {
  data: PropTypes.array,
  loading: PropTypes.bool,
  range: PropTypes.string,
  onRangeChange: PropTypes.func
};
