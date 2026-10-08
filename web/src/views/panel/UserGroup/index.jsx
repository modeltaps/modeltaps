import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { Plus, RotateCcw } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { PAGE_SIZE_OPTIONS } from 'constants';
import usePaginatedList from 'hooks/usePaginatedList';
import UserGroupSheet from './UserGroupSheet';
import { StatusCell, ActionsCell } from './UserGroupRowCells';
import Pagination from '../components/Pagination';

const boolBadge = (t, v) => (
  <Badge variant="outline" className={v ? 'border-border text-foreground' : 'border-destructive text-destructive'}>
    {v ? t('common.yes') : t('common.no')}
  </Badge>
);

export default function UserGroup() {
  const { t } = useTranslation();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editId, setEditId] = useState(0);

  // 分页列表状态 + 守卫化 fetch 生命周期(UX-13);本页固定 order=-id,无表头排序。
  const {
    page,
    setPage,
    rowsPerPage,
    listCount,
    searching,
    rows: groups,
    refresh,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'userGroup',
    fetcher: async ({ page, rowsPerPage }) => {
      const res = await API.get('/api/user_group/', { params: { page: page + 1, size: rowsPerPage, order: '-id' } });
      return res.data;
    }
  });

  const manageUserGroup = async (id, action) => {
    const url = '/api/user_group/';
    try {
      let res;
      if (action === 'delete') res = await API.delete(url + id);
      else if (action === 'status') res = await API.put(`${url}enable/${id}`);
      else return false;
      if (res.data.success) {
        showSuccess(t('userPage.operationSuccess'));
        refresh();
      } else {
        showError(res.data.message);
      }
      return res.data;
    } catch (e) {
      return undefined;
    }
  };

  const openEdit = (id) => {
    setEditId(id);
    setSheetOpen(true);
  };

  const columns = useMemo(
    () => [
      { accessorKey: 'id', header: t('userGroup.id') },
      { accessorKey: 'symbol', header: t('userGroup.symbol') },
      { accessorKey: 'name', header: t('userGroup.name') },
      { accessorKey: 'ratio', header: t('userGroup.ratio') },
      { accessorKey: 'api_rate', header: t('userGroup.apiRate') },
      { accessorKey: 'public', header: t('userGroup.public'), cell: (c) => boolBadge(t, c.getValue()) },
      { accessorKey: 'promotion', header: t('userGroup.promotion'), cell: (c) => boolBadge(t, c.getValue()) },
      { accessorKey: 'min', header: t('userGroup.min') + '($)' },
      { accessorKey: 'max', header: t('userGroup.max') + '($)' },
      { id: 'enable', header: t('userGroup.enable'), cell: (c) => <StatusCell item={c.row.original} manageUserGroup={manageUserGroup} /> },
      {
        id: 'action',
        header: t('userPage.action'),
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => <ActionsCell item={c.row.original} manageUserGroup={manageUserGroup} onEdit={openEdit} />
      }
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t]
  );

  const table = useTable({
    features: appTableFeatures,
    data: groups,
    columns
  });

  return (
    <div className="space-y-6">
      {/* 标题行右侧操作:刷新(图标) → 新建,对齐 User / ModelInfo 页。 */}
      <PageActions>
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                aria-label={t('userPage.refresh')}
                onClick={() => {
                  setPage(0);
                  refresh();
                }}
              >
                <RotateCcw className={searching ? 'size-4 animate-spin' : 'size-4'} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t('userPage.refresh')}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <ResponsiveToolbarButton primary icon={Plus} label={t('userGroup.create')} onClick={() => openEdit(0)} />
      </PageActions>

      <Card>
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id} className="hover:bg-transparent">
                  {hg.headers.map((h) => (
                    <TableHead key={h.id} className={`whitespace-nowrap ${h.column.columnDef.meta?.className ?? ''}`}>
                      {flexRender(h.column.columnDef.header, h.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={columns.length} className="py-10 text-center text-sm text-muted-foreground">
                    {t('common.noData', { defaultValue: 'No data' })}
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
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
          primaryColumnId="name"
          actionColumnIds={['action']}
          empty={t('common.noData', { defaultValue: 'No data' })}
          searching={searching}
        />
        <Pagination
          page={page}
          rowsPerPage={rowsPerPage}
          count={listCount}
          options={PAGE_SIZE_OPTIONS}
          onPageChange={setPage}
          onRowsPerPageChange={onRowsPerPageChange}
        />
      </Card>

      <UserGroupSheet
        open={sheetOpen}
        groupId={editId}
        onClose={() => setSheetOpen(false)}
        onSaved={() => {
          setSheetOpen(false);
          refresh();
        }}
      />
    </div>
  );
}
