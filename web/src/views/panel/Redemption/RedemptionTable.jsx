import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { Copy, Download, Pencil, Trash2, ChevronsUpDown, ArrowUp, ArrowDown } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Checkbox } from '@/components/ui/checkbox';
import { RowActions as ActionColumn } from '@/components/ui/row-actions';
import { renderQuota, timestamp2string } from 'utils/common';

function StatusCell({ item, onToggleStatus }) {
  const { t } = useTranslation();
  if (item.status !== 1 && item.status !== 2) {
    return (
      <Badge variant={item.status === 3 ? 'default' : 'secondary'}>
        {item.status === 3 ? t('analytics_index.used') : t('common.unknown')}
      </Badge>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <Switch checked={item.status === 1} onCheckedChange={() => onToggleStatus(item)} />
      <Badge variant={item.status === 1 ? 'default' : 'secondary'}>{item.status === 1 ? t('common.enable') : t('common.disable')}</Badge>
    </div>
  );
}

function RowActions({ item, onCopyKey, onEdit, onDelete, onExportBatch }) {
  const { t } = useTranslation();
  const editable = item.status === 1 || item.status === 2;
  const actions = [
    { label: t('redemptionPage.exportBatch'), icon: Download, onClick: () => onExportBatch(item) },
    { label: t('common.edit'), icon: Pencil, primary: true, disabled: !editable, onClick: () => onEdit(item.id) },
    { label: t('token_index.copy'), icon: Copy, onClick: () => onCopyKey(item) },
    { label: t('common.delete'), icon: Trash2, destructive: true, primary: true, onClick: () => onDelete(item) }
  ];
  return <ActionColumn actions={actions} />;
}

export default function RedemptionTable({
  redemptions,
  onCopyKey,
  onEdit,
  onDelete,
  onToggleStatus,
  onExportBatch,
  order,
  orderBy,
  onSort,
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
  allSelected,
  someSelected
}) {
  const { t } = useTranslation();

  const columns = useMemo(
    () => [
      {
        id: 'select',
        meta: { className: 'w-10' },
        header: () => (
          <Checkbox
            checked={allSelected}
            indeterminate={someSelected}
            onCheckedChange={() => onToggleSelectAll()}
            aria-label={t('common.selectAll')}
          />
        ),
        cell: (c) => (
          <Checkbox
            checked={selectedIds.includes(c.row.original.id)}
            onCheckedChange={() => onToggleSelect(c.row.original.id)}
            aria-label={t('common.select')}
          />
        )
      },
      { accessorKey: 'id', header: t('redemptionPage.headLabels.id'), meta: { sortable: true } },
      { accessorKey: 'name', header: t('redemptionPage.headLabels.name'), meta: { sortable: true } },
      {
        id: 'status',
        header: t('redemptionPage.headLabels.status'),
        meta: { sortable: true },
        cell: (c) => <StatusCell item={c.row.original} onToggleStatus={onToggleStatus} />
      },
      {
        accessorKey: 'quota',
        header: t('redemptionPage.headLabels.quota'),
        meta: { sortable: true },
        cell: (c) => <span className="font-mono tabular-nums">{renderQuota(c.getValue())}</span>
      },
      {
        accessorKey: 'scope',
        header: t('redemptionPage.headLabels.scope'),
        cell: (c) => {
          const scope = c.getValue() || 'any';
          return <Badge variant={scope === 'any' ? 'secondary' : 'outline'}>{t(`redemptionPage.scopes.${scope}`)}</Badge>;
        }
      },
      {
        accessorKey: 'created_time',
        header: t('redemptionPage.headLabels.createdTime'),
        meta: { sortable: true },
        cell: (c) => timestamp2string(c.getValue())
      },
      {
        accessorKey: 'redeemed_time',
        header: t('redemptionPage.headLabels.redeemedTime'),
        meta: { sortable: true },
        cell: (c) => (c.getValue() ? timestamp2string(c.getValue()) : t('redemptionPage.unredeemed'))
      },
      {
        accessorKey: 'expired_time',
        header: t('redemptionPage.headLabels.expiredTime'),
        meta: { sortable: true },
        cell: (c) => {
          const v = c.getValue();
          if (!v || v <= 0) return t('redemptionPage.neverExpire');
          const expired = v < Math.floor(Date.now() / 1000);
          return (
            <span className={expired ? 'flex items-center gap-1.5' : undefined}>
              {timestamp2string(v)}
              {expired && <Badge variant="destructive">{t('redemptionPage.expired')}</Badge>}
            </span>
          );
        }
      },
      {
        id: 'actions',
        header: t('redemptionPage.headLabels.action'),
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <RowActions item={c.row.original} onCopyKey={onCopyKey} onEdit={onEdit} onDelete={onDelete} onExportBatch={onExportBatch} />
        )
      }
    ],
    [
      t,
      onCopyKey,
      onEdit,
      onDelete,
      onToggleStatus,
      onExportBatch,
      selectedIds,
      onToggleSelect,
      onToggleSelectAll,
      allSelected,
      someSelected
    ]
  );

  const table = useTable({
    features: appTableFeatures,
    data: redemptions,
    columns
  });

  const SortIcon = ({ id }) => {
    if (orderBy !== id) return <ChevronsUpDown className="size-3.5 opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />;
  };

  return (
    <>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id} className="hover:bg-transparent">
                {hg.headers.map((header) => (
                  <TableHead key={header.id} className={`whitespace-nowrap ${header.column.columnDef.meta?.className ?? ''}`}>
                    {header.column.columnDef.meta?.sortable && onSort ? (
                      <button
                        type="button"
                        className="inline-flex items-center gap-1 hover:text-foreground"
                        onClick={() => onSort(header.column.id)}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        <SortIcon id={header.column.id} />
                      </button>
                    ) : (
                      flexRender(header.column.columnDef.header, header.getContext())
                    )}
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
      <DataCards table={table} className="md:hidden" primaryColumnId="name" actionColumnIds={['select', 'actions']} />
    </>
  );
}
