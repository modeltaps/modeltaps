import PropTypes from 'prop-types';
import { useMemo } from 'react';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ChevronsUpDown, Eye } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { cn } from '@/lib/utils';
import { copy, timestamp2string } from 'utils/common';
import { badgeClass, deriveTaskDuration, statusMeta } from './taskHelpers';

const Badge = ({ color, children }) => <span className={badgeClass(color)}>{children}</span>;
Badge.propTypes = { color: PropTypes.string, children: PropTypes.node };

function FailReason({ text }) {
  if (!text) return '';
  const truncated = text.length > 30 ? `${text.substring(0, 30)}...` : text;
  return (
    <button type="button" title={text} className="text-left hover:text-foreground" onClick={() => copy(text, '')}>
      {truncated}
    </button>
  );
}
FailReason.propTypes = { text: PropTypes.string };

function DurationCell({ item }) {
  const { requestTime, requestTimeStr } = deriveTaskDuration(item);
  if (!requestTimeStr) return '';
  return <Badge color={requestTime > 60 ? 'error' : 'success'}>{requestTimeStr}</Badge>;
}
DurationCell.propTypes = { item: PropTypes.object };

export default function TaskTable({ t, data, userIsAdmin, order, orderBy, onSort, onRowDetail, searching }) {
  const columns = useMemo(() => {
    const cols = [
      { id: 'task_id', header: t('taskPage.task'), cell: (i) => <span className="whitespace-nowrap">{i.task_id}</span> },
      { id: 'submit_time', sortable: true, header: t('taskPage.subTime'), cell: (i) => timestamp2string(i.submit_time) },
      { id: 'finish_time', sortable: true, header: t('taskPage.finishTime'), cell: (i) => timestamp2string(i.finish_time) }
    ];
    if (userIsAdmin) {
      cols.push({ id: 'channel_id', sortable: true, header: t('taskPage.channel'), cell: (i) => i.channel_id || '' });
      cols.push({ id: 'user_id', sortable: true, header: t('taskPage.user'), cell: (i) => i.user_id || '' });
    }
    cols.push({ id: 'platform', sortable: true, header: t('taskPage.platform'), cell: (i) => <Badge color="success">{i.platform}</Badge> });
    cols.push({ id: 'action', header: t('taskPage.type'), cell: (i) => <Badge color="success">{i.action}</Badge> });
    cols.push({ id: 'time', header: t('taskPage.time'), cell: (i) => <DurationCell item={i} /> });
    cols.push({ id: 'progress', header: t('taskPage.progress'), cell: (i) => `${i.progress ?? 0}%` });
    cols.push({
      id: 'status',
      sortable: true,
      header: t('taskPage.status'),
      cell: (i) => {
        const meta = statusMeta(i.status);
        return meta ? (
          <Badge color={meta.color}>{t(meta.labelKey)}</Badge>
        ) : (
          <Badge color="error">{t('taskPage.statusType.unknown')}</Badge>
        );
      }
    });
    cols.push({ id: 'fail_reason', header: t('taskPage.fail'), cell: (i) => <FailReason text={i.fail_reason} /> });
    cols.push({
      id: 'detail',
      className: 'sticky right-0 bg-card z-10 text-right w-[1%]',
      header: t('logPage.detailLabel'),
      cell: (i) => (
        <Button variant="ghost" size="icon" aria-label={t('logPage.detailLabel')} onClick={() => onRowDetail(i)}>
          <Eye className="size-4" />
        </Button>
      )
    });
    return cols.map((c) => ({
      id: c.id,
      header: () => c.header,
      cell: ({ row }) => c.cell(row.original),
      meta: { sortable: c.sortable, className: c.className }
    }));
  }, [t, userIsAdmin, onRowDetail]);

  const table = useTable({
    features: appTableFeatures,
    data: data || [],
    columns,
    manualSorting: true
  });

  const SortIcon = ({ id }) => {
    if (orderBy !== id) return <ChevronsUpDown className="size-3.5 opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />;
  };
  SortIcon.propTypes = { id: PropTypes.string };

  return (
    <>
      <div className="hidden md:block">
        <Table className="min-w-[800px]">
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((h) => {
                  const sortable = h.column.columnDef.meta?.sortable;
                  return (
                    <TableHead key={h.id} className={h.column.columnDef.meta?.className}>
                      {sortable ? (
                        <button
                          type="button"
                          className="inline-flex items-center gap-1 hover:text-foreground"
                          onClick={() => onSort(h.column.id)}
                        >
                          {flexRender(h.column.columnDef.header, h.getContext())}
                          <SortIcon id={h.column.id} />
                        </button>
                      ) : (
                        flexRender(h.column.columnDef.header, h.getContext())
                      )}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center text-sm text-muted-foreground">
                  {searching ? t('logPage.searching') : t('common.noData')}
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      className={cn(cell.column.id === 'detail' && 'text-right', cell.column.columnDef.meta?.className)}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
      <DataCards
        table={table}
        className="md:hidden"
        primaryColumnId="task_id"
        actionColumnIds={['detail']}
        empty={t('common.noData')}
        searching={searching}
      />
    </>
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
  searching: PropTypes.bool
};
