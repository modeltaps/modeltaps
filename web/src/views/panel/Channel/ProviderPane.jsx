import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, Copy, FlaskConical, Info, Loader2, MoreHorizontal, Play, Plus, RefreshCw, Trash2, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { channelSeries, formatBalance, metricsByChannel } from './channelMetrics';
import { channelDot, filterChannels, openIssues, selectionState, statusCounts, tagOptions } from './channelGroups';
import { Checkbox } from '@/components/ui/checkbox';
import ChannelToolbar from './ChannelToolbar';
import { Dot, ProviderTile, Segmented, useProviderName } from './channelParts';
import TrafficChart from './TrafficChart';

export const RANGES = [
  { key: '24h', seconds: 24 * 3600, bucket: 3600 },
  { key: '7d', seconds: 7 * 24 * 3600, bucket: 6 * 3600 },
  { key: '30d', seconds: 30 * 24 * 3600, bucket: 24 * 3600 }
];

const pct = (v) => (v === null || v === undefined ? null : `${(v * 100).toFixed(1)}%`);
const sec = (v) => (v === null || v === undefined ? null : `${(v / 1000).toFixed(2)} s`);

function Muted({ children = '—', title }) {
  return (
    <span className="text-muted-foreground" title={title}>
      {children}
    </span>
  );
}

Muted.propTypes = { children: PropTypes.node, title: PropTypes.string };

// Balance or subscription usage of a channel, for the table.
export function BalanceText({ channel }) {
  const { t } = useTranslation();
  const windows = channel.usage_windows?.windows || [];
  if (windows.length) {
    const top = windows.reduce((a, b) => (b.used_percent > a.used_percent ? b : a), windows[0]);
    return <span>{t('channels.table.usageUsed', { pct: Math.round(top.used_percent) })}</span>;
  }
  if (channel.balance_updated_time) return <span>{formatBalance(channel.type, channel.balance)}</span>;
  return <Muted title={t('channels.table.balanceUnknown')} />;
}

BalanceText.propTypes = { channel: PropTypes.object.isRequired };

// Attention row: what is wrong, one action, dismiss.
function IssueRow({ issue, onAction, onDismiss, busy }) {
  const { t } = useTranslation();
  const c = issue.channel;
  const text = {
    autoDisabled: t('channels.issue.autoDisabled'),
    keyMissing: t('channels.issue.keyMissing'),
    lowBalance: t('channels.issue.lowBalance', {
      balance: formatBalance(c.type, c.balance),
      threshold: formatBalance(c.type, c.balance_threshold)
    }),
    driftNew: t('channels.issue.driftNew', { count: issue.count }),
    driftMissing: t('channels.issue.driftMissing', { count: issue.count })
  }[issue.kind];
  const action = {
    autoDisabled: t('channels.issue.enable'),
    keyMissing: t('channels.issue.setKey'),
    lowBalance: t('channels.issue.refreshBalance'),
    driftNew: t('channels.issue.view'),
    driftMissing: t('channels.issue.view')
  }[issue.kind];
  return (
    <div className="flex items-center gap-3 py-2.5 pl-4 pr-2.5 text-[13px] [&+&]:border-t [&+&]:border-warning/30">
      <AlertTriangle className="size-4 shrink-0 text-warning" />
      <span className="min-w-0 flex-1">
        <b className="font-mono font-medium">{c.name}</b> · {text}
      </span>
      <Button variant="outline" size="sm" className="h-[30px] rounded-lg bg-card" disabled={busy} onClick={() => onAction(issue)}>
        {busy && <Loader2 className="size-3.5 animate-spin" />}
        {action}
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="size-[30px] rounded-lg"
        aria-label={t('channels.issue.dismiss')}
        onClick={() => onDismiss(issue)}
      >
        <X className="size-4" />
      </Button>
    </div>
  );
}

IssueRow.propTypes = { issue: PropTypes.object.isRequired, onAction: PropTypes.func, onDismiss: PropTypes.func, busy: PropTypes.bool };

