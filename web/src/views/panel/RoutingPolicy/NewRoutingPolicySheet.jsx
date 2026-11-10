import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Split } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { NewObjectSheet, Field, ObjectTile } from '@/components/objects';
import { showError } from 'utils/common';
import { EMPTY_POLICY, normalizePolicy, platformRoutingPolicyApi as api } from './routingPolicyApi';
import { ModelField, RoutesField, missingFields } from './policyFields';

const LABEL_KEY = {
  name: 'routingPolicy.form.name',
  decision_model: 'routingPolicy.form.decisionModel',
  routes: 'routingPolicy.form.routes',
  fallback_model: 'routingPolicy.form.fallbackModel'
};

// New routing policy: only what a policy cannot work without (model name, decision model,
// routes, fallback model). Scoring, threshold, pinning and limits start from the defaults and
// are changed in the policy's panel, which opens after the policy is created.
export default function NewRoutingPolicySheet({ modelOptions, onClose, onCreated }) {
  const { t } = useTranslation();
  const [form, setForm] = useState(EMPTY_POLICY);
  const [busy, setBusy] = useState(false);
  const normalized = useMemo(() => normalizePolicy(form), [form]);
  const missing = missingFields(normalized);
  const set = (key) => (value) => setForm((f) => ({ ...f, [key]: value }));
  const touched = JSON.stringify(form) !== JSON.stringify(EMPTY_POLICY);

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
          : t('routingPolicy.new.defaults', { threshold: '50%' })
      }
      onCreate={create}
      onClose={onClose}
    >
      <Field
        label={t('routingPolicy.form.name')}
        htmlFor="rp-new-name"
        hint={t('routingPolicy.form.nameHint')}
        help={t('routingPolicy.form.nameHelp')}
      >
        <Input
          id="rp-new-name"
          autoFocus
          className="font-mono text-[13px]"
          placeholder="auto/coder"
          value={form.name}
          onChange={(e) => set('name')(e.target.value)}
        />
      </Field>
      <ModelField
        id="rp-new-decision_model"
        label={t('routingPolicy.form.decisionModel')}
        hint={t('routingPolicy.form.decisionModelHint')}
        value={form.decision_model}
        onChange={set('decision_model')}
        options={modelOptions}
      />
      <RoutesField form={form} setForm={setForm} modelOptions={modelOptions} />
      <ModelField
        id="rp-new-fallback_model"
        label={t('routingPolicy.form.fallbackModel')}
        hint={t('routingPolicy.form.fallbackModelHint')}
        value={form.fallback_model}
        onChange={set('fallback_model')}
        options={modelOptions}
      />
    </NewObjectSheet>
  );
}

NewRoutingPolicySheet.propTypes = {
  modelOptions: PropTypes.array.isRequired,
  onClose: PropTypes.func.isRequired,
  onCreated: PropTypes.func.isRequired
};
