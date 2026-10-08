import { useTranslation } from 'react-i18next';
import { Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { SERIES_COLORS } from '../Analytics/chartUtils';

// ==============================|| DASHBOARD — 7-DAY MODEL USAGE PIE ||============================== //
// recharts donut equivalent of the v1 ApexCharts ModelUsagePieChart. Same input
// shape: [{ name, value }] aggregated from `/api/user/dashboard` request counts.

function PieTooltip({ active, payload }) {
  if (!active || !payload?.length) return null;
  const { name, value } = payload[0].payload;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-md">
      <span className="font-medium text-foreground">{name}: </span>
      <span className="font-mono font-semibold tabular-nums">{value.toLocaleString()}</span>
    </div>
  );
}

export default function ModelUsagePieChart({ data = [], loading = false }) {
  const { t } = useTranslation();
  const total = data.reduce((s, d) => s + d.value, 0);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-base">{t('dashboard_index.7days_model_usage_pie')}</CardTitle>
        <span className="font-mono text-sm font-medium tabular-nums text-muted-foreground">
          {t('dashboard_index.total')}: {total.toLocaleString()}
        </span>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="h-72 w-full animate-pulse rounded-md bg-muted" />
        ) : data.length === 0 ? (
          <div className="flex h-72 items-center justify-center text-sm text-muted-foreground">
            {t('dashboard_index.no_data_available')}
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={288}>
            <PieChart>
              <Pie
                data={data}
                dataKey="value"
                nameKey="name"
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={95}
                paddingAngle={1}
                stroke="var(--card)"
              >
                {data.map((entry, i) => (
                  <Cell key={entry.name} fill={SERIES_COLORS[i % SERIES_COLORS.length]} />
                ))}
              </Pie>
              <Tooltip content={<PieTooltip />} />
              <Legend wrapperStyle={{ fontSize: 12 }} formatter={(value) => <span className="text-muted-foreground">{value}</span>} />
            </PieChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
