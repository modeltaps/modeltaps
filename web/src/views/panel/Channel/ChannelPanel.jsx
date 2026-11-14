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
import { ObjectPanel } from '@/components/objects';
import { fetchChannel, updateChannel } from './channelApi';
import { ProviderTile, useProviderName } from './channelParts';
import { formatBalance } from './channelMetrics';
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
  onIssueAction,
  issueBusy
}) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const [channel, setChannel] = useState(null);
  const [error, setError] = useState('');
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

  const ctx = {
    t,
    allChannels,
    groups,
    supplyRouting,
    metricsRows,
    rangeKey,
    actions,
    busy: {
      refreshing: busyIds.refreshing.has(channelId),
      drift: busyIds.drift.has(channelId),
      dismissing: busyIds.dismissing.has(channelId)
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
    if (issue.kind === 'driftNew' || issue.kind === 'driftMissing') return t(`channels.issue.${issue.kind}`, { count: issue.count });
    return t(`channels.issue.${issue.kind}`);
  };
  const issueActionText = {
    autoDisabled: 'enable',
    keyMissing: 'setKey',
    lowBalance: 'refreshBalance',
    driftNew: 'view',
    driftMissing: 'view'
  };
  const notice = issues.length ? (
    <ul
      className="mb-3 space-y-1 rounded-[10px] border border-warning/40 bg-warning/10 px-3 py-2 text-[13px]"
      aria-label={t('channels.issue.title')}
    >
      {issues.map((issue) => (
        <li key={issue.key} className="flex items-center gap-2">
          <AlertTriangle className="size-3.5 shrink-0 text-warning" />
          <span className="min-w-0 flex-1">{issueText(issue)}</span>
          {onIssueAction && (
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
      object={channel}
      error={error}
      errorTitle={t('channelDetail.notFound')}
      icon={channel && <ProviderTile type={channel.type} size="lg" />}
      title={channel?.name}
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
        // While supplies route requests, models added in this save are attached on their own.
        return res?.success && res.supplies ? { ...res, toast: t('channels.models.savedSupplied', { count: res.supplies }) } : res;
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
  onIssueAction: PropTypes.func,
  issueBusy: PropTypes.func
};
