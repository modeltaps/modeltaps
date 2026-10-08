import {
  Bar,
  BarChart,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis
} from 'recharts';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { SERIES_COLORS } from './chartUtils';

// ==============================|| ANALYTICS — CHART CARDS ||============================== //
// recharts equivalents of the v1 ApexCharts: stacked bars per channel/model for
// cost/tokens/requests, a bar+line composed chart for redemptions, and plain bars.

const axis = {
  tick: { fill: 'var(--muted-foreground)', fontSize: 12 },
  axisLine: { stroke: 'var(--border)' },
  tickLine: false
};

function ChartShell({ title, total, loading, height = 288, children }) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-base">{title}</CardTitle>
        {total != null && <span className="font-mono text-sm font-medium tabular-nums text-muted-foreground">{total}</span>}
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="w-full animate-pulse rounded-md bg-muted" style={{ height }} />
        ) : (
          <ResponsiveContainer width="100%" height={height}>
            {children}
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}

const tooltipStyle = {
  contentStyle: {
    background: 'var(--card)',
    border: '1px solid var(--border)',
    borderRadius: 8,
    fontSize: 12
  },
  labelStyle: { color: 'var(--muted-foreground)' }
};

export function StackedBarCard({ title, total, loading, data, series, formatValue }) {
  return (
    <ChartShell title={title} total={total} loading={loading}>
      <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
        <XAxis dataKey="label" {...axis} />
        <YAxis width={56} {...axis} axisLine={false} tickFormatter={formatValue} />
        <Tooltip cursor={{ fill: 'var(--muted)', opacity: 0.3 }} {...tooltipStyle} />
        {series.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} />}
        {series.map((name, i) => (
          <Bar key={name} dataKey={name} stackId="a" fill={SERIES_COLORS[i % SERIES_COLORS.length]} radius={i === series.length - 1 ? [4, 4, 0, 0] : 0} />
        ))}
      </BarChart>
    </ChartShell>
  );
}

export function StackedRegistrationCard({ title, total, loading, data, directLabel, inviteLabel }) {
  return (
    <ChartShell title={title} total={total} loading={loading} height={320}>
      <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
        <XAxis dataKey="label" {...axis} />
        <YAxis width={48} {...axis} axisLine={false} />
        <Tooltip cursor={{ fill: 'var(--muted)', opacity: 0.3 }} {...tooltipStyle} />
        <Legend wrapperStyle={{ fontSize: 12 }} />
        <Bar name={directLabel} dataKey="direct" stackId="r" fill={SERIES_COLORS[0]} />
        <Bar name={inviteLabel} dataKey="invite" stackId="r" fill={SERIES_COLORS[1]} radius={[4, 4, 0, 0]} />
      </BarChart>
    </ChartShell>
  );
}

export function RedemptionCard({ title, loading, data, amountLabel, usersLabel }) {
  return (
    <ChartShell title={title} loading={loading} height={320}>
      <ComposedChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
        <XAxis dataKey="label" {...axis} />
        <YAxis yAxisId="left" width={56} {...axis} axisLine={false} tickFormatter={(v) => '$' + v} />
        <YAxis yAxisId="right" orientation="right" width={40} {...axis} axisLine={false} />
        <Tooltip cursor={{ fill: 'var(--muted)', opacity: 0.3 }} {...tooltipStyle} />
        <Legend wrapperStyle={{ fontSize: 12 }} />
        <Bar yAxisId="left" name={amountLabel} dataKey="amount" fill={SERIES_COLORS[0]} radius={[4, 4, 0, 0]} />
        <Line yAxisId="right" name={usersLabel} type="monotone" dataKey="users" stroke={SERIES_COLORS[2]} strokeWidth={2} dot={false} />
      </ComposedChart>
    </ChartShell>
  );
}

export function SingleBarCard({ title, total, loading, data, label, color = SERIES_COLORS[0] }) {
  return (
    <ChartShell title={title} total={total} loading={loading} height={320}>
      <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
        <XAxis dataKey="label" {...axis} />
        <YAxis width={56} {...axis} axisLine={false} />
        <Tooltip cursor={{ fill: 'var(--muted)', opacity: 0.3 }} {...tooltipStyle} />
        <Bar name={label} dataKey="money" fill={color} radius={[4, 4, 0, 0]} />
      </BarChart>
    </ChartShell>
  );
}
