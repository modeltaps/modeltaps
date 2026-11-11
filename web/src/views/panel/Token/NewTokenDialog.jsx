import { useEffect, useState } from 'react';
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

// A read-only value with an icon copy button inside its box: the whole value is shown (wrapped,
// never cut off) and a click on it selects all of it. The icon turns into a check for a moment.
function CopyField({ label, value, onCopy, testId }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return undefined;
    const id = setTimeout(() => setCopied(false), 1500);
    return () => clearTimeout(id);
  }, [copied]);
  return (
    <div>
      <p className="mb-1.5 text-sm font-medium">{label}</p>
      <div className="flex items-start gap-1 rounded-lg border border-input bg-muted/40 py-1 pl-3 pr-1">
        <code className="min-w-0 flex-1 select-all break-all py-1.5 font-mono text-[13px] leading-5" data-testid={testId}>
          {value}
        </code>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-8 shrink-0 rounded-md text-muted-foreground hover:text-foreground"
          aria-label={copied ? t('apiKey.new.copied') : t('token_index.copy')}
          title={copied ? t('apiKey.new.copied') : t('token_index.copy')}
          onClick={() => {
            onCopy();
            setCopied(true);
          }}
        >
          {copied ? <Check className="size-4 text-success" /> : <Copy className="size-4" />}
        </Button>
      </div>
    </div>
  );
}

CopyField.propTypes = { label: PropTypes.node, value: PropTypes.string, onCopy: PropTypes.func.isRequired, testId: PropTypes.string };

// New API key, in a dialog as most consoles do it: a name, and optionally a spending limit and an
// expiry. Groups, model and IP restrictions start open (the account's group, every model, any
// address) and are changed in the key's panel. After creating, the dialog shows the key with copy
// and the base URL, then closes or opens the key's panel.
export default function NewTokenDialog({ onClose, onCreated, onOpen, baseUrl, onCopy }) {
  const { t } = useTranslation();
  const [form, setForm] = useState({ name: '', quota: '', days: 0 });
  const [busy, setBusy] = useState(false);
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
            <div className="space-y-4 px-6 pb-6">
              <CopyField
                label={t('token_index.token')}
                value={created.key}
                onCopy={() => onCopy(created.key, t('token_index.token'))}
                testId="new-token-key"
              />
              <CopyField label={t('token_index.apiAddress')} value={baseUrl} onCopy={() => onCopy(baseUrl, t('token_index.apiAddress'))} />
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
