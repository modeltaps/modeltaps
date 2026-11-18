import { useCallback, useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, CheckCircle2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';
import { API } from 'utils/api';
import { admin } from 'routes/paths';
import { TextRow } from '../parts';
import OptionCard from './OptionCard';

// ==============================|| PLATFORM SETTINGS — PRICING ||============================== //
// Users pay the catalog (official) price times their user group's ratio, so the site's margin is
// the default group's ratio, set on the user groups page. This card saves the cost of taking
// payments and previews, for a default group ratio being tried, what each channel needs to break
// even, the supplies that would lose money (the price check holds such backups) and what every
// user group brings in.

const pct = (v) => `${(Math.round(v * 10) / 10).toFixed(1)}%`;
// A group ratio as "×1.094": three decimals at most, trailing zeros dropped.
export const ratioText = (v) => `×${+Number(v || 0).toFixed(3)}`;
const num = (v) => {
  const n = Number.parseFloat(v);
  return Number.isFinite(n) ? n : 0;
};
// The ratio being tried, or null to preview the saved one.
const tryRatioValue = (v) => {
  const n = Number.parseFloat(v);
  return Number.isFinite(n) && n > 0 ? n : null;
};

// The preview of the typed values, refreshed a moment after the typing stops. Without a ratio
// the server uses the default group's saved ratio.
function usePricingRisk(ratio, payment) {
  const [state, setState] = useState({ loading: true, data: null, error: '' });
  useEffect(() => {
    let alive = true;
    const id = setTimeout(async () => {
      try {
        const params = ratio == null ? { payment } : { ratio, payment };
        const res = await API.get('/api/model_supply/pricing_risk', { params });
        if (!alive) return;
        const { success, data, message } = res.data;
        setState({ loading: false, data: success ? data : null, error: success ? '' : message });
      } catch {
        if (alive) setState({ loading: false, data: null, error: '' });
      }
    }, 400);
    return () => {
      alive = false;
      clearTimeout(id);
    };
  }, [ratio, payment]);
  return state;
}

function PreviewSkeleton() {
  return (
    <div className="space-y-3" aria-busy="true">
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-10 w-full rounded-lg" />
      {[0, 1, 2].map((i) => (
        <div key={i} className="flex justify-between gap-6">
          <Skeleton className="h-3.5 w-32" />
          <Skeleton className="h-3.5 w-48" />
        </div>
      ))}
    </div>
  );
}

// `ratio` is the default group ratio being tried (null: the saved one); `onSavedRatio` receives
// the saved ratio from a preview made without one.
export function PricingRiskPreview({ ratio, payment, onSavedRatio }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`pricingSettings.risk.${k}`, o);
  const { loading, data, error } = usePricingRisk(ratio, payment);

  useEffect(() => {
    if (data && ratio == null) onSavedRatio?.(data.group_ratio);
  }, [data, ratio, onSavedRatio]);

  if (loading && !data) return <PreviewSkeleton />;
  if (!data) return error ? <p className="text-sm text-destructive">{error}</p> : null;

  const problems = [];
  if (data.models_at_loss?.length)
    problems.push(m('modelsAtLoss', { count: data.models_at_loss.length, models: data.models_at_loss.slice(0, 5).join(', ') }));
  if (data.serving_above > 0) problems.push(m('servingAbove', { count: data.serving_above }));
  if (data.backup_above > 0) problems.push(m('backupAbove', { count: data.backup_above }));
  const ok = problems.length === 0;

  return (
    <div className="space-y-3" data-testid="pricing-risk">
      <p className="text-sm text-muted-foreground">
        {m('net', { ratio: ratioText(data.group_ratio), payment: pct(data.payment_cost), factor: data.net_factor.toFixed(4) })}
      </p>
      <div
        className={cn(
          'space-y-1 rounded-lg border px-3 py-2 text-sm',
          ok ? 'border-success/40 bg-success/10' : 'border-warning/40 bg-warning/10'
        )}
      >
        {ok ? (
          <p className="flex items-center gap-2 text-success">
            <CheckCircle2 className="size-4 shrink-0" />
            {m('ok')}
          </p>
        ) : (
          problems.map((line) => (
            <p key={line} className="flex items-start gap-2 text-warning">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <span>{line}</span>
            </p>
          ))
        )}
        {data.cost_unknown > 0 && <p className="text-muted-foreground">{m('costUnknown', { count: data.cost_unknown })}</p>}
        {!data.supply_routing && <p className="text-muted-foreground">{m('notRouting')}</p>}
      </div>
      {data.channels?.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="py-1.5 pr-3 font-normal">{m('col.channel')}</th>
                <th className="py-1.5 pr-3 text-right font-normal">{m('col.fee')}</th>
                <th className="py-1.5 pr-3 text-right font-normal">{m('col.breakeven')}</th>
                <th className="py-1.5 pr-3 text-right font-normal">{m('col.margin')}</th>
                <th className="py-1.5 pr-3 text-right font-normal">{m('col.worst')}</th>
                <th className="py-1.5 text-right font-normal">{m('col.losing')}</th>
              </tr>
            </thead>
            <tbody>
              {data.channels.map((ch) => {
                const losing = ch.serving_above + ch.backup_above;
                return (
                  <tr key={ch.channel_id} className="border-t border-border">
                    <td className="py-1.5 pr-3">{ch.name}</td>
                    <td className="py-1.5 pr-3 text-right tabular-nums">{pct(ch.fee_rate * 100)}</td>
                    <td className="py-1.5 pr-3 text-right tabular-nums">{ratioText(ch.fee_breakeven)}</td>
                    <td className={cn('py-1.5 pr-3 text-right tabular-nums', ch.fee_margin < 0 && 'text-warning')}>{pct(ch.fee_margin)}</td>
                    <td className={cn('py-1.5 pr-3 text-right tabular-nums', ch.worst_breakeven > data.group_ratio && 'text-warning')}>
                      {ch.supplies > 0 ? ratioText(ch.worst_breakeven) : '—'}
                    </td>
                    <td className={cn('py-1.5 text-right tabular-nums', losing > 0 && 'text-warning')}>
                      {losing > 0 ? m('losingCount', { serving: ch.serving_above, backup: ch.backup_above }) : '—'}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      {data.groups?.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm" data-testid="pricing-risk-groups">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="py-1.5 pr-3 font-normal">{m('group.name')}</th>
                <th className="py-1.5 pr-3 text-right font-normal">{m('group.ratio')}</th>
                <th className="py-1.5 pr-3 text-right font-normal">{m('group.net')}</th>
                <th className="py-1.5 text-right font-normal">{m('group.losing')}</th>
              </tr>
            </thead>
            <tbody>
              {data.groups.map((g) => (
                <tr key={g.symbol} className="border-t border-border">
                  <td className="py-1.5 pr-3">
                    {g.name || g.symbol}
                    {g.name && g.name !== g.symbol && <span className="ml-1.5 font-mono text-xs text-muted-foreground">{g.symbol}</span>}
                  </td>
                  <td className="py-1.5 pr-3 text-right tabular-nums">{ratioText(g.ratio)}</td>
                  <td className={cn('py-1.5 pr-3 text-right tabular-nums', g.net_factor < 1 && 'text-warning')}>
                    ${g.net_factor.toFixed(4)}
                  </td>
                  <td className={cn('py-1.5 text-right tabular-nums', g.losing_channels > 0 && 'text-warning')}>
                    {g.losing_channels > 0 ? g.losing_channels : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="mt-1 text-xs text-muted-foreground">{m('group.hint')}</p>
        </div>
      )}
      {data.worst?.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer text-muted-foreground">{m('worstTitle', { count: data.worst.length })}</summary>
          <ul className="mt-1.5 space-y-0.5">
            {data.worst.map((s) => (
              <li key={`${s.model}-${s.channel_name}`} className="flex justify-between gap-3">
                <span className="truncate">
                  {s.model} · {s.channel_name} · {m(`traffic.${s.traffic}`)}
                </span>
                <span className="shrink-0 tabular-nums text-warning">{m('needs', { ratio: ratioText(s.breakeven) })}</span>
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}

PricingRiskPreview.propTypes = { ratio: PropTypes.number, payment: PropTypes.number.isRequired, onSavedRatio: PropTypes.func };

export function PricingCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const f = (k) => t(`pricingSettings.${k}`);
  // The default group ratio to try: preview only, filled with the saved ratio once known.
  const [tryRatio, setTryRatio] = useState(null);
  const onSavedRatio = useCallback((r) => setTryRatio((cur) => (cur === null ? String(+Number(r).toFixed(4)) : cur)), []);
  const validate = (i) => {
    const pc = Number.parseFloat(i.PaymentCostPercent || '0');
    if (!Number.isFinite(pc) || pc < 0 || pc >= 100) return f('invalidPayment');
    return null;
  };
  return (
    <OptionCard ctx={ctx} id="pricing" options={{ validate }}>
      <div className="grid gap-5 sm:grid-cols-2">
        <TextRow
          id="PaymentCostPercent"
          type="number"
          unit="%"
          label={f('payment.label')}
          value={inputs.PaymentCostPercent}
          onChange={(v) => setField('PaymentCostPercent', v)}
          placeholder="4"
          description={f('payment.description')}
        />
        <TextRow
          id="PricingTryRatio"
          type="number"
          unit="×"
          label={f('tryRatio.label')}
          value={tryRatio ?? ''}
          onChange={setTryRatio}
          placeholder="1.12"
          description={
            <>
              {f('tryRatio.description')}{' '}
              <Link to={admin.userGroups()} className="text-info underline-offset-2 hover:underline">
                {f('tryRatio.link')}
              </Link>
            </>
          }
        />
      </div>
      <PricingRiskPreview ratio={tryRatioValue(tryRatio)} payment={num(inputs.PaymentCostPercent)} onSavedRatio={onSavedRatio} />
    </OptionCard>
  );
}

PricingCard.propTypes = { ctx: PropTypes.object.isRequired };
