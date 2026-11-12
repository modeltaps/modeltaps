import PropTypes from 'prop-types';
import { useMemo } from 'react';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ROW_OPENS, rowOpenProps } from '@/components/objects';
import { estimateWidth, useNaturalWidth } from '@/components/list-page';
import { cn } from '@/lib/utils';
import { renderQuota, timestamp2string, timestamp2stringCompact } from 'utils/common';
import { deriveTaskDuration, statusMeta } from './taskHelpers';

// Columns for the shared column picker. By default a task shows when it was sent, what it was,
// how it went (status with the failure reason) and what it cost; ids, channel, user and the
// finish time are one click away.
export const TASK_COLUMNS = [
  { id: 'task_id', labelKey: 'taskPage.task', defaultHidden: true },
  { id: 'submit_time', labelKey: 'taskPage.subTime', fixed: true },
  { id: 'finish_time', labelKey: 'taskPage.finishTime', defaultHidden: true },
  { id: 'channel_id', labelKey: 'taskPage.channel', defaultHidden: true, adminOnly: true },
  { id: 'user_id', labelKey: 'taskPage.user', defaultHidden: true, adminOnly: true },
  { id: 'platform', labelKey: 'taskPage.platform' },
  { id: 'action', labelKey: 'taskPage.type' },
  { id: 'status', labelKey: 'userPage.status', fixed: true },
  { id: 'progress', labelKey: 'taskPage.progress' },
  { id: 'time', labelKey: 'taskPage.time' },
  { id: 'quota', labelKey: 'logPage.quotaLabel' }
];
const COLUMN_IDS = TASK_COLUMNS.map((c) => c.id);
const COLUMN_WIDTHS = {
  task_id: 140,
  submit_time: 104,
  finish_time: 104,
  channel_id: 64,
  user_id: 64,
  platform: 72,
  action: 140,
  status: 220,
  progress: 64,
  time: 80,
  quota: 90
};
export const estimateTaskTableWidth = (visibility) => estimateWidth(COLUMN_IDS, visibility, COLUMN_WIDTHS);

// A time that has not happened yet (an unfinished task) is a dash, not 1970.
const Time = ({ ts }) =>
  ts > 0 ? <span title={timestamp2string(ts)}>{timestamp2stringCompact(ts)}</span> : <span className="text-muted-foreground">—</span>;
Time.propTypes = { ts: PropTypes.number };

// Status as text (a failure is tinted, with its reason after it, truncated).
export function TaskStatus({ t, item, withReason = true }) {
  const meta = statusMeta(item.status);
  const failed = item.status === 'FAILURE';
  const label = meta ? t(meta.labelKey) : t('taskPage.statusType.unknown');
  return (
    <span className={cn('inline-flex min-w-0 max-w-72 items-baseline gap-1.5 whitespace-nowrap', failed && 'text-destructive')}>
      <span className={cn(!failed && item.status !== 'SUCCESS' && 'text-muted-foreground')}>{label}</span>
      {withReason && failed && item.fail_reason && (
        <span className="truncate text-xs text-muted-foreground" title={item.fail_reason}>
          {item.fail_reason}
        </span>
      )}
    </span>
  );
}
TaskStatus.propTypes = { t: PropTypes.func.isRequired, item: PropTypes.object.isRequired, withReason: PropTypes.bool };

const durationText = (item) => {
  const { requestTime } = deriveTaskDuration(item);
  return requestTime > 0 ? `${requestTime.toFixed(1)}s` : '';
};

