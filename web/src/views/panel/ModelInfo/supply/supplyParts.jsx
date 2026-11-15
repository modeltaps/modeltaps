import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, Info } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useIsRoot } from 'utils/common';
import { fetchSupplySwitches } from './supplyApi';
import { normalizeModelStatus } from './supplyHelpers';
import { amount, priceSummary } from '../../Channel/channelPrices';
import { lossOf } from '../../Channel/channelModelDetails';

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

const STATE_TONES = { ok: 'ok', paused: 'warn', probing: 'warn', cooling: 'warn', disabled: 'off', channel_off: 'bad' };

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
  // Break after "/" and ":" first (anthropic/ | claude-sonnet-4.5), mid-word only if a part is too long.
  const parts = upstream.split(/(?<=[/:])/);
  return (
    <span className={cn('font-mono text-xs leading-snug [overflow-wrap:anywhere]', className)}>
      {parts.map((part, i) => (
        <span key={i}>
          {i > 0 && <wbr />}
          {part}
        </span>
      ))}
    </span>
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

// What the supply's channel charges upstream for the model, from the channel's stored price list:
// "$3 / $15" per million tokens, a per-image / per-second / per-request price, free, or a dash
// when the channel has no price list or the list has no entry. A price above the model's own sell
// price is marked, so a model sold for free on a paid upstream stands out.
export function UpstreamPriceCell({ supply }) {
  const { t } = useTranslation();
  const price = supply.upstream_price;
  const summary = priceSummary(price);
  if (!summary) {
    const tip = supply.upstream_priced ? t('channels.models.priceMissing') : t('modelSupply.noPriceList');
    return (
      <span className="text-muted-foreground" title={tip}>
        —
      </span>
    );
  }
  if (summary.kind === 'tokens') {
    const sell = [{ model: supply.model, enabled: true, input_price: supply.sell_input_price, output_price: supply.sell_output_price }];
    const losses = ['input', 'output']
      .map((key) => {
        const loss = lossOf(price, sell, key);
        return loss && t(`channels.models.loss_${key}`, { model: loss.model, sell: amount(loss.sell), cost: amount(loss.cost) });
      })
      .filter(Boolean);
    const text = (
      <span className="whitespace-nowrap">
        ${amount(price.input)}
        <span className="text-muted-foreground"> / </span>${amount(price.output)}
      </span>
    );
    if (losses.length === 0) return text;
    return (
      <TooltipProvider delayDuration={150}>
        <Tooltip>
          <TooltipTrigger asChild>
            <span data-testid="supply-price-loss" className="inline-flex items-center gap-1 text-warning" aria-label={losses.join(' ')}>
              <AlertTriangle className="size-3 shrink-0" />
              {text}
            </span>
          </TooltipTrigger>
          <TooltipContent className="max-w-64 space-y-1 whitespace-normal text-xs">
            {losses.map((line) => (
              <p key={line}>{line}</p>
            ))}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    );
  }
  if (summary.kind === 'other') {
    return (
      <span className="text-muted-foreground" title={t('channels.models.priceOtherHint')}>
        {t('channels.models.priceOther')}
      </span>
    );
  }
  if (summary.kind === 'free') return <span className="text-success">{t('channels.models.priceFree')}</span>;
  return <span className="whitespace-nowrap">{t(`channels.models.price_${summary.kind}`, { price: summary.value })}</span>;
}

UpstreamPriceCell.propTypes = { supply: PropTypes.object.isRequired };
