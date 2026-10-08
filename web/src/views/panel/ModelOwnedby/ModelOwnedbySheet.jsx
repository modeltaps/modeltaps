import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError, trims } from 'utils/common';
import BrandIconPicker from '@/components/brand/BrandIconPicker';
import { iconCacheHost } from '@/components/brand/brandIconManifest';

const ORIGIN = { is_edit: false, id: '', name: '', icon: '', slug: '' };

export default function ModelOwnedbySheet({ open, onOpenChange, ownedbyId, onSaved }) {
  const { t } = useTranslation();
  const [values, setValues] = useState(ORIGIN);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState({});

  const setField = (key, value) => setValues((prev) => ({ ...prev, [key]: value }));
  const iconDomain = iconCacheHost(values.slug) || iconCacheHost(values.name) || undefined;

  useEffect(() => {
    if (!open) return;
    setErrors({});
    if (!ownedbyId) {
      setValues(ORIGIN);
      return;
    }
    const load = async () => {
      try {
        const res = await API.get(`/api/model_ownedby/${ownedbyId}`);
        const { success, message, data } = res.data;
        if (success) {
          setValues({ ...ORIGIN, ...data, id: String(data.id ?? ''), is_edit: true });
        } else {
          showError(message);
        }
      } catch (error) {
        showError(error);
      }
    };
    load();
  }, [open, ownedbyId]);

  const validate = () => {
    const e = {};
    if (!String(values.id).trim()) e.id = t('modelOwnedby.idTip');
    if (!values.name?.trim()) e.name = t('validation.requiredName');
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const submit = async () => {
    if (!validate()) return;
    setSaving(true);
    const payload = trims({ ...values });
    try {
      let res;
      if (values.is_edit) {
        res = await API.put('/api/model_ownedby/', { ...payload, id: parseInt(ownedbyId, 10) });
      } else {
        res = await API.post('/api/model_ownedby/', { ...payload, id: parseInt(payload.id, 10) });
      }
      const { success, message } = res.data;
      if (success) {
        toast.success(t('userPage.saveSuccess'));
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
          <SheetTitle>{ownedbyId ? t('common.edit') : t('common.create')}</SheetTitle>
          <SheetDescription>{t('modelOwnedby.title')}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="ownedby-id" label={t('modelOwnedby.id')} required error={errors.id} hint={t('modelOwnedby.idTip')}>
            <Input
              id="ownedby-id"
              type="number"
              value={values.id}
              disabled={values.is_edit}
              onChange={(e) => setField('id', e.target.value)}
            />
          </FormField>

          <FormField id="ownedby-name" label={t('modelOwnedby.name')} required error={errors.name} hint={t('modelOwnedby.nameTip')}>
            <Input id="ownedby-name" value={values.name} onChange={(e) => setField('name', e.target.value)} />
          </FormField>

          <FormField id="ownedby-slug" label={t('modelOwnedby.slug')} error={errors.slug} hint={t('modelOwnedby.slugTip')}>
            <Input id="ownedby-slug" value={values.slug || ''} onChange={(e) => setField('slug', e.target.value.toLowerCase())} />
          </FormField>

          <FormField id="ownedby-icon" label={t('modelOwnedby.icon')} error={errors.icon} hint={t('modelOwnedby.iconTip')}>
            <BrandIconPicker
              id="ownedby-icon"
              value={values.icon || ''}
              onChange={(icon) => setField('icon', icon)}
              brandKey={values.slug}
              ownedBy={values.name}
              domain={iconDomain}
              refreshDomain={iconDomain}
              fallbackText={values.name}
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
