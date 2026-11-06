import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import QuotaInput from 'components/QuotaInput';
import { API } from 'utils/api';
import { renderSpend, showError, showSuccess } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import useSelfUser from './useSelfUser';

// ==============================|| SETTINGS — ACCOUNT / QUOTA REMIND CARD ||============================== //
// Personal quota warning emails, on the Preferences page. The server keeps an optional personal
// threshold (null follows the site default) and an on/off switch (default on); the site switch
// always wins. The site values come back with /api/user/setting, so the card can show what applies.
// Only the personal balance is covered: organization tokens charge the organization's account.

export default function QuotaRemindCard() {
  const { t } = useTranslation();
  const { inputs: self } = useSelfUser();
  const [loaded, setLoaded] = useState(false);
  const [enabled, setEnabled] = useState(true);
  const [siteEnabled, setSiteEnabled] = useState(true);
  const [siteThreshold, setSiteThreshold] = useState(0);
  // savedThreshold: null while following the site default; draft: the input value ('' = default).
  const [savedThreshold, setSavedThreshold] = useState(null);
  const [draft, setDraft] = useState('');
  const [saving, setSaving] = useState(false);

  const apply = (data) => {
    setEnabled(data.quota_remind_enabled !== false);
    setSiteEnabled(data.site_quota_remind_enabled !== false);
    setSiteThreshold(Number(data.site_quota_remind_threshold) || 0);
    const personal = data.quota_remind_threshold ?? null;
    setSavedThreshold(personal);
    setDraft(personal ?? '');
  };

  useEffect(() => {
    const load = async () => {
      try {
        const res = await API.get('/api/user/setting');
        const { success, data } = res.data || {};
        if (success && data) apply(data);
      } catch {
        /* keep the defaults when loading fails */
      } finally {
        setLoaded(true);
      }
    };
    load();
  }, []);

  const save = async (partial) => {
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

  const draftValue = draft === '' ? null : Number(draft);
  const dirty = draftValue !== savedThreshold;
  const disabled = !loaded || saving || !siteEnabled;
  const hasEmail = Boolean(self?.email);

  let notice = '';
  if (loaded && !siteEnabled) notice = t('settingsPage.preferences.quotaRemind.siteDisabled');
  else if (loaded && self?.id && !hasEmail) notice = t('settingsPage.preferences.quotaRemind.noEmail');

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">{t('settingsPage.preferences.quotaRemind.title')}</CardTitle>
        <CardDescription>{t('settingsPage.preferences.quotaRemind.hint')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {notice && <p className="rounded-md border bg-muted/50 px-3 py-2 text-sm text-muted-foreground">{notice}</p>}
        <div className="flex items-center justify-between gap-4">
          <div className="space-y-0.5">
            <Label htmlFor="quota-remind-enabled" className="font-normal">
              {t('settingsPage.preferences.quotaRemind.toggle')}
            </Label>
            <p className="text-xs text-muted-foreground">{t('settingsPage.preferences.quotaRemind.toggleHint')}</p>
          </div>
          <Switch
            id="quota-remind-enabled"
            checked={siteEnabled && enabled}
            disabled={disabled}
            onCheckedChange={(next) => save({ quota_remind_enabled: next })}
          />
        </div>
        <div className="space-y-3">
          <QuotaInput
            id="quota-remind-threshold"
            label={t('settingsPage.preferences.quotaRemind.threshold')}
            value={draft}
            onChange={(v) => setDraft(v === '' ? '' : Math.max(0, Number(v) || 0))}
            allowEmpty
            disabled={disabled || !enabled}
            placeholder={t('settingsPage.preferences.quotaRemind.thresholdPlaceholder', { amount: renderSpend(siteThreshold) })}
            helperText={
              savedThreshold == null
                ? t('settingsPage.preferences.quotaRemind.followingSite', { amount: renderSpend(siteThreshold) })
                : t('settingsPage.preferences.quotaRemind.thresholdHint')
            }
            className="max-w-sm"
          />
          <div className="flex flex-wrap gap-2">
            <Button size="sm" disabled={disabled || !enabled || !dirty} onClick={() => save({ quota_remind_threshold: draftValue ?? -1 })}>
              {t('common.save')}
            </Button>
            {savedThreshold != null && (
              <Button size="sm" variant="outline" disabled={disabled || !enabled} onClick={() => save({ quota_remind_threshold: -1 })}>
                {t('settingsPage.preferences.quotaRemind.useSiteDefault')}
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
