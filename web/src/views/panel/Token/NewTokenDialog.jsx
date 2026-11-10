import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Copy, Loader2 } from 'lucide-react';

import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { Field } from '@/components/objects';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { DEFAULT_SETTING, EXPIRY_PRESETS, expiryFromPreset, quotaPatch } from './tokenData';

// New API key, in a dialog as most consoles do it: a name, and optionally a spending limit and an
// expiry. Groups, model and IP restrictions start open (the account's group, every model, any
// address) and are changed in the key's panel. After creating, the dialog shows the key with copy
// and the base URL, then closes or opens the key's panel.
export default function NewTokenDialog({ onClose, onCreated, onOpen, baseUrl, onCopy }) {
  const { t } = useTranslation();
  const [form, setForm] = useState({ name: '', quota: '', days: 0 });
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState(null);
  const [copied, setCopied] = useState(false);
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
        expired_time: expiryFromPreset(form.days),
        group: '',
        backup_group: '',
        unlimited_quota,
        remain_quota,
        setting: structuredClone(DEFAULT_SETTING)
      });
      const { success, message, data } = res.data;
      if (success) {
        setCreated({ id: data?.id, key: data?.key ? `sk-${data.key}` : '' });
        onCreated();
      } else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  const copyKey = () => {
    onCopy(created.key, t('token_index.token'));
    setCopied(true);
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
            <div className="space-y-3 px-6 pb-6">
              <div className="flex h-10 items-center gap-1 rounded-lg border border-input bg-background pl-3 pr-1">
                <code className="min-w-0 flex-1 truncate font-mono text-[13px]" data-testid="new-token-key">
                  {created.key}
                </code>
                <Button size="sm" variant={copied ? 'outline' : 'default'} className="h-8 rounded-md" onClick={copyKey}>
                  {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                  {copied ? t('apiKey.new.copied') : t('token_index.copy')}
                </Button>
              </div>
              <button
                type="button"
                onClick={() => onCopy(baseUrl, t('token_index.apiAddress'))}
                className="flex w-full items-center justify-between gap-3 rounded-lg px-1 text-left text-xs text-muted-foreground hover:text-foreground"
              >
                <span className="shrink-0">{t('token_index.apiAddress')}</span>
                <span className="inline-flex min-w-0 items-center gap-1.5 font-mono">
                  <span className="truncate">{baseUrl}</span>
                  <Copy className="size-3 shrink-0" />
                </span>
              </button>
            </div>
            <DialogFooter>
              {created.id ? (
                <Button variant="outline" className="rounded-lg" onClick={() => onOpen(created.id)}>
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
              <Field label={t('token_index.name')} htmlFor="nt-name">
                <Input
                  id="nt-name"
                  autoFocus
                  autoComplete="off"
                  maxLength={30}
                  placeholder={t('apiKey.new.namePlaceholder')}
                  value={form.name}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
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
                  <Select value={String(form.days)} onValueChange={(v) => setForm((f) => ({ ...f, days: Number(v) }))}>
                    <SelectTrigger id="nt-expiry">
                      <span className="truncate">
                        {form.days ? t('apiKey.new.days', { count: form.days }) : t('token_index.neverExpires')}
                      </span>
                    </SelectTrigger>
                    <SelectContent>
                      {EXPIRY_PRESETS.map((d) => (
                        <SelectItem key={d} value={String(d)}>
                          {d ? t('apiKey.new.days', { count: d }) : t('token_index.neverExpires')}
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
