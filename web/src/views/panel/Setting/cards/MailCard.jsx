import { useCallback, useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { CheckCircle2, Loader2, Lock, Power, Send, Trash2, XCircle } from 'lucide-react';

import { API } from 'utils/api';
import { showError } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { toast } from '@/components/ui/sonner';
import { ActionCard, CollectionCard, StatusDot } from '@/components/settings/cards';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cardText } from './OptionCard';
import useMailStatus from '../mail/useMailStatus';
import { detectMailProvider, getMailProvider, servicePayload, serviceToDraft, smtpTestPayload } from '../mail/mailProviders';
import MailServiceSheet, { ProviderLogo, TLS_SHORT, TestResult, useProviderName, useTestSend } from '../mail/MailServiceSheet';

// ==============================|| PLATFORM SETTINGS — MAIL SERVICES ||============================== //
// The ways the platform can send mail. Several can be stored so a new one can be set up and
// tested before switching; exactly one is in use. The first one added goes into use, and using
// another replaces it. The one in use can only be deleted when it is the last one.

// A test email through a stored service (its saved password) or the deployment's setup.
function TestEmailDialog({ open, onOpenChange, post }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const account = useSelector((state) => state.account);
  const [to, setTo] = useState('');
  const test = useTestSend(post);

  useEffect(() => {
    if (!open) return;
    setTo((v) => v || account?.user?.email || '');
    test.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const send = () => to.trim() && !test.sending && test.send(to.trim());

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
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
              onKeyDown={(e) => e.key === 'Enter' && send()}
              placeholder="you@example.com"
            />
          </FormField>
          <TestResult result={test.result} to={to.trim()} />
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.close')}
          </Button>
          <Button onClick={send} disabled={!to.trim() || test.sending}>
            {test.sending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
            {m('test.send')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

TestEmailDialog.propTypes = { open: PropTypes.bool, onOpenChange: PropTypes.func.isRequired, post: PropTypes.func.isRequired };

// The deployment sets mail (smtp.url): nothing to edit here, only what is used and a test.
function ManagedMail({ managed }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const [testOpen, setTestOpen] = useState(false);
  const sender = managed.from ? (managed.from_name ? `${managed.from_name} <${managed.from}>` : managed.from) : '';
  return (
    <ActionCard title={cardText(t, 'smtp', 'title')} description={cardText(t, 'smtp', 'description')} highlight="smtp">
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
          <Button variant="outline" size="sm" onClick={() => setTestOpen(true)} disabled={!!managed.error}>
            <Send className="size-4" />
            {m('test.open')}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">{m('managed.description')}</p>
        {sender && (
          <p className="text-sm">
            <span className="text-muted-foreground">{m('from')}</span> {sender}
          </p>
        )}
        {managed.error && (
          <p className="flex items-start gap-1.5 text-sm text-destructive">
            <XCircle className="mt-0.5 size-4 shrink-0" />
            {managed.error}
          </p>
        )}
      </div>
      <TestEmailDialog
        open={testOpen}
        onOpenChange={setTestOpen}
        post={(to) => API.post('/api/option/smtp/test', smtpTestPayload({}, to))}
      />
    </ActionCard>
  );
}

ManagedMail.propTypes = { managed: PropTypes.object.isRequired };

const senderOf = (row) => (row.from_name ? `${row.from_name} <${row.from}>` : row.from);

export function SmtpCard() {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mail.${k}`, o);
  const nameOf = useProviderName();
  const { status, refresh } = useMailStatus();
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [sheet, setSheet] = useState({ open: false, editing: null });
  const [testing, setTesting] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/mail_service/');
      if (res.data?.success) setRows(res.data.data || []);
      else showError(res.data?.message);
    } catch (e) {
      /* surfaced by the API interceptor */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const reload = () => {
    load();
    refresh();
  };

  if (status?.managed) return <ManagedMail managed={status.managed} />;

  const inUse = rows.find((r) => r.active) || null;
  const serviceName = (row) => row.name || nameOf(detectMailProvider(serviceToDraft(row)));

  const activate = async (row) => {
    try {
      const res = await API.post(`/api/mail_service/${row.id}/activate`);
      if (!res.data?.success) {
        showError(res.data?.message);
        return;
      }
      toast.success(inUse ? m('switched', { from: serviceName(inUse), to: serviceName(row) }) : m('nowInUse', { name: serviceName(row) }));
      reload();
    } catch (e) {
      /* surfaced by the API interceptor */
    }
  };

  const remove = async () => {
    const target = deleteTarget;
    setDeleteTarget(null);
    if (!target) return;
    try {
      const res = await API.delete(`/api/mail_service/${target.id}`);
      if (!res.data?.success) {
        showError(res.data?.message);
        return;
      }
      toast.success(t('common.deleteSuccess'));
      reload();
    } catch (e) {
      /* surfaced by the API interceptor */
    }
  };

  const lastOne = rows.length === 1;

  return (
    <>
      <CollectionCard
        title={cardText(t, 'smtp', 'title')}
        description={cardText(t, 'smtp', 'description')}
        highlight="smtp"
        addLabel={m('add')}
        onAdd={() => setSheet({ open: true, editing: null })}
        loading={loading}
        empty={m('empty')}
        rows={rows}
        onRowClick={(row) => setSheet({ open: true, editing: row })}
        phoneRow={(row) => ({ primary: serviceName(row), secondary: row.active ? m('inUse') : senderOf(row) })}
        columns={[
          {
            key: 'service',
            header: m('columns.service'),
            cell: (row) => (
              <span className="flex min-w-0 items-center gap-3">
                <ProviderLogo provider={getMailProvider(detectMailProvider(serviceToDraft(row)))} />
                <span className="min-w-0">
                  <span className="block truncate font-medium">{serviceName(row)}</span>
                  <span className="block truncate text-xs text-muted-foreground" title={senderOf(row)}>
                    {senderOf(row)}
                  </span>
                </span>
              </span>
            )
          },
          {
            key: 'server',
            header: m('columns.server'),
            className: 'max-w-[16rem]',
            cell: (row) => (
              <span className="block truncate font-mono text-xs text-muted-foreground">
                {row.server}:{row.port}
              </span>
            )
          },
          {
            key: 'status',
            header: t('settingsUi.collection.status'),
            cell: (row) =>
              row.active ? (
                <span className="inline-flex items-center gap-1.5 text-[13px] text-foreground">
                  <CheckCircle2 className="size-3.5 text-emerald-600 dark:text-emerald-400" />
                  {m('inUse')}
                </span>
              ) : (
                <StatusDot on={false} offLabel={m('notInUse')} />
              )
          }
        ]}
        menu={(row) => [
          { label: m('test.open'), icon: Send, onClick: () => setTesting(row) },
          !row.active && { label: m('use'), icon: Power, onClick: () => activate(row) },
          {
            label: row.active && !lastOne ? m('deleteInUse') : t('common.delete'),
            icon: Trash2,
            destructive: true,
            separator: true,
            disabled: row.active && !lastOne,
            onClick: () => setDeleteTarget(row)
          }
        ]}
      />
      <MailServiceSheet
        open={sheet.open}
        editing={sheet.editing}
        inUse={inUse}
        onOpenChange={(open) => setSheet((s) => ({ ...s, open }))}
        onSaved={() => {
          setSheet({ open: false, editing: null });
          reload();
        }}
      />
      <TestEmailDialog
        open={!!testing}
        onOpenChange={(open) => !open && setTesting(null)}
        post={(to) => API.post('/api/mail_service/test', { ...servicePayload(serviceToDraft(testing), testing.id), to })}
      />
      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
            <DialogDescription>
              {deleteTarget?.active ? m('deleteLast') : t('common.deleteConfirm', { title: deleteTarget ? serviceName(deleteTarget) : '' })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={remove}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
