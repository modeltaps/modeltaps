import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';

// ==============================|| DASHBOARD — STAT CARD ||============================== //
// A metric tile: the label, the figure, and the change against yesterday ("↑12% 较昨天") when
// there is a yesterday to compare with. No icons or sparklines; the trend chart below carries
// the shape of the week.

function Trend({ trend }) {
  const { t } = useTranslation();
  const { direction, percent } = trend;
  const arrow = direction === 'up' ? '↑' : direction === 'down' ? '↓' : '';
  return (
    <span className="text-xs text-muted-foreground">
      <span className={cn('font-medium tabular-nums', direction === 'up' && 'text-emerald-600', direction === 'down' && 'text-red-600')}>
        {arrow}
        {percent}%
      </span>{' '}
      {t('dashboardList.vsYesterday')}
    </span>
  );
}

Trend.propTypes = { trend: PropTypes.shape({ direction: PropTypes.string, percent: PropTypes.number }).isRequired };

export default function StatCard({ label, value, valueTitle, hint, loading = false, trend }) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-1 p-4 sm:p-5">
        <span className="text-[13px] text-muted-foreground">{label}</span>
        {loading ? (
          <div className="h-8 w-24 animate-pulse rounded-md bg-muted" />
        ) : (
          <span title={valueTitle} className="min-w-0 truncate text-2xl font-medium tabular-nums text-foreground">
            {value}
          </span>
        )}
        <span className="min-h-4">
          {!loading && trend && trend.direction !== 'flat' && <Trend trend={trend} />}
          {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
        </span>
      </CardContent>
    </Card>
  );
}

StatCard.propTypes = {
  label: PropTypes.string.isRequired,
  value: PropTypes.node,
  valueTitle: PropTypes.string,
  hint: PropTypes.string,
  loading: PropTypes.bool,
  trend: PropTypes.shape({ direction: PropTypes.string, percent: PropTypes.number })
};

// Secondary metrics on one quiet strip (rate, throughput, savings, cache hit): label over value,
// two per row on phones, four in a row otherwise.
export function MetricStrip({ items, loading = false }) {
  return (
    <Card>
      <CardContent className="grid grid-cols-2 gap-x-4 gap-y-3 p-4 sm:grid-cols-4">
        {items.map((m) => (
          <div key={m.label} className="min-w-0" title={m.hint}>
            <div className="truncate text-xs text-muted-foreground">{m.label}</div>
            <div className="mt-0.5 truncate text-sm font-medium tabular-nums" title={m.valueTitle}>
              {loading ? '—' : m.value}
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

MetricStrip.propTypes = {
  items: PropTypes.arrayOf(
    PropTypes.shape({ label: PropTypes.string.isRequired, value: PropTypes.node, valueTitle: PropTypes.string, hint: PropTypes.string })
  ).isRequired,
  loading: PropTypes.bool
};
