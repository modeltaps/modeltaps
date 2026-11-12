import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { Plus, RotateCcw } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ROW_OPENS, RowControl, rowOpenProps, useObjectRoute } from '@/components/objects';
import { ColumnPicker, ListEmptyState, ListFooter, ListSkeleton, useColumnVisibility, useListLayout } from '@/components/list-page';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import usePaginatedList from 'hooks/usePaginatedList';
import UserGroupPanel from './UserGroupPanel';
import NewUserGroupSheet from './NewUserGroupSheet';
import { StatusCell, ActionsCell } from './UserGroupRowCells';

// Columns: name (with the symbol) and the row menu are fixed; the id is one click away.
const GROUP_COLUMNS = [
  { id: 'id', labelKey: 'userGroup.id', defaultHidden: true },
  { id: 'name', fixed: true },
  { id: 'ratio', labelKey: 'userGroup.ratio' },
  { id: 'api_rate', labelKey: 'userGroup.rateLimit' },
  { id: 'public', labelKey: 'userGroup.publicShort' },
  { id: 'promotion', labelKey: 'userGroup.promotion' },
  { id: 'enable', labelKey: 'userGroup.enableShort' },
  { id: 'action', fixed: true }
];

const money = (v) => `$${Number(v || 0).toLocaleString()}`;

// Auto-promotion as one condition: users whose cumulative top-up falls in [min, max) move into this
// group; max 0 means no upper bound. A muted dash while promotion is off.
export function promotionText(t, group) {
  if (!group.promotion) return null;
  if (!group.max) return t('userGroup.promotionFrom', { min: money(group.min) });
  return t('userGroup.promotionRange', { min: money(group.min), max: money(group.max) });
}

function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

// One line: the display name, then the symbol in mono after it.
function NameCell({ group }) {
  return (
    <span className="flex min-w-0 items-center gap-2 whitespace-nowrap">
      <span className="truncate font-medium">{group.name || group.symbol}</span>
      <code className="shrink-0 font-mono text-xs text-muted-foreground">{group.symbol}</code>
    </span>
  );
}

NameCell.propTypes = { group: PropTypes.object.isRequired };

