import { Line, LineChart, ResponsiveContainer } from 'recharts';
import { Minus, TrendingDown, TrendingUp } from 'lucide-react';

import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';

// ==============================|| DASHBOARD — STAT CARD ||============================== //
// A single metric tile. `accent` highlights the key metric in the primary color;
// every other card stays monochrome. Numbers use a mono-feeling tabular font.
// Optional `series` (7-day values) renders a mini sparkline and `trend`
// ({ direction, percent }) renders a today-vs-yesterday badge; cards without
// them keep the plain number-only look.

function TrendBadge({ trend }) {
  const { direction, percent } = trend;
  const cfg =
    {
      up: { Icon: TrendingUp, cls: 'text-emerald-500' },
      down: { Icon: TrendingDown, cls: 'text-red-500' },
      flat: { Icon: Minus, cls: 'text-muted-foreground' }
    }[direction] || { Icon: Minus, cls: 'text-muted-foreground' };
  const { Icon: TIcon, cls } = cfg;
  return (
    <span className={cn('flex items-center gap-0.5 rounded-md bg-muted/60 px-1.5 py-0.5 text-xs font-medium tabular-nums', cls)}>
      <TIcon className="size-3.5" />
      {percent}%
    </span>
  );
}

export default function StatCard({ label, value, valueTitle, hint, icon: Icon, accent = false, loading = false, series, trend, seriesColor }) {
  const hasSeries = Array.isArray(series) && series.length > 0;
  const chartData = hasSeries ? series.map((v, i) => ({ i, v: Number(v) || 0 })) : [];

  return (
    <Card className={cn(accent && 'border-foreground/30')}>
      <CardContent className="flex flex-col gap-2 p-5">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
          {Icon && <Icon className={cn('size-4', accent ? 'text-foreground' : 'text-muted-foreground')} />}
        </div>
        {loading ? (
          <div className="h-9 w-24 animate-pulse rounded-md bg-muted" />
        ) : (
          <div className="flex min-w-0 items-end justify-between gap-2">
            <span
              title={valueTitle}
              className={cn(
                'min-w-0 truncate font-mono font-semibold leading-none tabular-nums',
                String(value ?? '').length > 7 ? 'text-2xl' : 'text-3xl',
                'text-foreground'
              )}
            >
              {value}
            </span>
            {trend && <TrendBadge trend={trend} />}
          </div>
        )}
        {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
        {!loading && hasSeries && (
          <div className="-mb-1 mt-1 h-10 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData} margin={{ top: 4, right: 2, left: 2, bottom: 0 }}>
                <Line
                  type="monotone"
                  dataKey="v"
                  stroke={seriesColor || 'var(--primary)'}
                  strokeWidth={2}
                  strokeLinecap="round"
                  dot={false}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
