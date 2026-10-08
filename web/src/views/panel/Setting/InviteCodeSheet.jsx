import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, RefreshCw } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError } from 'utils/common';

const tk = (k) => `setting_index.inviteCodeSettings.${k}`;

const ORIGIN = { name: '', code: '', max_uses: 0, count: 1, starts_at: 0, expires_at: 0, status: 1 };

// unix(seconds) <-> <input type="datetime-local"> (local "YYYY-MM-DDTHH:mm")
const unixToInput = (ts) => {
  if (!ts) return '';
  const d = new Date(ts * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
const inputToUnix = (s) => (s ? Math.floor(new Date(s).getTime() / 1000) : 0);

export default function InviteCodeSheet({ open, onOpenChange, editing, onSaved }) {
  const { t } = useTranslation();
  const [values, setValues] = useState(ORIGIN);
  const [saving, setSaving] = useState(false);
  const [generating, setGenerating] = useState(false);
  const isEdit = !!editing;

  const setField = (key, value) => setValues((prev) => ({ ...prev, [key]: value }));

  useEffect(() => {
    if (!open) return;
    if (editing) {
      setValues({
        name: editing.name || '',
        code: editing.code || '',
        max_uses: editing.max_uses ?? 0,
        count: 1,
        starts_at: editing.starts_at || 0,
        expires_at: editing.expires_at || 0,
        status: editing.status || 1
      });
    } else {
      setValues(ORIGIN);
    }
  }, [open, editing]);

  const generate = async () => {
    if (generating) return;
    setGenerating(true);
    try {
      const res = await API.get('/api/invite-code/generate');
      const { success, data, message } = res.data;
      if (success) setField('code', data.code);
      else showError(message || t(tk('generateFail')));
    } catch (error) {
      showError(error);
    } finally {
      setGenerating(false);
    }
  };

  const validate = () => {
    if (values.code && !/^[a-zA-Z0-9_-]+$/.test(values.code)) {
      showError(t(tk('errCodeChars')));
      return false;
    }
    if (values.code && (values.code.length < 3 || values.code.length > 32)) {
      showError(t(tk('errCodeLength')));
      return false;
    }
    if (values.starts_at && values.expires_at && values.starts_at >= values.expires_at) {
      showError(t(tk('errExpireBeforeStart')));
      return false;
    }
    return true;
  };

  const submit = async () => {
    if (saving || !validate()) return;
    setSaving(true);
    try {
      let res;
      if (isEdit) {
        res = await API.put(`/api/invite-code/${editing.id}`, {
          name: values.name,
          max_uses: parseInt(values.max_uses, 10) || 0,
          status: values.status,
          starts_at: values.starts_at,
          expires_at: values.expires_at
        });
      } else {
        res = await API.post('/api/invite-code/', {
          name: values.name,
          code: values.code,
          max_uses: parseInt(values.max_uses, 10) || 0,
          count: parseInt(values.count, 10) || 1,
          status: values.status,
          starts_at: values.starts_at,
          expires_at: values.expires_at
        });
      }
      const { success, message } = res.data;
      if (success) {
        toast.success(isEdit ? t(tk('editOk')) : t(tk('createOk')));
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const batch = !isEdit && Number(values.count) > 1;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{isEdit ? t('common.edit') : t('common.create')}</SheetTitle>
          <SheetDescription>{t(tk('title'))}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="invite-code-name" label={t(tk('form.name'))}>
            <Input id="invite-code-name" value={values.name} onChange={(e) => setField('name', e.target.value)} />
          </FormField>

          <FormField
            id="invite-code-code"
            label={t(tk('form.code'))}
            hint={isEdit ? t(tk('form.codeHintEdit')) : batch ? t(tk('form.codeHintBatch')) : t(tk('form.codeHint'))}
          >
            <div className="flex gap-2">
              <Input
                id="invite-code-code"
                value={values.code}
                placeholder={t(tk('form.codePlaceholder'))}
                disabled={isEdit || batch || generating}
                onChange={(e) => setField('code', e.target.value.replace(/[^a-zA-Z0-9_-]/g, ''))}
              />
              {!isEdit && (
                <Button type="button" variant="outline" disabled={batch || generating} onClick={generate}>
                  {generating ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
                  {t(tk('form.generate'))}
                </Button>
              )}
            </div>
          </FormField>

          <FormField id="invite-code-max-uses" label={t(tk('form.maxUses'))} hint={t(tk('form.maxUsesHint'))}>
            <Input
              id="invite-code-max-uses"
              type="number"
              min={0}
              value={values.max_uses}
              onChange={(e) => setField('max_uses', parseInt(e.target.value, 10) || 0)}
            />
          </FormField>

          {!isEdit && (
            <FormField id="invite-code-count" label={t(tk('form.count'))} hint={t(tk('form.countHint'))}>
              <Input
                id="invite-code-count"
                type="number"
                min={1}
                max={100}
                value={values.count}
                onChange={(e) => setField('count', e.target.value)}
              />
            </FormField>
          )}

          <FormField id="invite-code-starts-at" label={t(tk('form.startsAt'))} hint={t(tk('form.startsAtHint'))}>
            <Input
              id="invite-code-starts-at"
              type="datetime-local"
              value={unixToInput(values.starts_at)}
              onChange={(e) => setField('starts_at', inputToUnix(e.target.value))}
            />
          </FormField>

          <FormField id="invite-code-expires-at" label={t(tk('form.expiresAt'))} hint={t(tk('form.expiresAtHint'))}>
            <Input
              id="invite-code-expires-at"
              type="datetime-local"
              value={unixToInput(values.expires_at)}
              onChange={(e) => setField('expires_at', inputToUnix(e.target.value))}
            />
          </FormField>

          {isEdit && (
            <FormField id="invite-code-status" label={t(tk('form.status'))}>
              <Select value={String(values.status)} onValueChange={(v) => setField('status', parseInt(v, 10))}>
                <SelectTrigger id="invite-code-status">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">{t(tk('statusEnabled'))}</SelectItem>
                  <SelectItem value="2">{t(tk('statusDisabled'))}</SelectItem>
                </SelectContent>
              </Select>
            </FormField>
          )}
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
