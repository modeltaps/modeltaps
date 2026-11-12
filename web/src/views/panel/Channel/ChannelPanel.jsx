import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Copy, ExternalLink, FlaskConical, Gauge, ListChecks, Loader2, MoreHorizontal, Play, RefreshCw, Trash2 } from 'lucide-react';

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
  busyIds
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
        )
      }
      tabs={tabs}
      tab={tab}
      onTabChange={onTabChange}
      onClose={onClose}
      onSave={(patch) => updateChannel(channel, patch)}
      onSaved={async () => {
        await load();
        onChanged();
      }}
      ctx={ctx}
      tabsLabel={t('channels.panel.tabs')}
      scope={t('channels.panel.scope')}
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
  busyIds: PropTypes.object.isRequired
};
