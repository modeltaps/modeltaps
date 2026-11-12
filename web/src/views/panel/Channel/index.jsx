import { useCallback, useEffect, useMemo, useState } from 'react';
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, DollarSign, FlaskConical, Layers, MoreHorizontal, Play, Plus, RefreshCw, Trash2 } from 'lucide-react';

import { API } from 'utils/api';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/ui/sonner';
import { ListEmptyState, ListSkeleton, PHONE_QUERY, useColumnVisibility } from '@/components/list-page';
import { Skeleton } from '@/components/ui/skeleton';
import useMediaQuery from 'hooks/useMediaQuery';
import PageActions from '@/components/chrome/PageActions';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import useChannelActions from './useChannelActions';
import BatchModal from './BatchModal';
import ProviderStrip from './ProviderStrip';
import { useProviderName } from './channelParts';
import NewChannelMenu from './NewChannelMenu';
import ProviderPane, { CHANNEL_COLUMNS, RANGES } from './ProviderPane';
import ChannelPanel from './ChannelPanel';
import NewChannelSheet from './NewChannelSheet';
import { LEGACY_SECTION_TAB, PANEL_TABS } from './panelTabs';
import { ISSUE_FILTERS, NO_TAG, STATUS_FILTERS, channelMatches, groupChannelsByType } from './channelGroups';
import useBatchActions from './useBatchActions';
import { checkChannelDrift, fetchAllChannels, fetchChannelMetrics, fetchGroupOptions, fetchModels, manageChannel } from './channelApi';
import { fetchAllocation, saveAllocation } from '../Allocation/allocationApi';
import {
  joinPosition,
  moveBefore,
  moveStep,
  orderFromPolicy,
  orderInfo,
  orderToTiers,
  setWeight,
  shareWithAbove,
  stopSharing
} from './channelOrder';

const DISMISSED_KEY = 'channels.dismissedIssues';

function loadDismissed() {
  try {
    return new Set(JSON.parse(sessionStorage.getItem(DISMISSED_KEY) || '[]'));
  } catch {
    return new Set();
  }
}

// Tab of the channel panel named by the URL: ?tab=<key>, or the sections of the former
// settings form (?view=settings&section=...).
function panelTabFrom(params) {
  const tab = params.get('tab');
  if (PANEL_TABS.some((x) => x.key === tab)) return tab;
  if (tab === 'config') return 'connection';
  if (params.get('view') === 'settings') return LEGACY_SECTION_TAB[params.get('section')] || 'connection';
  return 'models';
}

