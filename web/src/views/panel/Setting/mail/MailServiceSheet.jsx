import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { CheckCircle2, ExternalLink, Loader2, Search, Send, Server, XCircle } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Sheet, SheetBody, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { FormField } from '@/components/ui/form-field';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { SelectRow, TextRow } from '../parts';
import {
  CUSTOM_PROVIDER,
  detectMailProvider,
  fromAddressPatch,
  getMailProvider,
  missingMailFields,
  providerPatch,
  savedSecretApplies,
  searchMailProviders,
  serviceToDraft,
  servicePayload
} from './mailProviders';

// ==============================|| PLATFORM SETTINGS — MAIL SERVICE SHEET ||============================== //
// Adds or edits one mail service in one go: pick the service (a preset or custom SMTP), type only
// what it hands out, send a test, save. A preset fills the server, port and encryption itself.

const SMTP_TLS_MODES = ['auto', 'ssl', 'starttls', 'starttls_opportunistic', 'none'];
export const TLS_SHORT = { ssl: 'SSL/TLS', starttls: 'STARTTLS' };

export function ProviderLogo({ provider, className }) {
  return (
    <span className={cn('flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-white', className)}>
      {provider ? <img src={provider.logo} alt="" className="size-5 object-contain" /> : <Server className="size-4 text-zinc-600" />}
    </span>
  );
}

ProviderLogo.propTypes = { provider: PropTypes.object, className: PropTypes.string };

export const useProviderName = () => {
  const { t } = useTranslation();
  return (idOrProvider) => {
    const p = typeof idOrProvider === 'string' ? getMailProvider(idOrProvider) : idOrProvider;
    return p ? t(`settingsUi.platform.mail.providers.${p.id}`, { defaultValue: p.name }) : t('settingsUi.platform.mail.custom');
  };
};

function ProviderTile({ provider, label, onSelect }) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        'flex min-w-0 items-center gap-2.5 rounded-lg border border-input bg-field px-2.5 py-2 text-left text-sm transition-colors',
        'hover:border-foreground/30 hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'
      )}
    >
      <ProviderLogo provider={provider} />
      <span className="truncate font-medium">{label}</span>
    </button>
  );
}

ProviderTile.propTypes = { provider: PropTypes.object, label: PropTypes.node, onSelect: PropTypes.func.isRequired };

function ProviderPicker({ onPick }) {
  const { t } = useTranslation();
  const m = (k) => t(`settingsUi.platform.mail.${k}`);
  const nameOf = useProviderName();
  const [query, setQuery] = useState('');
  const found = searchMailProviders(query, nameOf);
  const customMatches =
    !query.trim() || nameOf(null).toLowerCase().includes(query.trim().toLowerCase()) || 'smtp'.includes(query.trim().toLowerCase());
  return (
    <div className="space-y-4">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input autoFocus value={query} onChange={(e) => setQuery(e.target.value)} placeholder={m('search')} className="pl-9" />
      </div>
      {['service', 'mailbox'].map((group) => {
        const items = found.filter((p) => p.group === group);
        const showCustom = group === 'mailbox' && customMatches;
        if (!items.length && !showCustom) return null;
        return (
          <div key={group} className="space-y-2">
            <p className="text-xs font-medium text-muted-foreground">{m(`groups.${group}`)}</p>
            <div className="grid grid-cols-2 gap-2">
              {items.map((p) => (
                <ProviderTile key={p.id} provider={p} label={nameOf(p)} onSelect={() => onPick(p.id)} />
              ))}
              {showCustom && <ProviderTile label={m('custom')} onSelect={() => onPick(CUSTOM_PROVIDER)} />}
            </div>
          </div>
        );
      })}
      {!found.length && !customMatches && <p className="text-sm text-muted-foreground">{m('noMatch')}</p>}
    </div>
  );
}

ProviderPicker.propTypes = { onPick: PropTypes.func.isRequired };

export function TestResult({ result, to }) {
  const { t } = useTranslation();
  if (!result) return null;
  return (
    <div
      role="status"
      className={cn(
        'flex items-start gap-2 rounded-md px-3 py-2 text-sm',
        result.ok ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' : 'bg-destructive/10 text-destructive'
      )}
    >
      {result.ok ? <CheckCircle2 className="mt-0.5 size-4 shrink-0" /> : <XCircle className="mt-0.5 size-4 shrink-0" />}
      <span className="min-w-0 break-words">{result.ok ? t('settingsUi.platform.mail.test.sent', { to }) : result.message}</span>
    </div>
  );
}

