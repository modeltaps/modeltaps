import { useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Split } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { NewObjectSheet, Field, ObjectTile } from '@/components/objects';
import { showError } from 'utils/common';
import { EMPTY_POLICY, normalizePolicy } from './routingPolicyApi';
import { missingFields } from './policyFields';
import { applyTemplate, suggestFallbackChain, templateName } from './templates';
import RoutingMap, { FallbackChain, PolicyTypeSwitch, TemplateBar } from './RoutingMap';

const LABEL_KEY = {
  name: 'routingPolicy.form.name',
  decision_model: 'routingPolicy.form.decisionModel',
  routes: 'routingPolicy.form.routes',
  models: 'routingPolicy.chain.models',
  fallback_model: 'routingPolicy.form.fallbackModel'
};

// New routing policy, edited as the routing map it describes. It opens blank; a template from the
// bar above fills the whole map (name, decision model, question, routes, targets, fallback) from
// this instance's models, and everything stays editable. Scoring, pinning and limits start from
// the defaults and are changed in the policy's panel, which opens after the policy is created.
export default function NewRoutingPolicySheet({ api, modelOptions, classifiers, suggest, prices, onClose, onCreated }) {
  const { t } = useTranslation();
  const [form, setForm] = useState(EMPTY_POLICY);
  const [templateId, setTemplateId] = useState('');
  // What the last template filled in, to tell the user's own edits from it.
  const applied = useRef(EMPTY_POLICY);
  const [busy, setBusy] = useState(false);
  const normalized = useMemo(() => normalizePolicy(form), [form]);
  const missing = missingFields(normalized);
  const touched = JSON.stringify(form) !== JSON.stringify(EMPTY_POLICY);

  const apply = (template) => {
    const edited = JSON.stringify(form) !== JSON.stringify(applied.current);
    if (edited && touched && !window.confirm(t('routingPolicy.map.replaceConfirm'))) return;
    const prev = applied.current;
    // Name, display name and description are kept when the user typed their own.
    const own = (key) => form[key] && form[key] !== prev[key];
    const base = {
      ...EMPTY_POLICY,
      threshold: form.threshold,
      name: own('name') ? form.name : templateName(template, api.callPrefix),
      display_name: own('display_name') ? form.display_name : t(`routingPolicy.templates.${template.id}.title`),
      description: own('description') ? form.description : t(`routingPolicy.templates.${template.id}.summary`)
    };
    const next = applyTemplate(base, template, suggest || { models: modelOptions, classifiers });
    applied.current = next;
    setTemplateId(template.id);
    setForm(next);
  };

  const fallback = form.type === 'fallback';
  // A new fallback policy starts from the strongest model with the balanced one behind it.
  const setType = (type) =>
    setForm((f) => {
      if (type !== 'fallback' || f.models?.some(Boolean)) return { ...f, type };
      const chain = suggestFallbackChain((suggest || { models: modelOptions }).models);
      return { ...f, type, models: chain.length >= 2 ? chain : [...chain, '', ''].slice(0, 2) };
    });

  const create = async () => {
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
      ready={missing.length === 0}
      busy={busy}
      status={
        missing.length
          ? t('objects.new.missing', { items: missing.map((k) => t(LABEL_KEY[k])).join(t('objects.new.listSep')) })
          : t(fallback ? 'routingPolicy.new.defaultsFallback' : 'routingPolicy.new.defaults')
      }
      onCreate={create}
      onClose={onClose}
    >
      <div className="space-y-6 pb-2">
        <PolicyTypeSwitch value={form.type} onChange={setType} />
        {!fallback && <TemplateBar active={templateId} onApply={apply} />}
        <Field
          label={t('routingPolicy.form.name')}
          htmlFor="rp-new-name"
          hint={t(api.callPrefix ? 'routingPolicy.form.nameHintOwn' : 'routingPolicy.form.nameHint')}
          help={t('routingPolicy.form.nameHelp')}
          className="py-0"
        >
          <Input
            id="rp-new-name"
            autoFocus
            className="font-mono text-[13px]"
            placeholder={fallback ? (api.callPrefix ? 'steady' : 'auto/steady') : api.callPrefix ? 'coder' : 'auto/coder'}
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          />
        </Field>
        {fallback ? (
          <FallbackChain idPrefix="rp-new" form={form} setForm={setForm} modelOptions={modelOptions} prices={prices} />
        ) : (
          <RoutingMap
            idPrefix="rp-new"
            form={form}
            setForm={setForm}
            modelOptions={modelOptions}
            classifiers={classifiers}
            prices={prices}
          />
        )}
      </div>
    </NewObjectSheet>
  );
}

NewRoutingPolicySheet.propTypes = {
  api: PropTypes.object.isRequired,
  modelOptions: PropTypes.array.isRequired,
  classifiers: PropTypes.shape({ decision: PropTypes.array, chat: PropTypes.array }),
  suggest: PropTypes.shape({ models: PropTypes.array, classifiers: PropTypes.object }),
  prices: PropTypes.object,
  onClose: PropTypes.func.isRequired,
  onCreated: PropTypes.func.isRequired
};
