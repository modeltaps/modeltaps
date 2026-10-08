import PropTypes from 'prop-types';
import { useMemo } from 'react';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { calculateQuota, renderNumber } from 'utils/common';

// ==============================|| PANEL — MONTHLY INVOICE TABLE ||============================== //
// shadcn/Tailwind port of views/Invoice/component/TableRow. Server-side sorting.

export default function InvoiceTable({ t, data, order, orderBy, onSort, onView, searching }) {
  const columns = useMemo(
    () =>
      [
        { id: 'date', sortable: true, header: t('invoice_index.date'), cell: (i) => (i.date ? i.date.substring(0, 7) : '') },
        { id: 'quota', sortable: true, header: t('invoice_index.quota'), cell: (i) => `$${calculateQuota(i.quota, 6)}` },
        {
          id: 'tokens',
          header: t('invoice_index.tokens'),
          cell: (i) => `${renderNumber(i.prompt_tokens)} / ${renderNumber(i.completion_tokens)}`
        },
        { id: 'request_count', sortable: true, header: t('invoice_index.requestCount'), cell: (i) => i.request_count },
        {
          id: 'request_time',
          sortable: true,
          header: t('invoice_index.requestTime'),
          cell: (i) => `${(i.request_time / 1000).toFixed(3)}s`
        },
        {
          id: 'option',
          className: 'sticky right-0 bg-card z-10 text-right w-[1%]',
          header: t('invoice_index.option'),
          cell: (i) => (
            <Button variant="outline" size="sm" onClick={() => onView(i.date)}>
              {t('invoice_index.viewInvoice')}
            </Button>
          )
        }
      ].map((c) => ({
        id: c.id,
        header: () => c.header,
        cell: ({ row }) => c.cell(row.original),
        meta: { sortable: c.sortable, className: c.className }
      })),
    [t, onView]
  );

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
                    <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
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
        primaryColumnId="date"
        actionColumnIds={['option']}
        empty={t('common.noData')}
        searching={searching}
      />
    </>
  );
}

InvoiceTable.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  onView: PropTypes.func.isRequired,
  searching: PropTypes.bool
};
