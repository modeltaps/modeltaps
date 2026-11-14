import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Info } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useIsRoot } from 'utils/common';
import { fetchSupplySwitches } from './supplyApi';
import { normalizeModelStatus } from './supplyHelpers';

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
