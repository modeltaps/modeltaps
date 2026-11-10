import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Ticket } from 'lucide-react';

import { Input } from '@/components/ui/input';
import QuotaInput from '@/components/QuotaInput';
import { Field, NewObjectSheet, ObjectTile } from '@/components/objects';
import { API } from 'utils/api';
import { downloadTextAsFile, showError } from 'utils/common';

// New redemption codes: a name, the amount each code is worth and how many to generate. Codes
// can be redeemed by anyone and never expire until changed in a code's panel. More than one code
// downloads the keys as a text file.
export default function NewRedemptionSheet({ onClose, onCreated }) {
  const { t } = useTranslation();
  const [form, setForm] = useState({ name: '', quota: 100000, count: 1 });
  const [busy, setBusy] = useState(false);
  const name = form.name.trim();
  const quota = parseInt(form.quota, 10);
  const count = parseInt(form.count, 10);
  const missing = [!name && t('redemptionPage.headLabels.name')].filter(Boolean);
  let status;
  if (missing.length) status = t('objects.new.missing', { items: missing.join(t('objects.new.listSep')) });
  else if (!(quota >= 0)) status = <span className="text-destructive">{t('redemption_edit.requiredQuota')}</span>;
  else if (!(count >= 1)) status = <span className="text-destructive">{t('redemption_edit.requiredCount')}</span>;
  else status = count > 1 ? t('redemptionPage.new.defaultsMany', { count }) : t('redemptionPage.new.defaults');
  const ready = Boolean(name) && quota >= 0 && count >= 1;

  const create = async () => {
    setBusy(true);
    try {
      const res = await API.post('/api/redemption/', { name, quota, count, expired_time: 0, scope: 'any' });
      const { success, message, data } = res.data;
      if (success) {
        if (Array.isArray(data) && data.length > 1) downloadTextAsFile(data.join('\n') + '\n', `${name}.txt`);
        onCreated();
      } else showError(message);
    } catch (error) {
      showError(error);
    } finally {
      setBusy(false);
    }
  };

  return (
    <NewObjectSheet
      titleId="new-redemption-title"
      icon={
        <ObjectTile>
          <Ticket />
        </ObjectTile>
      }
      title={t('redemptionPage.createRedemptionCode')}
      subtitle={t('redemptionPage.new.subtitle')}
      touched={Boolean(form.name)}
      ready={ready}
      busy={busy}
      status={status}
      onCreate={create}
      onClose={onClose}
    >
      <Field label={t('redemptionPage.headLabels.name')} htmlFor="nr-name" hint={t('redemptionPage.new.nameHint')}>
        <Input id="nr-name" autoFocus value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
      </Field>
      <Field>
        <QuotaInput
          name="quota"
          label={t('redemptionPage.headLabels.quota')}
          value={form.quota}
          onChange={(v) => setForm((f) => ({ ...f, quota: v }))}
        />
      </Field>
      <Field label={t('redemption_edit.number')} htmlFor="nr-count">
        <Input
          id="nr-count"
          type="number"
          min={1}
          className="w-40 tabular-nums"
          value={form.count}
          onChange={(e) => setForm((f) => ({ ...f, count: e.target.value }))}
        />
      </Field>
    </NewObjectSheet>
  );
}

NewRedemptionSheet.propTypes = { onClose: PropTypes.func.isRequired, onCreated: PropTypes.func.isRequired };
