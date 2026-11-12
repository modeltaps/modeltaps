import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

// ==============================|| DASHBOARD — TOP MODELS (7 DAYS) ||============================== //
// Models ranked by requests over the last seven days, each with a thin bar for its share of the
// leader. Replaces the donut, which said little with one or two models.

export default function TopModelsCard({ data = [], loading = false }) {
  const { t } = useTranslation();
  const rows = [...data].sort((a, b) => b.value - a.value).slice(0, 5);
  const max = Math.max(...rows.map((r) => r.value), 1);
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('dashboardList.topModels')}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {loading ? (
          <div className="h-32 w-full animate-pulse rounded-md bg-muted" />
        ) : rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('dashboard_index.no_data_available')}</p>
        ) : (
          rows.map((r, idx) => (
            <div key={r.name} className="text-sm">
              <div className="flex items-baseline gap-3">
                <span className="w-4 shrink-0 text-right text-xs tabular-nums text-muted-foreground">{idx + 1}</span>
                <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{r.name}</span>
                <span className="shrink-0 tabular-nums">
                  {r.value.toLocaleString()} <span className="text-xs text-muted-foreground">{t('dashboardList.requests')}</span>
                </span>
              </div>
              <div className="ml-7 mt-1.5 h-1 rounded-full bg-muted">
                <div className="h-1 rounded-full bg-primary/70" style={{ width: `${Math.max((r.value / max) * 100, 2)}%` }} />
              </div>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  );
}

TopModelsCard.propTypes = {
  data: PropTypes.arrayOf(PropTypes.shape({ name: PropTypes.string, value: PropTypes.number })),
  loading: PropTypes.bool
};
