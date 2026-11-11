import { useCallback, useEffect, useMemo, useState } from 'react';
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, DollarSign, FlaskConical, Layers, MoreHorizontal, Plus, Trash2 } from 'lucide-react';

import { API } from 'utils/api';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/ui/sonner';
import { ListEmptyState, ListSkeleton, useColumnVisibility } from '@/components/list-page';
import { Skeleton } from '@/components/ui/skeleton';
import { useContentMinWidth } from '@/components/chrome/ContentWidth';
import useElementWidth from 'hooks/useElementWidth';
import PageActions from '@/components/chrome/PageActions';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { COMMON_CHANNEL_TYPES } from 'constants/ChannelConstants';
import useChannelActions from './useChannelActions';
import BatchModal from './BatchModal';
import ProviderLane from './ProviderLane';
import ProviderPane, { CHANNEL_COLUMNS, RANGES, estimateChannelTableWidth } from './ProviderPane';
import ProviderSwitcher from './ProviderSwitcher';
import ChannelPanel from './ChannelPanel';
import NewChannelSheet from './NewChannelSheet';
import { LEGACY_SECTION_TAB, PANEL_TABS } from './panelTabs';
import { ISSUE_FILTERS, NO_TAG, STATUS_FILTERS, channelMatches, groupChannelsByType } from './channelGroups';
import useBatchActions from './useBatchActions';
import { checkChannelDrift, fetchAllChannels, fetchChannelMetrics, fetchGroupOptions, fetchModels, manageChannel } from './channelApi';

const DISMISSED_KEY = 'channels.dismissedIssues';

// The provider lane is a 300px column with a 24px gap beside the channel table.
const LANE_WIDTH = 300 + 24;
// split: lane beside the table; stacked: the lane folds into a switcher above a full-width
// table; list: the phone list. `area` is the page width, `table` the table's natural width.
export const channelLayout = (area, table) => {
  if (!area || area >= LANE_WIDTH + table + 2) return 'split';
  return area >= table + 2 ? 'stacked' : 'list';
};

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

// /panel/channel: providers on the left (channels grouped by type), the selected provider on
// the right (attention list, traffic chart, channel table). /panel/channel/:id opens that
// channel's panel over its provider; /panel/channel/new opens the new channel sheet. The
// selected provider is ?type=<id>.
export default function Channel() {
  const { t } = useTranslation();
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
  }, [reload]);

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
  const openRow = openId ? channels.find((c) => c.id === openId) : null;
  const selectedType =
    (typeParam && visibleGroups.some((g) => g.type === typeParam) && typeParam) ||
    (openRow && visibleGroups.some((g) => g.type === openRow.type) && openRow.type) ||
    visibleGroups[0]?.type ||
    (query.trim() ? 0 : typeParam) ||
    0;
  const selected = visibleGroups.find((g) => g.type === selectedType);
  const selectedChannels = useMemo(() => selected?.channels || [], [selected]);
  const range = RANGES.find((r) => r.key === rangeKey) || RANGES[0];

  // Metrics of the selected provider's channels for the range (hourly rows per channel × model).
  const metricIds = selectedChannels.map((c) => c.id).join(',');
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
  const openNewOther = () => {
    const present = new Set(providerGroups.map((g) => g.type));
    openNew(COMMON_CHANNEL_TYPES.find((x) => !present.has(x)) || 1);
  };
  const selectType = (type) => setSearchParams(query ? { type: String(type), q: query } : { type: String(type) });
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

  // Make room before folding anything: the shell collapses the sidebar while the lane and the
  // table's natural width do not fit; then the lane folds into a switcher; then the phone list.
  const columns = useColumnVisibility({ storageKey: 'channel-column-visibility-v1', columns: CHANNEL_COLUMNS });
  const [pageRef, pageWidth] = useElementWidth();
  const [measuredTable, setMeasuredTable] = useState(null);
  const columnsKey = JSON.stringify(columns.visibility);
  useEffect(() => {
    setMeasuredTable(null);
  }, [columnsKey]);
  const tableWidth = measuredTable ?? estimateChannelTableWidth(columns.visibility);
  useContentMinWidth(LANE_WIDTH + tableWidth + 2);
  const layout = channelLayout(pageWidth, tableWidth);

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
    navigate(withType('/panel/channel', type));
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
      <Button className="rounded-lg" onClick={() => openNew()}>
        <Plus />
        {t('channels.newChannel')}
      </Button>
    </PageActions>
  );

  let body;
  if (loading) {
    // Shaped like the page: the provider lane beside a header, the chart and the channel table.
    body = (
      <div className="grid items-start gap-6 lg:grid-cols-[300px_minmax(0,1fr)]" aria-busy="true">
        <div className="hidden space-y-2 rounded-2xl bg-muted/60 p-3 lg:block">
          <Skeleton className="h-9 w-full rounded-lg" />
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-10 w-full rounded-lg" />
          ))}
        </div>
        <div className="space-y-4">
          <div className="flex items-center gap-3">
            <Skeleton className="size-10 rounded-xl" />
            <div className="space-y-2">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-3 w-48" />
            </div>
          </div>
          <Skeleton className="h-56 w-full rounded-2xl" />
          <div className="overflow-hidden rounded-2xl border border-border bg-card">
            <ListSkeleton rows={3} cells={[72, 80, 72, 88, 32]} />
          </div>
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
    const laneProps = {
      groups: visibleGroups,
      selected: selectedType,
      onSelect: selectType,
      dismissed,
      onNewOther: openNewOther,
      query,
      onQueryChange: setQuery
    };
    const split = layout === 'split';
    body = (
      <div className={split ? 'grid items-start gap-6 grid-cols-[300px_minmax(0,1fr)]' : 'space-y-4'}>
        {split ? <ProviderLane {...laneProps} /> : !selected && <ProviderSwitcher {...laneProps} />}
        {!selected && query.trim() && (
          <ListEmptyState
            variant="noMatch"
            title={t('channels.search.emptyTitle')}
            description={t('channels.search.emptyDescription', { query })}
            onClearFilters={() => setQuery('')}
          />
        )}
        {selected && (
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
            created={created && created.type === selectedType ? created : null}
            onAdjustCreated={() => created && openChannel(created.ids[0], 'routing')}
            onDismissCreated={() => {
              setCreated(null);
              setHighlightIds(new Set());
            }}
            highlightIds={highlightIds}
            onOpen={(id) => openChannel(id)}
            layout={layout}
            switcher={split ? null : <ProviderSwitcher {...laneProps} />}
            columns={columns}
            onNaturalWidth={setMeasuredTable}
            onRefreshModels={refreshModels}
            onTestAll={testAllOfProvider}
            refreshingModels={refreshingModels}
            testingAll={testingAll}
            actions={actions}
            testingIds={testingIds}
          />
        )}
      </div>
    );
  }

  return (
    <>
      {pageActions}
      <div ref={pageRef}>{body}</div>

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
          groups={groups}
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
