import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';

import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { Button } from '@/components/ui/button';
import { FormCard, OptionList } from '@/components/settings/cards';
import { ToggleRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / LOG IO CARD ||============================== //
// Personal request / response retention, in 数据与隐私 (ORG-10). Each API key has its own switch;
// this card holds the default a new personal key starts with. Changing it leaves existing keys
// alone; "apply to my keys" sets all of them to it once. When the site switch log_io_enabled is
// off the page shows a notice instead (see Privacy.jsx). The log detail dialog deep-links here
// with ?highlight=log-io.

const DEFAULT_MAX_BODY_KB = 1024;

export default function LogIOCard() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const maxBodyKb = Number(siteInfo?.log_io_max_body_kb) || DEFAULT_MAX_BODY_KB;
  const [stored, setStored] = useState(false);
  const [draft, setDraft] = useState(null);
  const [saving, setSaving] = useState(false);
  const [applying, setApplying] = useState(false);

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

  const put = async (body) => {
    const res = await API.put('/api/user/setting', body);
    if (!res.data?.success) {
      showError(res.data?.message);
      return null;
    }
    return res.data.data || {};
  };

  const save = async () => {
    setSaving(true);
    try {
      if (await put({ log_io_default: checked ? 'on' : 'off' })) {
        setStored(checked);
        setDraft(null);
        showSuccess(t('common.saveSuccess'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  // Sets every personal key to the saved default.
  const applyToKeys = async () => {
    if (!window.confirm(t(stored ? 'profilePage.logIOApplyConfirmOn' : 'profilePage.logIOApplyConfirmOff'))) return;
    setApplying(true);
    try {
      const data = await put({ log_io_default: stored ? 'on' : 'off', log_io_apply_to_keys: true });
      if (data) showSuccess(t('profilePage.logIOApplied', { count: data.log_io_keys_changed ?? 0 }));
    } catch (error) {
      showError(error);
    } finally {
      setApplying(false);
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
      <div className="flex flex-wrap items-center justify-between gap-2 pt-3">
        <p className="text-xs text-muted-foreground">{t('profilePage.logIOApplyHint')}</p>
        <Button type="button" variant="outline" size="sm" className="rounded-[10px]" disabled={dirty || applying} onClick={applyToKeys}>
          {t('profilePage.logIOApply')}
        </Button>
      </div>
    </FormCard>
  );
}
