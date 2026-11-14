import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import QuotaInput from 'components/QuotaInput';
import { API } from 'utils/api';
import { renderSpend, showError, showSuccess } from 'utils/common';
import { FormCard } from '@/components/settings/cards';
import { ToggleRow } from '../../Setting/parts';

// ==============================|| SETTINGS — ACCOUNT / QUOTA REMIND CARD ||============================== //
// Personal quota warning emails, in 偏好. The server keeps an optional personal threshold (null
// follows the site default) and an on/off switch (default on); the site switch always wins. The
// site values come back with /api/user/setting, so the card can show what applies. The switch and
// the threshold save together from the footer; an empty threshold means "follow the site default"
// (sent as -1). Only the personal balance is covered: organization tokens charge the organization.

export default function QuotaRemindCard({ self }) {
  const { t } = useTranslation();
  const qr = (k, opts) => t(`settingsPage.preferences.quotaRemind.${k}`, opts);
  const [loaded, setLoaded] = useState(false);
  const [saved, setSaved] = useState({ enabled: true, threshold: null });
  const [site, setSite] = useState({ enabled: true, threshold: 0 });
  // draft.threshold: '' while following the site default, otherwise the amount.
  const [draft, setDraft] = useState({ enabled: true, threshold: '' });
  const [saving, setSaving] = useState(false);

  const apply = (data) => {
    const next = { enabled: data.quota_remind_enabled !== false, threshold: data.quota_remind_threshold ?? null };
    setSaved(next);
    setSite({ enabled: data.site_quota_remind_enabled !== false, threshold: Number(data.site_quota_remind_threshold) || 0 });
    setDraft({ enabled: next.enabled, threshold: next.threshold ?? '' });
  };

  useEffect(() => {
    API.get('/api/user/setting')
      .then((res) => {
        const { success, data } = res.data || {};
        if (success && data) apply(data);
      })
      .catch(() => {})
      .finally(() => setLoaded(true));
  }, []);

  const draftThreshold = draft.threshold === '' ? null : Number(draft.threshold);
  const enabledChanged = draft.enabled !== saved.enabled;
  const thresholdChanged = draftThreshold !== saved.threshold;
  const dirty = loaded && (enabledChanged || thresholdChanged);
  const locked = !loaded || !site.enabled;

  const save = async () => {
    const partial = {};
    if (enabledChanged) partial.quota_remind_enabled = draft.enabled;
    if (thresholdChanged) partial.quota_remind_threshold = draftThreshold ?? -1;
    setSaving(true);
    try {
      const res = await API.put('/api/user/setting', partial);
      const { success, message, data } = res.data || {};
      if (success && data) {
        apply(data);
        showSuccess(t('common.saveSuccess'));
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  let notice = '';
  if (loaded && !site.enabled) notice = qr('siteDisabled');
  else if (loaded && self?.inputs?.id && !self.inputs.email) notice = qr('noEmail');

  return (
    <FormCard
      title={qr('title')}
      description={qr('hint')}
      hint={t('settingsUi.personal.cards.quotaRemind.hint')}
      highlight="quota-remind"
      dirty={dirty}
      saving={saving}
      disabled={locked}
      onSave={save}
      onReset={() => setDraft({ enabled: saved.enabled, threshold: saved.threshold ?? '' })}
    >
      {notice && <p className="rounded-lg bg-muted/60 px-3 py-2 text-sm text-muted-foreground">{notice}</p>}
      {/* One option: no inner frame, the row sits flush in the card. */}
      <div className="[&>div]:px-0 [&>div]:py-0">
        <ToggleRow
          id="quota-remind-enabled"
          label={qr('toggle')}
          description={qr('toggleHint')}
          checked={site.enabled && draft.enabled}
          disabled={locked}
          onCheckedChange={(v) => setDraft((d) => ({ ...d, enabled: v }))}
        >
          <QuotaInput
            id="quota-remind-threshold"
            label={qr('threshold')}
            value={draft.threshold}
            onChange={(v) => setDraft((d) => ({ ...d, threshold: v === '' ? '' : Math.max(0, Number(v) || 0) }))}
            allowEmpty
            disabled={locked || !draft.enabled}
            placeholder={qr('thresholdPlaceholder', { amount: renderSpend(site.threshold) })}
            helperText={draftThreshold == null ? qr('followingSite', { amount: renderSpend(site.threshold) }) : qr('thresholdHint')}
            className="max-w-sm"
          />
          {draft.threshold !== '' && !locked && draft.enabled && (
            <button
              type="button"
              className="text-xs text-primary hover:underline"
              onClick={() => setDraft((d) => ({ ...d, threshold: '' }))}
            >
              {qr('useSiteDefault')}
            </button>
          )}
        </ToggleRow>
      </div>
    </FormCard>
  );
}

QuotaRemindCard.propTypes = {
  self: PropTypes.shape({ inputs: PropTypes.object })
};
