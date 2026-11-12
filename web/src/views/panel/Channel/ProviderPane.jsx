import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  AlertTriangle,
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronUp,
  Copy,
  GripVertical,
  Link2,
  Unlink,
  FlaskConical,
  Info,
  Loader2,
  MoreHorizontal,
  Power,
  PowerOff,
  Trash2,
  X
} from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { channelSeries, formatBalance, metricsByChannel } from './channelMetrics';
import { NO_TAG, channelDot, filterChannels, groupOptions, openIssues, selectionState, statusCounts, tagOptions } from './channelGroups';
import { Checkbox } from '@/components/ui/checkbox';
import { ChannelBatchBar, TagActionsMenu } from './ChannelToolbar';
import { Dot, ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { FilterBar, FilterSheet, describeEntry, isEntryActive } from '@/components/filter-bar';
import { ColumnPicker, PhoneListToolbar, PresetChips, StatusSegments, useFilterPresets } from '@/components/list-page';
import { ProviderTile, useProviderName } from './channelParts';
import { relayHost } from './providerDefaults';

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

// What needs doing on a channel, as a tag on its row: what is wrong and the one action, the
// action runs on click; × dismisses it for this session.
function IssueTag({ issue, onAction, onDismiss, busy }) {
  const { t } = useTranslation();
  const label = {
    autoDisabled: t('channels.issueTag.autoDisabled'),
    keyMissing: t('channels.issueTag.keyMissing'),
    lowBalance: t('channels.issueTag.lowBalance'),
    driftNew: t('channels.issueTag.driftNew', { count: issue.count }),
    driftMissing: t('channels.issueTag.driftMissing', { count: issue.count })
  }[issue.kind];
  const action = {
    autoDisabled: t('channels.issue.enable'),
    keyMissing: t('channels.issue.setKey'),
    lowBalance: t('channels.issueTag.refresh'),
    driftNew: t('channels.issue.view'),
    driftMissing: t('channels.issue.view')
  }[issue.kind];
  return (
    // Gives way before the channel name: it shrinks down to its icon when the row is narrow.
    <span
      title={`${label} · ${action}`}
      className="inline-flex h-[22px] min-w-[44px] shrink-[8] items-center overflow-hidden rounded-full border border-warning/40 bg-warning/10 text-xs text-warning"
    >
      <button
        type="button"
        disabled={busy}
        onClick={(e) => {
          e.stopPropagation();
          onAction(issue);
        }}
        className="inline-flex h-full min-w-0 items-center gap-1 pl-2 pr-1.5 hover:bg-warning/15 disabled:opacity-60"
      >
        {busy ? <Loader2 className="size-3 shrink-0 animate-spin" /> : <AlertTriangle className="size-3 shrink-0" />}
        <span className="truncate">
          {label} · <span className="underline-offset-2 hover:underline">{action}</span>
        </span>
      </button>
      <button
        type="button"
        aria-label={t('channels.issue.dismiss')}
        onClick={(e) => {
          e.stopPropagation();
          onDismiss(issue);
        }}
        className="inline-flex h-full shrink-0 items-center pl-0.5 pr-1.5 hover:bg-warning/15"
      >
        <X className="size-3" />
      </button>
    </span>
  );
}

IssueTag.propTypes = { issue: PropTypes.object.isRequired, onAction: PropTypes.func, onDismiss: PropTypes.func, busy: PropTypes.bool };

// Table columns: selection, name and the row menu are fixed; the rest are in the column picker.
export const CHANNEL_COLUMNS = [
  { id: 'select', fixed: true },
  { id: 'name', fixed: true },
  { id: 'priority', labelKey: 'channels.table.priorityWeight' },
  { id: 'requests', labelKey: 'channels.columns.requests' },
  { id: 'success', labelKey: 'channels.table.successRate' },
  { id: 'latency', labelKey: 'channels.table.avgLatency' },
  { id: 'balance', labelKey: 'channels.table.balance' },
  { id: 'enabled', labelKey: 'channels.table.enabled' },
  { id: 'actions', fixed: true }
];

// Fixed widths of the channel table's columns (padding included); the name takes the rest. With
// the name's minimum they set the table's minimum width: a narrower pane scrolls the table
// sideways inside its card, and a long name or tag is cut off instead of widening the table.
// Requests per bucket as a small line, for the requests column.
function Sparkline({ values }) {
  const w = 72;
  const h = 20;
  if (values.length < 2) return <span className="inline-block w-[72px]" />;
  const max = Math.max(1, ...values);
  const d = values
    .map((v, i) => `${i ? 'L' : 'M'}${((i / (values.length - 1)) * w).toFixed(1)},${(h - 2 - (v / max) * (h - 4)).toFixed(1)}`)
    .join('');
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="shrink-0 text-muted-foreground" aria-hidden="true">
      <path d={d} fill="none" stroke="currentColor" strokeWidth="1.25" />
    </svg>
  );
}

