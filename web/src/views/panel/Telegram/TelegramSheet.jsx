import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Textarea } from '@/components/ui/textarea';
import { API } from 'utils/api';
import { showError, showSuccess, trims } from 'utils/common';

const ORIGIN = {
  is_edit: false,
  command: '',
  description: '',
  parse_mode: 'MarkdownV2',
  reply_message: ''
};

const PARSE_MODES = ['MarkdownV2', 'Markdown', 'html'];

// ==============================|| TELEGRAM — CREATE / EDIT SHEET ||============================== //
// Reuses v1 endpoints verbatim: POST /api/option/telegram/ (create + edit), GET /api/option/telegram/:id.
export default function TelegramSheet({ open, menuId, onClose, onSaved }) {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState(ORIGIN);
  const [submitting, setSubmitting] = useState(false);
  const isEdit = Boolean(menuId);

  const set = (k, v) => setInputs((p) => ({ ...p, [k]: v }));

  useEffect(() => {
    if (!open) return;
    if (menuId) {
      (async () => {
        try {
          const res = await API.get(`/api/option/telegram/${menuId}`);
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
  }, [open, menuId]);

  const submit = async () => {
    const values = trims({ ...inputs });
    if (!values.command) return showError(t('telegram_edit.requiredCommand'));
    if (!values.description) return showError(t('telegram_edit.requiredDes'));
    if (!values.parse_mode) return showError(t('telegram_edit.requiredParseMode'));
    if (!values.reply_message) return showError(t('telegram_edit.requiredMes'));

    setSubmitting(true);
    try {
      const res = isEdit
        ? await API.post('/api/option/telegram/', { ...values, id: parseInt(menuId) })
        : await API.post('/api/option/telegram/', values);
      const { success, message } = res.data;
      if (success) {
        showSuccess(isEdit ? t('telegram_edit.updateOk') : t('telegram_edit.addOk'));
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

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose?.()}>
      <SheetContent onClose={onClose}>
        <SheetHeader>
          <SheetTitle>{isEdit ? t('common.edit') : t('common.create')}</SheetTitle>
        </SheetHeader>
        <SheetBody className="space-y-5">
          <FormField id="tg-command" label={t('telegramPage.command')} required>
            <Input id="tg-command" value={inputs.command} onChange={(e) => set('command', e.target.value)} />
          </FormField>
          <FormField id="tg-description" label={t('telegramPage.description')} required>
            <Input id="tg-description" value={inputs.description} onChange={(e) => set('description', e.target.value)} />
          </FormField>
          <FormField id="tg-parse_mode" label={t('telegram_edit.msgType')} required>
            <Select value={inputs.parse_mode} onValueChange={(v) => set('parse_mode', v)}>
              <SelectTrigger id="tg-parse_mode">
                <SelectValue placeholder={t('telegram_edit.msgType')} />
              </SelectTrigger>
              <SelectContent>
                {PARSE_MODES.map((m) => (
                  <SelectItem key={m} value={m}>
                    {m}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormField>
          <FormField id="tg-reply_message" label={t('telegram_edit.msgInfo')} required>
            <Textarea
              id="tg-reply_message"
              rows={5}
              value={inputs.reply_message}
              placeholder={t('telegram_edit.msgInfo')}
              onChange={(e) => set('reply_message', e.target.value)}
            />
          </FormField>
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button disabled={submitting} onClick={submit}>
            {t('common.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
