import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { Field } from '@/components/objects';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { DEFAULT_SETTING, EXPIRY_PRESETS, expiryFromPreset, quotaPatch } from './tokenData';
import { SecretKeyReveal } from './KeySecret';

// Server answers that belong under the name field rather than in a toast.
const NAME_ERRORS = new Set(['token_name_taken', 'token_name_reserved']);

// New API key, in a dialog as most consoles do it: a name, and optionally a spending limit and an
// expiry. Groups, model and IP restrictions start open (the account's group, every model, any
// address) and are changed in the key's panel. After creating, the dialog shows the key with copy
// and the base URL, then closes or opens the key's panel. This is the only time the key is shown:
// only its hash is stored. `onCreated` receives { id, key } so the page can still build the
// test command and client setup for it while it stays open.
export default function NewTokenDialog({ onClose, onCreated, onOpen, baseUrl, onCopy }) {
  const { t } = useTranslation();
  const [form, setForm] = useState({ name: '', quota: '', expiry: 'never' });
  const [busy, setBusy] = useState(false);
  const [nameError, setNameError] = useState('');
  const [created, setCreated] = useState(null);
  const name = form.name.trim();

  const requestClose = () => {
    if (created || !form.name || window.confirm(t('objects.new.discardConfirm'))) onClose();
  };

  const create = async () => {
    if (!name || busy) return;
    const quota = quotaPatch({ quota: form.quota, period: '' }, t);
    if (quota.error) {
      showError(quota.error);
      return;
    }
    setBusy(true);
    try {
      const { unlimited_quota, remain_quota } = quota;
      const res = await API.post('/api/token/', {
        name,
        expired_time: expiryFromPreset(form.expiry),
        group: '',
        backup_group: '',
        unlimited_quota,
        remain_quota,
        setting: structuredClone(DEFAULT_SETTING)
      });
      const { success, message, data, code } = res.data;
      if (success) {
        const next = { id: data?.id, key: data?.key ? `sk-${data.key}` : '' };
        setCreated(next);
        onCreated(next);
      } else if (NAME_ERRORS.has(code)) setNameError(message);
      else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={(o) => !o && requestClose()}>
      <DialogContent className="max-w-md" aria-labelledby="new-token-title">
        {created ? (
          <>
            <DialogHeader className="border-b-0 pb-2">
              <DialogTitle id="new-token-title" className="text-base font-semibold">
                {t('apiKey.new.createdTitle', { name })}
              </DialogTitle>
              <DialogDescription>{t('apiKey.new.createdHint')}</DialogDescription>
            </DialogHeader>
            <div className="px-6 pb-6">
              <SecretKeyReveal secret={created.key} baseUrl={baseUrl} onCopy={onCopy} />
            </div>
            <DialogFooter>
              {created.id ? (
                <Button variant="ghost" className="rounded-lg text-muted-foreground" onClick={() => onOpen(created.id)}>
                  {t('apiKey.new.openDetails')}
                </Button>
              ) : null}
              <Button className="rounded-lg" onClick={onClose}>
                {t('apiKey.new.done')}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              create();
            }}
          >
            <DialogHeader className="border-b-0 pb-2">
              <DialogTitle id="new-token-title" className="text-base font-semibold">
                {t('token_index.createToken')}
              </DialogTitle>
              <DialogDescription>{t('apiKey.new.description')}</DialogDescription>
            </DialogHeader>
            <div className="px-6 pb-4">
              <Field label={t('token_index.name')} htmlFor="nt-name" error={nameError}>
                <Input
                  id="nt-name"
                  autoFocus
                  autoComplete="off"
                  maxLength={30}
                  placeholder={t('apiKey.new.namePlaceholder')}
                  value={form.name}
                  aria-invalid={Boolean(nameError)}
                  onChange={(e) => {
                    setNameError('');
                    setForm((f) => ({ ...f, name: e.target.value }));
                  }}
                />
              </Field>
              <div className="grid grid-cols-2 gap-3">
                <Field label={t('apiKey.new.limit')} optional className="[&+&]:border-t-0">
                  <QuotaInput
                    name="nt-quota"
                    value={form.quota}
                    placeholder={t('token_index.noLimit')}
                    allowEmpty
                    onChange={(v) => setForm((f) => ({ ...f, quota: v }))}
                  />
                </Field>
                <Field label={t('apiKey.new.expiry')} htmlFor="nt-expiry" className="[&+&]:border-t-0">
                  <Select value={form.expiry} onValueChange={(v) => setForm((f) => ({ ...f, expiry: v }))}>
                    <SelectTrigger id="nt-expiry">
                      <span className="truncate">{t(`apiKey.expiry.${form.expiry}`)}</span>
                    </SelectTrigger>
                    <SelectContent>
                      {EXPIRY_PRESETS.map((p) => (
                        <SelectItem key={p} value={p}>
                          {t(`apiKey.expiry.${p}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
              </div>
            </div>
            <DialogFooter className="items-center sm:justify-between">
              <span className="hidden text-xs text-muted-foreground sm:block">{t('apiKey.new.later')}</span>
              <div className="flex gap-2">
                <Button type="button" variant="outline" className="rounded-lg" onClick={requestClose}>
                  {t('common.cancel')}
                </Button>
                <Button type="submit" className="rounded-lg" disabled={!name || busy}>
                  {busy && <Loader2 className="animate-spin" />}
                  {t('apiKey.new.create')}
                </Button>
              </div>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

NewTokenDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  onCreated: PropTypes.func.isRequired,
  onOpen: PropTypes.func.isRequired,
  baseUrl: PropTypes.string,
  onCopy: PropTypes.func.isRequired
};
