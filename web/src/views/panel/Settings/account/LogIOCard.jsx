import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { FormCard, OptionList } from '@/components/settings/cards';
import { ToggleRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / LOG IO CARD ||============================== //
// Personal request / response retention, in 数据与隐私. The server keeps three states (unset / on /
// off); the switch shows the effective value (unset follows the site default log_io_default_user)
// and saving writes an explicit on / off. When the site switch log_io_enabled is off the page shows
// a notice instead (see Privacy.jsx). The log detail dialog deep-links here with ?highlight=log-io.

// The server may answer "inherit" | "on" | "off" or a *bool; normalise to the sentinel strings.
const triFromServer = (v) => (v === 'on' || v === true ? 'on' : v === 'off' || v === false ? 'off' : 'inherit');

const DEFAULT_MAX_BODY_KB = 1024;

export default function LogIOCard() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const siteDefault = Boolean(siteInfo?.log_io_default_user);
  const maxBodyKb = Number(siteInfo?.log_io_max_body_kb) || DEFAULT_MAX_BODY_KB;
  const [stored, setStored] = useState('inherit');
  const [draft, setDraft] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    API.get('/api/user/setting')
      .then((res) => {
        const { success, data } = res.data || {};
        if (success && data) setStored(triFromServer(data.log_io_default));
      })
      .catch(() => {});
  }, []);

  const effective = stored === 'inherit' ? siteDefault : stored === 'on';
  const checked = draft ?? effective;
  const dirty = draft !== null && draft !== effective;

  const save = async () => {
    const value = checked ? 'on' : 'off';
    setSaving(true);
    try {
      const res = await API.put('/api/user/setting', { log_io_default: value });
      if (res.data?.success) {
        setStored(value);
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