Sparkline.propTypes = { values: PropTypes.arrayOf(PropTypes.number).isRequired };

const COLUMN_WIDTHS = {
  select: 60,
  order: 64,
  priority: 112,
  requests: 136,
  success: 80,
  latency: 80,
  balance: 104,
  enabled: 64,
  actions: 52
};
const NAME_MIN_WIDTH = 200;

const STATUS_KEYS = ['all', 'enabled', 'manual', 'auto'];
const one = (value) => ({ op: 'in', values: [value] });
// Built-in presets: the usual "what needs me" questions.
const BUILTIN_PRESETS = [
  { id: 'attention', labelKey: 'channels.preset.attention', status: 'all', conditions: { issue: one('any') } },
  { id: 'lowBalance', labelKey: 'channels.preset.lowBalance', status: 'all', conditions: { issue: one('lowBalance') } },
  { id: 'auto', labelKey: 'channels.preset.autoDisabled', status: 'auto', conditions: {} }
];

// Filters <-> condition-bar state. Status is the segment; the rest are conditions.
const toState = (filters) => {
  const state = {};
  if (filters.tag) state.tag = one(filters.tag);
  if (filters.group) state.group = one(filters.group);
  if (filters.issue) state.issue = one(filters.issue);
  if (filters.model) state.model = { value: filters.model };
  return state;
};
const fromState = (state) => ({
  tag: state.tag?.values?.[0] || '',
  group: state.group?.values?.[0] || '',
  issue: state.issue?.values?.[0] || '',
  model: String(state.model?.value || '').trim()
});

