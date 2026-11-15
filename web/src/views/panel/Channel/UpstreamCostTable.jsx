import PropTypes from 'prop-types';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { formatCost, formatDrift, isFlagged, summarizeUpstreamCosts } from './upstreamCosts';

const ROW =
  '[&>td]:h-11 [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0';

function Drift({ drift, title }) {
  const flagged = isFlagged(drift);
  return (
    <td className={`text-right tabular-nums ${flagged ? 'font-medium text-warning' : ''}`} title={flagged ? title : undefined}>
      {formatDrift(drift)}
    </td>
  );
}

Drift.propTypes = { drift: PropTypes.number, title: PropTypes.string };

// What the upstream charged against what the gateway expected, per model, for the requests of
// the range whose upstream reported a cost. Renders nothing when no request did.
export default function UpstreamCostTable({ rows }) {
  const { t } = useTranslation();
  const { models, total } = useMemo(() => summarizeUpstreamCosts(rows), [rows]);
  if (!models.length) return null;
  const flag = t('channels.upstreamCosts.flag');

  return (
    <div>
      <p className="text-[13px] font-medium">{t('channels.upstreamCosts.title')}</p>
      <p className="pb-2 pt-0.5 text-xs text-muted-foreground">{t('channels.upstreamCosts.hint')}</p>
      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full table-fixed border-collapse text-[13px]">
          <colgroup>
            <col />
            <col className="w-[4.5rem]" />
            <col className="w-[5.5rem]" />
            <col className="w-[5.5rem]" />
            <col className="w-[5rem]" />
          </colgroup>
          <thead>
            <tr className="text-left text-xs text-muted-foreground [&>th]:h-9 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
              <th>{t('channels.upstreamCosts.model')}</th>
              <th className="text-right">{t('channels.performance.requests')}</th>
              <th className="text-right">{t('channels.upstreamCosts.expected')}</th>
              <th className="text-right">{t('channels.upstreamCosts.upstream')}</th>
              <th className="text-right">{t('channels.upstreamCosts.diff')}</th>
            </tr>
          </thead>
          <tbody>
            {models.map((m) => (
              <tr key={m.model} className={ROW}>
                <td className="font-mono" title={m.model}>
                  {m.model}
                </td>
                <td className="text-right tabular-nums">{m.requests.toLocaleString()}</td>
                <td className="text-right tabular-nums">{formatCost(m.expected)}</td>
                <td className="text-right tabular-nums">{formatCost(m.upstream)}</td>
                <Drift drift={m.drift} title={flag} />
              </tr>
            ))}
            {models.length > 1 && (
              <tr className={`${ROW} bg-muted/40 font-medium`}>
                <td>{t('channels.upstreamCosts.total')}</td>
                <td className="text-right tabular-nums">{total.requests.toLocaleString()}</td>
                <td className="text-right tabular-nums">{formatCost(total.expected)}</td>
                <td className="text-right tabular-nums">{formatCost(total.upstream)}</td>
                <Drift drift={total.drift} title={flag} />
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

UpstreamCostTable.propTypes = { rows: PropTypes.array };