// User groups: a row opens the group panel (?id=, saved as a whole); the new group sheet (?new=1)
// asks only for the symbol and the name.
export default function UserGroup() {
  const { t } = useTranslation();
  const route = useObjectRoute(['settings']);
  const [pendingDelete, setPendingDelete] = useState(null);
  // Bumped after a list action so an open group panel reloads.
  const [reloadKey, setReloadKey] = useState(0);

  // Paginated list (fixed order -id; no header sorting on this page).
  const {
    page,
    setPage,
    rowsPerPage,
    listCount,
    searching,
    loaded,
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
        if (action === 'delete' && id === route.openId) route.close();
        else setReloadKey((k) => k + 1);
        refresh();
      } else {
        showError(res.data.message);
      }
      return res.data;
    } catch (e) {
      return undefined;
    }
  };

  const toggle = (item) => manageUserGroup(item.id, 'status');
  const confirmDelete = () => {
    const item = pendingDelete;
    setPendingDelete(null);
    if (item) manageUserGroup(item.id, 'delete');
  };

  const columnState = useColumnVisibility({ storageKey: 'user-group-column-visibility-v1', columns: GROUP_COLUMNS });
  // Full table while it fits (the sidebar collapses first), phone list otherwise.
  const { narrow } = useListLayout();

  const columns = useMemo(
    () => [
      { id: 'id', header: t('userGroup.id'), cell: (c) => <span className="tabular-nums text-muted-foreground">{c.row.original.id}</span> },
      { id: 'name', header: t('userGroup.name'), meta: { className: 'max-w-72' }, cell: (c) => <NameCell group={c.row.original} /> },
      {
        id: 'ratio',
        header: t('userGroup.ratio'),
        meta: { className: 'text-right' },
        cell: (c) => <span className="tabular-nums">×{c.row.original.ratio}</span>
      },
      {
        id: 'api_rate',
        header: t('userGroup.rateLimit'),
        meta: { className: 'text-right' },
        cell: (c) => <span className="tabular-nums">{t('userGroup.perMinute', { count: c.row.original.api_rate })}</span>
      },
      { id: 'public', header: t('userGroup.publicShort'), cell: (c) => (c.row.original.public ? t('userGroup.publicYes') : <Dash />) },
      { id: 'promotion', header: t('userGroup.promotion'), cell: (c) => promotionText(t, c.row.original) || <Dash /> },
      {
        id: 'enable',
        header: t('userGroup.enableShort'),
        cell: (c) => (
          <RowControl className="flex">
            <StatusCell item={c.row.original} onToggle={toggle} />
          </RowControl>
        )
      },
      {
        id: 'action',
        header: () => <span className="sr-only">{t('userPage.action')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <RowControl className="justify-end">
            <ActionsCell item={c.row.original} onOpen={(id) => route.open(id)} onDelete={setPendingDelete} />
          </RowControl>
        )
      }
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, route.openId]
  );
  const table = useTable({ features: appTableFeatures, data: groups, columns, state: { columnVisibility: columnState.visibility } });

  let body;
  if (!loaded) {
    body = <ListSkeleton layout={narrow ? 'list' : 'table'} rows={3} checkbox={false} cells={[48, 72, 120, 32]} />;
  } else if (!searching && groups.length === 0) {
    body = (
      <ListEmptyState
        title={t('userGroup.emptyTitle')}
        action={
          <Button size="sm" onClick={route.openNew}>
            <Plus className="size-4" /> {t('userGroup.create')}
          </Button>
        }
      />
    );
  } else if (narrow) {
    // Phone list: one 56px row per group. Enabled dot, name and symbol, the ratio on the right;
    // then public · auto-promotion · rate limit.
    body = (
      <ul className="divide-y divide-border">
        {groups.map((g) => {
          const promo = promotionText(t, g);
          return (
            <li
              key={g.id}
              className={cn('flex h-14 items-center gap-3 px-4', ROW_OPENS, g.id === route.openId && 'bg-muted/60')}
              {...rowOpenProps(() => route.open(g.id))}
            >
              <span
                className={cn('size-2 shrink-0 rounded-full', g.enable ? 'bg-success' : 'bg-muted-foreground/40')}
                title={g.enable ? t('userGroup.enabled') : t('userGroup.disabled')}
                aria-hidden="true"
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-baseline gap-2">
                  <span className="min-w-0 truncate text-sm font-medium">{g.name || g.symbol}</span>
                  <code className="shrink-0 font-mono text-xs text-muted-foreground">{g.symbol}</code>
                  <span className="ml-auto shrink-0 text-sm tabular-nums">×{g.ratio}</span>
                </div>
                <div className="mt-0.5 truncate text-xs text-muted-foreground">
                  {[g.public ? t('userGroup.publicYes') : null, promo, t('userGroup.perMinute', { count: g.api_rate })]
                    .filter(Boolean)
                    .join(' · ')}
                </div>
              </div>
              <RowControl className="-mr-2 shrink-0">
                <ActionsCell item={g} onOpen={(id) => route.open(id)} onDelete={setPendingDelete} onToggle={toggle} />
              </RowControl>
            </li>
          );
        })}
      </ul>
    );
  } else {
    body = (
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((hg) => (
            <TableRow key={hg.id} className="hover:bg-transparent">
              {hg.headers.map((h) => (
                <TableHead key={h.id} className={cn('whitespace-nowrap', h.column.columnDef.meta?.className)}>
                  {flexRender(h.column.columnDef.header, h.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.map((row) => (
            <TableRow
              key={row.id}
              className={cn(ROW_OPENS, !row.original.enable && 'text-muted-foreground')}
              data-state={row.original.id === route.openId ? 'selected' : undefined}
              {...rowOpenProps(() => route.open(row.original.id))}
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

  return (
    <div className="space-y-3">
      {/* Title row: refresh (icon), then create. */}
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
        <ResponsiveToolbarButton primary icon={Plus} label={t('userGroup.create')} onClick={route.openNew} />
      </PageActions>

      {!narrow && (
        <div className="flex justify-end">
          <ColumnPicker columns={columnState} />
        </div>
      )}

      <Card>
        {body}
        <div className="border-t border-border">
          <ListFooter total={listCount} page={page} pageSize={rowsPerPage} onPageChange={setPage} onPageSizeChange={onRowsPerPageChange} />
        </div>
      </Card>

      {route.openId > 0 && (
        <UserGroupPanel
          key={route.openId}
          groupId={route.openId}
          onClose={route.close}
          onChanged={refresh}
          onToggle={toggle}
          onDelete={setPendingDelete}
          reloadKey={reloadKey}
        />
      )}

      {route.isNew && (
        <NewUserGroupSheet
          onClose={route.close}
          onCreated={(symbol) => {
            showSuccess(t('userGroup.new.created', { symbol }));
            route.close();
            setPage(0);
            refresh();
          }}
        />
      )}

      <Dialog open={Boolean(pendingDelete)} onOpenChange={(o) => !o && setPendingDelete(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <p className="text-sm text-muted-foreground">{t('common.deleteConfirm', { title: pendingDelete?.name })}</p>
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)}>
              {t('userPage.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmDelete}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
