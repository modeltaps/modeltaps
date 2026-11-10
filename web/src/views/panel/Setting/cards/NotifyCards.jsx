import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Eye, EyeOff, Info, RotateCw, Send, Trash2 } from 'lucide-react';

import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { cn } from '@/lib/utils';
import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { CollectionCard, OptionList, StatusDot } from '@/components/settings/cards';
import TelegramSheet from '../../Telegram/TelegramSheet';
import ChatLinkSheet, { parseChatLinks, serializeChatLinks } from '../ChatLinkSheet';
import { SelectRow, TextRow, ToggleRow } from '../parts';
import OptionCard, { bindSwitch, cardText } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 通知与集成 ||============================== //
// Mail sending (SMTP), admin notifications, the Telegram bot menu, built-in chat and chat links,
// and the Midjourney callback.

const SMTP_TLS_MODES = ['auto', 'ssl', 'starttls', 'starttls_opportunistic', 'none'];

export function SmtpCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, secrets, setField } = ctx;
  const sys = (k) => t(`setting_index.systemSettings.configureSMTP.${k}`);
  const field = (key, name, type = 'text') => (
    <TextRow
      id={key}
      type={type}
      label={sys(name)}
      value={inputs[key]}
      onChange={(v) => setField(key, v)}
      placeholder={sys(`${name}Placeholder`)}
      secret={secrets?.[key]}
    />
  );

  return (
    <OptionCard ctx={ctx} id="smtp">
      <Alert variant="info">
        <Info />
        <span>{sys('alert')}</span>
      </Alert>
      <div className="grid gap-5 sm:grid-cols-2">
        {field('SMTPServer', 'smtpServer')}
        {field('SMTPPort', 'smtpPort')}
        {field('SMTPAccount', 'smtpAccount')}
        {field('SMTPFrom', 'smtpFrom')}
        {field('SMTPToken', 'smtpToken', 'password')}
        <SelectRow
          id="SMTPTLSMode"
          label={sys('smtpTlsMode')}
          description={sys('smtpTlsModeDescription')}
          value={SMTP_TLS_MODES.includes(inputs.SMTPTLSMode) ? inputs.SMTPTLSMode : 'auto'}
          onChange={(v) => setField('SMTPTLSMode', v)}
          options={SMTP_TLS_MODES.map((mode) => ({ value: mode, label: sys(`smtpTlsModes.${mode}`) }))}
        />
      </div>
    </OptionCard>
  );
}

// One switch per notification kind; stored as the comma-joined list of disabled kinds.
const NOTIFY_KINDS = [
  'channel_disabled',
  'channel_enabled',
  'channel_test',
  'low_balance',
  'price_change',
  'model_drift',
  'model_consistency',
  'credential'
];

export function NotificationsCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const nt = (k, opts) => t(`setting_index.notifications.${k}`, opts);
  const disabled = new Set(
    (inputs.NotifyDisabledKinds || '')
      .split(',')
      .map((k) => k.trim())
      .filter(Boolean)
  );
  const setKind = (kind, on) => {
    const next = new Set(disabled);
    if (on) next.delete(kind);
    else next.add(kind);
    setField('NotifyDisabledKinds', NOTIFY_KINDS.filter((k) => next.has(k)).join(','));
  };

  return (
    <OptionCard ctx={ctx} id="notifications" description={nt('description')}>
      <OptionList>
        {NOTIFY_KINDS.map((kind) => (
          <ToggleRow
            key={kind}
            id={`notify-${kind}`}
            label={nt(`kind.${kind}`)}
            checked={!disabled.has(kind)}
            onCheckedChange={(on) => setKind(kind, on)}
          />
        ))}
      </OptionList>
      <TextRow
        id="NotifyKindLimitPerHour"
        type="number"
        label={nt('limit.label')}
        value={inputs.NotifyKindLimitPerHour}
        onChange={(v) => setField('NotifyKindLimitPerHour', v)}
        placeholder="20"
        description={nt('limit.description')}
      />
    </OptionCard>
  );
}

// The settings card lists the first page of menu commands, newest first.
const TELEGRAM_PAGE_SIZE = 100;

