import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, Info } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { copy, useIsRoot } from 'utils/common';
import { fetchSupplySwitches } from './supplyApi';
import { normalizeModelStatus } from './supplyHelpers';
import { amount, priceSummary } from '../../Channel/channelPrices';
import { lossOf } from '../../Channel/channelModelDetails';
import UpstreamPriceDetails from '../../Channel/UpstreamPriceDetails';

const pad = (n) => String(n).padStart(2, '0');

// "14:05" for today, "11-12 14:05" for another day.
export function formatUntil(until, now = new Date()) {
  const d = new Date(until * 1000);
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  return d.toDateString() === now.toDateString() ? time : `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${time}`;
}

// "30 minutes", "6 hours", "45 seconds".
export function durationLabel(t, seconds) {
  if (seconds >= 3600 && seconds % 3600 === 0) return t('modelSupply.hours', { count: seconds / 3600 });
  if (seconds >= 60) return t('modelSupply.minutes', { count: Math.round(seconds / 60) });
  return t('modelSupply.seconds', { count: seconds });
}

const DOT = { ok: 'bg-success', warn: 'bg-warning', off: 'bg-muted-foreground/45', bad: 'bg-destructive' };

export function StateDot({ tone = 'ok', className }) {
  return <span aria-hidden className={cn('inline-block size-1.5 shrink-0 rounded-full', DOT[tone], className)} />;
}

StateDot.propTypes = { tone: PropTypes.oneOf(Object.keys(DOT)), className: PropTypes.string };

const STATUS_TONES = { draft: 'off', published: 'ok', retired: 'warn' };

// Public status of a model: draft / published / retired.
export function ModelStatusPill({ status, className }) {
  const { t } = useTranslation();
  const value = normalizeModelStatus(status);
  return (
    <span className={cn('inline-flex items-center gap-1.5 whitespace-nowrap text-[13px]', className)}>
      <StateDot tone={STATUS_TONES[value]} />
      {t(`modelSupply.status.${value}`)}
    </span>
  );
}

ModelStatusPill.propTypes = { status: PropTypes.string, className: PropTypes.string };

const STATE_TONES = {
  ok: 'ok',
  paused: 'warn',
  probing: 'warn',
  cooling: 'warn',
  pending: 'warn',
  disabled: 'off',
  gone: 'bad',
  channel_off: 'bad'
};

// State of one supply (see supplyState and previewEntryState). Pauses and cool-downs show until
// when; pauses show the rule and upstream error on hover.
// The upstream model of a supply. Most supplies send the public name unchanged, so those read
// as a quiet "Same name"; a real mapping is shown in full and wraps instead of being cut off.
export function UpstreamName({ upstream, model, className }) {
  const { t } = useTranslation();
  if (!upstream || upstream === model) {
    return (
      <span className={cn('text-muted-foreground', className)} title={t('modelSupply.upstreamSameTip', { name: model })}>
        {t('modelSupply.upstreamSame')}
      </span>
    );
  }
  // A long name reads as a namespace and a model: the namespace ("accounts/fireworks/models/")
  // is muted on one line, cut short if it has to be; the model under it wraps to two lines at
  // most. The full name is in the hover title, and a click copies it.
  const cut = upstream.lastIndexOf('/') + 1;
  const prefix = upstream.slice(0, cut);
  const name = upstream.slice(cut);
  return (
    <button
      type="button"
      title={`${upstream}\n${t('modelSupply.upstreamCopyTip')}`}
      onClick={() => copy(upstream)}
      className={cn(
        'block w-full min-w-0 cursor-copy rounded-sm text-left font-mono text-xs leading-snug hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        className
      )}
    >
      {prefix && <span className="block truncate text-muted-foreground">{prefix}</span>}
      <span className="line-clamp-2 [overflow-wrap:anywhere]">{name}</span>
    </button>
  );
}

UpstreamName.propTypes = {
  upstream: PropTypes.string,
  model: PropTypes.string,
  className: PropTypes.string
};

