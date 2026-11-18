import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field, InfoRow, InfoRows, Section } from '@/components/objects';
import StatsPanel from './StatsPanel';
import TryItPanel from './TryItPanel';
import { normalizePolicy } from './routingPolicyApi';
import { NumberField, ScoringField, isDecisionClassifier, missingFields } from './policyFields';
import { applyTemplate } from './templates';
import RoutingMap, { FallbackChain, TemplateMenu } from './RoutingMap';

// Tabs of the routing policy panel (see components/objects/ObjectPanel). Overview and Try it
// are read-only; Routes and Settings each save on their own. A save sends the whole policy with
// the tab's fields replaced, which is what the update endpoint takes.

const pct = (v) => `${Math.round(Number(v || 0) * 100)}%`;

function OverviewBody({ object: policy, ctx }) {
  const { t } = useTranslation();
  return (
    <>
      <Section title={t('routingPolicy.panel.summary')}>
        {policy.type === 'fallback' ? (
          <InfoRows>
            <InfoRow label={t('routingPolicy.type.label')}>{t('routingPolicy.type.fallback')}</InfoRow>
            <InfoRow label={t('routingPolicy.chain.models')}>
              <span className="flex flex-col items-end gap-0.5">
                {(policy.models || []).map((name, i) => (
                  <span key={name} className="font-mono text-[13px]">
                    <span className="text-muted-foreground">{i + 1}.</span> {name}
                  </span>
                ))}
              </span>
            </InfoRow>
          </InfoRows>
        ) : (
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
            <InfoRow label={t('routingPolicy.form.scoring')}>
              {policy.decision_model_kind === 'decision' || isDecisionClassifier(ctx.classifiers, policy.decision_model)
                ? t('routingPolicy.scoringUsed.decision')
                : t(`routingPolicy.scoring.${policy.scoring || 'auto'}`)}
            </InfoRow>
          </InfoRows>
        )}
      </Section>
      {!ctx.readOnly && (
        <Section title={t('routingPolicy.form.tabStats')}>
          <StatsPanel policyId={policy.id} fallbackModel={policy.fallback_model} api={ctx.api} />
        </Section>
      )}
    </>
  );
}

const overviewTab = { key: 'overview', Body: OverviewBody };

function RoutesBody({ draft, setDraft, ctx }) {
  const decisionModel = (draft.decision_model || '').trim();
  if (draft.type === 'fallback') {
    return (
      <div className="py-3">
        <FallbackChain form={draft} setForm={setDraft} modelOptions={ctx.modelOptions} prices={ctx.prices} />
      </div>
    );
  }
  return (
    <div className="space-y-4 py-3">
      <div className="flex justify-end">
        <TemplateMenu onApply={(template) => setDraft((d) => applyTemplate(d, template, ctx.suggest))} />
      </div>
      <RoutingMap
        form={draft}
        setForm={setDraft}
        modelOptions={ctx.modelOptions}
        classifiers={ctx.classifiers}
        prices={ctx.prices}
        decisionExtra={
          !isDecisionClassifier(ctx.classifiers, decisionModel) && (
            <div className="mt-3 [&>div]:py-0">
              <ScoringField
                value={draft.scoring}
                onChange={(v) => setDraft((d) => ({ ...d, scoring: v }))}
                decisionModel={decisionModel}
                api={ctx.api}
              />
            </div>
          )
        }
      />
    </div>
  );
}

const ROUTE_KEYS = ['decision_model', 'scoring', 'question', 'routes', 'fallback_model', 'threshold', 'models'];

const routesTab = {
  key: 'routes',
  make: (p) => ({
    decision_model: p.decision_model || '',
    scoring: p.scoring || 'auto',
    question: p.question || '',
    routes: p.routes?.length ? p.routes : [],
    fallback_model: p.fallback_model || '',
    threshold: p.threshold ?? 0.5,
    type: p.type || 'auto',
    models: p.models?.length ? p.models : []
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
  return <TryItPanel policy={normalizePolicy(policy)} policyId={policy.id} classifiers={ctx.classifiers} api={ctx.api} />;
}

const tryTab = { key: 'try', Body: TryBody };

function SettingsBody({ draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const set = (key) => (value) => setDraft((d) => ({ ...d, [key]: value }));
  return (
    <>
      <Field
        label={t('routingPolicy.form.name')}
        htmlFor="rp-name"
        hint={t(ctx.api?.callPrefix ? 'routingPolicy.form.nameHintOwn' : 'routingPolicy.form.nameHint')}
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
      {draft.type !== 'fallback' && (
        <>
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
      )}
    </>
  );
}

const SETTING_KEYS = ['name', 'display_name', 'description', 'pin_ttl_seconds', 'max_input_chars', 'decision_timeout_ms'];

const settingsTab = {
  key: 'settings',
  make: (p) => ({
    name: p.name || '',
    type: p.type || 'auto',
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
// Policies reviewed from the platform console and organization policies seen by members are read-only.
export const REVIEW_TAB_KEYS = ['overview'];

const bodyProps = { object: PropTypes.object, draft: PropTypes.object, setDraft: PropTypes.func, ctx: PropTypes.object };
OverviewBody.propTypes = bodyProps;
RoutesBody.propTypes = bodyProps;
TryBody.propTypes = bodyProps;
SettingsBody.propTypes = bodyProps;
