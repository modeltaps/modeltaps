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
        <span className="font-mono text-sm font-medium tabular-nums text-muted-foreground">
          {t('dashboard_index.total')}: {formatSpendAmount(total)}
        </span>
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
              <YAxis width={56} {...axis} axisLine={false} tickFormatter={(v) => '$' + v} />
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
