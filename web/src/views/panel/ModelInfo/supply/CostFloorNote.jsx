import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { API } from 'utils/api';
import { renderQuota } from 'utils/common';

export const COST_FLOOR_DAYS = 7;

export async function fetchCostFloorStats(days = COST_FLOOR_DAYS) {
  const res = await API.get('/api/model_supply/cost_floor_stats', { params: { days } });
  return res.data?.success ? res.data.data : null;
}

// How often the cost floor charged the channel cost over the last week, and where: the
// requests that cost more than the quote. Shown only when it happened, so the admin can raise
// the quote, move the supply down or put the channel in a group of its own.
export default function CostFloorNote({ className }) {
  const { t } = useTranslation();
  const [stats, setStats] = useState(null);
  useEffect(() => {
    let live = true;
    fetchCostFloorStats()
      .then((data) => live && setStats(data))
      .catch(() => {});
    return () => {
      live = false;
    };
  }, []);
  if (!stats || !(stats.requests > 0)) return null;
  const extra = Math.max(0, (stats.charged || 0) - (stats.quoted || 0));
  const rows = (stats.rows || []).slice(0, 8);
  return (
    <div className={className ?? 'mt-2 text-xs text-muted-foreground'}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" data-testid="cost-floor-note" className="underline-offset-2 hover:underline">
            {t('modelsPage.costFloorNote', { days: stats.days, count: stats.requests, extra: renderQuota(extra, 4) })}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent role="dialog" align="start" className="w-[26rem] space-y-2 p-3 text-xs">
          <p className="text-muted-foreground">{t('modelsPage.costFloorHint')}</p>
          <table className="w-full tabular-nums">
            <thead className="text-muted-foreground">
              <tr>
                <th className="py-1 text-left font-normal">{t('modelsPage.costFloorModel')}</th>
                <th className="py-1 text-left font-normal">{t('modelsPage.costFloorChannel')}</th>
                <th className="py-1 text-right font-normal">{t('modelsPage.costFloorRequests')}</th>
                <th className="py-1 text-right font-normal">{t('modelsPage.costFloorExtra')}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={`${r.model}-${r.channel_id}-${r.group}`} className="border-t border-border">
                  <td className="max-w-[9rem] truncate py-1 font-mono" title={r.model}>
                    {r.model}
                  </td>
                  <td className="max-w-[8rem] truncate py-1" title={r.group ? `${r.channel_name} · ${r.group}` : r.channel_name}>
                    {r.channel_name || `#${r.channel_id}`}
                    {r.group && <span className="text-muted-foreground"> · {r.group}</span>}
                  </td>
                  <td className="py-1 text-right">{r.requests}</td>
                  <td className="py-1 text-right">{renderQuota(Math.max(0, r.charged - r.quoted), 4)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

CostFloorNote.propTypes = { className: PropTypes.string };
