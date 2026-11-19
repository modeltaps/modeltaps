import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { FormCard, OptionList } from '@/components/settings/cards';
import { ToggleRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / LOG IO CARD ||============================== //
// Personal request / response retention, in 数据与隐私 (ORG-10): one switch for all the account's
// personal API keys, off until turned on. Keys have no switch of their own. When the site switch
// log_io_enabled is off the page shows a notice instead (see Privacy.jsx). The log detail dialog
// deep-links here with ?highlight=log-io.

const DEFAULT_MAX_BODY_KB = 1024;

export default function LogIOCard() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const maxBodyKb = Number(siteInfo?.log_io_max_body_kb) || DEFAULT_MAX_BODY_KB;
  const [stored, setStored] = useState(false);
  const [draft, setDraft] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    API.get('/api/user/setting')
      .then((res) => {
        const { success, data } = res.data || {};
        if (success && data) setStored(data.log_io_default === 'on');
      })
      .catch(() => {});
  }, []);

  const checked = draft ?? stored;
  const dirty = draft !== null && draft !== stored;

  const save = async () => {
    setSaving(true);
    try {
      const res = await API.put('/api/user/setting', { log_io_default: checked ? 'on' : 'off' });
      if (res.data?.success) {
        setStored(checked);
        setDraft(null);
        showSuccess(t('common.saveSuccess'));
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormCard
      title={t('profilePage.logIOSection')}
      description={t('settingsUi.personal.cards.logIO.description')}
      hint={t('settingsUi.personal.cards.logIO.hint')}
      highlight="log-io"
      dirty={dirty}
      saving={saving}
      onSave={save}
      onReset={() => setDraft(null)}
    >
      <OptionList>
        <ToggleRow
          id="log-io-default"
          label={t('profilePage.logIOToggle')}
          description={t('profilePage.logIOHint', { kb: maxBodyKb })}
          checked={checked}
          onCheckedChange={setDraft}
        />
      </OptionList>
    </FormCard>
  );
}
