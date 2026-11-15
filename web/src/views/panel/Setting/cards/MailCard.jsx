import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { CheckCircle2, ExternalLink, Loader2, Lock, Pencil, Send, Server, XCircle } from 'lucide-react';

import { API } from 'utils/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { SelectRow, TextRow } from '../parts';
import OptionCard from './OptionCard';
import { cardKeys } from '../registry';
import useMailStatus from '../mail/useMailStatus';
import {
  CUSTOM_PROVIDER,
  MAIL_PROVIDERS,
  detectMailProvider,
  fromAddressPatch,
  getMailProvider,
  missingMailFields,
  savedSecretApplies,
  providerPatch,
  smtpTestPayload
} from '../mail/mailProviders';

// ==============================|| PLATFORM SETTINGS — MAIL SENDING ||============================== //
// Pick a mail service first; the card then asks only for what that service hands out (an API key,
// or a mailbox and its app password) and fills in the SMTP server, port and encryption itself.
// "Custom SMTP" keeps every field. A test send uses the form as it stands, saved or not.

const SMTP_TLS_MODES = ['auto', 'ssl', 'starttls', 'starttls_opportunistic', 'none'];
const TLS_SHORT = { ssl: 'SSL/TLS', starttls: 'STARTTLS' };

function ProviderLogo({ provider, className }) {
  return (
    <span className={cn('flex size-8 shrink-0 items-center justify-center rounded-md border border-border bg-white', className)}>
      {provider ? <img src={provider.logo} alt="" className="size-5 object-contain" /> : <Server className="size-4 text-zinc-600" />}
    </span>
  );
}

ProviderLogo.propTypes = { provider: PropTypes.object, className: PropTypes.string };

function ProviderTile({ selected, provider, label, onSelect }) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={cn(
        'flex min-w-0 items-center gap-2.5 rounded-lg border bg-field px-2.5 py-2 text-left text-sm transition-colors',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        selected ? 'border-primary ring-1 ring-primary' : 'border-input hover:border-foreground/30 hover:bg-muted'
      )}
    >
      <ProviderLogo provider={provider} />
      <span className="truncate font-medium" title={typeof label === 'string' ? label : undefined}>
        {label}
      </span>
    </button>
  );
}

ProviderTile.propTypes = {
  selected: PropTypes.bool,
  provider: PropTypes.object,
  label: PropTypes.node,
  onSelect: PropTypes.func.isRequired
};

