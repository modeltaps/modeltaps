import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { SpendAmount } from 'utils/common';

// ==============================|| DASHBOARD — RECENT ACTIVITY TABLE ||============================== //
// TanStack Table over the same per-day rows the v1 `QuotaLogWeek` renders.

export default function ActivityTable({ rows, loading = false }) {
  const { t } = useTranslation();

  const columns = useMemo(
    () => [
      { accessorKey: 'date', header: t('dashboard_index.date') },
      {
        accessorKey: 'requests',
        header: t('dashboard_index.request_count'),
        cell: (c) => <span className="font-mono tabular-nums">{c.getValue()}</span>,
        meta: { align: 'right' }
      },
      {
        accessorKey: 'amount',
        header: t('dashboard_index.amount'),
        cell: (c) => <SpendAmount value={c.getValue()} className="font-mono tabular-nums" />,
        meta: { align: 'right' }
      },
      {
        accessorKey: 'tokens',
        header: t('dashboard_index.tokens'),
        cell: (c) => <span className="font-mono tabular-nums">{c.getValue()}</span>,
        meta: { align: 'right' }
      },
      {
        accessorKey: 'duration',
        header: t('dashboard_index.request_time'),
        cell: (c) => <span className="font-mono tabular-nums">{c.getValue()}</span>,
        meta: { align: 'right' }
      }
    ],
    [t]
  );

  const table = useTable({
    features: appTableFeatures,
    data: rows,
    columns
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('dashboard_index.week_consumption_log')}</CardTitle>
      </CardHeader>
      <CardContent className="px-0 pb-0">
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id} className="hover:bg-transparent">
                  {hg.headers.map((header) => (
                    <TableHead key={header.id} className={header.column.columnDef.meta?.align === 'right' ? 'text-right' : undefined}>
                      {flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={columns.length} className="py-8 text-center text-sm text-muted-foreground">
                    {t('dashboard_index.loading')}
                  </TableCell>
                </TableRow>
              ) : table.getRowModel().rows.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={columns.length} className="py-8 text-center text-sm text-muted-foreground">
                    {t('dashboard_index.no_data')}
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id} className={cell.column.columnDef.meta?.align === 'right' ? 'text-right' : undefined}>
                        {flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext())}
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
          primaryColumnId="date"
          empty={t('dashboard_index.no_data')}
          searching={loading}
          deframeContainer={false}
        />
      </CardContent>
    </Card>
  );
}
