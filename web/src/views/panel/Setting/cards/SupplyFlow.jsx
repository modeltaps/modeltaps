import { Fragment, useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { Link, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ArrowDownUp, ArrowUpRight, ChevronRight, ListChecks, ListOrdered, PackagePlus, Pin, Route, Send, Server } from 'lucide-react';

import { cn } from '@/lib/utils';
import { settingEnumLabel } from '@/components/settings/catalog';
import { admin } from 'routes/paths';
import { durationLabel } from 'views/panel/ModelInfo/supply/supplyParts';
import { parseIntakeRules } from 'views/panel/ModelInfo/supply/IntakeRules';
import { fetchIntakeRules } from 'views/panel/ModelInfo/supply/supplyApi';

// ==============================|| PLATFORM SETTINGS — SUPPLY OVERVIEW ||============================== //
// The path of a request through the supply settings, drawn above the cards: each step names
// the setting that decides it and its current value (unsaved edits included), and opens that
// setting. While supply routing is off, the steps that only apply with it are dimmed.

const text = (t, key, values) => t(`settingsUi.platform.supplyFlow.${key}`, values);

function Step({ icon: Icon, label, value, muted = false, to, highlight, onPick }) {
  const body = (
    <>
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground">
        <Icon className="size-4" />
      </span>
      <span className="min-w-0 text-left">
        <span className="flex items-center gap-1 text-[13px] font-medium leading-5">
          {label}
          {to && <ArrowUpRight className="size-3.5 text-muted-foreground" />}
        </span>
        <span className="block truncate text-xs leading-4 text-muted-foreground">{value}</span>
      </span>
    </>
  );
  const className = cn(
    'flex min-w-0 items-center gap-2.5 rounded-xl px-2 py-1.5 transition-colors hover:bg-muted/60',
    muted && 'opacity-45'
  );
  if (to) {
    return (
      <Link to={to} className={className}>
        {body}
      </Link>
    );
  }
  if (highlight) {
    return (
      <button type="button" className={className} onClick={() => onPick(highlight)}>
        {body}
      </button>
    );
  }
  return <div className={className}>{body}</div>;
}

Step.propTypes = {
  icon: PropTypes.elementType.isRequired,
  label: PropTypes.node.isRequired,
  value: PropTypes.node,
  muted: PropTypes.bool,
  to: PropTypes.string,
  highlight: PropTypes.string,
  onPick: PropTypes.func
};

const Arrow = () => <ChevronRight aria-hidden className="size-4 shrink-0 text-muted-foreground/60" />;

export default function SupplyFlow({ ctx }) {
  const { t } = useTranslation();
  const [, setSearchParams] = useSearchParams();
  const { inputs } = ctx;
  const routing = inputs.ModelSupplyEnabled === 'true';
  const affinity = inputs.SessionAffinityEnabled === 'true';
  const [defaultRules, setDefaultRules] = useState(null);
  useEffect(() => {
    let live = true;
    fetchIntakeRules()
      .then((res) => live && res.success && setDefaultRules(res.data?.defaults || []))
      .catch(() => {});
    return () => {
      live = false;
    };
  }, []);
  const rules = parseIntakeRules(inputs.SupplyIntakeRules) || defaultRules;
  const pick = (highlight) => setSearchParams({ highlight }, { replace: true });
  const costOrder = inputs.SupplyCostOrder === 'cost' ? 'cost' : 'priority';
  const ttl = Number(inputs.SessionAffinityTTLSeconds) || 0;

  const steps = [
    { key: 'request', icon: Send, label: text(t, 'request'), value: text(t, 'requestValue') },
    {
      key: 'routing',
      icon: Route,
      label: text(t, 'routing'),
      value: routing ? text(t, 'on') : text(t, 'routingOff'),
      highlight: 'ModelSupplyEnabled'
    },
    { key: 'order', icon: ListOrdered, label: text(t, 'order'), value: text(t, 'orderValue'), to: admin.providers(), muted: !routing },
    {
      key: 'orderBy',
      icon: ArrowDownUp,
      label: text(t, 'orderBy'),
      value: settingEnumLabel(t, 'SupplyCostOrder', costOrder),
      highlight: 'SupplyCostOrder',
      muted: !routing
    },
    {
      key: 'affinity',
      icon: Pin,
      label: text(t, 'affinity'),
      value: affinity ? (ttl > 0 ? text(t, 'affinityValue', { time: durationLabel(t, ttl) }) : text(t, 'on')) : text(t, 'off'),
      highlight: 'session-affinity'
    },
    { key: 'channel', icon: Server, label: text(t, 'channel'), value: text(t, 'channelValue') }
  ];

  return (
    <section aria-label={text(t, 'label')} className="space-y-2 rounded-2xl border border-border bg-card px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-1 gap-y-2">
        {steps.map((step, i) => (
          <Fragment key={step.key}>
            {i > 0 && <Arrow />}
            <Step {...step} onPick={pick} />
          </Fragment>
        ))}
      </div>
      {/* New upstream models join the call order through the intake rules. */}
      <div className={cn('flex flex-wrap items-center gap-x-1 gap-y-2 border-t border-border pt-2', !routing && 'opacity-45')}>
        <Step icon={PackagePlus} label={text(t, 'newModel')} value={text(t, 'newModelValue')} />
        <Arrow />
        <Step
          icon={ListChecks}
          label={text(t, 'intake')}
          value={rules ? text(t, 'intakeValue', { count: rules.length }) : '…'}
          highlight="supply-intake-rules"
          onPick={pick}
        />
        <Arrow />
        <span className="px-2 text-xs text-muted-foreground">{text(t, 'joinsOrder')}</span>
      </div>
    </section>
  );
}

SupplyFlow.propTypes = { ctx: PropTypes.object.isRequired };
