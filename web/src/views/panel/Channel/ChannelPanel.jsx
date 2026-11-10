import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Copy, ExternalLink, FlaskConical, Gauge, ListChecks, Loader2, MoreHorizontal, Play, RefreshCw, Trash2, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Sheet } from '@/components/ui/sheet';
import { toast } from '@/components/ui/sonner';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { fetchChannel, updateChannel } from './channelApi';
import { ProviderTile, useProviderName } from './channelParts';
import { DEFAULT_BASE_URLS, consoleUrl, hostOf } from './providerDefaults';
import { PANEL_TABS } from './panelTabs';

const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

// Right-side panel of one channel. Header: name, `#id · provider · host`, enable switch, test,
// more actions, close. Below, a bordered box with one tab per part of the channel; each tab
// saves on its own, marks unsaved changes and asks before they are dropped.
export default function ChannelPanel({
  channelId,
  tab,
  onTabChange,
  onClose,
  onChanged,
  allChannels,
  groups,
  metricsRows,
  rangeKey,
  actions,
  busyIds
}) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const [channel, setChannel] = useState(null);
  const [error, setError] = useState('');
  const [draft, setDraft] = useState(null);
  // Tab the draft belongs to: right after a tab switch the old draft is still around for a render.
  const [draftTab, setDraftTab] = useState(null);
  const [base, setBase] = useState(null);
  const [version, setVersion] = useState(0);
  const [saving, setSaving] = useState(false);

  const current = PANEL_TABS.find((x) => x.key === tab) || PANEL_TABS[0];
  const ctxBase = useMemo(() => ({ t, allChannels }), [t, allChannels]);

  const load = useCallback(async () => {
    try {
      const { success, message, data } = await fetchChannel(channelId);
      if (success) {
        setChannel(data);
        setError('');
      } else setError(message || t('channelDetail.notFound'));
    } catch (e) {
      setError(e.message);
    }
  }, [channelId, t]);

  useEffect(() => {
    setChannel(null);
    setError('');
    load();
  }, [load]);

  // A fresh draft whenever the tab or the stored channel changes. A reload of the same tab keeps
  // unsaved edits (a status toggle elsewhere must not wipe what is being typed).
  const resetTab = useRef(null);
  const dirtyRef = useRef(false);
  useEffect(() => {
    if (!channel) return;
    if (resetTab.current === current.key && dirtyRef.current) return;
    const d = current.make(channel, ctxBase);
    resetTab.current = current.key;
    setDraftTab(current.key);
    setDraft(d);
    setBase(d);
    setVersion((v) => v + 1);
    // ctxBase is left out on purpose: a list refresh must not reset what is being typed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [channel, current]);

  // Reload when the list reports a change to this channel (status toggle, test, balance...).
  const listRow = allChannels.find((c) => c.id === channelId);
  const listStamp = listRow
    ? [
        listRow.status,
        listRow.test_time,
        listRow.balance_updated_time,
        listRow.model_drift?.checked_at,
        listRow.priority,
        listRow.weight
      ].join('|')
    : '';
  const stampRef = useRef(listStamp);
  useEffect(() => {
    if (stampRef.current === listStamp) return;
    stampRef.current = listStamp;
    if (listStamp) load();
  }, [listStamp, load]);

  const ready = Boolean(channel && draft && draftTab === current.key);
  const dirty = Boolean(ready && base && !same(draft, base));
  useEffect(() => {
    dirtyRef.current = dirty;
  }, [dirty]);

  useEffect(() => {
    if (!dirty) return undefined;
    const onBeforeUnload = (e) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  const confirmDrop = () => !dirty || window.confirm(t('channels.panel.discardConfirm'));
  const requestClose = () => {
    if (confirmDrop()) onClose();
  };
  const switchTab = (key) => {
    if (key === current.key) return;
    if (!dirty || window.confirm(t('channels.panel.switchConfirm'))) onTabChange(key);
  };
  const discard = () => {
    setDraft(base);
    setVersion((v) => v + 1);
  };

  const save = async () => {
    const patch = current.patch(draft, channel, ctxBase);
    if (patch?.error) {
      toast.error(patch.error);
      return;
    }
    setSaving(true);
    try {
      const { success, message } = await updateChannel(channel, patch);
      if (success) {
        toast.success(t('channels.panel.saved'));
        resetTab.current = null;
        await load();
        onChanged();
      } else toast.error(message);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  const ctx = {
    ...ctxBase,
    groups,
    metricsRows,
    rangeKey,
    actions,
    version,
    busy: {
      refreshing: busyIds.refreshing.has(channelId),
      drift: busyIds.drift.has(channelId),
      dismissing: busyIds.dismissing.has(channelId)
    }
  };
  const Body = current.Body;
  const host = channel ? hostOf(channel.base_url || DEFAULT_BASE_URLS[channel.type] || '') : '';
  const webUrl = channel ? consoleUrl(channel.type) : '';
  const testing = busyIds.testing.has(channelId);

  return (
    <Sheet open onOpenChange={(open) => !open && requestClose()}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="channel-panel-title"
        className="absolute inset-y-2 right-2 flex w-[calc(100%-1rem)] max-w-[720px] flex-col rounded-2xl border border-border bg-popover shadow-2xl"
      >
        <div className="flex items-center justify-between gap-4 border-b border-border px-6 py-[18px]">
          <div className="flex min-w-0 items-center gap-3">
            {channel && <ProviderTile type={channel.type} size="lg" />}
            <div className="min-w-0">
              <h2 id="channel-panel-title" className="truncate text-base font-medium">
                {channel?.name || (error ? t('channelDetail.notFound') : '…')}
              </h2>
              {channel && (
                <p className="truncate font-mono text-xs text-muted-foreground">
                  {[`#${channel.id}`, providerName(channel.type), host].filter(Boolean).join(' · ')}
                </p>
              )}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {channel && (
              <>
                <span className="text-xs text-muted-foreground">{t('channels.panel.enabled')}</span>
                <Switch
                  checked={channel.status === 1}
                  aria-label={t('channels.panel.enabled')}
                  onCheckedChange={() => actions.onToggleStatus(channel)}
                />
                <Button variant="outline" size="sm" className="rounded-lg" disabled={testing} onClick={() => actions.onTest(channel)}>
                  {testing ? <Loader2 className="animate-spin" /> : <Play />}
                  {t('channels.action.test')}
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('channels.more')}>
                      <MoreHorizontal className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-52">
                    <DropdownMenuItem onClick={() => actions.onMultiTest(channel)}>
                      <Gauge className="size-4" />
                      {t('channelTest.action')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => actions.onCheck(channel)}>
                      <ListChecks className="size-4" />
                      {t('channel_row.check')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => actions.onRefreshBalance(channel)}>
                      <RefreshCw className="size-4" />
                      {t('channels.conn.refreshBalance')}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => actions.onCheckDrift(channel)}>
                      <FlaskConical className="size-4" />
                      {t('channels.monitor.checkNow')}
                    </DropdownMenuItem>
                    <DropdownMenuItem disabled={!webUrl} onClick={() => webUrl && window.open(webUrl, '_blank', 'noopener,noreferrer')}>
                      <ExternalLink className="size-4" />
                      {t('channel_row.channelWeb')}
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => actions.onCopy(channel)}>
                      <Copy className="size-4" />
                      {t('channels.action.copy')}
                    </DropdownMenuItem>
                    <DropdownMenuItem className="text-destructive" onClick={() => actions.onDelete(channel)}>
                      <Trash2 className="size-4" />
                      {t('channels.action.delete')}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </>
            )}
            <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('common.close')} onClick={requestClose}>
              <X className="size-4" />
            </Button>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
          {error ? (
            <p className="py-16 text-center text-sm text-muted-foreground">{error}</p>
          ) : !channel ? (
            <div className="flex h-48 items-center justify-center text-muted-foreground">
              <Loader2 className="size-5 animate-spin" />
            </div>
          ) : (
            <div className="rounded-2xl border border-border bg-card">
              <div role="tablist" aria-label={t('channels.panel.tabs')} className="flex gap-0.5 overflow-x-auto border-b border-border p-1">
                {PANEL_TABS.map((x) => (
                  <button
                    key={x.key}
                    type="button"
                    role="tab"
                    aria-selected={x.key === current.key}
                    onClick={() => switchTab(x.key)}
                    className={cn(
                      'inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-3 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                      x.key === current.key
                        ? 'bg-muted text-foreground'
                        : 'text-muted-foreground hover:bg-foreground/[0.05] hover:text-foreground'
                    )}
                  >
                    {t(`channels.panel.tab.${x.key}`)}
                    {x.key === current.key && dirty && (
                      <span className="size-1.5 rounded-full bg-warning" aria-label={t('channels.panel.unsaved')} />
                    )}
                  </button>
                ))}
              </div>
              <div role="tabpanel" className="px-5 pb-5 pt-1">
                {ready && <Body key={version} channel={channel} draft={draft} setDraft={setDraft} ctx={ctx} />}
              </div>
            </div>
          )}
        </div>

        {ready && (
          <div className="flex items-center justify-between gap-3 border-t border-border py-3 pl-6 pr-4">
            <span className={cn('text-[13px]', dirty ? 'text-foreground' : 'text-muted-foreground')}>
              {dirty && <span className="mr-2 inline-block size-1.5 rounded-full bg-warning align-[1px]" />}
              {dirty ? t('channels.panel.unsaved') : t('channels.panel.scope')}
            </span>
            <div className="flex gap-2">
              <Button variant="outline" className="rounded-lg" disabled={!dirty || saving} onClick={discard}>
                {t('channels.panel.discard')}
              </Button>
              <Button className="rounded-lg" disabled={!dirty || saving} onClick={save}>
                {saving && <Loader2 className="animate-spin" />}
                {t('channels.panel.save')}
              </Button>
            </div>
          </div>
        )}
      </div>
    </Sheet>
  );
}

ChannelPanel.propTypes = {
  channelId: PropTypes.number.isRequired,
  tab: PropTypes.string,
  onTabChange: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  onChanged: PropTypes.func.isRequired,
  allChannels: PropTypes.array.isRequired,
  groups: PropTypes.array.isRequired,
  metricsRows: PropTypes.array.isRequired,
  rangeKey: PropTypes.string.isRequired,
  actions: PropTypes.object.isRequired,
  busyIds: PropTypes.object.isRequired
};
