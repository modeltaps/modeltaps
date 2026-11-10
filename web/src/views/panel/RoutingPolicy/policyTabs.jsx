import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field, InfoRow, InfoRows, Section } from '@/components/objects';
import StatsPanel from './StatsPanel';
import TryItPanel from './TryItPanel';
import { normalizePolicy } from './routingPolicyApi';
import { ModelField, NumberField, QuestionField, RoutesField, ScoringField, missingFields } from './policyFields';

// Tabs of the routing policy panel (see components/objects/ObjectPanel). Overview and Try it
// are read-only; Routes and Settings each save on their own. A save sends the whole policy with
// the tab's fields replaced, which is what the update endpoint takes.

const pct = (v) => `${Math.round(Number(v || 0) * 100)}%`;

function OverviewBody({ object: policy }) {
  const { t } = useTranslation();
  return (
    <>
      <Section title={t('routingPolicy.panel.summary')}>
        <InfoRows>
          <InfoRow label={t('routingPolicy.form.decisionModel')} mono>
            {policy.decision_model}
          </InfoRow>
          <InfoRow label={t('routingPolicy.form.routes')}>
            <span className="flex flex-col items-end gap-0.5">
              {(policy.routes || []).map((r) => (
                <span key={r.label} className="font-mono text-[13px]">
                  {r.label} <span className="text-muted-foreground">→</span> {r.target_model}
                </span>
              ))}
            </span>
          </InfoRow>
          <InfoRow label={t('routingPolicy.form.fallbackModel')} mono>
            {policy.fallback_model}
          </InfoRow>
          <InfoRow label={t('routingPolicy.form.threshold')}>{pct(policy.threshold)}</InfoRow>
          <InfoRow label={t('routingPolicy.form.scoring')}>{t(`routingPolicy.scoring.${policy.scoring || 'auto'}`)}</InfoRow>
        </InfoRows>
      </Section>
      <Section title={t('routingPolicy.form.tabStats')}>
        <StatsPanel policyId={policy.id} fallbackModel={policy.fallback_model} />
      </Section>
    </>
  );
}

const overviewTab = { key: 'overview', Body: OverviewBody };

