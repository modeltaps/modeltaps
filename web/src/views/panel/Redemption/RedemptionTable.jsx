import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ArrowUpDown, Ban, Check, Copy, Download, MoreHorizontal, PanelRight, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Checkbox } from '@/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { estimateWidth, useNaturalWidth } from '@/components/list-page';
import { renderQuota, timestamp2string } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';

const ENABLED = 1;
const USED = 3;

// The state a code is in, as the segments split it: used, disabled, expired or usable.
export function codeState(item, now = Math.floor(Date.now() / 1000)) {
  if (item.status === USED) return 'used';
  if (item.status !== ENABLED) return 'disabled';
  if (item.expired_time > 0 && item.expired_time <= now) return 'expired';
  return 'usable';
}

// Last characters of the code, enough to tell codes of one batch apart.
export const keyTail = (key) => (key ? `…${key.slice(-4)}` : '');

// Columns: selection, name and the row menu are fixed; the id and redemption time are optional.
// The on/off switch sits last, next to the menu, where row actions are.
export const REDEMPTION_COLUMNS = [
  { id: 'select', fixed: true },
  { id: 'id', labelKey: 'redemptionPage.headLabels.id', defaultHidden: true },
  { id: 'name', labelKey: 'redemptionPage.headLabels.name', fixed: true },
  { id: 'quota', labelKey: 'redemptionPage.headLabels.quota' },
  { id: 'scope', labelKey: 'redemptionPage.headLabels.scope' },
  { id: 'created_time', labelKey: 'redemptionPage.headLabels.createdTime' },
  { id: 'redeemed_time', labelKey: 'redemptionPage.headLabels.redeemedTime', defaultHidden: true },
  { id: 'expired_time', labelKey: 'redemptionPage.headLabels.expiredTime' },
  { id: 'status', labelKey: 'redemptionPage.headLabels.status' },
  { id: 'actions', fixed: true }
];
const COLUMN_IDS = REDEMPTION_COLUMNS.map((c) => c.id);
const COLUMN_WIDTHS = {
  select: 40,
  id: 56,
  name: 220,
  quota: 96,
  scope: 80,
  created_time: 112,
  redeemed_time: 112,
  expired_time: 112,
  status: 64,
  actions: 52
};
export const estimateRedemptionTableWidth = (visibility) => estimateWidth(COLUMN_IDS, visibility, COLUMN_WIDTHS);

// Expiry in words: never, "in 7 days", or expired (tinted).
function Expiry({ item, rtf }) {
  const { t } = useTranslation();
  const v = item.expired_time;
  if (!v || v <= 0) return <span className="text-muted-foreground">{t('redemptionList.never')}</span>;
  if (codeState(item) === 'expired') {
    return (
      <span className="text-warning" title={timestamp2string(v)}>
        {t('redemptionList.expiredState')}
      </span>
    );
  }
  return <span title={timestamp2string(v)}>{relativeTime(rtf, v)}</span>;
}

Expiry.propTypes = { item: PropTypes.object.isRequired, rtf: PropTypes.object.isRequired };

// The on/off switch alone; a redeemed code shows that instead, since it cannot be switched.
export function StatusSwitch({ item, onToggleStatus }) {
  const { t } = useTranslation();
  if (item.status === USED) return <span className="text-[13px] text-muted-foreground">{t('redemptionList.redeemed')}</span>;
  return (
    <RowControl className="flex">
      <Switch
        size="sm"
        checked={item.status === ENABLED}
        aria-label={t('redemptionPage.headLabels.status')}
        onCheckedChange={() => onToggleStatus(item)}
      />
    </RowControl>
  );
}

StatusSwitch.propTypes = { item: PropTypes.object.isRequired, onToggleStatus: PropTypes.func.isRequired };