TestResult.propTypes = { result: PropTypes.object, to: PropTypes.string };

// Posts a test email and keeps its outcome.
export function useTestSend(post) {
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState(null);
  const send = async (to) => {
    setSending(true);
    setResult(null);
    try {
      const res = await post(to);
      const { success, message } = res.data;
      setResult(success ? { ok: true } : { ok: false, message });
    } catch (e) {
      setResult({ ok: false, message: e.message });
    } finally {
      setSending(false);
    }
  };
  return { sending, result, send, reset: () => setResult(null) };
}

// editing: the stored service, or null to add one. inUse: the service in use, if any.
export default function MailServiceSheet({ open, onOpenChange, editing, inUse, onSaved }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const nameOf = useProviderName();
  const account = useSelector((state) => state.account);
  const [draft, setDraft] = useState(serviceToDraft(null));
  const [useNow, setUseNow] = useState(true);
  const [saving, setSaving] = useState(false);
  const [to, setTo] = useState('');
  const isEdit = !!editing;
  const origin = useMemo(() => serviceToDraft(editing), [editing]);

  const payload = () => servicePayload(draft, editing?.id || 0);
  const test = useTestSend((recipient) => API.post('/api/mail_service/test', { ...payload(), to: recipient }));

  useEffect(() => {
    if (!open) return;
    setDraft(serviceToDraft(editing));
    // Adding one usually means switching to it; editing a spare should not switch by accident.
    setUseNow(!editing);
    setTo((v) => v || account?.user?.email || '');
    test.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editing]);

  const providerId = detectMailProvider(draft);
  const provider = getMailProvider(providerId);
  const isCustom = providerId === CUSTOM_PROVIDER;
  const secretSaved = savedSecretApplies(draft, origin, Boolean(editing?.token_masked));
  const missing = missingMailFields(providerId, draft, secretSaved);
  // The switch matters only when another service is in use: the first one is used anyway.
  const offerUse = Boolean(inUse) && (!isEdit || !editing.active);

  const setField = (key, value) => {
    test.reset();
    setDraft((d) => ({ ...d, [key]: value }));
  };
  const patch = (values) => {
    test.reset();
    setDraft((d) => ({ ...d, ...values }));
  };
  const pick = (id) => setDraft((d) => ({ ...d, ...providerPatch(id, d) }));

  const text = (key, label, { type = 'text', placeholder, description, onChange } = {}) => (
    <TextRow
      id={`mail-${key}`}
      type={type}
      label={label}
      value={draft[key]}
      onChange={onChange ?? ((v) => setField(key, v))}
      placeholder={placeholder}
      description={description}
      secret={key === 'SMTPToken' && secretSaved ? editing.token_masked : undefined}
    />
  );

  const submit = async () => {
    if (saving || missing.length) return;
    setSaving(true);
    try {
      const res = isEdit ? await API.put(`/api/mail_service/${editing.id}`, payload()) : await API.post('/api/mail_service/', payload());
      if (!res.data?.success) {
        showError(res.data?.message);
        return;
      }
      let saved = res.data.data;
      if (offerUse && useNow && !saved.active) {
        const act = await API.post(`/api/mail_service/${saved.id}/activate`);
        if (!act.data?.success) {
          showError(act.data?.message);
          onSaved?.();
          return;
        }
        saved = act.data.data;
        toast.success(m('switched', { from: nameOf(inUse.provider), to: nameOf(saved.provider) }));
      } else {
        toast.success(saved.active && !isEdit ? m('nowInUse', { name: nameOf(saved.provider) }) : m('saved'));
      }
      onSaved?.();
    } catch (e) {
      /* surfaced by the API interceptor */
    } finally {
      setSaving(false);
    }
  };

  const picking = !providerId;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent onClose={() => onOpenChange(false)}>
        <SheetHeader>
          <SheetTitle>{isEdit ? t('common.edit') : m('add')}</SheetTitle>
          <SheetDescription>{m('sheetDescription')}</SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-5">
          {picking && <ProviderPicker onPick={pick} />}

          {!picking && (
            <>
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-border p-3">
                <ProviderLogo provider={provider} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{nameOf(provider)}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {provider
                      ? `${draft.SMTPServer || provider.servers[0].host} · ${provider.port} · ${TLS_SHORT[provider.tls]}`
                      : m('customHint')}
                  </p>
                </div>
                {provider && (
                  <a
                    href={provider.docs}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                  >
                    {m(`docs.${provider.secret}`)}
                    <ExternalLink className="size-3" />
                  </a>
                )}
                {!isEdit && (
                  <Button variant="ghost" size="sm" onClick={() => setDraft(serviceToDraft(null))}>
                    {m('changeProvider')}
                  </Button>
                )}
              </div>

              {provider && (
                <div className="grid gap-5 sm:grid-cols-2">
                  {provider.servers.length > 1 && (
                    <SelectRow
                      id="mail-SMTPServer"
                      label={m(`fields.${provider.serverLabel}`)}
                      value={provider.servers.find((s) => s.host === draft.SMTPServer)?.host ?? provider.servers[0].host}
                      onChange={(v) => setField('SMTPServer', v)}
                      options={provider.servers.map((s) => ({ value: s.host, label: s.label }))}
                    />
                  )}
                  {provider.account.fromAddress &&
                    text('SMTPFrom', m('fields.mailbox'), {
                      type: 'email',
                      placeholder: 'name@example.com',
                      onChange: (v) => patch(fromAddressPatch(provider.id, v))
                    })}
                  {provider.account.field &&
                    text('SMTPAccount', m(`fields.${provider.account.field}`), { placeholder: provider.account.placeholder })}
                  {text('SMTPToken', m(`fields.${provider.secret}`), { type: 'password' })}
                </div>
              )}

              {isCustom && (
                <div className="grid gap-5 sm:grid-cols-2">
                  {text('SMTPServer', m('fields.server'), { placeholder: 'smtp.example.com' })}
                  {text('SMTPPort', m('fields.port'), { placeholder: '587' })}
                  {text('SMTPAccount', m('fields.username'))}
                  {text('SMTPToken', m('fields.password'), { type: 'password' })}
                  <SelectRow
                    id="mail-SMTPTLSMode"
                    label={m('fields.tlsMode')}
                    value={SMTP_TLS_MODES.includes(draft.SMTPTLSMode) ? draft.SMTPTLSMode : 'auto'}
                    onChange={(v) => setField('SMTPTLSMode', v)}
                    options={SMTP_TLS_MODES.map((mode) => ({ value: mode, label: m(`tlsModes.${mode}`) }))}
                  />
                </div>
              )}

              <div className="grid gap-5 sm:grid-cols-2">
                {text('SMTPFromName', m('fields.fromName'), { placeholder: m('fields.fromNamePlaceholder') })}
                {!provider?.account.fromAddress &&
                  text('SMTPFrom', m('fields.fromAddress'), {
                    type: 'email',
                    placeholder: 'noreply@example.com',
                    description: provider ? m('fields.fromAddressHint') : undefined
                  })}
                {text('SMTPReplyTo', m('fields.replyTo'), {
                  type: 'email',
                  placeholder: 'support@example.com',
                  description: m('fields.replyToHint')
                })}
              </div>
              {provider?.group === 'mailbox' && <p className="text-xs text-muted-foreground">{m('mailboxNote')}</p>}

              <FormField id="mail-test-to" label={m('test.toInline')} hint={missing.length ? m('test.incomplete') : m('test.hint')}>
                <div className="flex gap-2">
                  <Input
                    id="mail-test-to"
                    type="email"
                    value={to}
                    onChange={(e) => {
                      setTo(e.target.value);
                      test.reset();
                    }}
                    placeholder="you@example.com"
                  />
                  <Button
                    variant="outline"
                    className="shrink-0"
                    onClick={() => test.send(to.trim())}
                    disabled={!to.trim() || test.sending || missing.length > 0}
                  >
                    {test.sending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
                    {m('test.send')}
                  </Button>
                </div>
              </FormField>
              <TestResult result={test.result} to={to.trim()} />

              {offerUse && (
                <FormField id="mail-use-now" label={m('useNow')} hint={m('useNowHint', { name: nameOf(inUse.provider) })} row>
                  <div className="flex justify-end">
                    <Switch id="mail-use-now" checked={useNow} onCheckedChange={setUseNow} />
                  </div>
                </FormField>
              )}
            </>
          )}
        </SheetBody>
        {!picking && (
          <SheetFooter>
            {missing.length > 0 && <p className="mr-auto self-center text-xs text-muted-foreground">{m('incomplete')}</p>}
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={submit} disabled={saving || missing.length > 0}>
              {saving && <Loader2 className="size-4 animate-spin" />}
              {t('common.submit')}
            </Button>
          </SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  );
}

MailServiceSheet.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  editing: PropTypes.object,
  inUse: PropTypes.object,
  onSaved: PropTypes.func
};
