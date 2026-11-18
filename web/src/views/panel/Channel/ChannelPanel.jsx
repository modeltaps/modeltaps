import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  AlertTriangle,
  Copy,
  ExternalLink,
  FlaskConical,
  Gauge,
  ListChecks,
  Loader2,
  MoreHorizontal,
  Pencil,
  Play,
  RefreshCw,
  Trash2
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { toast } from '@/components/ui/sonner';
import { EditableTitle, ObjectPanel } from '@/components/objects';
import { adoptChannelRenames, fetchChannel, updateChannel } from './channelApi';
import { ProviderTile, useProviderName } from './channelParts';
import { formatBalance } from './channelMetrics';
import { RETIRED_CHANNEL_TYPES } from 'constants/ChannelConstants';
import { DEFAULT_BASE_URLS, consoleUrl, hostOf } from './providerDefaults';
import { PANEL_TABS } from './panelTabs';

// Right-side panel of one channel. Header: name, `#id · provider · host`, enable switch, test,
// more actions, close. Below, one tab per part of the channel; each tab saves on its own, marks
// unsaved changes and asks before they are dropped (components/objects/ObjectPanel).
export default function ChannelPanel({
  channelId,
  tab,
  onTabChange,
  onClose,
  onChanged,
  allChannels,
  groups,
  supplyRouting = false,
  metricsRows,
  rangeKey,
  actions,
  busyIds,
  issues = [],
  modelsFocus = null,
  onIssueAction,
  issueBusy
}) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const [channel, setChannel] = useState(null);
  const [error, setError] = useState('');
  const [renaming, setRenaming] = useState(false);
  // With supplies routing, the routing tab keeps only groups and the tag: the order lives in the
  // allocation policy.
  const tabs = useMemo(
    () =>
      PANEL_TABS.map((x) => ({
        ...x,
        label: t(`channels.panel.tab.${x.key === 'routing' && supplyRouting ? 'routingGroups' : x.key}`)
      })),
    [t, supplyRouting]
  );

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
    setRenaming(false);
    load();
  }, [load]);

  // Reload when the list reports a change to this channel (status toggle, test, balance...).
  const listRow = allChannels.find((c) => c.id === channelId);
  const listStamp = listRow
    ? [
        listRow.status,
        listRow.test_time,
        listRow.balance_updated_time,
        listRow.model_drift?.checked_at,
        (listRow.model_drift?.dismissed_missing || []).join(','),
        (listRow.model_drift?.new_models || []).join(','),
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

  // The latest stored channel, for saves that finish (or are undone) after a reload.
  const channelRef = useRef(channel);
  useEffect(() => {
    channelRef.current = channel;
  }, [channel]);
  const ctx = {
    t,
    // Saves the models tab outside its draft (the one-off cleanup of upstream-gone models).
    saveModels: async (patch) => {
      const res = await updateChannel(channelRef.current, patch);
      if (res?.success) {
        await load();
        onChanged();
      }
      return res;
    },
    // Switches the listed models the upstream renamed to their new names, saved at once (the
    // drift impact on the stored channel says which).
    adoptRenames: async (models) => {
      try {
        const res = await adoptChannelRenames(channelRef.current.id, models);
        if (!res?.success) {
          toast.error(res?.message || t('channels.models.adoptFailed'));
          return;
        }
        toast.success(t('channels.models.adopted', { count: res.data?.switched || 0 }));
        await load();
        onChanged();
      } catch (e) {
        toast.error(e.message);
      }
    },
    allChannels,
    groups,
    supplyRouting,
    metricsRows,
    rangeKey,
    actions,
    modelsFocus,
    busy: {
      refreshing: busyIds.refreshing.has(channelId),
      drift: busyIds.drift.has(channelId),
      dismissing: busyIds.dismissing.has(channelId)
    }
  };
  // Renaming from the title or the menu saves the name alone, apart from the tabs' drafts.
  const rename = async (name) => {
    try {
      const res = await updateChannel(channel, { name });
      if (!res?.success) {
        toast.error(res?.message || t('channels.panel.renameFailed'));
        return false;
      }
      toast.success(t('channels.panel.renamed'));
      await load();
      onChanged();
      return true;
    } catch (e) {
      toast.error(e.message);
      return false;
    }
  };

  const host = channel ? hostOf(channel.base_url || DEFAULT_BASE_URLS[channel.type] || '') : '';
  const webUrl = channel ? consoleUrl(channel.type) : '';
  const testing = busyIds.testing.has(channelId);

  // What needs doing on this channel, the same items as the tags on its list row, spelled out.
  const issueText = (issue) => {
    const c = issue.channel;
    if (issue.kind === 'lowBalance')
      return t('channels.issue.lowBalance', {
        balance: formatBalance(c.type, c.balance),
        threshold: formatBalance(c.type, c.balance_threshold)
      });
    if (issue.kind === 'retired') {
      const { date, successor } = RETIRED_CHANNEL_TYPES[c.type];
      const gone = t('channels.issue.retired', { date });
      return successor ? `${gone} ${t(`channels.retired.successor.${successor}`)}` : `${gone} ${t('channels.retired.moveElsewhere')}`;
    }
    if (issue.kind === 'driftMissing' || issue.kind === 'driftRenamed') return t(`channels.issue.${issue.kind}`, { count: issue.count });
    return t(`channels.issue.${issue.kind}`);
  };
  const issueActionText = {
    retired: null,
    autoDisabled: 'enable',
    keyMissing: 'setKey',
    lowBalance: 'refreshBalance',
    driftMissing: 'view',
    driftRenamed: 'view'
  };
  // The models tab shows the upstream's gone models in its own filter, so the notice leaves them
  // out there.
  const currentTab = tab || PANEL_TABS[0].key;
  const shown = currentTab === 'models' ? issues.filter((i) => i.kind !== 'driftMissing' && i.kind !== 'driftRenamed') : issues;
  const notice = shown.length ? (
    <ul
      className="mb-3 space-y-1 rounded-[10px] border border-warning/40 bg-warning/10 px-3 py-2 text-[13px]"
      aria-label={t('channels.issue.title')}
    >
      {shown.map((issue) => (
        <li key={issue.key} className="flex items-center gap-2">
          <AlertTriangle className="size-3.5 shrink-0 text-warning" />
          <span className="min-w-0 flex-1">{issueText(issue)}</span>
          {onIssueAction && issueActionText[issue.kind] && (
            <button
              type="button"
              disabled={issueBusy?.(issue)}
              onClick={() => onIssueAction(issue)}
              className="shrink-0 font-medium text-info underline-offset-2 hover:underline disabled:opacity-60"
            >
              {t(`channels.issue.${issueActionText[issue.kind]}`)}
            </button>
          )}
        </li>
      ))}
    </ul>
  ) : null;

  return (
    <ObjectPanel
      titleId="channel-panel-title"
      size="xl"
      object={channel}
      error={error}
      errorTitle={t('channelDetail.notFound')}
      icon={channel && <ProviderTile type={channel.type} size="lg" />}
      title={
        channel && (
          <EditableTitle
            value={channel.name}
            editing={renaming}
            onEditingChange={setRenaming}
            onSave={rename}
            label={t('channels.action.rename')}
          />
        )
      }
      subtitle={channel && [`#${channel.id}`, providerName(channel.type), host].filter(Boolean).join(' · ')}
      subtitleMono
      actions={
        channel && (
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
              <DropdownMenuContent align="end" className="w-64">
                <DropdownMenuItem onClick={() => setRenaming(true)}>
                  <Pencil className="size-4" />
                  {t('channels.action.rename')}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="items-start" onClick={() => actions.onMultiTest(channel)}>
                  <Gauge className="mt-0.5 size-4" />
                  <span className="flex flex-col">
                    {t('channels.menu.multiTest')}
                    <span className="text-xs text-muted-foreground">{t('channels.menu.multiTestHint')}</span>
                  </span>
                </DropdownMenuItem>
                <DropdownMenuItem className="items-start" onClick={() => actions.onCheck(channel)}>
                  <FlaskConical className="mt-0.5 size-4" />
                  <span className="flex flex-col">
                    {t('channels.menu.check')}
                    <span className="text-xs text-muted-foreground">{t('channels.menu.checkHint')}</span>
                  </span>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => actions.onRefreshBalance(channel)}>
                  <RefreshCw className="size-4" />
                  {t('channels.conn.refreshBalance')}
                </DropdownMenuItem>
                <DropdownMenuItem className="items-start" onClick={() => actions.onCheckDrift(channel)}>
                  <ListChecks className="mt-0.5 size-4" />
                  <span className="flex flex-col">
                    {t('channels.menu.drift')}
                    <span className="text-xs text-muted-foreground">{t('channels.menu.driftHint')}</span>
                  </span>
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
        )
      }
      tabs={tabs}
      tab={tab}
      onTabChange={onTabChange}
      onClose={onClose}
      onSave={async (patch) => {
        const res = await updateChannel(channel, patch);
        // While supplies route requests, edits of the model list and mapping reach the supplies
        // made from it: say what changed.
        if (!res?.success) return res;
        const parts = [
          res.supplies && t('channels.models.saved.attached', { count: res.supplies }),
          res.supplies_updated && t('channels.models.saved.updated', { count: res.supplies_updated }),
          res.supplies_removed && t('channels.models.saved.removed', { count: res.supplies_removed })
        ].filter(Boolean);
        return parts.length
          ? { ...res, toast: t('channels.models.saved.summary', { changes: parts.join(t('channels.created.comma')) }) }
          : res;
      }}
      onSaved={async () => {
        await load();
        onChanged();
      }}
      ctx={ctx}
      tabsLabel={t('channels.panel.tabs')}
      scope={t('channels.panel.scope')}
      notice={notice}
    />
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
  supplyRouting: PropTypes.bool,
  metricsRows: PropTypes.array.isRequired,
  rangeKey: PropTypes.string.isRequired,
  actions: PropTypes.object.isRequired,
  busyIds: PropTypes.object.isRequired,
  issues: PropTypes.array,
  modelsFocus: PropTypes.shape({ filter: PropTypes.string, n: PropTypes.number }),
  onIssueAction: PropTypes.func,
  issueBusy: PropTypes.func
};
