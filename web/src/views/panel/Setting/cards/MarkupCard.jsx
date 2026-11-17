import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, CheckCircle2, Loader2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { TextRow } from '../parts';
import OptionCard from './OptionCard';

// ==============================|| PLATFORM SETTINGS — PLATFORM MARKUP ||============================== //
// The markup on the catalog (official) price and the cost of taking payments, with a preview of
// what the values being typed mean for each channel: the markup its fee needs to break even, the
// margin left, and the supplies that would lose money (the price check holds such backups).

const pct = (v) => `${(Math.round(v * 10) / 10).toFixed(1)}%`;
const num = (v) => {
  const n = Number.parseFloat(v);
  return Number.isFinite(n) ? n : 0;
};

// The preview of the typed values, refreshed a moment after the typing stops.
function useMarkupRisk(markup, payment) {
  const [state, setState] = useState({ loading: true, data: null, error: '' });
  useEffect(() => {
    let alive = true;
    const id = setTimeout(async () => {
      try {
        const res = await API.get('/api/model_supply/markup_risk', { params: { markup, payment } });
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
  }, [markup, payment]);
  return state;
}

export function MarkupRiskPreview({ markup, payment }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`markupSettings.risk.${k}`, o);
  const { loading, data, error } = useMarkupRisk(markup, payment);

  if (loading && !data) {
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        {m('loading')}
      </p>
    );
  }
  if (!data) return error ? <p className="text-sm text-destructive">{error}</p> : null;

  const problems = [];
  if (data.models_at_loss?.length)
    problems.push(m('modelsAtLoss', { count: data.models_at_loss.length, models: data.models_at_loss.slice(0, 5).join(', ') }));
  if (data.serving_above > 0) problems.push(m('servingAbove', { count: data.serving_above }));
  if (data.backup_above > 0) problems.push(m('backupAbove', { count: data.backup_above }));
  const ok = problems.length === 0;

  return (
    <div className="space-y-3" data-testid="markup-risk">
      <p className="text-sm text-muted-foreground">
        {m('net', { markup: pct(data.markup), payment: pct(data.payment_cost), factor: data.net_factor.toFixed(4) })}
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
                    <td className="py-1.5 pr-3 text-right tabular-nums">{pct(ch.fee_breakeven)}</td>
                    <td className={cn('py-1.5 pr-3 text-right tabular-nums', ch.fee_margin < 0 && 'text-warning')}>{pct(ch.fee_margin)}</td>
                    <td className={cn('py-1.5 pr-3 text-right tabular-nums', ch.worst_breakeven > data.markup && 'text-warning')}>
                      {ch.supplies > 0 ? pct(ch.worst_breakeven) : '—'}
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
      {data.worst?.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer text-muted-foreground">{m('worstTitle', { count: data.worst.length })}</summary>
          <ul className="mt-1.5 space-y-0.5">
            {data.worst.map((s) => (
              <li key={`${s.model}-${s.channel_name}`} className="flex justify-between gap-3">
                <span className="truncate">
                  {s.model} · {s.channel_name} · {m(`traffic.${s.traffic}`)}
                </span>
                <span className="shrink-0 tabular-nums text-warning">{m('needs', { markup: pct(s.breakeven) })}</span>
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}

MarkupRiskPreview.propTypes = { markup: PropTypes.number.isRequired, payment: PropTypes.number.isRequired };

export function MarkupCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const f = (k) => t(`markupSettings.${k}`);
  const validate = (i) => {
    const mk = Number.parseFloat(i.PlatformMarkupPercent || '0');
    const pc = Number.parseFloat(i.PaymentCostPercent || '0');
    if (!Number.isFinite(mk) || mk < 0 || mk > 1000) return f('invalidMarkup');
    if (!Number.isFinite(pc) || pc < 0 || pc >= 100) return f('invalidPayment');
    return null;
  };
  return (
    <OptionCard ctx={ctx} id="pricing-markup" options={{ validate }}>
      <div className="grid gap-5 sm:grid-cols-2">
        <TextRow
          id="PlatformMarkupPercent"
          type="number"
          unit="%"
          label={f('markup.label')}
          value={inputs.PlatformMarkupPercent}
          onChange={(v) => setField('PlatformMarkupPercent', v)}
          placeholder="12"
          description={f('markup.description')}
        />
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
      </div>
      <MarkupRiskPreview markup={num(inputs.PlatformMarkupPercent)} payment={num(inputs.PaymentCostPercent)} />
    </OptionCard>
  );
}

MarkupCard.propTypes = { ctx: PropTypes.object.isRequired };
