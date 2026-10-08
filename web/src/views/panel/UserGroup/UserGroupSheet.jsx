import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { FormField } from '@/components/ui/form-field';
import { API } from 'utils/api';
import { showError, showSuccess, trims } from 'utils/common';

const ORIGIN = {
  is_edit: false,
  symbol: '',
  name: '',
  ratio: 1,
  public: false,
  api_rate: 300,
  promotion: false,
  min: 0,
  max: 0,
  enable: true
};

// ==============================|| USER GROUP — CREATE / EDIT SHEET ||============================== //
// Reuses v1 endpoints verbatim: POST/PUT /api/user_group/, GET /api/user_group/:id.
export default function UserGroupSheet({ open, groupId, onClose, onSaved }) {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState(ORIGIN);
  const [submitting, setSubmitting] = useState(false);
  const isEdit = Boolean(groupId);

  const set = (k, v) => setInputs((p) => ({ ...p, [k]: v }));

  useEffect(() => {
    if (!open) return;
    if (groupId) {
      (async () => {
        try {
          const res = await API.get(`/api/user_group/${groupId}`);
          const { success, message, data } = res.data;
          if (success) setInputs({ ...data, is_edit: true });
          else showError(message);
        } catch (e) {
          // surfaced globally
        }
      })();
    } else {
      setInputs(ORIGIN);
    }
  }, [open, groupId]);

  const submit = async () => {
    setSubmitting(true);
    try {
      const values = trims({ ...inputs });
      ['ratio', 'api_rate', 'min', 'max'].forEach((k) => {
        values[k] = Number(values[k]) || 0;
      });
      const res = isEdit
        ? await API.put('/api/user_group/', { ...values, id: parseInt(groupId) })
        : await API.post('/api/user_group/', values);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('userPage.saveSuccess'));
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (e) {
      // surfaced globally
    } finally {
      setSubmitting(false);
    }
  };

  const numField = (key, label, { hint, help } = {}) => (
    <FormField id={`g-${key}`} label={label} hint={hint} help={help}>
      <Input id={`g-${key}`} type="number" value={inputs[key]} onChange={(e) => set(key, e.target.value)} />
    </FormField>
  );

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose?.()}>
      <SheetContent onClose={onClose}>
        <SheetHeader>
          <SheetTitle>{isEdit ? t('common.edit') : t('common.create')}</SheetTitle>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="g-symbol" label={t('userGroup.symbol')} hint={t('userGroup.symbolTip')}>
            <Input id="g-symbol" value={inputs.symbol} disabled={inputs.is_edit} onChange={(e) => set('symbol', e.target.value)} />
          </FormField>
          <FormField id="g-name" label={t('userGroup.name')} hint={t('userGroup.nameTip')}>
            <Input id="g-name" value={inputs.name} onChange={(e) => set('name', e.target.value)} />
          </FormField>
          {numField('ratio', t('userGroup.ratio'))}
          {numField('api_rate', t('userGroup.apiRate'), { help: t('userGroup.apiRateTip') })}
          <div className="flex items-center justify-between">
            <div>
              <Label>{t('userGroup.promotion')}</Label>
              <p className="text-xs text-muted-foreground">{t('userGroup.promotionTip')}</p>
            </div>
            <Switch checked={!!inputs.promotion} onCheckedChange={(v) => set('promotion', v)} />
          </div>
          {numField('min', t('userGroup.min'), { hint: t('userGroup.minTip') })}
          {numField('max', t('userGroup.max'), { hint: t('userGroup.maxTip') })}
          <div className="flex items-center justify-between">
            <Label>{t('userGroup.public')}</Label>
            <Switch checked={!!inputs.public} onCheckedChange={(v) => set('public', v)} />
          </div>
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={onClose}>
            {t('userPage.cancel')}
          </Button>
          <Button disabled={submitting} onClick={submit}>
            {t('userPage.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