// /panel/channel: a row of provider cards (each with its traffic) above one channel list. While
// supplies route requests the list is the call order (tried top to bottom, a shared position
// splits traffic by weight), saved as the platform allocation policy. A card filters the list to
// one provider (?type=<id>); /panel/channel/:id opens a channel's panel, /panel/channel/new
// the new channel sheet for ?type.
export default function Channel() {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const navigate = useNavigate();
  const { id: idParam } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const isNew = idParam === 'new';
  const openId = !isNew && idParam ? Number(idParam) || 0 : 0;
  const legacyId = Number(searchParams.get('channel_id')) || 0;
  const typeParam = Number(searchParams.get('type')) || 0;
  const tab = panelTabFrom(searchParams);

  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [groups, setGroups] = useState([]);
  // While supplies route requests the allocation policy orders channels, not their own numbers.
  const [supplyRouting, setSupplyRouting] = useState(false);
  // { tiers, providers } of the allocation policy, read while supplies route requests.
  const [allocation, setAllocation] = useState(null);
  const [orderGroups, setOrderGroups] = useState(null);
  const [rangeKey, setRangeKey] = useState('24h');
  const [metrics, setMetrics] = useState({ loading: true, rows: [], start: 0, end: 0 });
  const [dismissed, setDismissed] = useState(loadDismissed);
  const [created, setCreated] = useState(null);
  const [highlightIds, setHighlightIds] = useState(() => new Set());
  const [refreshingModels, setRefreshingModels] = useState(false);
  const [testingAll, setTestingAll] = useState(false);
  const [batchOpen, setBatchOpen] = useState(false);
  const [modelOptions, setModelOptions] = useState([]);
  const [metricsTick, setMetricsTick] = useState(0);

  const reload = useCallback(async () => {
    try {
      setChannels(await fetchAllChannels());
      setLoadError('');
    } catch (e) {
      setLoadError(e.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    reload();
    fetchGroupOptions().then(setGroups);
    fetchAllocation()
      .then((res) => {
        const on = Boolean(res.success && res.data?.supply_routing);
        setSupplyRouting(on);
        setAllocation(on ? res.data : null);
      })
      .catch(() => setSupplyRouting(false));
  }, [reload]);

  // The order shown: the policy expanded into channels, every channel placed (new ones last).
  const policyOrder = useMemo(
    () => (allocation ? orderFromPolicy(allocation.tiers || [], allocation.providers || [], channels) : null),
    [allocation, channels]
  );
  const groupsShown = orderGroups || policyOrder;
  const order = useMemo(
    () =>
      groupsShown
        ? {
            groups: groupsShown,
            info: orderInfo(groupsShown, allocation?.providers || []),
            names: new Map(channels.map((c) => [c.id, c.name]))
          }
        : null,
    [groupsShown, allocation, channels]
  );
  const saveOrder = async (next) => {
    setOrderGroups(next);
    try {
      const res = await saveAllocation(orderToTiers(next));
      if (!res.success) {
        toast.error(res.message);
        setOrderGroups(null);
        return;
      }
      setAllocation(res.data);
      setOrderGroups(null);
    } catch {
      setOrderGroups(null);
    }
  };
  const orderActions = order
    ? {
        step: (id, dir) => saveOrder(moveStep(order.groups, id, dir)),
        share: (id) => saveOrder(shareWithAbove(order.groups, id)),
        unshare: (id) => saveOrder(stopSharing(order.groups, id)),
        weight: (id, w) => saveOrder(setWeight(order.groups, id, w)),
        drop: (id, beforeId) => saveOrder(moveBefore(order.groups, id, beforeId)),
        join: (id, targetId) => saveOrder(joinPosition(order.groups, id, targetId))
      }
    : null;

  // Search (?q=) across every provider, and the table filters (?status=, ?tag= / ?untagged=1).
  const query = searchParams.get('q') || '';
  const statusFilter = STATUS_FILTERS[searchParams.get('status')] ? searchParams.get('status') : '';
  const tagFilter = searchParams.get('untagged') === '1' ? NO_TAG : searchParams.get('tag') || '';
  const groupFilter = searchParams.get('group') || '';
  const modelFilter = searchParams.get('model') || '';
  const issueFilter = ISSUE_FILTERS.includes(searchParams.get('issue')) ? searchParams.get('issue') : '';
  const filters = useMemo(
    () => ({ status: statusFilter, tag: tagFilter, group: groupFilter, model: modelFilter, issue: issueFilter }),
    [statusFilter, tagFilter, groupFilter, modelFilter, issueFilter]
  );

  const providerGroups = useMemo(() => groupChannelsByType(channels), [channels]);
  const visibleGroups = useMemo(
    () =>
      query.trim()
        ? providerGroups
            .map((g) => ({ ...g, channels: g.channels.filter((c) => channelMatches(c, query)) }))
            .filter((g) => g.channels.length)
        : providerGroups,
    [providerGroups, query]
  );
  // 0 = every provider; a card picks one (?type=).
  const selectedType = typeParam && visibleGroups.some((g) => g.type === typeParam) ? typeParam : 0;
  const selected = selectedType ? visibleGroups.find((g) => g.type === selectedType) : null;
  const visibleChannels = useMemo(() => visibleGroups.flatMap((g) => g.channels), [visibleGroups]);
  const selectedChannels = useMemo(() => (selected ? selected.channels : visibleChannels), [selected, visibleChannels]);
  const range = RANGES.find((r) => r.key === rangeKey) || RANGES[0];

  // Metrics of every channel for the range (hourly rows per channel × model): the cards show
  // each provider's traffic, the list and chart the selected ones.
  const metricIds = channels.map((c) => c.id).join(',');
  useEffect(() => {
    if (!metricIds) {
      setMetrics({ loading: false, rows: [], start: 0, end: 0 });
      return undefined;
    }
    let alive = true;
    const end = Math.floor(Date.now() / 1000);
    const start = end - range.seconds;
    setMetrics((m) => ({ ...m, loading: true }));
    Promise.all(metricIds.split(',').map((id) => fetchChannelMetrics(Number(id), start, end).catch(() => [])))
      .then((lists) => alive && setMetrics({ loading: false, rows: lists.flat(), start, end }))
      .catch(() => alive && setMetrics({ loading: false, rows: [], start, end }));
    return () => {
      alive = false;
    };
  }, [metricIds, range.seconds, metricsTick]);

  // Links keep the provider, the search and the filters.
  const withType = (path, type = selectedType, extra = {}) => {
    const p = new URLSearchParams();
    if (type) p.set('type', String(type));
    if (query) p.set('q', query);
    if (type === selectedType) {
      if (statusFilter) p.set('status', statusFilter);
      if (tagFilter === NO_TAG) p.set('untagged', '1');
      else if (tagFilter) p.set('tag', tagFilter);
      if (groupFilter) p.set('group', groupFilter);
      if (modelFilter) p.set('model', modelFilter);
      if (issueFilter) p.set('issue', issueFilter);
    }
    for (const [k, v] of Object.entries(extra)) if (v) p.set(k, v);
    const q = p.toString();
    return `${path}${q ? `?${q}` : ''}`;
  };
  const openChannel = (id, nextTab) =>
    navigate(withType(`/panel/channel/${id}`, selectedType, { tab: nextTab && nextTab !== 'models' ? nextTab : '' }));
  const closeOverlay = () => navigate(withType('/panel/channel'));
  const openNew = (type = selectedType) => navigate(withType('/panel/channel/new', type || 1));
  const selectType = (type) => {
    const p = {};
    if (type) p.type = String(type);
    if (query) p.q = query;
    setSearchParams(p);
  };
  const updateParams = useCallback(
    (patch) =>
      setSearchParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          for (const [k, v] of Object.entries(patch)) v ? p.set(k, v) : p.delete(k);
          return p;
        },
        { replace: true }
      ),
    [setSearchParams]
  );
  const setQuery = useCallback(
    (q) => updateParams({ q, status: '', tag: '', untagged: '', group: '', model: '', issue: '' }),
    [updateParams]
  );
  const setFilters = ({ status = '', tag = '', group = '', model = '', issue = '' }) =>
    updateParams({ status, tag: tag && tag !== NO_TAG ? tag : '', untagged: tag === NO_TAG ? '1' : '', group, model, issue });

  const columns = useColumnVisibility({ storageKey: 'channel-column-visibility-v1', columns: CHANNEL_COLUMNS });
  // Phones get the compact channel list instead of the table.
  const phone = useMediaQuery(PHONE_QUERY);
  const layout = phone ? 'list' : 'split';

  const {
    handlers: actions,
    testingIds,
    driftCheckingIds,
    dismissingIds,
    refreshingIds,
    requestConfirm,
    dialogs
  } = useChannelActions({
    onChanged: () => {
      reload();
      setMetricsTick((x) => x + 1);
    },
    onDriftDismissed: (row, drift) => setChannels((prev) => prev.map((c) => (c.id === row.id ? { ...c, model_drift: drift } : c))),
    onDeleted: (row) => {
      reload();
      if (row.id === openId) closeOverlay();
    }
  });

  const {
    batch,
    tag: tagActions,
    busy: batchBusy,
    dialogs: batchDialogs
  } = useBatchActions({
    groups,
    requestConfirm,
    onDone: () => {
      reload();
      setMetricsTick((x) => x + 1);
    }
  });

  const dismissIssue = (issue) => {
    if (issue.kind === 'driftNew') {
      actions.onDismissNewModels(issue.channel);
      return;
    }
    setDismissed((prev) => {
      const next = new Set(prev).add(issue.key);
      try {
        sessionStorage.setItem(DISMISSED_KEY, JSON.stringify([...next]));
      } catch {
        // storage full or blocked: the dismissal lasts until reload
      }
      return next;
    });
  };
  const issueAction = (issue) => {
    const c = issue.channel;
    if (issue.kind === 'autoDisabled') actions.onToggleStatus(c);
    else if (issue.kind === 'keyMissing') openChannel(c.id, 'connection');
    else if (issue.kind === 'lowBalance') actions.onRefreshBalance(c);
    else openChannel(c.id, 'models');
  };
  const issueBusy = (issue) =>
    (issue.kind === 'lowBalance' && refreshingIds.has(issue.channel.id)) ||
    (issue.kind === 'driftNew' && dismissingIds.has(issue.channel.id));

  // Refresh the model list of every enabled channel of the provider (the drift check reads the
  // upstream list and, where auto-add is on, adds what is new).
  const refreshModels = async () => {
    const list = selectedChannels.filter((c) => c.status === 1);
    if (!list.length || refreshingModels) return;
    setRefreshingModels(true);
    try {
      const results = await Promise.allSettled(list.map((c) => checkChannelDrift(c.id)));
      const failed = results.filter((r) => r.status !== 'fulfilled' || !r.value?.success).length;
      failed
        ? toast.warning(t('channels.pane.refreshPartial', { failed, count: list.length }))
        : toast.success(t('channels.pane.refreshDone', { count: list.length }));
      reload();
    } finally {
      setRefreshingModels(false);
    }
  };

  const testAllOfProvider = async () => {
    const list = selectedChannels.filter((c) => c.status === 1);
    if (!list.length || testingAll) return;
    setTestingAll(true);
    try {
      const results = await Promise.allSettled(list.map((c) => manageChannel(c.id, 'test', c.test_model)));
      const failed = results.filter((r) => r.status !== 'fulfilled' || !r.value?.success).length;
      failed
        ? toast.warning(t('channels.pane.testPartial', { failed, count: list.length }))
        : toast.success(t('channels.pane.testDone', { count: list.length }));
      reload();
    } finally {
      setTestingAll(false);
    }
  };

  const callAll = (url, method, okText) => async () => {
    try {
      const res = await API[method](url);
      if (res.data.success) {
        toast.info(okText(res.data));
        reload();
      } else toast.error(res.data.message);
    } catch (e) {
      toast.error(e.message);
    }
  };
  const maintenance = [
    {
      icon: RefreshCw,
      label: t(selectedType ? 'channels.pane.refreshModelsOf' : 'channels.pane.refreshModels', { name: providerName(selectedType) }),
      run: refreshModels
    },
    {
      icon: Play,
      label: t(selectedType ? 'channels.pane.testAllOf' : 'channels.pane.testAll', { name: providerName(selectedType) }),
      run: testAllOfProvider
    },
    'sep',
    {
      icon: Layers,
      label: t('channel_index.batchProcessing'),
      run: () => {
        if (!modelOptions.length) fetchModels().then(setModelOptions);
        setBatchOpen(true);
      }
    },
    'sep',
    {
      icon: FlaskConical,
      label: t('channel_index.testAllChannels'),
      confirm: true,
      run: callAll('/api/channel/test', 'get', () => t('channel_row.testAllChannel'))
    },
    {
      icon: DollarSign,
      label: t('channel_index.updateEnabledBalance'),
      confirm: true,
      run: callAll('/api/channel/update_balance', 'get', () => t('channel_row.updateChannelBalance'))
    },
    {
      icon: AlertTriangle,
      label: t('channel_index.checkAllDrift'),
      confirm: true,
      run: async () => {
        const { success, message, data } = await checkChannelDrift();
        if (success) {
          toast.info(t('channel_index.checkDriftDone', { checked: data?.checked ?? 0, drifted: data?.drifted ?? 0 }));
          reload();
        } else if (message) toast.error(message);
      }
    },
    'sep',
    {
      icon: Trash2,
      label: t('channel_index.deleteDisabledChannels'),
      danger: true,
      confirm: true,
      run: callAll('/api/channel/disabled', 'delete', (data) => t('channel_row.delChannelCount', { count: data.data }))
    }
  ];

  const onCreated = ({ ids, type, name, summary }) => {
    setCreated({ ids, type, name, summary });
    setHighlightIds(new Set(ids));
    navigate(withType('/panel/channel', selectedType ? type : 0));
    reload();
  };

  if (legacyId) return <Navigate to={`/panel/channel/${legacyId}`} replace />;

  const pageActions = (
    <PageActions>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="icon" className="size-9 rounded-lg" aria-label={t('channels.more')}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          {maintenance.map((m, i) =>
            m === 'sep' ? (
              <DropdownMenuSeparator key={`sep-${i}`} />
            ) : (
              <DropdownMenuItem
                key={m.label}
                className={m.danger ? 'text-destructive' : undefined}
                onClick={() => (m.confirm ? requestConfirm(m.label, '', m.run) : m.run())}
              >
                <m.icon className="size-4" />
                {m.label}
              </DropdownMenuItem>
            )
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <NewChannelMenu onPick={(type) => openNew(type)} present={providerGroups.map((g) => g.type)} />
    </PageActions>
  );

  let body;
  if (loading) {
    // Shaped like the page: the provider cards, the chart and the channel table.
    body = (
      <div className="space-y-5" aria-busy="true">
        <Skeleton className="h-9 w-80 rounded-lg" />
        <div className="flex gap-2.5">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-[84px] w-52 rounded-xl" />
          ))}
        </div>
        <Skeleton className="h-56 w-full rounded-2xl" />
        <div className="overflow-hidden rounded-2xl border border-border bg-card">
          <ListSkeleton rows={3} cells={[72, 80, 72, 88, 32]} />
        </div>
      </div>
    );
  } else if (loadError && !channels.length) {
    body = (
      <div className="py-16 text-center">
        <p className="text-sm text-muted-foreground">{loadError}</p>
        <Button variant="outline" className="mt-4 rounded-lg" onClick={reload}>
          {t('channels.retry')}
        </Button>
      </div>
    );
  } else if (!providerGroups.length) {
    body = (
      <ListEmptyState
        title={t('channelPage.emptyTitle')}
        description={t('channelPage.emptyDescription')}
        action={
          <Button size="sm" className="rounded-lg" onClick={() => openNew(1)}>
            <Plus /> {t('channels.newChannel')}
          </Button>
        }
      />
    );
  } else {
    body = (
      <div>
        <ProviderPane
          key={`${selectedType}|${query}`}
          filters={filters}
          onFiltersChange={setFilters}
          batch={batch}
          tagActions={tagActions}
          batchBusy={batchBusy}
          type={selectedType}
          channels={selectedChannels}
          metrics={metrics}
          rangeKey={rangeKey}
          onRangeChange={setRangeKey}
          dismissed={dismissed}
          onDismissIssue={dismissIssue}
          onIssueAction={issueAction}
          issueBusy={issueBusy}
          created={created && (!selectedType || created.type === selectedType) ? created : null}
          onAdjustCreated={() => created && openChannel(created.ids[0], 'routing')}
          onDismissCreated={() => {
            setCreated(null);
            setHighlightIds(new Set());
          }}
          highlightIds={highlightIds}
          onOpen={(id) => openChannel(id)}
          layout={layout}
          columns={columns}
          actions={actions}
          testingIds={testingIds}
          order={order}
          orderActions={query.trim() ? null : orderActions}
          leading={
            <ProviderStrip
              groups={visibleGroups}
              selected={selectedType}
              onSelect={selectType}
              dismissed={dismissed}
              query={query}
              onQueryChange={setQuery}
            />
          }
        />
      </div>
    );
  }

  return (
    <>
      {pageActions}
      {body}

      {openId > 0 && (
        <ChannelPanel
          key={openId}
          channelId={openId}
          tab={tab}
          onTabChange={(next) => {
            const p = new URLSearchParams(searchParams);
            ['view', 'section'].forEach((k) => p.delete(k));
            next === 'models' ? p.delete('tab') : p.set('tab', next);
            setSearchParams(p, { replace: true });
          }}
          onClose={closeOverlay}
          onChanged={() => {
            reload();
            setMetricsTick((x) => x + 1);
          }}
          allChannels={channels}
          groups={groups}
          supplyRouting={supplyRouting}
          metricsRows={metrics.rows}
          rangeKey={rangeKey}
          actions={actions}
          busyIds={{ testing: testingIds, drift: driftCheckingIds, dismissing: dismissingIds, refreshing: refreshingIds }}
        />
      )}

      {isNew && !loading && (
        <NewChannelSheet
          initialType={typeParam || selectedType || 1}
          allChannels={channels}
          supplyRouting={supplyRouting}
          onClose={closeOverlay}
          onCreated={onCreated}
        />
      )}

      {dialogs}
      {batchDialogs}

      <BatchModal open={batchOpen} onOpenChange={setBatchOpen} groupOptions={groups.map((g) => g.symbol)} modelOptions={modelOptions} />
    </>
  );
}