export function TelegramCard() {
  const { t } = useTranslation();
  const tp = (k) => t(`telegramPage.${k}`);
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState({ online: false, webhook: false });
  const [sheet, setSheet] = useState({ open: false, id: 0 });
  const [deleteTarget, setDeleteTarget] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/option/telegram/', { params: { page: 1, size: TELEGRAM_PAGE_SIZE, keyword: '', order: '-id' } });
      const { success, data } = res.data;
      if (success) setRows(data?.data || []);
    } catch (e) {
      /* surfaced by the API interceptor */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    API.get('/api/option/telegram/status')
      .then((res) => {
        if (res.data?.success) setStatus({ online: !!res.data.data.status, webhook: !!res.data.data.is_webhook });
      })
      .catch(() => {});
  }, [load]);

  const reload = async () => {
    try {
      const res = await API.put('/api/option/telegram/reload');
      if (res.data.success) showSuccess(tp('reloadSuccess'));
      else showError(res.data.message);
    } catch (e) {
      /* surfaced by the API interceptor */
    }
  };

  const remove = async () => {
    const target = deleteTarget;
    setDeleteTarget(null);
    if (!target) return;
    try {
      const res = await API.delete(`/api/option/telegram/${target.id}`);
      if (res.data.success) {
        showSuccess(tp('operationSuccess'));
        load();
      } else {
        showError(res.data.message);
      }
    } catch (e) {
      /* surfaced by the API interceptor */
    }
  };

  return (
    <>
      <CollectionCard
        title={cardText(t, 'telegram', 'title')}
        description={tp('infoMessage')}
        highlight="telegram"
        addLabel={t('settingsUi.platform.telegram.add')}
        onAdd={() => setSheet({ open: true, id: 0 })}
        aside={
          <>
            <span className={cn('inline-flex items-center gap-1.5 text-xs', status.online ? 'text-muted-foreground' : 'text-destructive')}>
              <Send className="size-3.5" />
              {(status.online ? tp('online') : tp('offline')) + (status.webhook ? ' · Webhook' : ' · Polling')}
            </span>
            <Button size="sm" variant="ghost" onClick={reload}>
              <RotateCw className="size-4" />
              {tp('reloadMenu')}
            </Button>
          </>
        }
        loading={loading}
        rows={rows}
        onRowClick={(row) => setSheet({ open: true, id: row.id })}
        columns={[
          { key: 'command', header: tp('command'), cell: (row) => <span className="font-mono text-xs">{row.command}</span> },
          { key: 'description', header: tp('description') },
          { key: 'parse_mode', header: tp('replyType'), cell: (row) => <span className="text-muted-foreground">{row.parse_mode}</span> }
        ]}
        menu={(row) => [{ label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => setDeleteTarget(row) }]}
      />
      <TelegramSheet
        open={sheet.open}
        menuId={sheet.id}
        onClose={() => setSheet((s) => ({ ...s, open: false }))}
        onSaved={() => {
          setSheet((s) => ({ ...s, open: false }));
          load();
        }}
      />
      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
            <DialogDescription>{t('common.deleteConfirm', { title: deleteTarget?.command })}</DialogDescription>
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

export function BuiltinChatCard({ ctx }) {
  const { t } = useTranslation();
  return (
    <OptionCard ctx={ctx} id="builtin-chat">
      <OptionList>
        <ToggleRow
          id="BuiltinChatEnabled"
          label={t('setting_index.operationSettings.chatLinkSettings.builtinChatEnabled')}
          {...bindSwitch(ctx, 'BuiltinChatEnabled')}
        />
      </OptionList>
    </OptionCard>
  );
}

// Each entry saves the whole ChatLinks option right away (the stored value, not a pending edit).
export function ChatLinksCard({ ctx }) {
  const { t } = useTranslation();
  const ed = (k) => t(`setting_index.operationSettings.chatLinkSettings.editor.${k}`);
  const links = parseChatLinks(ctx.origin.ChatLinks);
  const [sheet, setSheet] = useState({ open: false, index: -1 });
  const [deleteIndex, setDeleteIndex] = useState(-1);

  const write = (next) => ctx.saveKeys(['ChatLinks'], { values: { ChatLinks: serializeChatLinks(next) } });

  const saveEntry = (entry) => {
    const next = [...links];
    if (sheet.index >= 0) next[sheet.index] = entry;
    else next.unshift({ id: Math.random().toString(36).slice(2, 11), ...entry });
    return write(next);
  };

  return (
    <>
      <CollectionCard
        title={cardText(t, 'chat-links', 'title')}
        description={t('setting_index.operationSettings.chatLinkSettings.description')}
        highlight="chat-links"
        addLabel={t('settingsUi.platform.chatLinks.add')}
        onAdd={() => setSheet({ open: true, index: -1 })}
        empty={ed('empty')}
        rows={links.map((link, index) => ({ ...link, index }))}
        rowKey={(row) => row.id || `${row.index}`}
        onRowClick={(row) => setSheet({ open: true, index: row.index })}
        columns={[
          { key: 'name', header: ed('name'), cell: (row) => <span className="font-medium">{row.name}</span> },
          {
            key: 'url',
            header: ed('url'),
            className: 'max-w-[20rem]',
            cell: (row) => <span className="block truncate font-mono text-xs text-muted-foreground">{row.url}</span>
          },
          { key: 'sort', header: ed('sort'), className: 'tabular-nums' },
          { key: 'show', header: t('settingsUi.collection.status'), cell: (row) => <StatusDot on={row.show !== false} /> }
        ]}
        menu={(row) => [
          {
            label: row.show !== false ? t('settingsUi.collection.disable') : t('settingsUi.collection.enable'),
            icon: row.show !== false ? EyeOff : Eye,
            onClick: () => write(links.map((link, i) => (i === row.index ? { ...link, show: row.show === false } : link)))
          },
          { label: t('common.delete'), icon: Trash2, destructive: true, separator: true, onClick: () => setDeleteIndex(row.index) }
        ]}
      />
      <ChatLinkSheet
        open={sheet.open}
        editing={sheet.index >= 0 ? links[sheet.index] : null}
        onOpenChange={(open) => setSheet((s) => ({ ...s, open }))}
        onSave={saveEntry}
      />
      <Dialog open={deleteIndex >= 0} onOpenChange={(o) => !o && setDeleteIndex(-1)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
            <DialogDescription>{t('common.deleteConfirm', { title: links[deleteIndex]?.name || '' })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteIndex(-1)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                const index = deleteIndex;
                setDeleteIndex(-1);
                write(links.filter((_, i) => i !== index));
              }}
            >
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function MjNotifyCard({ ctx }) {
  const { t } = useTranslation();
  const os = (k) => t(`setting_index.operationSettings.otherSettings.${k}`);
  return (
    <OptionCard ctx={ctx} id="mj-notify">
      <OptionList>
        <ToggleRow
          id="MjNotifyEnabled"
          label={os('mjNotify')}
          description={os('mjNotifyDescription')}
          {...bindSwitch(ctx, 'MjNotifyEnabled')}
        />
      </OptionList>
    </OptionCard>
  );
}
