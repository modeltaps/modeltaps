import PropTypes from 'prop-types';
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ROW_OPENS, rowOpenProps } from '@/components/objects';

import { calculateQuota, renderNumber } from 'utils/common';

// ==============================|| PANEL — MONTHLY INVOICE TABLE ||============================== //
// One row per month (server-side sorting); a row opens that month's statement. Phones get 56px
// rows: the month and amount, then requests and tokens.

const month = (i) => (i.date ? i.date.substring(0, 7) : '');
const amount = (i) => `$${Number(calculateQuota(i.quota, 6)).toFixed(2)}`;

export default function InvoiceTable({ t, data, order, orderBy, onSort, onView, searching, layout = 'table' }) {
  const rows = data || [];

  if (rows.length === 0) {
    return (
      <p className="px-4 py-10 text-center text-sm text-muted-foreground">{searching ? t('logPage.searching') : t('common.noData')}</p>
    );
  }

  if (layout === 'list') {
    return (
      <ul className="divide-y divide-border">
        {rows.map((i) => (
          <li key={i.date} className={cn('flex h-14 items-center gap-3 px-4', ROW_OPENS)} {...rowOpenProps(() => onView(i.date))}>
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="text-sm font-medium tabular-nums">{month(i)}</span>
                <span className="ml-auto shrink-0 font-mono text-sm tabular-nums">{amount(i)}</span>
              </div>
              <div className="mt-0.5 truncate text-xs tabular-nums text-muted-foreground">
                {t('invoice_index.requestCount')} {renderNumber(i.request_count)} · {renderNumber(i.prompt_tokens)} →{' '}
                {renderNumber(i.completion_tokens)}
              </div>
            </div>
          </li>
        ))}
      </ul>
    );
  }

  // Sort arrows: the active column shows its direction; others show a faint arrow on hover only.
  const head = (id, label, className) => (
    <TableHead className={cn('whitespace-nowrap', className)}>
      <button
        type="button"
        className={cn('group/sort inline-flex items-center gap-1 hover:text-foreground', orderBy === id && 'text-foreground')}
        onClick={() => onSort(id)}
      >
        {label}
        {orderBy !== id ? (
          <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />
        ) : order === 'asc' ? (
          <ArrowUp className="size-3" />
        ) : (
          <ArrowDown className="size-3" />
        )}
      </button>
    </TableHead>
  );

  return (
    <Table>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          {head('date', t('invoice_index.date'))}
          {head('quota', t('invoice_index.quota'), 'text-right')}
          <TableHead className="whitespace-nowrap text-right">{t('invoice_index.tokens')}</TableHead>
          {head('request_count', t('invoice_index.requestCount'), 'text-right')}
          {head('request_time', t('invoice_index.requestTime'), 'text-right')}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((i) => (
          <TableRow key={i.date} className={ROW_OPENS} {...rowOpenProps(() => onView(i.date))}>
            <TableCell className="whitespace-nowrap font-medium tabular-nums">{month(i)}</TableCell>
            <TableCell className="whitespace-nowrap text-right font-mono tabular-nums">{amount(i)}</TableCell>
            <TableCell className="whitespace-nowrap text-right tabular-nums">
              {renderNumber(i.prompt_tokens)} / {renderNumber(i.completion_tokens)}
            </TableCell>
            <TableCell className="whitespace-nowrap text-right tabular-nums">{renderNumber(i.request_count)}</TableCell>
            <TableCell className="whitespace-nowrap text-right tabular-nums">{(i.request_time / 1000).toFixed(1)}s</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

InvoiceTable.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  onView: PropTypes.func.isRequired,
  searching: PropTypes.bool,
  layout: PropTypes.oneOf(['table', 'list'])
};