function RoutesBody({ draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const set = (key) => (value) => setDraft((d) => ({ ...d, [key]: value }));
  return (
    <>
      <ModelField
        id="rp-decision_model"
        label={t('routingPolicy.form.decisionModel')}
        hint={t('routingPolicy.form.decisionModelHint')}
        value={draft.decision_model}
        onChange={set('decision_model')}
        options={ctx.modelOptions}
      />
      <ScoringField value={draft.scoring} onChange={set('scoring')} decisionModel={(draft.decision_model || '').trim()} />
      <QuestionField value={draft.question} onChange={set('question')} />
      <RoutesField form={draft} setForm={setDraft} modelOptions={ctx.modelOptions} />
      <ModelField
        id="rp-fallback_model"
        label={t('routingPolicy.form.fallbackModel')}
        hint={t('routingPolicy.form.fallbackModelHint')}
        value={draft.fallback_model}
        onChange={set('fallback_model')}
        options={ctx.modelOptions}
      />
      <NumberField
        id="rp-threshold"
        label={t('routingPolicy.form.threshold')}
        hint={t('routingPolicy.form.thresholdHint')}
        help={t('routingPolicy.form.thresholdHelp')}
        min={0}
        max={1}
        step={0.05}
        value={draft.threshold}
        onChange={set('threshold')}
      />
    </>
  );
}

const ROUTE_KEYS = ['decision_model', 'scoring', 'question', 'routes', 'fallback_model', 'threshold'];

const routesTab = {
  key: 'routes',
  make: (p) => ({
    decision_model: p.decision_model || '',
    scoring: p.scoring || 'auto',
    question: p.question || '',
    routes: p.routes?.length ? p.routes : [],
    fallback_model: p.fallback_model || '',
    threshold: p.threshold ?? 0.5
  }),
  patch: (d, p, { t }) => {
    const next = normalizePolicy({ ...p, ...d });
    const [missing] = missingFields(next).filter((k) => k !== 'name');
    if (missing) return { error: t(`routingPolicy.form.missing.${missing}`) };
    return Object.fromEntries(ROUTE_KEYS.map((k) => [k, next[k]]));
  },
  Body: RoutesBody
};

function TryBody({ object: policy, ctx }) {
  return <TryItPanel policy={normalizePolicy(policy)} policyId={policy.id} modelOptions={ctx.modelOptions} />;
}

const tryTab = { key: 'try', Body: TryBody };

function SettingsBody({ draft, setDraft }) {
  const { t } = useTranslation();
  const set = (key) => (value) => setDraft((d) => ({ ...d, [key]: value }));
  return (
    <>
      <Field
        label={t('routingPolicy.form.name')}
        htmlFor="rp-name"
        hint={t('routingPolicy.form.nameHint')}
        help={t('routingPolicy.form.nameHelp')}
      >
        <Input id="rp-name" className="font-mono text-[13px]" value={draft.name} onChange={(e) => set('name')(e.target.value)} />
      </Field>
      <Field label={t('routingPolicy.form.displayName')} htmlFor="rp-display-name" hint={t('routingPolicy.form.displayNameHint')} optional>
        <Input id="rp-display-name" maxLength={100} value={draft.display_name} onChange={(e) => set('display_name')(e.target.value)} />
      </Field>
      <Field label={t('routingPolicy.form.description')} htmlFor="rp-description" optional>
        <Textarea id="rp-description" rows={2} value={draft.description} onChange={(e) => set('description')(e.target.value)} />
      </Field>
      <NumberField
        id="rp-pin_ttl_seconds"
        label={t('routingPolicy.form.pinTtl')}
        hint={t('routingPolicy.form.pinTtlHint')}
        help={t('routingPolicy.form.pinTtlHelp')}
        min={0}
        max={604800}
        step={60}
        value={draft.pin_ttl_seconds}
        onChange={set('pin_ttl_seconds')}
      />
      <NumberField
        id="rp-max_input_chars"
        label={t('routingPolicy.form.maxInputChars')}
        hint={t('routingPolicy.form.maxInputCharsHint')}
        min={0}
        max={200000}
        step={100}
        value={draft.max_input_chars}
        onChange={set('max_input_chars')}
      />
      <NumberField
        id="rp-decision_timeout_ms"
        label={t('routingPolicy.form.decisionTimeout')}
        hint={t('routingPolicy.form.decisionTimeoutHint')}
        min={0}
        max={60000}
        step={100}
        value={draft.decision_timeout_ms}
        onChange={set('decision_timeout_ms')}
      />
    </>
  );
}

const SETTING_KEYS = ['name', 'display_name', 'description', 'pin_ttl_seconds', 'max_input_chars', 'decision_timeout_ms'];

const settingsTab = {
  key: 'settings',
  make: (p) => ({
    name: p.name || '',
    display_name: p.display_name || '',
    description: p.description || '',
    pin_ttl_seconds: p.pin_ttl_seconds ?? 3600,
    max_input_chars: p.max_input_chars ?? 0,
    decision_timeout_ms: p.decision_timeout_ms ?? 0
  }),
  patch: (d, p, { t }) => {
    const next = normalizePolicy({ ...p, ...d });
    if (!next.name) return { error: t('routingPolicy.form.missing.name') };
    return Object.fromEntries(SETTING_KEYS.map((k) => [k, next[k]]));
  },
  Body: SettingsBody
};

export const POLICY_TABS = [overviewTab, routesTab, tryTab, settingsTab];
export const POLICY_TAB_KEYS = POLICY_TABS.map((x) => x.key);

const bodyProps = { object: PropTypes.object, draft: PropTypes.object, setDraft: PropTypes.func, ctx: PropTypes.object };
OverviewBody.propTypes = bodyProps;
RoutesBody.propTypes = bodyProps;
TryBody.propTypes = bodyProps;
SettingsBody.propTypes = bodyProps;
