import { useMemo } from 'react';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { Pencil, Trash2 } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { RowActions as ActionColumn } from '@/components/ui/row-actions';
import { VendorIcon } from '../ModelInfo/modelInfoHelpers';

// 模型页签按厂商(可选再按状态)筛选的深链。
export const vendorModelsPath = (vendorId, status) => `/panel/pricing?tab=models&vendor_id=${vendorId}${status ? `&status=${status}` : ''}`;

// 计数 > 0 时做成链接,跳到按该厂商筛选的模型表。
const countCell = (warnWhenPositive, status) => (c) => {
  const value = c.getValue() || 0;
  const className = `tabular-nums ${warnWhenPositive && value > 0 ? 'text-warning' : ''}`;
  if (value === 0) return <span className={className}>{value}</span>;
  return (
    <Link to={vendorModelsPath(c.row.original.id, status)} className={`${className} underline-offset-2 hover:underline`}>
      {value}
    </Link>
  );
};

function RowActions({ item, onEdit, onDelete }) {
  const { t } = useTranslation();
  return (
    <ActionColumn
      actions={[
        { label: t('common.edit'), icon: Pencil, onClick: () => onEdit(item.id) },
        { label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => onDelete(item) }
      ]}
    />
  );
}

export default function ModelOwnedbyTable({ items, onEdit, onDelete }) {
  const { t } = useTranslation();

  const columns = useMemo(
    () => [
      {
        accessorKey: 'name',
        header: t('modelOwnedby.name'),
        cell: (c) => (
          <span className="flex items-center gap-2">
            <VendorIcon vendor={c.row.original} />
            <span className="font-medium">{c.getValue()}</span>
          </span>
        )
      },
      {
        accessorKey: 'slug',
        header: t('modelOwnedby.slug'),
        cell: (c) => <span className="font-mono text-xs text-muted-foreground">{c.getValue() || '—'}</span>
      },
      { accessorKey: 'model_count', header: t('modelsPage.vendorModelCount'), meta: { className: 'text-right' }, cell: countCell(false) },
      {
        accessorKey: 'nochannel_count',
        header: t('modelsPage.stats.nochannel'),
        meta: { className: 'text-right' },
        cell: countCell(true, 'nochannel')
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('modelOwnedby.action')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => <RowActions item={c.row.original} onEdit={onEdit} onDelete={onDelete} />
      }
    ],
    [t, onEdit, onDelete]
  );

  const table = useTable({
    features: appTableFeatures,
    data: items,
    columns
  });

  return (
    <>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id} className="hover:bg-transparent">
                {hg.headers.map((header) => (
                  <TableHead key={header.id} className={`whitespace-nowrap ${header.column.columnDef.meta?.className ?? ''}`}>
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
                    {flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <DataCards table={table} className="md:hidden" primaryColumnId="name" actionColumnIds={['actions']} />
    </>
  );
}