export default function TaskTable({
  t,
  data,
  userIsAdmin,
  order,
  orderBy,
  onSort,
  onRowDetail,
  searching,
  columnVisibility,
  layout = 'table',
  onNaturalWidth
}) {
  const columns = useMemo(() => {
    const cells = {
      task_id: (i) => <span className="font-mono text-xs">{i.task_id}</span>,
      submit_time: (i) => <Time ts={i.submit_time} />,
      finish_time: (i) => <Time ts={i.finish_time} />,
      channel_id: (i) => i.channel_id || '',
      user_id: (i) => i.user_id || '',
      platform: (i) => i.platform,
      action: (i) => <span className="font-mono text-xs">{i.action}</span>,
      status: (i) => <TaskStatus t={t} item={i} />,
      progress: (i) => <span className="tabular-nums">{i.progress ?? 0}%</span>,
      time: (i) => <span className="tabular-nums">{durationText(i) || <span className="text-muted-foreground">—</span>}</span>,
      quota: (i) => <span className="font-mono tabular-nums">{i.quota ? renderQuota(i.quota) : '—'}</span>
    };
    const sortable = new Set(['submit_time', 'finish_time', 'channel_id', 'user_id', 'platform', 'status']);
    const right = new Set(['progress', 'time', 'quota']);
    return TASK_COLUMNS.filter((c) => !c.adminOnly || userIsAdmin).map((c) => ({
      id: c.id,
      header: () => t(c.labelKey),
      cell: ({ row }) => cells[c.id](row.original),
      meta: { sortable: sortable.has(c.id), className: right.has(c.id) ? 'text-right' : undefined }
    }));
  }, [t, userIsAdmin]);

  const table = useTable({ features: appTableFeatures, data: data || [], columns, manualSorting: true, state: { columnVisibility } });
  const tableRef = useNaturalWidth(layout === 'table' ? onNaturalWidth : null);
  const rows = data || [];

  if (rows.length === 0) {
    return (
      <p className="px-4 py-10 text-center text-sm text-muted-foreground">{searching ? t('logPage.searching') : t('common.noData')}</p>
    );
  }

  if (layout === 'list') {
    // Phone row: the task type and its status, then time, duration and platform.
    return (
      <ul className="divide-y divide-border">
        {rows.map((i) => (
          <li key={i.id} className={cn('flex h-14 items-center gap-3 px-4', ROW_OPENS)} {...rowOpenProps(() => onRowDetail(i))}>
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="min-w-0 truncate font-mono text-[13px] font-medium">{i.action}</span>
                <span className="ml-auto shrink-0 text-[13px]">
                  <TaskStatus t={t} item={i} withReason={false} />
                </span>
              </div>
              <div className="mt-0.5 truncate text-xs tabular-nums text-muted-foreground">
                {[i.submit_time > 0 ? timestamp2stringCompact(i.submit_time) : null, durationText(i), i.platform]
                  .filter(Boolean)
                  .join(' · ')}
              </div>
            </div>
          </li>
        ))}
      </ul>
    );
  }

  // Sort arrows: the active column shows its direction; others show a faint arrow on hover only.
  const sortIcon = (id) => {
    if (orderBy !== id) return <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  return (
    <Table ref={tableRef}>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id} className="hover:bg-transparent">
            {hg.headers.map((h) => (
              <TableHead key={h.id} className={cn('whitespace-nowrap', h.column.columnDef.meta?.className)}>
                {h.column.columnDef.meta?.sortable ? (
                  <button
                    type="button"
                    className={cn(
                      'group/sort inline-flex items-center gap-1 hover:text-foreground',
                      orderBy === h.column.id && 'text-foreground'
                    )}
                    onClick={() => onSort(h.column.id)}
                  >
                    {flexRender(h.column.columnDef.header, h.getContext())}
                    {sortIcon(h.column.id)}
                  </button>
                ) : (
                  flexRender(h.column.columnDef.header, h.getContext())
                )}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id} className={ROW_OPENS} {...rowOpenProps(() => onRowDetail(row.original))}>
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

TaskTable.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array,
  userIsAdmin: PropTypes.bool,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  onRowDetail: PropTypes.func.isRequired,
  searching: PropTypes.bool,
  columnVisibility: PropTypes.object,
  layout: PropTypes.oneOf(['table', 'list']),
  onNaturalWidth: PropTypes.func
};