export function SupplyStateLabel({ state }) {
  const { t } = useTranslation();
  const { kind, pause } = state;
  let text = t(`modelSupply.state.${kind}`);
  if (kind === 'paused') text = t('modelSupply.state.pausedUntil', { time: formatUntil(pause.until) });
  if (kind === 'cooling' && state.until) text = t('modelSupply.state.coolingUntil', { time: formatUntil(state.until) });
  const label = (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-[13px]">
      <StateDot tone={STATE_TONES[kind]} />
      {text}
      {pause && !pause.model && <span className="text-muted-foreground">· {t('modelSupply.state.wholeChannel')}</span>}
    </span>
  );
  if (kind === 'cooling') return <TooltipLabel label={label} lines={[t('modelSupply.state.coolingHint')]} />;
  if (kind === 'pending') return <TooltipLabel label={label} lines={[t('modelSupply.state.pendingHint')]} />;
  if (kind === 'gone') return <TooltipLabel label={label} lines={[t('modelSupply.state.goneHint')]} />;
  if (!pause) return label;
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-default">{label}</span>
        </TooltipTrigger>
        <TooltipContent className="max-w-xs space-y-1 text-xs">
          <p>{t('modelSupply.pause.rule', { rule: ruleLabel(t, pause.rule) })}</p>
          {pause.reason && <p className="break-words text-muted-foreground">{pause.reason}</p>}
          {pause.probe && <p className="text-muted-foreground">{t('modelSupply.pause.probeHint')}</p>}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

SupplyStateLabel.propTypes = { state: PropTypes.object.isRequired };

function TooltipLabel({ label, lines }) {
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-default">{label}</span>
        </TooltipTrigger>
        <TooltipContent className="max-w-xs space-y-1 text-xs">
          {lines.map((line) => (
            <p key={line}>{line}</p>
          ))}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

TooltipLabel.propTypes = { label: PropTypes.node.isRequired, lines: PropTypes.arrayOf(PropTypes.string).isRequired };

const KNOWN_RULES = ['quota', 'credential', 'model_not_found', 'upstream_failure', 'manual'];

// Built-in rule names are translated; custom rule names are shown as typed.
export function ruleLabel(t, rule) {
  return KNOWN_RULES.includes(rule) ? t(`modelSupply.rule.${rule}`) : rule || '—';
}

// The supply switches can only be read by root; others get null and no note is shown.
let switchesCache = null;
export function useSupplySwitches() {
  const isRoot = useIsRoot();
  const [switches, setSwitches] = useState(switchesCache);
  useEffect(() => {
    if (!isRoot) return undefined;
    let alive = true;
    fetchSupplySwitches().then((value) => {
      switchesCache = value;
      if (alive) setSwitches(value);
    });
    return () => {
      alive = false;
    };
  }, [isRoot]);
  return isRoot ? switches : null;
}

// Shown while supplies are configured but do not route yet: `active` comes from the preview
// (any admin), the switches from the options (root only).
export function SupplySwitchNote({ switches, active, className }) {
  const { t } = useTranslation();
  const off = active === undefined ? switches && !switches.supplyEnabled : !active;
  if (!off) return null;
  return (
    <p className={cn('flex items-start gap-2 rounded-[10px] bg-muted px-3 py-2 text-[13px] text-muted-foreground', className)}>
      <Info className="mt-0.5 size-3.5 shrink-0" />
      <span>{t('modelSupply.switchOff')}</span>
    </p>
  );
}

SupplySwitchNote.propTypes = { switches: PropTypes.object, active: PropTypes.bool, className: PropTypes.string };

// A cheaper id of the same model on the supply's channel (Requesty lists a model under its
// managed name and under provider-pinned ids at different prices), with a button that switches
// the supply to it. Nothing is switched on its own.
export function CheaperUpstreamHint({ supply, busy, onUse }) {
  const { t } = useTranslation();
  const cheaper = supply.cheaper_upstream;
  if (!cheaper?.model) return null;
  return (
    <p className="mt-0.5 flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground" data-testid="cheaper-upstream">
      <span>
        {t('modelSupply.cheaperUpstream', { model: cheaper.model, input: amount(cheaper.input), output: amount(cheaper.output) })}
      </span>
      <button
        type="button"
        disabled={busy}
        onClick={() => onUse(cheaper.model)}
        className="font-medium text-foreground underline-offset-2 hover:underline disabled:opacity-50"
      >
        {t('modelSupply.useCheaperUpstream')}
      </button>
    </p>
  );
}

CheaperUpstreamHint.propTypes = { supply: PropTypes.object.isRequired, busy: PropTypes.bool, onUse: PropTypes.func.isRequired };

// What the supply's channel charges upstream for the model, from the channel's stored price list:
// "$3 / $15" per million tokens, a per-image / per-second / per-request price, free, or a dash
// when the channel has no price list or the list has no entry. A price above the model's own sell
// price is marked, so a model sold for free on a paid upstream stands out. A click opens the
// price's details with a link to the upstream's page for the model.
export function UpstreamPriceCell({ supply }) {
  const { t } = useTranslation();
  const price = supply.upstream_price;
  const summary = priceSummary(price);
  if (!summary) {
    const tip = supply.upstream_priced ? t('channels.models.priceMissing') : t('modelSupply.noPriceList');
    // No upstream price, but the channel's cost ratio gives a cost: show it, marked when above the quote.
    const known = supply.cost?.known ? supply.cost : null;
    if (known && (known.input != null || known.flat != null)) {
      const above = known.above_quote;
      return (
        <span
          className={cn('inline-flex items-center gap-1 whitespace-nowrap', above && 'text-warning')}
          title={above ? t('modelSupply.aboveQuote') : t('modelSupply.costFromRatio')}
          data-testid="supply-cost-ratio"
        >
          {above && <AlertTriangle data-testid="supply-price-loss" className="size-3 shrink-0" />}
          {known.flat != null ? `$${amount(known.flat)}` : `$${amount(known.input)} / $${amount(known.output ?? 0)}`}
        </span>
      );
    }
    // A supply waiting for review says so: confirming it is accepting a cost nobody can check here.
    if (supply.pending && !supply.cost?.known) {
      return (
        <span className="whitespace-nowrap text-warning" title={tip} data-testid="supply-cost-unknown">
          {t('modelSupply.costUnknown')}
        </span>
      );
    }
    return (
      <span className="text-muted-foreground" title={tip}>
        —
      </span>
    );
  }
  const sell = { input: supply.sell_input_price ?? undefined, output: supply.sell_output_price ?? undefined };
  const sellers = [{ model: supply.model, enabled: true, input_price: supply.sell_input_price, output_price: supply.sell_output_price }];
  // The channel cost (the price with the channel's fee) decides whether the supply is above the
  // quote; the upstream price alone misses the fee.
  const cost = supply.cost?.known ? supply.cost : null;
  const losses = cost
    ? cost.above_quote
      ? [t(supply.traffic === 'backup' ? 'modelSupply.aboveQuoteBackup' : 'modelSupply.aboveQuote')]
      : []
    : summary.kind === 'tokens'
      ? ['input', 'output']
          .map((key) => {
            const loss = lossOf(price, sellers, key);
            return loss && t(`channels.models.loss_${key}`, { model: loss.model, sell: amount(loss.sell), cost: amount(loss.cost) });
          })
          .filter(Boolean)
      : [];
  let content;
  if (summary.kind === 'tokens') {
    content = (
      <span className={cn('inline-flex items-center gap-1 whitespace-nowrap', losses.length > 0 && 'text-warning')}>
        {losses.length > 0 && <AlertTriangle data-testid="supply-price-loss" className="size-3 shrink-0" />}
        <span>
          ${amount(price.input)}
          <span className={losses.length > 0 ? undefined : 'text-muted-foreground'}> / </span>${amount(price.output)}
        </span>
      </span>
    );
  } else if (summary.kind === 'other') {
    content = <span className="text-muted-foreground">{t('channels.models.priceOther')}</span>;
  } else if (summary.kind === 'free') {
    content = <span className="text-success">{t('channels.models.priceFree')}</span>;
  } else {
    content = <span className="whitespace-nowrap">{t(`channels.models.price_${summary.kind}`, { price: summary.value })}</span>;
  }
  return (
    <UpstreamPriceDetails price={price} upstream={supply.upstream_model || supply.model} sell={sell} cost={cost} losses={losses}>
      {content}
    </UpstreamPriceDetails>
  );
}

UpstreamPriceCell.propTypes = { supply: PropTypes.object.isRequired };
