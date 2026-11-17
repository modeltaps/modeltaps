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
import {
  DEFAULT_SETTING,
  EXPIRY_PRESETS,
  KEY_TYPES,
  MANAGEMENT_EXPIRY_DEFAULT,
  MANAGEMENT_EXPIRY_PRESETS,
  expiryFromPreset,
  isManagementKeyType,
  managementExpiryFromPreset,
  quotaPatch
} from './tokenData';
import { SecretKeyReveal } from './KeySecret';
import { PairingCodeReveal } from './PairingCode';

// Server answers that belong under the name field rather than in a toast.
const NAME_ERRORS = new Set(['token_name_taken', 'token_name_reserved', 'token_name_too_long', 'management_key_name_required']);

// New key, in a dialog as most consoles do it. The type comes first: an API key (the default)
// takes a name, and optionally a spending limit and an expiry; groups, model and IP restrictions
// start open and are changed in the key's panel. A management key (read-only or read-write,
// AGENT-4) takes a name and a required expiry, and calls the management API instead of models;
// `allowManagement` is off in an organization, where management keys are not created.
// After creating, the dialog shows the key with copy (and the base URL for an API key). This is
// the only time the key is shown: only its hash is stored. An API key also comes with a pairing
// code (AGENT-7), which an agent redeems for the key so it never goes through the chat. `onCreated` receives { id, key } of a
// new API key so the page can still build its test command and client setup while it stays open;
// `onManagementCreated` is called after a management key was created.
export default function NewTokenDialog({ onClose, onCreated, onManagementCreated, onOpen, baseUrl, onCopy, allowManagement = false }) {
  const { t } = useTranslation();
  const [form, setForm] = useState({ type: 'api', name: '', quota: '', expiry: 'never', managementExpiry: MANAGEMENT_EXPIRY_DEFAULT });
  const [busy, setBusy] = useState(false);
  const [nameError, setNameError] = useState('');
  const [created, setCreated] = useState(null);
  const name = form.name.trim();
  const management = allowManagement && isManagementKeyType(form.type);

  const requestClose = () => {
    if (created || !form.name || window.confirm(t('objects.new.discardConfirm'))) onClose();
  };

  const createManagementKey = async () => {
    const res = await API.post('/api/management_key/', {
      name,
      scope: form.type,
      expired_time: managementExpiryFromPreset(form.managementExpiry)
    });
    const { success, message, data, code } = res.data;
    if (success) {
      setCreated({ id: data?.id, key: data?.key || '', management: true });
      onManagementCreated?.();
    } else if (NAME_ERRORS.has(code)) setNameError(message);
    else showError(message);
  };

  const createApiKey = async () => {
    const quota = quotaPatch({ quota: form.quota, period: '' }, t);
    if (quota.error) {
      showError(quota.error);
      return;
    }
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
      setCreated({ ...next, pairingCode: data?.pairing_code || '', pairingExpiresAt: data?.pairing_expires_at || 0 });
      onCreated(next);
    } else if (NAME_ERRORS.has(code)) setNameError(message);
    else showError(message);
  };

  const create = async () => {
    if (!name || busy) return;
    setBusy(true);
    try {
      if (management) await createManagementKey();
      else await createApiKey();
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  const expiryField = management ? (
    <Field label={t('apiKey.new.expiry')} htmlFor="nt-expiry" hint={t('apiKey.management.expiryHint')} className="[&+&]:border-t-0">
      <Select value={form.managementExpiry} onValueChange={(v) => setForm((f) => ({ ...f, managementExpiry: v }))}>
        <SelectTrigger id="nt-expiry">
          <span className="truncate">{t(`apiKey.expiry.${form.managementExpiry}`)}</span>
        </SelectTrigger>
        <SelectContent>
          {MANAGEMENT_EXPIRY_PRESETS.map((p) => (
            <SelectItem key={p} value={p}>
              {t(`apiKey.expiry.${p}`)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  ) : (
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
  );

  return (
    <Dialog open onOpenChange={(o) => !o && requestClose()}>
      <DialogContent className="max-w-md" aria-labelledby="new-token-title">
        {created ? (
          <>
            <DialogHeader className="border-b-0 pb-2">
              <DialogTitle id="new-token-title" className="text-base font-semibold">
                {t('apiKey.new.createdTitle', { name })}
              </DialogTitle>
              <DialogDescription>{created.management ? t('apiKey.management.createdHint') : t('apiKey.new.createdHint')}</DialogDescription>
            </DialogHeader>
            <div className="px-6 pb-6">
              {created.management ? (
                <SecretKeyReveal
                  secret={created.key}
                  onCopy={onCopy}
                  label={t('apiKey.management.key')}
                  note={t('apiKey.management.onceNote')}
                />
              ) : (
                <div className="space-y-4">
                  <SecretKeyReveal secret={created.key} baseUrl={baseUrl} onCopy={onCopy} />
                  {created.pairingCode && (
                    <PairingCodeReveal code={created.pairingCode} expiresAt={created.pairingExpiresAt} onCopy={onCopy} />
                  )}
                </div>
              )}
            </div>
            <DialogFooter>
              {created.id && !created.management ? (
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
              <DialogDescription>{management ? t('apiKey.management.hint') : t('apiKey.new.description')}</DialogDescription>
            </DialogHeader>
            <div className="px-6 pb-4">
              {allowManagement && (
                <Field label={t('apiKey.type.label')} htmlFor="nt-type" hint={t(`apiKey.type.${form.type}Hint`)}>
                  <Select value={form.type} onValueChange={(v) => setForm((f) => ({ ...f, type: v }))}>
                    <SelectTrigger id="nt-type" data-testid="key-type-trigger">
                      <span className="truncate">{t(`apiKey.type.${form.type}`)}</span>
                    </SelectTrigger>
                    <SelectContent>
                      {KEY_TYPES.map((type) => (
                        <SelectItem key={type} value={type}>
                          {t(`apiKey.type.${type}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
              )}
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
              {management ? (
                expiryField
              ) : (
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
                  {expiryField}
                </div>
              )}
            </div>
            <DialogFooter className="items-center sm:justify-between">
              <span className="hidden text-xs text-muted-foreground sm:block">{management ? '' : t('apiKey.new.later')}</span>
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
  onManagementCreated: PropTypes.func,
  onOpen: PropTypes.func.isRequired,
  baseUrl: PropTypes.string,
  onCopy: PropTypes.func.isRequired,
  allowManagement: PropTypes.bool
};
