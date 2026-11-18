import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Field } from '@/components/objects';
import { showError } from 'utils/common';
import { MIN_FALLBACK_MODELS, MIN_ROUTES } from './templates';
import { SCORING_MODES, platformRoutingPolicyApi } from './routingPolicyApi';

// Fields shared by the policy panel's tabs and the new policy sheet. Each takes the form (or
// tab draft) and a setter; `setForm` takes an updater for changes that touch several fields.

// What a policy cannot be saved without, in the order the fields appear.
export function missingFields(p) {
  const out = [];
  if (p.name !== undefined && !p.name) out.push('name');
  if (p.type === 'fallback') {
    if ((p.models || []).length < MIN_FALLBACK_MODELS) out.push('models');
    return out;
  }
  if (!p.decision_model) out.push('decision_model');
  const routes = p.routes || [];
  if (routes.length < MIN_ROUTES || routes.some((r) => !r.label || !r.target_model)) out.push('routes');
  if (!p.fallback_model) out.push('fallback_model');
  return out;
}

// classifierGroups: the decision model picker's suggestions, decision models first.
export function useClassifierGroups(classifiers) {
  const { t } = useTranslation();
  return [
    { label: t('routingPolicy.form.decisionModelGroupDecision'), options: classifiers?.decision || [] },
    { label: t('routingPolicy.form.decisionModelGroupChat'), options: classifiers?.chat || [] }
  ];
}

// isDecisionClassifier: the picked decision model is a decision model, asked on /v1/decisions
// (scoring does not apply to it).
export function isDecisionClassifier(classifiers, name) {
  return Boolean(name) && (classifiers?.decision || []).includes(name);
}

export function NumberField({ id, label, hint, help, value, onChange, min, max, step }) {
  return (
    <Field label={label} htmlFor={id} hint={hint} help={help}>
      <Input
        id={id}
        type="number"
        className="w-40 tabular-nums"
        min={min}
        max={max}
        step={step}
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value)}
      />
    </Field>
  );
}

NumberField.propTypes = {
  id: PropTypes.string.isRequired,
  label: PropTypes.node,
  hint: PropTypes.node,
  help: PropTypes.node,
  value: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  onChange: PropTypes.func.isRequired,
  min: PropTypes.number,
  max: PropTypes.number,
  step: PropTypes.number
};

// Scoring mode with a "check support" button: asks whether the decision model returns logprobs
// on the channel it is routed to. A result is only shown for the model it was checked with.
export function ScoringField({ value, onChange, decisionModel, api = platformRoutingPolicyApi }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState({ model: '', running: false, result: null });
  const runProbe = async () => {
    if (!decisionModel) return;
    setProbe({ model: decisionModel, running: true, result: null });
    try {
      const { success, message, data } = await api.probe(decisionModel);
      if (!success) showError(message);
      setProbe({ model: decisionModel, running: false, result: success ? data : null });
    } catch {
      setProbe({ model: decisionModel, running: false, result: null });
    }
  };
  const result = probe.model === decisionModel ? probe.result : null;
  const text = !result
    ? ''
    : result.supported === true
      ? t('routingPolicy.form.probeSupported')
      : result.supported === false
        ? t('routingPolicy.form.probeUnsupported')
        : t('routingPolicy.form.probeFailed', { error: result.error || '' });

  return (
    <Field
      label={t('routingPolicy.form.scoring')}
      htmlFor="rp-scoring"
      hint={t('routingPolicy.form.scoringHint')}
      help={t('routingPolicy.form.scoringHelp')}
    >
      <div className="flex flex-wrap items-center gap-2">
        <Select value={value || 'auto'} onValueChange={onChange}>
          <SelectTrigger id="rp-scoring" className="w-64">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SCORING_MODES.map((mode) => (
              <SelectItem key={mode} value={mode}>
                {t(`routingPolicy.scoring.${mode}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button type="button" variant="outline" className="rounded-lg" disabled={!decisionModel || probe.running} onClick={runProbe}>
          {probe.running ? t('routingPolicy.form.probing') : t('routingPolicy.form.probe')}
        </Button>
      </div>
      {text && (
        <p
          data-testid="rp-probe-result"
          className={result?.supported === true ? 'mt-1.5 text-xs text-foreground' : 'mt-1.5 text-xs text-muted-foreground'}
        >
          {text}
        </p>
      )}
    </Field>
  );
}

ScoringField.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  decisionModel: PropTypes.string,
  api: PropTypes.object
};