function TestEmailDialog({ open, onOpenChange, inputs }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const account = useSelector((state) => state.account);
  const [to, setTo] = useState('');
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState(null);

  const onOpen = (next) => {
    if (next) {
      setTo((v) => v || account?.user?.email || '');
      setResult(null);
    }
    onOpenChange(next);
  };

  const send = async () => {
    setSending(true);
    setResult(null);
    try {
      const res = await API.post('/api/option/smtp/test', smtpTestPayload(inputs, to.trim()));
      const { success, message } = res.data;
      setResult(success ? { ok: true } : { ok: false, message });
    } catch (e) {
      setResult({ ok: false, message: e.message });
    } finally {
      setSending(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpen}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{m('test.title')}</DialogTitle>
          <DialogDescription>{m('test.description')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <FormField id="smtp-test-to" label={m('test.to')}>
            <Input
              id="smtp-test-to"
              type="email"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && to.trim() && !sending && send()}
              placeholder="you@example.com"
            />
          </FormField>
          {result && (
            <div
              role="status"
              className={cn(
                'flex items-start gap-2 rounded-md px-3 py-2 text-sm',
                result.ok ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' : 'bg-destructive/10 text-destructive'
              )}
            >
              {result.ok ? <CheckCircle2 className="mt-0.5 size-4 shrink-0" /> : <XCircle className="mt-0.5 size-4 shrink-0" />}
              <span className="min-w-0 break-words">{result.ok ? m('test.sent', { to: to.trim() }) : result.message}</span>
            </div>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpen(false)}>
            {t('common.close')}
          </Button>
          <Button onClick={send} disabled={!to.trim() || sending}>
            {sending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
            {m('test.send')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

TestEmailDialog.propTypes = { open: PropTypes.bool, onOpenChange: PropTypes.func.isRequired, inputs: PropTypes.object.isRequired };

// The deployment sets mail (smtp.url): nothing to edit here, only what is used and a test.
function ManagedMail({ managed, onTest }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const sender = managed.from ? (managed.from_name ? `${managed.from_name} <${managed.from}>` : managed.from) : '';
  return (
    <div className="space-y-3 rounded-lg border border-border p-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <ProviderLogo provider={getMailProvider(detectMailProvider({ SMTPServer: managed.server }))} />
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-2 text-sm font-medium">
            {m('managed.title')}
            <Lock className="size-3.5 text-muted-foreground" />
          </p>
          <p className="truncate text-xs text-muted-foreground">
            {managed.server ? `${managed.server} · ${managed.port} · ${TLS_SHORT[managed.tls_mode] || managed.tls_mode}` : '—'}
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={onTest} disabled={!!managed.error}>
          <Send className="size-4" />
          {m('test.open')}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">{m('managed.description')}</p>
      {sender && (
        <p className="text-sm">
          <span className="text-muted-foreground">{m('summary.from')}</span> {sender}
        </p>
      )}
      {managed.error && (
        <p className="flex items-start gap-1.5 text-sm text-destructive">
          <XCircle className="mt-0.5 size-4 shrink-0" />
          {managed.error}
        </p>
      )}
    </div>
  );
}

ManagedMail.propTypes = { managed: PropTypes.object.isRequired, onTest: PropTypes.func.isRequired };

// A saved, complete setup folds into one line; "Change" opens the editor again.
function MailSummary({ provider, inputs, onChange, onTest }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const name = provider ? m(`providers.${provider.id}`, { defaultValue: provider.name }) : m('custom');
  const sender = inputs.SMTPFromName ? `${inputs.SMTPFromName} <${inputs.SMTPFrom}>` : inputs.SMTPFrom;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border border-border p-4">
      <ProviderLogo provider={provider} />
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-sm font-medium">
          {name}
          <span className="inline-flex items-center gap-1 text-xs font-normal text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="size-3.5" />
            {m('summary.ready')}
          </span>
        </p>
        <p className="truncate text-xs text-muted-foreground" title={sender}>
          {m('summary.from')} {sender}
          {inputs.SMTPReplyTo ? ` · ${m('summary.replyTo')} ${inputs.SMTPReplyTo}` : ''}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Button variant="outline" size="sm" onClick={onTest}>
          <Send className="size-4" />
          {m('test.open')}
        </Button>
        <Button variant="outline" size="sm" onClick={onChange}>
          <Pencil className="size-4" />
          {m('summary.change')}
        </Button>
      </div>
    </div>
  );
}

MailSummary.propTypes = {
  provider: PropTypes.object,
  inputs: PropTypes.object.isRequired,
  onChange: PropTypes.func.isRequired,
  onTest: PropTypes.func.isRequired
};

export function SmtpCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, secrets, setField } = ctx;
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const [testOpen, setTestOpen] = useState(false);
  const [editingFor, setEditingFor] = useState(null);
  const { status, refresh } = useMailStatus();
  const managed = status?.managed;

  const providerId = detectMailProvider(inputs);
  const provider = getMailProvider(providerId);
  const isCustom = providerId === CUSTOM_PROVIDER;
  // The saved password only counts while the server and username are still the saved ones: after
  // switching provider a new key is needed (the server reuses the saved one only for its own host).
  const savedSecretFor = (values) => savedSecretApplies(values, ctx.origin, Boolean(secrets?.SMTPToken));
  const missing = missingMailFields(providerId, inputs, savedSecretFor(inputs));
  const savedProviderId = detectMailProvider(ctx.origin);
  const savedComplete =
    Boolean(savedProviderId) && missingMailFields(savedProviderId, ctx.origin, Boolean(secrets?.SMTPToken)).length === 0;
  const dirty = ctx.isDirty(cardKeys('smtp'));
  // "Change" opens the editor for the settings as saved; a save replaces them, which folds the
  // card back into the summary.
  const editing = editingFor !== null && editingFor === ctx.origin;
  const collapsed = savedComplete && !editing && !dirty;

  const providerName = (p) => m(`providers.${p.id}`, { defaultValue: p.name });
  const patch = (values) => Object.entries(values).forEach(([k, v]) => setField(k, v));
  const select = (id) => id !== providerId && patch(providerPatch(id, inputs));

  const groups = useMemo(
    () => [
      { id: 'service', items: MAIL_PROVIDERS.filter((p) => p.group === 'service') },
      { id: 'mailbox', items: MAIL_PROVIDERS.filter((p) => p.group === 'mailbox') }
    ],
    []
  );

  const text = (key, label, { type = 'text', placeholder, description, onChange } = {}) => (
    <TextRow
      id={key}
      type={type}
      label={label}
      value={inputs[key]}
      onChange={onChange ?? ((v) => setField(key, v))}
      placeholder={placeholder}
      description={description}
      secret={secrets?.[key]}
    />
  );

  // Who the mail comes from, on a row of its own below the credentials.
  const senderFields = (
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
  );

  const presetFields = provider && (
    <div className="grid gap-5 sm:grid-cols-2">
      {provider.servers.length > 1 && (
        <SelectRow
          id="SMTPServer"
          label={m(`fields.${provider.serverLabel}`)}
          value={provider.servers.find((s) => s.host === inputs.SMTPServer)?.host ?? provider.servers[0].host}
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
      {provider.account.field && text('SMTPAccount', m(`fields.${provider.account.field}`), { placeholder: provider.account.placeholder })}
      {text('SMTPToken', m(`fields.${provider.secret}`), { type: 'password' })}
    </div>
  );

  const customFields = isCustom && (
    <div className="grid gap-5 sm:grid-cols-2">
      {text('SMTPServer', m('fields.server'), { placeholder: 'smtp.example.com' })}
      {text('SMTPPort', m('fields.port'), { placeholder: '587' })}
      {text('SMTPAccount', m('fields.username'))}
      {text('SMTPToken', m('fields.password'), { type: 'password' })}
      <SelectRow
        id="SMTPTLSMode"
        label={m('fields.tlsMode')}
        value={SMTP_TLS_MODES.includes(inputs.SMTPTLSMode) ? inputs.SMTPTLSMode : 'auto'}
        onChange={(v) => setField('SMTPTLSMode', v)}
        options={SMTP_TLS_MODES.map((mode) => ({ value: mode, label: m(`tlsModes.${mode}`) }))}
      />
    </div>
  );

  const actions = providerId && !collapsed && (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <p className="text-xs text-muted-foreground">{missing.length ? m('test.incomplete') : m('test.hint')}</p>
      <Button variant="outline" size="sm" onClick={() => setTestOpen(true)} disabled={missing.length > 0}>
        <Send className="size-4" />
        {m('test.open')}
      </Button>
    </div>
  );

  if (managed) {
    return (
      <OptionCard ctx={ctx} id="smtp">
        <ManagedMail managed={managed} onTest={() => setTestOpen(true)} />
        <TestEmailDialog open={testOpen} onOpenChange={setTestOpen} inputs={inputs} />
      </OptionCard>
    );
  }

  return (
    <OptionCard
      ctx={ctx}
      id="smtp"
      actions={actions}
      options={{
        validate: (current) =>
          missingMailFields(detectMailProvider(current), current, savedSecretFor(current)).length ? m('incomplete') : null,
        afterSave: refresh
      }}
    >
      {collapsed && (
        <MailSummary provider={provider} inputs={inputs} onChange={() => setEditingFor(ctx.origin)} onTest={() => setTestOpen(true)} />
      )}

      {!collapsed && (
        <>
          <div role="radiogroup" aria-label={m('pick')} className="space-y-4">
            {groups.map((g) => (
              <div key={g.id} className="space-y-2">
                <p className="text-xs font-medium text-muted-foreground">{m(`groups.${g.id}`)}</p>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
                  {g.items.map((p) => (
                    <ProviderTile
                      key={p.id}
                      provider={p}
                      label={providerName(p)}
                      selected={providerId === p.id}
                      onSelect={() => select(p.id)}
                    />
                  ))}
                  {g.id === 'mailbox' && <ProviderTile label={m('custom')} selected={isCustom} onSelect={() => select(CUSTOM_PROVIDER)} />}
                </div>
              </div>
            ))}
          </div>

          {!providerId && <p className="text-sm text-muted-foreground">{m('empty')}</p>}

          {providerId && (
            <div className="space-y-5 rounded-lg border border-border p-4">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <ProviderLogo provider={provider} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{provider ? providerName(provider) : m('custom')}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {provider
                      ? `${inputs.SMTPServer || provider.servers[0].host} · ${provider.port} · ${TLS_SHORT[provider.tls]}`
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
              </div>
              {presetFields}
              {customFields}
              {senderFields}
              {provider?.group === 'mailbox' && <p className="text-xs text-muted-foreground">{m('mailboxNote')}</p>}
            </div>
          )}
        </>
      )}

      <TestEmailDialog open={testOpen} onOpenChange={setTestOpen} inputs={inputs} />
    </OptionCard>
  );
}

SmtpCard.propTypes = { ctx: PropTypes.object.isRequired };