// The selected provider: header with totals and actions, attention list, one traffic chart with
// a line per channel and the channel table. A row opens the channel panel.
export default function ProviderPane({
  type,
  channels,
  metrics,
  rangeKey,
  onRangeChange,
  dismissed,
  onDismissIssue,
  onIssueAction,
  issueBusy,
  created,
  onAdjustCreated,
  onDismissCreated,
  highlightIds,
  onOpen,
  onNew,
  onRefreshModels,
  onTestAll,
  refreshingModels,
  testingAll,
  actions,
  testingIds,
  filters,
  onFiltersChange,
  batch,
  tagActions,
  batchBusy
}) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const [hoverId, setHoverId] = useState(null);
  const range = RANGES.find((r) => r.key === rangeKey) || RANGES[0];
  const rangeLabel = t(`channels.range.${range.key}`);

  const ids = useMemo(() => channels.map((c) => c.id), [channels]);
  const perChannel = useMemo(() => metricsByChannel(metrics.rows), [metrics.rows]);
  const points = useMemo(
    () => (metrics.end ? channelSeries(metrics.rows, ids, metrics.start, metrics.end, range.bucket) : []),
    [metrics.rows, ids, metrics.start, metrics.end, range.bucket]
  );
  // Lines only for channels that had traffic in the range, busiest first.
  const charted = useMemo(
    () =>
      channels
        .filter((c) => (perChannel.get(c.id)?.requests || 0) > 0)
        .sort((a, b) => (perChannel.get(b.id)?.requests || 0) - (perChannel.get(a.id)?.requests || 0)),
    [channels, perChannel]
  );
  const total = useMemo(() => channels.reduce((a, c) => a + (perChannel.get(c.id)?.requests || 0), 0), [channels, perChannel]);
  const issues = openIssues(channels, dismissed);
  const name = providerName(type);
  const rows = useMemo(() => filterChannels(channels, filters), [channels, filters]);
  const counts = useMemo(() => statusCounts(channels, filters.tag), [channels, filters.tag]);
  const { tags, hasUntagged } = useMemo(() => tagOptions(channels), [channels]);
  // Selection lives here; the page remounts the pane when the provider, query or filters change.
  const [selectedIds, setSelectedIds] = useState(() => new Set());
  const rowIds = rows.map((c) => c.id);
  const headState = selectionState(rowIds, selectedIds);
  const selectedRows = rows.filter((c) => selectedIds.has(c.id));
  const toggleRow = (id) =>
    setSelectedIds((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  const toggleAll = () => setSelectedIds(headState === 'all' ? new Set() : new Set(rowIds));

  return (
    <section aria-label={name} className="min-w-0">
      <div className="flex min-h-11 flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <ProviderTile type={type} size="lg" />
          <div>
            <h2 className="text-[17px] font-medium">{name}</h2>
            <p className="text-[13px] text-muted-foreground">
              {t('channels.pane.summary', { count: channels.length, range: rangeLabel, requests: total.toLocaleString() })}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" className="rounded-lg bg-card" disabled={refreshingModels} onClick={onRefreshModels}>
            {refreshingModels ? <Loader2 className="animate-spin" /> : <RefreshCw />}
            {t('channels.pane.refreshModels')}
          </Button>
          <Button variant="outline" className="rounded-lg bg-card" disabled={testingAll} onClick={onTestAll}>
            {testingAll ? <Loader2 className="animate-spin" /> : <Play />}
            {t('channels.pane.testAll')}
          </Button>
          <Button variant="outline" className="rounded-lg bg-card" onClick={onNew}>
            <Plus />
            {t('channels.newChannel')}
          </Button>
        </div>
      </div>

      {created && (
        <div className="mt-4 flex items-center gap-3 rounded-2xl border border-info/30 bg-[color-mix(in_srgb,var(--info)_7%,var(--card))] py-2.5 pl-4 pr-2.5 text-[13px]">
          <Info className="size-4 shrink-0 text-info" />
          <span className="min-w-0 flex-1">
            {t('channels.created.prefix')} <b className="font-mono font-medium">{created.name}</b>
            {t('channels.created.colon')}
            {created.summary}
          </span>
          <Button variant="outline" size="sm" className="h-[30px] rounded-lg bg-card" onClick={onAdjustCreated}>
            {t('channels.created.adjust')}
          </Button>
          <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('common.close')} onClick={onDismissCreated}>
            <X className="size-4" />
          </Button>
        </div>
      )}

      {issues.length > 0 && (
        <div className="mt-4 overflow-hidden rounded-2xl border border-warning/30 bg-[color-mix(in_srgb,var(--warning)_7%,var(--card))]">
          {issues.map((i) => (
            <IssueRow key={i.key} issue={i} onAction={onIssueAction} onDismiss={onDismissIssue} busy={issueBusy(i)} />
          ))}
        </div>
      )}

      <div className="mt-4 rounded-2xl border border-border bg-card px-4 pb-2 pt-4">
        <div className="flex items-center justify-between gap-4 px-1 pb-2">
          <h3 className="text-sm font-medium">{t('channels.chart.title')}</h3>
          <Segmented
            label={t('channels.range.label')}
            value={range.key}
            onChange={onRangeChange}
            options={RANGES.map((r) => ({ value: r.key, label: t(`channels.range.${r.key}`) }))}
          />
        </div>
        <TrafficChart points={points} channels={charted} leadId={hoverId} bucket={range.bucket} loading={metrics.loading} />
      </div>

      <div className="mb-2.5 mt-6 flex items-center justify-between">
        <h3 className="text-sm font-medium">{t('channels.table.title')}</h3>
        <span className="text-xs text-muted-foreground">{t('channels.table.hint')}</span>
      </div>
      <div className="overflow-hidden rounded-2xl border border-border bg-card">
        <ChannelToolbar
          counts={counts}
          tags={tags}
          hasUntagged={hasUntagged}
          filters={filters}
          onFiltersChange={onFiltersChange}
          selected={selectedRows}
          onClear={() => setSelectedIds(new Set())}
          batch={batch}
          tagActions={tagActions}
          busy={batchBusy}
        />
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] table-fixed border-collapse text-sm" onMouseLeave={() => setHoverId(null)}>
            <colgroup>
              <col className="w-11" />
              <col />
              <col className="w-[13%]" />
              <col className="w-[13%]" />
              <col className="w-[10%]" />
              <col className="w-[11%]" />
              <col className="w-[12%]" />
              <col className="w-[76px]" />
              <col className="w-[52px]" />
            </colgroup>
            <thead>
              <tr className="text-left text-xs text-muted-foreground [&>th]:h-10 [&>th]:leading-tight [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
                <th className="!pr-0">
                  <Checkbox
                    checked={headState === 'all'}
                    indeterminate={headState === 'some'}
                    disabled={!rows.length}
                    aria-label={t('channels.batch.selectAll')}
                    onCheckedChange={toggleAll}
                  />
                </th>
                <th>{t('channels.table.name')}</th>
                <th className="text-right">{t('channels.table.priorityWeight')}</th>
                <th className="text-right">{t('channels.table.requests', { range: t(`channels.range.short.${range.key}`) })}</th>
                <th className="text-right">{t('channels.table.successRate')}</th>
                <th className="text-right">{t('channels.table.avgLatency')}</th>
                <th className="text-right">{t('channels.table.balance')}</th>
                <th>{t('channels.table.enabled')}</th>
                <th>
                  <span className="sr-only">{t('channels.table.actions')}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 && (
                <tr>
                  <td colSpan={9} className="h-24 text-center text-sm text-muted-foreground">
                    {t('channels.filter.empty')}{' '}
                    <button type="button" className="text-info hover:underline" onClick={() => onFiltersChange({ status: '', tag: '' })}>
                      {t('channels.filter.clear')}
                    </button>
                  </td>
                </tr>
              )}
              {rows.map((c) => {
                const m = perChannel.get(c.id);
                const off = c.status !== 1;
                const isNew = highlightIds.has(c.id);
                return (
                  <tr
                    key={c.id}
                    onClick={() => onOpen(c.id)}
                    onMouseEnter={() => setHoverId(c.id)}
                    className={cn(
                      'cursor-pointer transition-colors hover:bg-foreground/[0.04] [&>td]:h-[52px] [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0',
                      off && '[&>td:not(.keep)]:text-muted-foreground',
                      isNew && 'bg-info/[0.06]',
                      selectedIds.has(c.id) && 'bg-accent/50'
                    )}
                  >
                    <td className="keep !pr-0" onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        checked={selectedIds.has(c.id)}
                        aria-label={t('channels.batch.selectRow', { name: c.name })}
                        onCheckedChange={() => toggleRow(c.id)}
                      />
                    </td>
                    <td>
                      <span className="flex min-w-0 items-center gap-2">
                        <Dot tone={channelDot(c, dismissed)} />
                        <span className="truncate font-mono text-[13px]" title={c.name}>
                          {c.name}
                        </span>
                        {c.tag && (
                          <span className="inline-flex h-[22px] items-center rounded-full bg-muted px-2 text-xs text-muted-foreground">
                            {c.tag}
                          </span>
                        )}
                        {isNew && (
                          <span className="inline-flex h-[22px] items-center rounded-full bg-info/10 px-2 text-xs text-info">
                            {t('channels.table.new')}
                          </span>
                        )}
                      </span>
                    </td>
                    <td className="text-right tabular-nums">
                      {c.priority ?? 0} · {c.weight ?? 1}
                    </td>
                    <td className="text-right tabular-nums">{m?.requests ? m.requests.toLocaleString() : <Muted>0</Muted>}</td>
                    <td className="text-right tabular-nums">
                      {m?.requests ? (
                        <span className={cn(m.successRate < 0.95 && 'text-destructive')}>{pct(m.successRate)}</span>
                      ) : (
                        <Muted />
                      )}
                    </td>
                    <td className="text-right tabular-nums">{m?.avgLatencyMs != null ? sec(m.avgLatencyMs) : <Muted />}</td>
                    <td className="text-right tabular-nums">
                      <BalanceText channel={c} />
                    </td>
                    <td className="keep !overflow-visible" onClick={(e) => e.stopPropagation()}>
                      <Switch
                        checked={c.status === 1}
                        aria-label={t('channels.table.enable', { name: c.name })}
                        onCheckedChange={() => actions.onToggleStatus(c)}
                      />
                    </td>
                    <td className="keep !overflow-visible !px-2 text-right" onClick={(e) => e.stopPropagation()}>
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('channels.more')}>
                            {testingIds.has(c.id) ? <Loader2 className="size-4 animate-spin" /> : <MoreHorizontal className="size-4" />}
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-44">
                          <DropdownMenuItem onClick={() => actions.onTest(c)}>
                            <FlaskConical className="size-4" />
                            {t('channels.action.test')}
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => actions.onCopy(c)}>
                            <Copy className="size-4" />
                            {t('channels.action.copy')}
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem className="text-destructive" onClick={() => actions.onDelete(c)}>
                            <Trash2 className="size-4" />
                            {t('channels.action.delete')}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

ProviderPane.propTypes = {
  type: PropTypes.number.isRequired,
  channels: PropTypes.array.isRequired,
  metrics: PropTypes.object.isRequired,
  rangeKey: PropTypes.string.isRequired,
  onRangeChange: PropTypes.func.isRequired,
  dismissed: PropTypes.instanceOf(Set).isRequired,
  onDismissIssue: PropTypes.func.isRequired,
  onIssueAction: PropTypes.func.isRequired,
  issueBusy: PropTypes.func.isRequired,
  created: PropTypes.object,
  onAdjustCreated: PropTypes.func,
  onDismissCreated: PropTypes.func,
  highlightIds: PropTypes.instanceOf(Set).isRequired,
  onOpen: PropTypes.func.isRequired,
  onNew: PropTypes.func.isRequired,
  onRefreshModels: PropTypes.func.isRequired,
  onTestAll: PropTypes.func.isRequired,
  refreshingModels: PropTypes.bool,
  testingAll: PropTypes.bool,
  actions: PropTypes.object.isRequired,
  testingIds: PropTypes.instanceOf(Set).isRequired,
  filters: PropTypes.object.isRequired,
  onFiltersChange: PropTypes.func.isRequired,
  batch: PropTypes.object.isRequired,
  tagActions: PropTypes.object.isRequired,
  batchBusy: PropTypes.bool
};
