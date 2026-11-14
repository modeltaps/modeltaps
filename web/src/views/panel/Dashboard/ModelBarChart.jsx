import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { formatSpendAmount } from 'utils/common';
import { SERIES_COLORS } from '../Analytics/chartUtils';

// ==============================|| DASHBOARD — 7-DAY MODEL STATISTICS ||============================== //
// recharts stacked bar equivalent of the v1 ApexCharts per-model cost chart.
// `data` is per-day rows keyed by model name; `series` lists the model names.

const axis = {
  tick: { fill: 'var(--muted-foreground)', fontSize: 12 },
  axisLine: { stroke: 'var(--border)' },
  tickLine: false
};

// Spend on the axis: whole dollars from $1, two decimals from $0.01, two significant digits
// below that ("$0.00018" rather than a clipped "$0.000135").
export const axisSpend = (v) => {
  const n = Number(v) || 0;
  const abs = Math.abs(n);
  if (abs === 0) return '$0';
  if (abs >= 1) return '$' + Number(n.toFixed(abs >= 100 ? 0 : 1));
  if (abs >= 0.01) return '$' + n.toFixed(2);
  return '$' + Number(n.toPrecision(2));
};

const tooltipStyle = {
  contentStyle: {
    background: 'var(--card)',
    border: '1px solid var(--border)',
    borderRadius: 8,
    fontSize: 12
  },
  labelStyle: { color: 'var(--muted-foreground)' }
};

export default function ModelBarChart({ data = [], series = [], total = 0, loading = false }) {
  const { t } = useTranslation();

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-base">{t('dashboard_index.week_model_statistics')}</CardTitle>
        {/* The week's spend across models (the old label read "total count" next to an amount). */}
        <span className="text-sm tabular-nums text-muted-foreground">{formatSpendAmount(total)}</span>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="h-72 w-full animate-pulse rounded-md bg-muted" />
        ) : series.length === 0 ? (
          <div className="flex h-72 items-center justify-center text-sm text-muted-foreground">
            {t('dashboard_index.no_data_available')}
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={288}>
            <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
              <XAxis dataKey="label" {...axis} />
              <YAxis width={64} {...axis} axisLine={false} tickFormatter={axisSpend} />
              <Tooltip cursor={{ fill: 'var(--muted)', opacity: 0.3 }} {...tooltipStyle} />
              {series.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} />}
              {series.map((name, i) => (
                <Bar
                  key={name}
                  dataKey={name}
                  stackId="a"
                  fill={SERIES_COLORS[i % SERIES_COLORS.length]}
                  radius={i === series.length - 1 ? [4, 4, 0, 0] : 0}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
