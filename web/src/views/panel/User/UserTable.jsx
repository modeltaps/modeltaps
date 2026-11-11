import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { estimateWidth, useNaturalWidth } from '@/components/list-page';
import { renderNumber, renderQuota, renderSpend, timestamp2string } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';
import { ActionsCell, StatusCell } from './UserRowCells';

// Typical rendered widths (px) per column, the estimate used before the table has been measured.
const COLUMN_WIDTHS = {
  select: 32,
  user: 220,
  role: 96,
  group: 96,
  org: 120,
  quota: 96,
  used_quota: 96,
  request_count: 72,
  last_login_time: 96,
  status: 64,
  actions: 56
};
export const estimateUserTableWidth = (columnIds, visibility) => estimateWidth(columnIds, visibility, COLUMN_WIDTHS);

export function roleBadge(t, role) {
  const map = {
    1: ['secondary', t('userPage.cUserRole')],
    3: ['secondary', t('userPage.reliableUserRole')],
    10: ['outline', t('userPage.adminUserRole')],
    100: ['outline', t('userPage.superAdminRole')]
  };
  const [variant, label] = map[role] || ['destructive', t('userPage.uUserRole')];
  return (
    <Badge variant={variant} className="whitespace-nowrap">
      {label}
    </Badge>
  );
}

// Plain role name, for the phone list's second line.
export const roleName = (t, role) =>
  ({ 1: t('userPage.cUserRole'), 3: t('userPage.reliableUserRole'), 10: t('userPage.adminUserRole'), 100: t('userPage.superAdminRole') })[
    role
  ] || t('userPage.uUserRole');

// One line: initials, the username, then the display name or email muted after it.
function UserCell({ user }) {
  const { t } = useTranslation();
  const secondary = user.display_name || user.email;
  return (
    <div className="flex min-w-0 items-center gap-2 whitespace-nowrap">
      <span className="flex size-6 flex-none items-center justify-center rounded-full bg-muted text-[11px] font-medium uppercase">
        {(user.username || '?').slice(0, 2)}
      </span>
      <span className="truncate font-medium">{user.username}</span>
      {user.is_shadow && (
        <Badge variant="secondary" className="shrink-0 px-1.5 py-0 font-normal">
          {t('userPage.shadowAccount')}
        </Badge>
      )}
      {secondary && <span className="min-w-0 truncate text-xs text-muted-foreground">{secondary}</span>}
    </div>
  );
}

UserCell.propTypes = { user: PropTypes.object.isRequired };

// Last login as a short relative time; the exact time and the IP are in the tooltip.
export function LastLogin({ user, rtf }) {
  const { t } = useTranslation();
  if (!user.last_login_time) return <span className="text-[13px] text-muted-foreground">{t('userPage.neverLoggedIn')}</span>;
  const title = [timestamp2string(user.last_login_time), user.last_login_ip].filter(Boolean).join(' · ');
  return (
    <span className="text-[13px]" title={title}>
      {relativeTime(rtf, user.last_login_time)}
    </span>
  );
}

LastLogin.propTypes = { user: PropTypes.object.isRequired, rtf: PropTypes.object.isRequired };