// One channel's row menu (table and phone list): test, copy, delete, plus enable / disable where
// the row has no switch.
function ChannelMenu({ channel, actions, testing, withToggle = false, order = null, orderActions = null }) {
  const { t } = useTranslation();
  const place = order?.info.get(channel.id);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('channels.more')}>
          {testing ? <Loader2 className="size-4 animate-spin" /> : <MoreHorizontal className="size-4" />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        {place && orderActions && (
          <>
            <DropdownMenuItem disabled={place.index === 0} onClick={() => orderActions.step(channel.id, 'up')}>
              <ArrowUp className="size-4" />
              {t('channels.order.up')}
            </DropdownMenuItem>
            <DropdownMenuItem disabled={place.index === order.groups.length - 1} onClick={() => orderActions.step(channel.id, 'down')}>
              <ArrowDown className="size-4" />
              {t('channels.order.down')}
            </DropdownMenuItem>
            {place.size > 1 ? (
              <DropdownMenuItem onClick={() => orderActions.unshare(channel.id)}>
                <Unlink className="size-4" />
                {t('channels.order.unshare')}
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem disabled={place.index === 0} onClick={() => orderActions.share(channel.id)}>
                <Link2 className="size-4" />
                {t('channels.order.share')}
              </DropdownMenuItem>
            )}
            <DropdownMenuSeparator />
          </>
        )}
        {withToggle && (
          <DropdownMenuItem onClick={() => actions.onToggleStatus(channel)}>
            {channel.status === 1 ? <PowerOff className="size-4" /> : <Power className="size-4" />}
            {channel.status === 1 ? t('channels.batch.disable') : t('channels.batch.enable')}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={() => actions.onTest(channel)}>
          <FlaskConical className="size-4" />
          {t('channels.action.test')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => actions.onCopy(channel)}>
          <Copy className="size-4" />
          {t('channels.action.copy')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive" onClick={() => actions.onDelete(channel)}>
          <Trash2 className="size-4" />
          {t('channels.action.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

ChannelMenu.propTypes = {
  channel: PropTypes.object.isRequired,
  actions: PropTypes.object.isRequired,
  testing: PropTypes.bool,
  withToggle: PropTypes.bool,
  order: PropTypes.object,
  orderActions: PropTypes.object
};

// Weight of a channel in a shared position, saved when the field loses focus or on Enter.
function WeightInput({ id, name, value, onSave }) {
  const { t } = useTranslation();
  const [text, setText] = useState(String(value));
  const [synced, setSynced] = useState(value);
  if (value !== synced) {
    setSynced(value);
    setText(String(value));
  }
  const commit = () => {
    const n = Math.max(1, Math.round(Number(text)) || 1);
    setText(String(n));
    if (n !== value) onSave(id, n);
  };
  return (
    <Input
      aria-label={t('channels.order.weightOf', { name })}
      inputMode="numeric"
      value={text}
      onChange={(e) => setText(e.target.value.replace(/\D/g, ''))}
      onBlur={commit}
      onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
      className="h-7 w-12 rounded-md px-2 text-right tabular-nums"
    />
  );
}

WeightInput.propTypes = {
  id: PropTypes.number.isRequired,
  name: PropTypes.string,
  value: PropTypes.number.isRequired,
  onSave: PropTypes.func.isRequired
};

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
  actions,
  testingIds,
  filters,
  onFiltersChange,
  batch,
  tagActions,
  batchBusy,
  layout = 'split',
  columns,
  order = null,
  orderActions = null,
  leading = null
}) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const [filterSheetOpen, setFilterSheetOpen] = useState(false);
  const [selectMode, setSelectMode] = useState(false);
  const range = RANGES.find((r) => r.key === rangeKey) || RANGES[0];
  const phone = layout === 'list';

  const ids = useMemo(() => channels.map((c) => c.id), [channels]);
  const perChannel = useMemo(() => metricsByChannel(metrics.rows), [metrics.rows]);
  // Requests per bucket of each channel, for the line in the requests column.
  const series = useMemo(() => {
    const out = new Map(ids.map((id) => [id, []]));
    if (!metrics.end) return out;
    for (const p of channelSeries(metrics.rows, ids, metrics.start, metrics.end, range.bucket))
      for (const id of ids) out.get(id).push(p.values[id] || 0);
    return out;
  }, [metrics.rows, ids, metrics.start, metrics.end, range.bucket]);
  const issues = openIssues(channels, dismissed);
  const issuesById = useMemo(() => {
    const out = new Map();
    for (const i of issues) out.set(i.channel.id, [...(out.get(i.channel.id) || []), i]);
    return out;
  }, [issues]);
  const name = type ? providerName(type) : t('channels.strip.allTitle');
  // With a call order the rows follow it; otherwise they keep the page's order.
  const rows = useMemo(() => {
    const list = filterChannels(channels, filters, dismissed);
    if (!order) return list;
    const rank = (c) => order.info.get(c.id)?.rank ?? Number.MAX_SAFE_INTEGER;
    return [...list].sort((a, b) => rank(a) - rank(b));
  }, [channels, filters, dismissed, order]);
  // Order cell of each shown row: the first row of a run of one cluster spans the run.
  const spans = useMemo(() => {
    const out = new Map();
    if (!order) return out;
    let lead = null;
    for (const c of rows) {
      const n = order.info.get(c.id)?.number;
      if (lead && n !== undefined && order.info.get(lead)?.number === n) {
        out.set(lead, out.get(lead) + 1);
        out.set(c.id, 0);
      } else {
        lead = c.id;
        out.set(c.id, 1);
      }
    }
    return out;
  }, [rows, order]);
  const [dragId, setDragId] = useState(0);
  // Where a dragged row would land: before the row (`before`) or in its position (`join`).
  const [drop, setDrop] = useState(null);
  const endDrag = () => {
    setDragId(0);
    setDrop(null);
  };
  const counts = useMemo(() => statusCounts(channels, filters, dismissed), [channels, filters, dismissed]);
  const { tags, hasUntagged } = useMemo(() => tagOptions(channels), [channels]);
  const groups = useMemo(() => groupOptions(channels), [channels]);

  // Conditions of the "Show channels that" bar, with options from this provider's channels.
  const fields = useMemo(
    () => [
      {
        key: 'issue',
        labelKey: 'channels.condition.issue',
        single: true,
        supportsExclude: false,
        paramInclude: 'issue',
        options: [
          { value: 'any', labelKey: 'channels.condition.issueAny' },
          { value: 'lowBalance', labelKey: 'channels.condition.issueLowBalance' },
          { value: 'keyMissing', labelKey: 'channels.condition.issueKeyMissing' },
          { value: 'drift', labelKey: 'channels.condition.issueDrift' }
        ]
      },
      ...(tags.length || hasUntagged
        ? [
            {
              key: 'tag',
              labelKey: 'channels.condition.tag',
              single: true,
              supportsExclude: false,
              paramInclude: 'tag',
              options: [
                ...(hasUntagged ? [{ value: NO_TAG, labelKey: 'channels.filter.noTag' }] : []),
                ...tags.map((x) => ({ value: x, label: x }))
              ]
            }
          ]
        : []),
      ...(groups.length
        ? [
            {
              key: 'group',
              labelKey: 'channels.condition.group',
              single: true,
              supportsExclude: false,
              paramInclude: 'group',
              options: groups.map((x) => ({ value: x, label: x }))
            }
          ]
        : []),
      { key: 'model', labelKey: 'channels.condition.model', type: 'text', placeholderKey: 'channels.condition.modelHint' }
    ],
    [tags, hasUntagged, groups]
  );
  const state = useMemo(() => toState(filters), [filters]);
  const onStateChange = (next) => onFiltersChange({ ...filters, ...fromState(typeof next === 'function' ? next(state) : next) });
  const clearConditions = () => onFiltersChange({ status: filters.status });
  const segment = filters.status || 'all';
  const setSegment = (v) => onFiltersChange({ ...filters, status: v === 'all' ? '' : v });
  const presets = useFilterPresets({
    storageKey: 'channel-filter-presets-v1',
    builtin: BUILTIN_PRESETS,
    fields,
    state,
    status: segment,
    statusLabel: (v) => t(`channels.filter.${v}`),
    apply: (p) => onFiltersChange({ status: p.status && p.status !== 'all' ? p.status : '', ...fromState(p.conditions) })
  });
  const activeFields = fields.filter((f) => isEntryActive(f, state[f.key]));
  const realTag = filters.tag && filters.tag !== NO_TAG ? filters.tag : '';

  // Selection lives here: the page remounts the pane when the provider or query changes, and it
  // clears when the filters change, so a bulk action never touches rows that are not shown.
  const [selectedIds, setSelectedIds] = useState(() => new Set());
  const filtersKey = JSON.stringify(filters);
  useEffect(() => {
    setSelectedIds(new Set());
  }, [filtersKey]);
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
  const exitSelectMode = () => {
    setSelectMode(false);
    setSelectedIds(new Set());
  };

  const visible = columns.visibility;
  const show = (id) => visible[id] !== false;
  const segmentOptions = STATUS_KEYS.map((k) => ({ value: k, label: t(`channels.filter.${k}`) }));

  const emptyRow = (
    <div className="flex h-24 items-center justify-center gap-1 text-sm text-muted-foreground">
      {t('channels.filter.empty')}
      <button type="button" className="text-info hover:underline" onClick={() => onFiltersChange({ status: '' })}>
        {t('channels.filter.clear')}
      </button>
    </div>
  );

  const fixedColumns = Object.keys(COLUMN_WIDTHS).filter(
    (id) => id === 'select' || id === 'actions' || (id === 'order' ? Boolean(order) : show(id))
  );
  const minTableWidth = NAME_MIN_WIDTH + fixedColumns.reduce((sum, id) => sum + COLUMN_WIDTHS[id], 0);
  const col = (id) => <col key={id} style={{ width: COLUMN_WIDTHS[id] }} />;
  const table = (
    <div className="overflow-x-auto rounded-2xl border border-border bg-card">
      <table className="w-full table-fixed border-collapse text-sm" style={{ minWidth: minTableWidth }}>
        <colgroup>
          {col('select')}
          {order && col('order')}
          <col />
          {fixedColumns.filter((id) => id !== 'select' && id !== 'actions' && id !== 'order').map(col)}
          {col('actions')}
        </colgroup>
        <thead>
          <tr className="whitespace-nowrap text-left text-xs text-muted-foreground [&>th]:h-10 [&>th]:border-b [&>th]:border-border [&>th]:truncate [&>th]:px-3 [&>th]:font-medium">
            <th className="!pr-0">
              <Checkbox
                checked={headState === 'all'}
                indeterminate={headState === 'some'}
                disabled={!rows.length}
                aria-label={t('channels.batch.selectAll')}
                onCheckedChange={toggleAll}
              />
            </th>
            {order && (
              <th className="text-center" title={t('channels.orderHint')}>
                {t('channels.order.column')}
              </th>
            )}
            <th>{t('channels.table.name')}</th>
            {show('priority') && (
              <th className="text-right">{order ? t('channels.order.shareColumn') : t('channels.table.priorityWeight')}</th>
            )}
            {show('requests') && (
              <th className="text-right">{t('channels.columns.requestsIn', { range: t(`channels.range.short.${range.key}`) })}</th>
            )}
            {show('success') && <th className="text-right">{t('channels.table.successRate')}</th>}
            {show('latency') && <th className="text-right">{t('channels.table.avgLatency')}</th>}
            {show('balance') && <th className="text-right">{t('channels.table.balance')}</th>}
            {show('enabled') && <th>{t('channels.table.enabled')}</th>}
            <th>
              <span className="sr-only">{t('channels.table.actions')}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && (
            <tr>
              <td colSpan={10}>{emptyRow}</td>
            </tr>
          )}
          {rows.map((c) => {
            const m = perChannel.get(c.id);
            const off = c.status !== 1;
            const isNew = highlightIds.has(c.id);
            const place = order?.info.get(c.id);
            return (
              <tr
                key={c.id}
                onClick={() => onOpen(c.id)}
                {...(place && orderActions
                  ? {
                      onDragOver: (e) => {
                        if (!dragId || dragId === c.id) return;
                        e.preventDefault();
                        e.dataTransfer.dropEffect = 'move';
                        // Top third: before this row; bottom third: after it; middle: share its position.
                        const box = e.currentTarget.getBoundingClientRect();
                        const y = (e.clientY - box.top) / box.height;
                        const mode = y < 0.3 ? 'before' : y > 0.7 ? 'after' : 'join';
                        if (drop?.id !== c.id || drop.mode !== mode) setDrop({ id: c.id, mode });
                      },
                      onDrop: (e) => {
                        e.preventDefault();
                        if (dragId && drop && dragId !== c.id) {
                          if (drop.mode === 'join') orderActions.join(dragId, c.id);
                          else if (drop.mode === 'after') orderActions.after(dragId, c.id);
                          else orderActions.drop(dragId, c.id);
                        }
                        endDrag();
                      }
                    }
                  : {})}
                className={cn(
                  drop?.id === c.id && drop.mode === 'before' && 'shadow-[inset_0_3px_0_var(--info)]',
                  drop?.id === c.id && drop.mode === 'after' && 'shadow-[inset_0_-3px_0_var(--info)]',
                  drop?.id === c.id && drop.mode === 'join' && '!bg-info/[0.12] shadow-[inset_0_0_0_2px_var(--info)]',
                  dragId === c.id && 'opacity-40',
                  place && place.clusterSize > 1 && 'bg-muted/40',
                  'cursor-pointer whitespace-nowrap transition-colors hover:bg-foreground/[0.04] [&>td]:h-[52px] [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0',
                  off && '[&>td:not(.keep)]:text-muted-foreground',
                  isNew && 'bg-info/[0.06]',
                  selectedIds.has(c.id) && 'bg-accent/50'
                )}
              >
                <td className="keep !pr-0" onClick={(e) => e.stopPropagation()}>
                  <span className="flex items-center gap-1">
                    <Checkbox
                      checked={selectedIds.has(c.id)}
                      aria-label={t('channels.batch.selectRow', { name: c.name })}
                      onCheckedChange={() => toggleRow(c.id)}
                    />
                    {place && orderActions && (
                      <span
                        draggable
                        onDragStart={(e) => {
                          e.dataTransfer.effectAllowed = 'move';
                          e.dataTransfer.setData('text/plain', String(c.id));
                          // The whole row follows the pointer, not just the handle.
                          const row = e.currentTarget.closest('tr');
                          if (row) {
                            const box = row.getBoundingClientRect();
                            e.dataTransfer.setDragImage(row, e.clientX - box.left, e.clientY - box.top);
                          }
                          setDragId(c.id);
                        }}
                        onDragEnd={endDrag}
                        className="-m-1 flex cursor-grab rounded p-1 text-muted-foreground/50 hover:bg-muted hover:text-foreground active:cursor-grabbing"
                        title={t('channels.order.drag')}
                      >
                        <GripVertical className="size-4" />
                      </span>
                    )}
                  </span>
                </td>
                {order && spans.get(c.id) !== 0 && (
                  <td
                    rowSpan={spans.get(c.id) || 1}
                    className="keep !px-1 text-center align-middle"
                    title={
                      place && place.clusterSize > 1
                        ? t('channels.order.groupWhy', { count: place.clusterSize, models: place.sharedModels })
                        : undefined
                    }
                    onClick={(e) => e.stopPropagation()}
                  >
                    <span className="inline-flex items-center gap-0.5">
                      <span className="w-5">
                        <span className="block text-[13px] font-medium tabular-nums">{place ? place.number : ''}</span>
                        {place && place.clusterSize > 1 && spans.get(c.id) > 1 && (
                          <span className="block text-[11px] leading-tight text-muted-foreground">{t('channels.order.sharedTag')}</span>
                        )}
                      </span>
                      {place && orderActions && (
                        // Moves the whole position (the group) one step.
                        <span className="flex flex-col">
                          <button
                            type="button"
                            disabled={place.index === 0}
                            aria-label={t('channels.order.up')}
                            title={t('channels.order.up')}
                            onClick={() => orderActions.step(c.id, 'up')}
                            className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-25"
                          >
                            <ChevronUp className="size-3.5" />
                          </button>
                          <button
                            type="button"
                            disabled={place.index === order.groups.length - 1}
                            aria-label={t('channels.order.down')}
                            title={t('channels.order.down')}
                            onClick={() => orderActions.step(c.id, 'down')}
                            className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-25"
                          >
                            <ChevronDown className="size-3.5" />
                          </button>
                        </span>
                      )}
                    </span>
                  </td>
                )}
                <td>
                  <span className="flex min-w-0 items-center gap-2.5">
                    {!type && <ProviderTile type={c.type} />}
                    <span className="min-w-0 flex-1">
                      <span className="flex min-w-0 items-center gap-2">
                        <Dot tone={channelDot(c, dismissed)} />
                        <span className="min-w-[6rem] shrink truncate font-mono text-[13px]" title={c.name}>
                          {c.name}
                        </span>
                        {c.tag && (
                          // The tag gives way before the name does.
                          <span
                            className="inline-block h-[22px] min-w-0 shrink-[4] truncate rounded-full bg-muted px-2 text-xs leading-[22px] text-muted-foreground"
                            title={c.tag}
                          >
                            {c.tag}
                          </span>
                        )}
                        {(issuesById.get(c.id) || []).map((i) => (
                          <IssueTag key={i.key} issue={i} onAction={onIssueAction} onDismiss={onDismissIssue} busy={issueBusy(i)} />
                        ))}
                        {drop?.id === c.id && (
                          <span className="inline-flex h-[22px] shrink-0 items-center rounded-full bg-info px-2 text-xs text-info-foreground">
                            {t(`channels.order.dropHint.${drop.mode}`)}
                          </span>
                        )}
                        {isNew && (
                          <span className="inline-flex h-[22px] shrink-0 items-center rounded-full bg-info/10 px-2 text-xs text-info">
                            {t('channels.table.new')}
                          </span>
                        )}
                      </span>
                      {(() => {
                        // Second line: the provider (in the all-providers list) and who it shares with.
                        const sub = [
                          [!type ? providerName(c.type) : '', relayHost(c)].filter(Boolean).join(' · '),
                          place?.peers?.length
                            ? t('channels.order.sharesWith', {
                                names: place.peers.map((id) => order.names.get(id) || `#${id}`).join(t('channels.order.nameSep'))
                              })
                            : ''
                        ].filter(Boolean);
                        return sub.length > 0 ? (
                          <span className="block truncate text-xs text-muted-foreground">{sub.join(' · ')}</span>
                        ) : null;
                      })()}
                    </span>
                  </span>
                </td>
                {show('priority') &&
                  (order ? (
                    <td className="keep text-right" onClick={(e) => e.stopPropagation()}>
                      {place && place.share !== null && place.share !== undefined && (
                        <span className="inline-flex items-center gap-2">
                          {orderActions ? (
                            <WeightInput id={c.id} name={c.name} value={place.weight} onSave={orderActions.weight} />
                          ) : (
                            <span className="tabular-nums">{place.weight}</span>
                          )}
                          <span className="w-9 text-right text-xs tabular-nums text-muted-foreground">{place.share}%</span>
                        </span>
                      )}
                    </td>
                  ) : (
                    <td className="text-right tabular-nums">
                      {c.priority ?? 0} · {c.weight ?? 1}
                    </td>
                  ))}
                {show('requests') && (
                  <td className="tabular-nums">
                    <span className="flex items-center justify-end gap-2">
                      <Sparkline values={series.get(c.id) || []} />
                      <span className="min-w-[2.5rem] text-right">{m?.requests ? m.requests.toLocaleString() : <Muted>0</Muted>}</span>
                    </span>
                  </td>
                )}
                {show('success') && (
                  <td className="text-right tabular-nums">
                    {m?.requests ? <span className={cn(m.successRate < 0.95 && 'text-destructive')}>{pct(m.successRate)}</span> : <Muted />}
                  </td>
                )}
                {show('latency') && (
                  <td className="text-right tabular-nums">{m?.avgLatencyMs != null ? sec(m.avgLatencyMs) : <Muted />}</td>
                )}
                {show('balance') && (
                  <td className="text-right tabular-nums">
                    <BalanceText channel={c} />
                  </td>
                )}
                {show('enabled') && (
                  <td className="keep" onClick={(e) => e.stopPropagation()}>
                    <span className="flex">
                      <Switch
                        size="sm"
                        checked={c.status === 1}
                        aria-label={t('channels.table.enable', { name: c.name })}
                        onCheckedChange={() => actions.onToggleStatus(c)}
                      />
                    </span>
                  </td>
                )}
                <td className="keep !px-2 text-right" onClick={(e) => e.stopPropagation()}>
                  <ChannelMenu
                    channel={c}
                    actions={actions}
                    testing={testingIds.has(c.id)}
                    withToggle={!show('enabled')}
                    order={order}
                    orderActions={orderActions}
                  />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );

  // Phone list: one 56px row per channel. Health dot, name and success rate on the first line;
  // priority, requests and latency on the second with the balance on the right.
  const list = (
    <div className="overflow-hidden rounded-2xl border border-border bg-card">
      {rows.length === 0 && emptyRow}
      <ul className="divide-y divide-border">
        {rows.map((c) => {
          const m = perChannel.get(c.id);
          const selected = selectedIds.has(c.id);
          return (
            <li
              key={c.id}
              className={cn(
                'flex h-14 items-center gap-3 px-4',
                ROW_OPENS,
                selected && 'bg-muted/60',
                c.status !== 1 && 'text-muted-foreground'
              )}
              {...rowOpenProps(() => (selectMode ? toggleRow(c.id) : onOpen(c.id)))}
            >
              {selectMode ? (
                <RowControl>
                  <Checkbox
                    checked={selected}
                    onCheckedChange={() => toggleRow(c.id)}
                    aria-label={t('channels.batch.selectRow', { name: c.name })}
                  />
                </RowControl>
              ) : (
                <Dot tone={channelDot(c, dismissed)} />
              )}
              <div className="min-w-0 flex-1">
                <div className="flex items-baseline gap-3">
                  <span className="min-w-0 truncate font-mono text-[13px] font-medium">{c.name}</span>
                  <span className={cn('ml-auto shrink-0 text-sm tabular-nums', m?.requests && m.successRate < 0.95 && 'text-destructive')}>
                    {m?.requests ? pct(m.successRate) : '—'}
                  </span>
                </div>
                <div className="mt-0.5 flex items-baseline gap-3 text-xs text-muted-foreground">
                  <span className="min-w-0 truncate tabular-nums">
                    {t('channels.list.meta', {
                      priority: order?.info.get(c.id)
                        ? t('channels.order.positionShort', { n: order.info.get(c.id).position })
                        : `${c.priority ?? 0} · ${c.weight ?? 1}`,
                      requests: (m?.requests || 0).toLocaleString(),
                      latency: m?.avgLatencyMs != null ? sec(m.avgLatencyMs) : '—'
                    })}
                  </span>
                  <span className="ml-auto shrink-0 tabular-nums">
                    <BalanceText channel={c} />
                  </span>
                </div>
              </div>
              {!selectMode && (
                <div className="-mr-2 flex shrink-0 items-center gap-1">
                  <RowControl className="flex">
                    <Switch
                      size="sm"
                      checked={c.status === 1}
                      aria-label={t('channels.table.enable', { name: c.name })}
                      onCheckedChange={() => actions.onToggleStatus(c)}
                    />
                  </RowControl>
                  <RowControl>
                    <ChannelMenu channel={c} actions={actions} testing={testingIds.has(c.id)} order={order} orderActions={orderActions} />
                  </RowControl>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );

  return (
    <section aria-label={name} className={cn('min-w-0', phone && selectMode && 'pb-20')}>
      {created && (
        <div className="mb-3 flex items-center gap-3 rounded-2xl border border-info/30 bg-[color-mix(in_srgb,var(--info)_7%,var(--card))] py-2.5 pl-4 pr-2.5 text-[13px]">
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

      <div className="mb-3 space-y-2">
        {phone ? (
          <>
            <div className="flex flex-wrap items-center gap-2">{leading}</div>
            <StatusSegments chips value={segment} onChange={setSegment} options={segmentOptions} counts={counts} />
            <PhoneListToolbar
              filter={{ count: activeFields.length, onOpen: () => setFilterSheetOpen(true) }}
              select={{ active: selectMode, onToggle: () => (selectMode ? exitSelectMode() : setSelectMode(true)) }}
              chips={activeFields.map((f) => ({
                key: f.key,
                label: describeEntry(t, f, state[f.key]),
                onRemove: () => onStateChange((prev) => ({ ...prev, [f.key]: undefined }))
              }))}
            />
          </>
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-2">
              {leading}
              <StatusSegments value={segment} onChange={setSegment} options={segmentOptions} counts={counts} />
              {issues.length > 0 && (
                <Button
                  variant="outline"
                  size="sm"
                  aria-pressed={filters.issue === 'any'}
                  className={cn(
                    'h-8 rounded-lg border-warning/40 text-[13px] text-warning hover:text-warning',
                    filters.issue === 'any' && 'bg-warning/10'
                  )}
                  onClick={() => onFiltersChange({ ...filters, issue: filters.issue === 'any' ? '' : 'any' })}
                >
                  <AlertTriangle className="size-3.5" />
                  {t('channels.attention', { count: issues.length })}
                </Button>
              )}
              <FilterBar
                fields={fields}
                state={state}
                onChange={onStateChange}
                onClearAll={clearConditions}
                triggerVariant="plus"
                triggerLabel={t('listPage.addCondition')}
                t={t}
                className="min-w-0"
              />
              <span className="flex-1" />
              {realTag && (
                <TagActionsMenu
                  tag={realTag}
                  tagActions={tagActions}
                  busy={batchBusy}
                  onTagGone={() => onFiltersChange({ ...filters, tag: '' })}
                />
              )}
              <Select value={range.key} onValueChange={onRangeChange}>
                <SelectTrigger className="h-8 w-[92px] rounded-lg text-[13px]" aria-label={t('channels.range.label')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {RANGES.map((r) => (
                    <SelectItem key={r.key} value={r.key}>
                      {t(`channels.range.${r.key}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <ColumnPicker columns={columns} />
            </div>
            {selectedRows.length > 0 && (
              <ChannelBatchBar selected={selectedRows} onClear={() => setSelectedIds(new Set())} batch={batch} busy={batchBusy} />
            )}
          </>
        )}
      </div>
      {phone ? list : table}

      {phone && selectMode && <ChannelBatchBar floating selected={selectedRows} onClear={exitSelectMode} batch={batch} busy={batchBusy} />}
      {phone && (
        <FilterSheet
          open={filterSheetOpen}
          onOpenChange={setFilterSheetOpen}
          title={t('listPage.filter')}
          fields={fields}
          state={state}
          onChange={onStateChange}
          onClearAll={clearConditions}
          t={t}
          top={<PresetChips presets={presets} onPick={() => setFilterSheetOpen(false)} />}
        />
      )}
    </section>
  );
}

ProviderPane.propTypes = {
  type: PropTypes.number.isRequired,
  leading: PropTypes.node,
  order: PropTypes.object,
  orderActions: PropTypes.object,
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
  actions: PropTypes.object.isRequired,
  testingIds: PropTypes.instanceOf(Set).isRequired,
  filters: PropTypes.object.isRequired,
  onFiltersChange: PropTypes.func.isRequired,
  batch: PropTypes.object.isRequired,
  tagActions: PropTypes.object.isRequired,
  batchBusy: PropTypes.bool,
  layout: PropTypes.oneOf(['split', 'stacked', 'list']),
  columns: PropTypes.object.isRequired
};
