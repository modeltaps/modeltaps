import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { Link, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ArrowUpDown, List, MoreHorizontal, Pencil, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { useNaturalWidth } from '@/components/list-page';
import { VendorIcon } from '../ModelInfo/modelInfoHelpers';

// Deep link to the models tab filtered by vendor (and optionally by catalog state).
export const vendorModelsPath = (vendorId, status) => `/panel/pricing?tab=models&vendor_id=${vendorId}${status ? `&status=${status}` : ''}`;

// Typical table width before it has been measured.
export const VENDOR_TABLE_ESTIMATE = 560;

function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

// A count links to the vendor's models when positive; "no channel" counts are tinted.
function CountCell({ row, field, status }) {
  const value = row[field] || 0;
  if (value === 0) return <Dash />;
  return (
    <RowControl>
      <Link
        to={vendorModelsPath(row.id, status)}
        className={cn('tabular-nums underline-offset-2 hover:underline', status === 'nochannel' && 'text-warning')}
      >
        {value}
      </Link>
    </RowControl>
  );
}

CountCell.propTypes = { row: PropTypes.object.isRequired, field: PropTypes.string.isRequired, status: PropTypes.string };

// Row menu: edit, the vendor's models, then delete (destructive, last).
export function VendorActions({ item, onEdit, onDelete }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('modelOwnedby.action')}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuItem onClick={() => onEdit(item.id)}>
          <Pencil className="size-4" />
          {t('common.edit')}
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!item.model_count} onClick={() => navigate(vendorModelsPath(item.id))}>
          <List className="size-4" />
          {t('modelsPage.viewVendorModels')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDelete(item)}>
          <Trash2 className="size-4" />
          {t('common.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

VendorActions.propTypes = { item: PropTypes.object.isRequired, onEdit: PropTypes.func.isRequired, onDelete: PropTypes.func.isRequired };

// Vendors table: one line per vendor (icon, name, slug in mono), the two counts, the row menu.
// A row opens the vendor; name and counts sort locally.
export default function ModelOwnedbyTable({ items, sort, onSort, onEdit, onDelete, onNaturalWidth }) {
  const { t } = useTranslation();

  const columns = useMemo(
    () => [
      {
        id: 'name',
        header: t('modelOwnedby.name'),
        meta: { sortable: 'name', className: 'max-w-80' },
        cell: (c) => (
          <span className="flex min-w-0 items-center gap-2 whitespace-nowrap">
            <VendorIcon vendor={c.row.original} />
            <span className="truncate font-medium">{c.row.original.name}</span>
            {c.row.original.slug && <code className="shrink-0 font-mono text-xs text-muted-foreground">{c.row.original.slug}</code>}
          </span>
        )
      },
      {
        id: 'model_count',
        header: t('modelsPage.vendorModelCount'),
        meta: { sortable: 'model_count', className: 'w-28 text-right' },
        cell: (c) => <CountCell row={c.row.original} field="model_count" />
      },
      {
        id: 'nochannel_count',
        header: t('modelsPage.stats.nochannel'),
        meta: { sortable: 'nochannel_count', className: 'w-28 text-right' },
        cell: (c) => <CountCell row={c.row.original} field="nochannel_count" status="nochannel" />
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('modelOwnedby.action')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <RowControl className="justify-end">
            <VendorActions item={c.row.original} onEdit={onEdit} onDelete={onDelete} />
          </RowControl>
        )
      }
    ],
    [t, onEdit, onDelete]
  );

  const tableRef = useNaturalWidth(onNaturalWidth);
  const table = useTable({ features: appTableFeatures, data: items, columns, getRowId: (row) => String(row.id) });

  // Sort arrows: the active column shows its direction; others show a faint arrow on hover only.
  const sortIcon = (key) => {
    if (sort.key !== key) return <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />;
    return sort.dir === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  return (
    <Table ref={tableRef}>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id} className="hover:bg-transparent">
            {hg.headers.map((h) => {
              const sortKey = h.column.columnDef.meta?.sortable;
              const label = flexRender(h.column.columnDef.header, h.getContext());
              return (
                <TableHead key={h.id} className={cn('whitespace-nowrap', h.column.columnDef.meta?.className)}>
                  {sortKey ? (
                    <button
                      type="button"
                      className={cn(
                        'group/sort inline-flex items-center gap-1 hover:text-foreground',
                        sort.key === sortKey && 'text-foreground'
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
            className={cn(ROW_OPENS, !row.original.model_count && 'text-muted-foreground')}
            {...rowOpenProps(() => onEdit(row.original.id))}
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

ModelOwnedbyTable.propTypes = {
  items: PropTypes.array.isRequired,
  sort: PropTypes.shape({ key: PropTypes.string, dir: PropTypes.string }).isRequired,
  onSort: PropTypes.func.isRequired,
  onEdit: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  onNaturalWidth: PropTypes.func
};

// Phone list: one 56px row per vendor. The logo tile leads; the name over the slug; on the right
// the model count over the models without a channel; ⋯ holds edit, models and delete.
export function VendorList({ items, onEdit, onDelete }) {
  const { t } = useTranslation();
  return (
    <ul className="divide-y divide-border">
      {items.map((v) => (
        <li key={v.id} className={cn('flex h-14 items-center gap-3 pl-3 pr-1', ROW_OPENS)} {...rowOpenProps(() => onEdit(v.id))}>
          <span className="flex size-7 shrink-0 items-center justify-center rounded-lg border border-border bg-field">
            <VendorIcon vendor={v} />
          </span>
          <div className="min-w-0 flex-1">
            <div className={cn('truncate text-sm font-medium leading-5', !v.model_count && 'text-muted-foreground')}>{v.name}</div>
            <div className="truncate font-mono text-xs leading-4 text-muted-foreground">{v.slug || '—'}</div>
          </div>
          <div className="flex shrink-0 flex-col items-end text-xs leading-4 tabular-nums">
            {v.model_count > 0 ? (
              <>
                <span className="text-[13px] leading-5">{t('modelsPage.vendorModels', { count: v.model_count })}</span>
                {v.nochannel_count > 0 && (
                  <span className="text-warning">{t('modelsPage.vendorNoChannel', { count: v.nochannel_count })}</span>
                )}
              </>
            ) : (
              <span className="text-muted-foreground">{t('modelsPage.vendorUnused')}</span>
            )}
          </div>
          <RowControl className="shrink-0">
            <VendorActions item={v} onEdit={onEdit} onDelete={onDelete} />
          </RowControl>
        </li>
      ))}
    </ul>
  );
}

VendorList.propTypes = { items: PropTypes.array.isRequired, onEdit: PropTypes.func.isRequired, onDelete: PropTypes.func.isRequired };