// Row menu: open, copy the code, export its batch, enable / disable, then delete (last).
export function CodeActions({ item, onOpen, onCopyKey, onExportBatch, onToggleStatus, onDelete }) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('redemptionPage.headLabels.action')}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuItem onClick={() => onOpen(item.id)}>
          <PanelRight className="size-4" />
          {t('objects.open')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onCopyKey(item)}>
          <Copy className="size-4" />
          {t('redemptionList.copyCode')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onExportBatch(item)}>
          <Download className="size-4" />
          {t('redemptionPage.exportBatch')}
        </DropdownMenuItem>
        {item.status !== USED && (
          <DropdownMenuItem onClick={() => onToggleStatus(item)}>
            {item.status === ENABLED ? <Ban className="size-4" /> : <Check className="size-4" />}
            {item.status === ENABLED ? t('redemptionList.disable') : t('redemptionList.enable')}
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDelete(item)}>
          <Trash2 className="size-4" />
          {t('common.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

CodeActions.propTypes = {
  item: PropTypes.object.isRequired,
  onOpen: PropTypes.func.isRequired,
  onCopyKey: PropTypes.func.isRequired,
  onExportBatch: PropTypes.func.isRequired,
  onToggleStatus: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired
};

const SORTABLE = {
  id: 'id',
  name: 'name',
  quota: 'quota',
  created_time: 'created_time',
  redeemed_time: 'redeemed_time',
  expired_time: 'expired_time'
};

export default function RedemptionTable({
  redemptions,
  columnVisibility,
  rtf,
  openId,
  order,
  orderBy,
  onSort,
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
  onNaturalWidth,
  actions
}) {
  const { t } = useTranslation();
  const allSelected = redemptions.length > 0 && redemptions.every((r) => selectedIds.has(r.id));
  const someSelected = redemptions.some((r) => selectedIds.has(r.id));

  const columns = useMemo(
    () => [
      {
        id: 'select',
        meta: { className: 'w-[1%] pr-0' },
        header: () => (
          <Checkbox
            checked={allSelected}
            indeterminate={!allSelected && someSelected}
            onCheckedChange={() => onToggleSelectAll(!allSelected)}
            aria-label={t('listPage.selectAllOnPage')}
          />
        ),
        cell: (c) => (
          <RowControl className="flex">
            <Checkbox
              checked={selectedIds.has(c.row.original.id)}
              onCheckedChange={() => onToggleSelect(c.row.original.id)}
              aria-label={t('listPage.selectRow', { name: c.row.original.name })}
            />
          </RowControl>
        )
      },
      {
        id: 'id',
        header: t('redemptionPage.headLabels.id'),
        cell: (c) => <span className="tabular-nums text-muted-foreground">{c.row.original.id}</span>
      },
      {
        id: 'name',
        header: t('redemptionPage.headLabels.name'),
        meta: { className: 'max-w-72' },
        cell: (c) => (
          <span className="flex min-w-0 items-center gap-2">
            <span className={cn('truncate font-medium', codeState(c.row.original) !== 'usable' && 'text-muted-foreground')}>
              {c.row.original.name}
            </span>
            <code className="shrink-0 font-mono text-xs text-muted-foreground">{keyTail(c.row.original.key)}</code>
          </span>
        )
      },
      {
        id: 'quota',
        header: t('redemptionPage.headLabels.quota'),
        meta: { className: 'text-right' },
        cell: (c) => <span className="font-mono tabular-nums">{renderQuota(c.row.original.quota)}</span>
      },
      {
        id: 'scope',
        header: t('redemptionPage.headLabels.scope'),
        cell: (c) => {
          const scope = c.row.original.scope || 'any';
          return <span className={scope === 'any' ? 'text-muted-foreground' : undefined}>{t(`redemptionPage.scopes.${scope}`)}</span>;
        }
      },
      {
        id: 'created_time',
        header: t('redemptionPage.headLabels.createdTime'),
        cell: (c) => <span title={timestamp2string(c.row.original.created_time)}>{relativeTime(rtf, c.row.original.created_time)}</span>
      },
      {
        id: 'redeemed_time',
        header: t('redemptionPage.headLabels.redeemedTime'),
        cell: (c) =>
          c.row.original.redeemed_time ? (
            <span title={timestamp2string(c.row.original.redeemed_time)}>{relativeTime(rtf, c.row.original.redeemed_time)}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )
      },
      { id: 'expired_time', header: t('redemptionPage.headLabels.expiredTime'), cell: (c) => <Expiry item={c.row.original} rtf={rtf} /> },
      {
        id: 'status',
        header: t('redemptionPage.headLabels.status'),
        cell: (c) => <StatusSwitch item={c.row.original} onToggleStatus={actions.onToggleStatus} />
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('redemptionPage.headLabels.action')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <RowControl className="justify-end">
            <CodeActions item={c.row.original} {...actions} />
          </RowControl>
        )
      }
    ],
    [t, rtf, selectedIds, allSelected, someSelected, onToggleSelect, onToggleSelectAll, actions]
  );

  const tableRef = useNaturalWidth(onNaturalWidth);
  const table = useTable({
    features: appTableFeatures,
    data: redemptions,
    columns,
    state: { columnVisibility },
    getRowId: (row) => String(row.id)
  });

  // Sort arrows: the active column shows its direction; others show a faint arrow on hover only.
  const sortIcon = (key) => {
    if (orderBy !== key) return <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  return (
    <Table ref={tableRef}>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id} className="hover:bg-transparent">
            {hg.headers.map((h) => {
              const sortKey = SORTABLE[h.column.id];
              const label = flexRender(h.column.columnDef.header, h.getContext());
              return (
                <TableHead key={h.id} className={cn('whitespace-nowrap', h.column.columnDef.meta?.className)}>
                  {sortKey ? (
                    <button
                      type="button"
                      className={cn(
                        'group/sort inline-flex items-center gap-1 hover:text-foreground',
                        orderBy === sortKey && 'text-foreground'
                      )}
                      onClick={() => onSort(sortKey)}
                    >
                      {label}
                      {sortIcon(sortKey)}
                    </button>
                  ) : (
                    label
                  )}
                </TableHead>
              );
            })}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow
            key={row.id}
            className={ROW_OPENS}
            data-state={selectedIds.has(row.original.id) || row.original.id === openId ? 'selected' : undefined}
            {...rowOpenProps(() => actions.onOpen(row.original.id))}
          >
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id} className={cn('whitespace-nowrap', cell.column.columnDef.meta?.className)}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

RedemptionTable.propTypes = {
  redemptions: PropTypes.array.isRequired,
  columnVisibility: PropTypes.object.isRequired,
  rtf: PropTypes.object.isRequired,
  openId: PropTypes.number,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  selectedIds: PropTypes.instanceOf(Set).isRequired,
  onToggleSelect: PropTypes.func.isRequired,
  onToggleSelectAll: PropTypes.func.isRequired,
  onNaturalWidth: PropTypes.func,
  actions: PropTypes.object.isRequired
};

// Phone cards, one per code. Top: name and code tail, the switch on the right. Middle: the amount,
// large. Bottom: scope · expiry · created, with the row menu. In select mode a checkbox leads.
export function RedemptionCards({ redemptions, rtf, selectMode, selectedIds, onToggleSelect, openId, actions }) {
  const { t } = useTranslation();
  return (
    <ul className="space-y-2">
      {redemptions.map((item) => {
        const selected = selectedIds.has(item.id);
        const state = codeState(item);
        const scope = item.scope || 'any';
        return (
          <li
            key={item.id}
            className={cn(
              'rounded-xl border border-border bg-card px-4 pb-2 pt-3 transition-colors',
              ROW_OPENS,
              (selected || item.id === openId) && 'border-primary/40 bg-muted/40'
            )}
            {...rowOpenProps(() => (selectMode ? onToggleSelect(item.id) : actions.onOpen(item.id)))}
          >
            <div className="flex items-center gap-3">
              {selectMode && (
                <RowControl className="flex">
                  <Checkbox
                    checked={selected}
                    onCheckedChange={() => onToggleSelect(item.id)}
                    aria-label={t('listPage.selectRow', { name: item.name })}
                  />
                </RowControl>
              )}
              <div className="flex min-w-0 flex-1 items-baseline gap-2">
                <span className={cn('truncate text-sm font-medium', state !== 'usable' && 'text-muted-foreground')}>{item.name}</span>
                <code className="shrink-0 font-mono text-xs text-muted-foreground">{keyTail(item.key)}</code>
              </div>
              <StatusSwitch item={item} onToggleStatus={actions.onToggleStatus} />
            </div>
            <div className={cn('mt-1 font-mono text-xl font-medium tabular-nums', state !== 'usable' && 'text-muted-foreground')}>
              {renderQuota(item.quota)}
            </div>
            <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
              <span className="min-w-0 flex-1 truncate">
                {t(`redemptionPage.scopes.${scope}`)} · <Expiry item={item} rtf={rtf} /> ·{' '}
                {t('redemptionList.createdAgo', { time: relativeTime(rtf, item.created_time) })}
              </span>
              <RowControl className="-mr-2 shrink-0">
                <CodeActions item={item} {...actions} />
              </RowControl>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

RedemptionCards.propTypes = {
  redemptions: PropTypes.array.isRequired,
  rtf: PropTypes.object.isRequired,
  selectMode: PropTypes.bool,
  selectedIds: PropTypes.instanceOf(Set).isRequired,
  onToggleSelect: PropTypes.func.isRequired,
  openId: PropTypes.number,
  actions: PropTypes.object.isRequired
};