export default function UserTable({
  users,
  columnVisibility,
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
  openId,
  onOpen,
  order,
  orderBy,
  onSort,
  manageUser,
  onNaturalWidth
}) {
  const { t, i18n } = useTranslation();
  const rtfLocale = (i18n.language || 'zh_CN').replace('_', '-');

  const columns = useMemo(() => {
    const rtf = new Intl.RelativeTimeFormat(rtfLocale, { numeric: 'auto' });
    const allSelected = users.length > 0 && users.every((u) => selectedIds.has(u.id));
    const someSelected = users.some((u) => selectedIds.has(u.id));
    const num = (key, render) => ({
      id: key,
      meta: { sortable: key, className: 'text-right' },
      cell: (c) => <span className="tabular-nums">{render(c.row.original[key])}</span>
    });
    return [
      {
        id: 'select',
        header: () => (
          <Checkbox
            checked={allSelected}
            indeterminate={!allSelected && someSelected}
            onCheckedChange={() => onToggleSelectAll(!allSelected)}
            aria-label={t('listPage.selectAllOnPage')}
          />
        ),
        meta: { className: 'w-[1%] pr-0' },
        cell: (c) => (
          <RowControl className="flex">
            <Checkbox
              checked={selectedIds.has(c.row.original.id)}
              onCheckedChange={() => onToggleSelect(c.row.original.id)}
              aria-label={t('listPage.selectRow', { name: c.row.original.username })}
            />
          </RowControl>
        )
      },
      {
        id: 'user',
        header: t('userPage.username'),
        meta: { sortable: 'id', className: 'max-w-72' },
        cell: (c) => <UserCell user={c.row.original} />
      },
      { id: 'role', header: t('userPage.userRole'), cell: (c) => roleBadge(t, c.row.original.role) },
      {
        id: 'group',
        header: t('userPage.group'),
        cell: (c) => (
          <Badge variant="outline" className="whitespace-nowrap">
            {c.row.original.group || '-'}
          </Badge>
        )
      },
      {
        id: 'org',
        header: t('userPage.org'),
        cell: (c) => <span className="text-[13px]">{c.row.original.org_name || <span className="text-muted-foreground">—</span>}</span>
      },
      { ...num('quota', renderQuota), header: t('userPage.balance') },
      { ...num('used_quota', renderSpend), header: t('userPage.usedShort') },
      { ...num('request_count', renderNumber), header: t('userPage.requestCountShort') },
      {
        id: 'last_login_time',
        header: t('userPage.lastLoginTime'),
        meta: { sortable: 'last_login_time' },
        cell: (c) => <LastLogin user={c.row.original} rtf={rtf} />
      },
      // One status control: the switch (its label says enabled / disabled).
      {
        id: 'status',
        header: t('userPage.status'),
        cell: (c) => (
          <RowControl className="flex">
            <StatusCell item={c.row.original} manageUser={manageUser} />
          </RowControl>
        )
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('userPage.action')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <RowControl className="justify-end">
            <ActionsCell item={c.row.original} manageUser={manageUser} onOpen={onOpen} />
          </RowControl>
        )
      }
    ];
  }, [t, rtfLocale, users, selectedIds, onToggleSelect, onToggleSelectAll, onOpen, manageUser]);

  const tableRef = useNaturalWidth(onNaturalWidth);
  const table = useTable({
    features: appTableFeatures,
    data: users,
    columns,
    state: { columnVisibility },
    getRowId: (row) => String(row.id)
  });

  // Sort arrows: the active column shows its direction; others show a faint arrow on hover only.
  const sortIcon = (key) => {
    if (orderBy !== key) return <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
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
                        orderBy === sortKey && 'text-foreground'
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
            className={ROW_OPENS}
            data-state={selectedIds.has(row.original.id) || row.original.id === openId ? 'selected' : undefined}
            {...rowOpenProps(() => onOpen(row.original.id))}
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

UserTable.propTypes = {
  users: PropTypes.array.isRequired,
  columnVisibility: PropTypes.object.isRequired,
  selectedIds: PropTypes.object.isRequired,
  onToggleSelect: PropTypes.func.isRequired,
  onToggleSelectAll: PropTypes.func.isRequired,
  openId: PropTypes.number,
  onOpen: PropTypes.func.isRequired,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  manageUser: PropTypes.func.isRequired,
  onNaturalWidth: PropTypes.func
};

// Phone list: one 56px row per user. The status dot leads; the first line is the username with
// the balance on the right, the second the display name or email · role · last login, with the
// spend on the right. A tap opens the user; ⋯ holds the row actions; select mode shows checkboxes.
export function UserList({ users, rtf, selectMode, selectedIds, onToggleSelect, openId, onOpen, manageUser }) {
  const { t } = useTranslation();
  return (
    <ul className="divide-y divide-border">
      {users.map((u) => {
        const selected = selectedIds.has(u.id);
        const enabled = u.status === 1;
        const who = u.display_name || u.email;
        return (
          <li
            key={u.id}
            className={cn('flex h-14 items-center gap-3 px-4', ROW_OPENS, (selected || u.id === openId) && 'bg-muted/60')}
            {...rowOpenProps(() => (selectMode ? onToggleSelect(u.id) : onOpen(u.id)))}
          >
            {selectMode ? (
              <RowControl>
                <Checkbox
                  checked={selected}
                  onCheckedChange={() => onToggleSelect(u.id)}
                  aria-label={t('listPage.selectRow', { name: u.username })}
                />
              </RowControl>
            ) : (
              <span
                className={cn('size-2 shrink-0 rounded-full', enabled ? 'bg-success' : 'bg-muted-foreground/40')}
                title={enabled ? t('userPage.statusEnabled') : t('userPage.statusDisabled')}
                aria-hidden="true"
              />
            )}
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="min-w-0 truncate text-sm font-medium">{u.username}</span>
                <span className="ml-auto shrink-0 text-sm tabular-nums">{renderQuota(u.quota)}</span>
              </div>
              <div className="mt-0.5 flex items-baseline gap-3 text-xs text-muted-foreground">
                <span className="min-w-0 truncate">
                  {[who, roleName(t, u.role)].filter(Boolean).join(' · ')}
                  {' · '}
                  {u.last_login_time ? relativeTime(rtf, u.last_login_time) : t('userPage.neverLoggedIn')}
                </span>
                <span className="ml-auto shrink-0 tabular-nums">{renderSpend(u.used_quota)}</span>
              </div>
            </div>
            {!selectMode && (
              <RowControl className="-mr-2 shrink-0">
                <ActionsCell item={u} manageUser={manageUser} onOpen={onOpen} />
              </RowControl>
            )}
          </li>
        );
      })}
    </ul>
  );
}

UserList.propTypes = {
  users: PropTypes.array.isRequired,
  rtf: PropTypes.object.isRequired,
  selectMode: PropTypes.bool,
  selectedIds: PropTypes.object.isRequired,
  onToggleSelect: PropTypes.func.isRequired,
  openId: PropTypes.number,
  onOpen: PropTypes.func.isRequired,
  manageUser: PropTypes.func.isRequired
};
