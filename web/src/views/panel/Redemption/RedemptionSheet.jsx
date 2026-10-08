import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { toast } from '@/components/ui/sonner';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { showError, trims, downloadTextAsFile } from 'utils/common';

const ORIGIN = { is_edit: false, name: '', quota: 100000, count: 1, expired_time: 0, scope: 'any' };
const SCOPES = ['any', 'personal', 'org'];

// unix(seconds) <-> <input type="datetime-local"> (local "YYYY-MM-DDTHH:mm")
const unixToInput = (ts) => {
  if (!ts || ts <= 0) return '';
  const d = new Date(ts * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
const inputToUnix = (s) => (s ? Math.floor(new Date(s).getTime() / 1000) : 0);

export default function RedemptionSheet({ open, onOpenChange, redemptionId, onSaved }) {
  const { t } = useTranslation();
  const [values, setValues] = useState(ORIGIN);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState({});

  const setField = (key, value) => setValues((prev) => ({ ...prev, [key]: value }));

  useEffect(() => {
    if (!open) return;
    setErrors({});
    if (!redemptionId) {
      setValues(ORIGIN);
      return;
    }
    const load = async () => {
      try {
        const res = await API.get(`/api/redemption/${redemptionId}`);
        const { success, message, data } = res.data;
        if (success) {
          setValues({ ...ORIGIN, ...data, scope: data.scope || 'any', is_edit: true });
        } else {
          showError(message);
        }
      } catch (error) {
        showError(error);
      }
    };
    load();
  }, [open, redemptionId]);

  const validate = () => {
    const e = {};
    if (!values.name?.trim()) e.name = t('validation.requiredName');
    if (Number(values.quota) < 0) e.quota = t('redemption_edit.requiredQuota');
    if (!values.is_edit && Number(values.count) < 1) e.count = t('redemption_edit.requiredCount');
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const submit = async () => {
    if (!validate()) return;
    setSaving(true);
    const payload = trims({ ...values });
    payload.quota = parseInt(payload.quota, 10) || 0;
    payload.count = parseInt(payload.count, 10) || 1;
    payload.expired_time = parseInt(payload.expired_time, 10) || 0;
    try {
      let res;
      if (values.is_edit) {
        res = await API.put('/api/redemption/', { ...payload, id: parseInt(redemptionId, 10) });
      } else {
        res = await API.post('/api/redemption/', payload);
      }
      const { success, message, data } = res.data;
      if (success) {
        if (values.is_edit) {
          toast.success(t('redemption_edit.editOk'));
        } else {
          toast.success(t('redemption_edit.addOk'));
          if (Array.isArray(data) && data.length > 1) {
            downloadTextAsFile(data.join('\n') + '\n', `${payload.name}.txt`);
          }
        }
        onSaved?.();
      } else {
        showError(message);
        setErrors((prev) => ({ ...prev, submit: message }));
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{redemptionId ? t('common.edit') : t('common.create')}</SheetTitle>
          <SheetDescription>{t('redemptionPage.pageTitle')}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="redemption-name" label={t('redemptionPage.headLabels.name')} required error={errors.name}>
            <Input id="redemption-name" value={values.name} onChange={(e) => setField('name', e.target.value)} />
          </FormField>

          <QuotaInput
            name="quota"
            label={t('redemptionPage.headLabels.quota')}
            value={values.quota}
            onChange={(v) => setField('quota', v)}
            error={Boolean(errors.quota)}
            helperText={errors.quota}
          />

          <FormField id="redemption-scope" label={t('redemptionPage.headLabels.scope')} hint={t('redemptionPage.scopeHint')}>
            <Select value={values.scope} onValueChange={(v) => setField('scope', v)}>
              <SelectTrigger id="redemption-scope">
                <span className="truncate">{t(`redemptionPage.scopes.${values.scope}`)}</span>
              </SelectTrigger>
              <SelectContent>
                {SCOPES.map((s) => (
                  <SelectItem key={s} value={s}>
                    {t(`redemptionPage.scopes.${s}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormField>

          {!values.is_edit && (
            <FormField id="redemption-count" label={t('redemption_edit.number')} required error={errors.count}>
              <Input id="redemption-count" type="number" value={values.count} onChange={(e) => setField('count', e.target.value)} />
            </FormField>
          )}

          <FormField id="redemption-expired-time" label={t('redemption_edit.expiredTime')} hint={t('redemption_edit.expiredTimeHint')}>
            <Input
              id="redemption-expired-time"
              type="datetime-local"
              value={unixToInput(values.expired_time)}
              onChange={(e) => setField('expired_time', inputToUnix(e.target.value))}
            />
          </FormField>
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
