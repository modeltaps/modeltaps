import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { FilePlus2, Split } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { NewObjectSheet, Field, ObjectTile } from '@/components/objects';
import { showError } from 'utils/common';
import { EMPTY_POLICY, normalizePolicy } from './routingPolicyApi';
import { DecisionModelField, ModelField, QuestionField, RoutesField, missingFields } from './policyFields';
import { POLICY_TEMPLATES, applyTemplate, templateName } from './templates';
import TemplatePicker from './TemplatePicker';

const LABEL_KEY = {
  name: 'routingPolicy.form.name',
  decision_model: 'routingPolicy.form.decisionModel',
  routes: 'routingPolicy.form.routes',
  fallback_model: 'routingPolicy.form.fallbackModel'
};

// New routing policy. Step one picks a template, previewed with this instance's models, or a
// blank policy from the footer; step two shows what a policy
// cannot work without (model name, decision model, question, routes, fallback model), filled from
// the template. Scoring, threshold, pinning and limits start from the defaults and are changed in
// the policy's panel, which opens after the policy is created.
export default function NewRoutingPolicySheet({ api, modelOptions, classifiers, suggest, onClose, onCreated }) {
  const { t } = useTranslation();
  // undefined: still picking; null: blank policy; otherwise the template picked.
  const [template, setTemplate] = useState(undefined);
  const [selected, setSelected] = useState(POLICY_TEMPLATES[0].id);
  const [form, setForm] = useState(EMPTY_POLICY);
  const [busy, setBusy] = useState(false);
  const normalized = useMemo(() => normalizePolicy(form), [form]);
  const missing = missingFields(normalized);
  const set = (key) => (value) => setForm((f) => ({ ...f, [key]: value }));
  const touched = JSON.stringify(form) !== JSON.stringify(EMPTY_POLICY);
  const picking = template === undefined;

  const pick = (next) => {
    setTemplate(next);
    if (!next) {
      setForm(EMPTY_POLICY);
      return;
    }
    const base = {
      ...EMPTY_POLICY,
      name: templateName(next, api.callPrefix),
      display_name: t(`routingPolicy.templates.${next.id}.title`),
      description: t(`routingPolicy.templates.${next.id}.summary`)
    };
    setForm(applyTemplate(base, next, suggest || { models: modelOptions, classifiers }));
  };

  const create = async () => {
    if (picking) {
      pick(POLICY_TEMPLATES.find((x) => x.id === selected) || POLICY_TEMPLATES[0]);
      return;
    }
    if (missing.length) return;
    setBusy(true);
    try {
      const { success, message, data } = await api.create(normalized);
      if (success) onCreated(data);
      else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  return (
    <NewObjectSheet
      titleId="new-routing-policy-title"
      size="md"
      icon={
        <ObjectTile>
          <Split />
        </ObjectTile>
      }
      title={t('routingPolicy.createTitle')}
      subtitle={t('routingPolicy.new.subtitle')}
      touched={touched}
      ready={picking || missing.length === 0}
      createLabel={picking ? t('routingPolicy.new.useTemplate') : undefined}
      busy={busy}
      status={
        picking ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="-ml-2 h-8 rounded-lg px-2 text-[13px] font-normal text-muted-foreground hover:text-foreground"
            data-testid="rp-template-blank"
            onClick={() => pick(null)}
          >
            <FilePlus2 className="size-4" />
            {t('routingPolicy.new.startBlank')}
          </Button>
        ) : missing.length ? (
          t('objects.new.missing', { items: missing.map((k) => t(LABEL_KEY[k])).join(t('objects.new.listSep')) })
        ) : (
          t('routingPolicy.new.defaults', { threshold: '50%' })
        )
      }
      onCreate={create}
      onClose={onClose}
    >
      {picking ? (
        <TemplatePicker
          selected={selected}
          onSelect={setSelected}
          onPick={pick}
          suggest={suggest || { models: modelOptions, classifiers }}
          callPrefix={api.callPrefix}
        />
      ) : (
        <>
          <div className="flex items-center justify-between gap-2 pb-1 text-xs text-muted-foreground">
            <span data-testid="rp-template-current">
              {template
                ? t('routingPolicy.new.fromTemplate', { name: t(`routingPolicy.templates.${template.id}.title`) })
                : t('routingPolicy.new.blank')}
            </span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 rounded-lg px-2 text-xs font-normal"
              onClick={() => {
                if (template) setSelected(template.id);
                setTemplate(undefined);
              }}
            >
              {t('routingPolicy.new.changeTemplate')}
            </Button>
          </div>
          <Field
            label={t('routingPolicy.form.name')}
            htmlFor="rp-new-name"
            hint={t(api.callPrefix ? 'routingPolicy.form.nameHintOwn' : 'routingPolicy.form.nameHint')}
            help={t('routingPolicy.form.nameHelp')}
          >
            <Input
              id="rp-new-name"
              autoFocus
              className="font-mono text-[13px]"
              placeholder={api.callPrefix ? 'coder' : 'auto/coder'}
              value={form.name}
              onChange={(e) => set('name')(e.target.value)}
            />
          </Field>
          <DecisionModelField
            id="rp-new-decision_model"
            value={form.decision_model}
            onChange={set('decision_model')}
            classifiers={classifiers}
          />
          <QuestionField id="rp-new-question" value={form.question} onChange={set('question')} />
          <RoutesField form={form} setForm={setForm} modelOptions={modelOptions} showTemplates={false} />
          <ModelField
            id="rp-new-fallback_model"
            label={t('routingPolicy.form.fallbackModel')}
            hint={t('routingPolicy.form.fallbackModelHint')}
            value={form.fallback_model}
            onChange={set('fallback_model')}
            options={modelOptions}
          />
        </>
      )}
    </NewObjectSheet>
  );
}

NewRoutingPolicySheet.propTypes = {
  api: PropTypes.object.isRequired,
  modelOptions: PropTypes.array.isRequired,
  classifiers: PropTypes.shape({ decision: PropTypes.array, chat: PropTypes.array }),
  suggest: PropTypes.shape({ models: PropTypes.array, classifiers: PropTypes.object }),
  onClose: PropTypes.func.isRequired,
  onCreated: PropTypes.func.isRequired
};
