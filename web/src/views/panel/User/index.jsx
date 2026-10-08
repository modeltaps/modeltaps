import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ArrowUpDown, Building2, Plus, RotateCcw, Trash2, UserCheck, UserX } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { filterStateToParams, paramsToFilterState } from '@/components/filter-bar';
import { BatchBar, ListEmptyState, ListFooter, ListPage, ListToolbar, StatChip } from '@/components/list-page';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { API } from 'utils/api';
import { showError, showSuccess, renderQuota, renderSpend, renderNumber, timestamp2string, useIsRoot } from 'utils/common';
import usePaginatedList from 'hooks/usePaginatedList';
import UserSheet from './UserSheet';
import { StatusCell, StatusDot, ActionsCell } from './UserRowCells';

// 工具栏分面:登录方式(当前仅 no_oidc,未绑任何启用提供方身份)与组织(org_id,候选项来自 root 专属的
// 组织列表接口;非 root 管理员无候选项时不展示该分面)。关键字走工具栏搜索框,URL 键仍为 keyword。
const buildFilterFields = (orgs) => [
  {
    key: 'login_method',
    labelKey: 'userPage.loginMethodFilterLabel',
    type: 'enum',
    single: true,
    supportsExclude: false,
    paramInclude: 'login_method',
    options: [{ value: 'no_oidc', labelKey: 'userPage.loginMethodNoOidc' }]
  },
  ...(orgs.length > 0
    ? [
        {
          key: 'org_id',
          labelKey: 'userPage.orgFilterLabel',
          type: 'enum',
          single: true,
          supportsExclude: false,
          paramInclude: 'org_id',
          options: orgs.map((o) => ({ value: String(o.id), label: o.name }))
        }
      ]
    : [])
];
// 参数映射只依赖 key/type/single/paramInclude,URL 解析用带占位组织的静态描述即可。
const USER_FILTER_FIELDS = buildFilterFields([{ id: 0, name: '' }]);
const USER_FILTER_PARAM_KEYS = ['keyword', 'login_method', 'org_id', 'quick'];

// 页头统计 chip 对应的列表参数;「未绑 OIDC」与覆盖统计口径一致(仅已启用用户)。
const QUICK_FILTERS = {
  enabled: { status: 1 },
  disabled: { status: 2 },
  admin: { min_role: 10 },
  nooidc: { status: 1, login_method: 'no_oidc' }
};

const KEYWORD_DEBOUNCE_MS = 300;

function roleBadge(t, role) {
  const map = {
    1: ['secondary', t('userPage.cUserRole')],
    3: ['default', t('userPage.reliableUserRole')],
    10: ['default', t('userPage.adminUserRole')],
    100: ['outline', t('userPage.superAdminRole')]
  };
  const [variant, label] = map[role] || ['destructive', t('userPage.uUserRole')];
  const cls = role === 10 ? 'bg-orange-500 text-white' : role === 100 ? 'bg-green-600 text-white' : undefined;
  return (
    <Badge variant={variant} className={cls ? cls + ' whitespace-nowrap' : 'whitespace-nowrap'}>
      {label}
    </Badge>
  );
}

function SortHeader({ id, label, order, orderBy, onSort, alignEnd = false }) {
  const active = orderBy === id;
  const Icon = active ? (order === 'asc' ? ArrowUp : ArrowDown) : ArrowUpDown;
  return (
    <button
      type="button"
      onClick={() => onSort(id)}
      className={cn(
        'inline-flex items-center gap-1 whitespace-nowrap',
        active ? 'font-semibold text-foreground' : 'hover:text-foreground',
        alignEnd && 'ml-auto'
      )}
    >
      {label}
      <Icon className={cn('size-3.5', !active && 'opacity-50')} />
    </button>
  );
}

SortHeader.propTypes = {
  id: PropTypes.string.isRequired,
  label: PropTypes.node.isRequired,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  alignEnd: PropTypes.bool
};

function UserCell({ user }) {
  const { t } = useTranslation();
  const secondary = [user.display_name, user.email].filter(Boolean).join(' · ');
  return (
    <div className="flex min-w-0 items-center gap-2">
      <span className="flex size-8 flex-none items-center justify-center rounded-full bg-muted text-xs font-semibold uppercase">
        {(user.username || '?').slice(0, 2)}
      </span>
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="truncate font-medium">{user.username}</span>
          {user.is_shadow && (
            <Badge variant="secondary" className="whitespace-nowrap px-1.5 py-0 font-normal">
              {t('userPage.shadowAccount')}
            </Badge>
          )}
        </div>
        {secondary && <div className="truncate text-xs text-muted-foreground">{secondary}</div>}
      </div>
    </div>
  );
}

UserCell.propTypes = { user: PropTypes.object.isRequired };

function OrgCell({ user }) {
  if (!user.org_name) return <span className="text-muted-foreground">—</span>;
  return (
    <span className="inline-flex max-w-44 items-center gap-1 text-sm">
      <Building2 className="size-3.5 flex-none text-muted-foreground" />
      <span className="truncate">{user.org_name}</span>
    </span>
  );
}

OrgCell.propTypes = { user: PropTypes.object.isRequired };

export default function Users() {
  const { t } = useTranslation();
  const isRoot = useIsRoot();
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(USER_FILTER_FIELDS, searchParams));
  const [quick, setQuick] = useState(() => (QUICK_FILTERS[searchParams.get('quick')] ? searchParams.get('quick') : ''));
  const [keywordInput, setKeywordInput] = useState(() => searchParams.get('keyword') || '');
  const [keyword, setKeyword] = useState(() => (searchParams.get('keyword') || '').trim());
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editId, setEditId] = useState(0);
  const [stats, setStats] = useState(null);
  const [coverage, setCoverage] = useState(null);
  const [orgs, setOrgs] = useState([]);
  const [selectedIds, setSelectedIds] = useState(() => new Set());
  const [batchBusy, setBatchBusy] = useState(false);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);

  // 页头统计(/api/user/stats)与 OIDC 迁移覆盖(/api/user/oidc_coverage)与列表分页/筛选无关,
  // 单独取数,随刷新按钮与行 / 批量操作一起更新。
  const loadStats = useCallback(async () => {
    try {
      const [sres, cres] = await Promise.all([API.get('/api/user/stats'), API.get('/api/user/oidc_coverage')]);
      if (sres.data?.success) setStats(sres.data.data);
      if (cres.data?.success) setCoverage(cres.data.data);
    } catch (e) {
      // 全局拦截器已提示
    }
  }, []);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  // 组织分面候选项:组织列表接口仅 root 可用。
  useEffect(() => {
    if (!isRoot) return;
    API.get('/api/organization/', { params: { page: 1, size: 100 } })
      .then((res) => {
        if (res.data?.success) setOrgs(res.data.data?.data || []);
      })
      .catch(() => {});
  }, [isRoot]);

  // 分页列表状态 + 守卫化 fetch 生命周期(UX-13);刷新按钮回到第一页但保留排序、排序切换回到第一页。
  const {
    page,
    setPage,
    rowsPerPage,
    order,
    orderBy,
    listCount,
    searching,
    rows: users,
    refresh,
    handleRefresh,
    handleSort,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'user',
    onSuccess: () => setLoaded(true),
    fetcher: async ({ page, rowsPerPage, order, orderBy }) => {
      const sortOrder = orderBy ? (order === 'desc' ? '-' + orderBy : orderBy) : '-id';
      const params = {
        page: page + 1,
        size: rowsPerPage,
        order: sortOrder,
        ...(keyword ? { keyword } : {}),
        ...filterStateToParams(USER_FILTER_FIELDS, filterState),
        ...(QUICK_FILTERS[quick] || {})
      };
      const res = await API.get('/api/user/', { params });
      return res.data;
    },
    resetPageOnSort: true,
    resetSortOnRefresh: false,
    resetPageOnRefresh: true,
    deps: [filterState, keyword, quick]
  });

  const reload = () => {
    refresh();
    loadStats();
  };

  // 搜索框输入防抖后才应用为关键字筛选并回到第一页。
  useEffect(() => {
    const handle = setTimeout(() => {
      const next = keywordInput.trim();
      if (next === keyword) return;
      setPage(0);
      setKeyword(next);
    }, KEYWORD_DEBOUNCE_MS);
    return () => clearTimeout(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keywordInput]);

  // 列表重拉后清空勾选(翻页 / 筛选 / 操作后的行集合已变化)。
  useEffect(() => {
    setSelectedIds(new Set());
  }, [users]);

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        USER_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(USER_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        if (keyword) params.set('keyword', keyword);
        if (quick) params.set('quick', quick);
        return params;
      },
      { replace: true }
    );
  }, [filterState, keyword, quick, setSearchParams]);

  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
    setQuick('');
    setKeywordInput('');
    setKeyword('');
  };
  const applyQuick = (value) => {
    setPage(0);
    setQuick(value);
  };

  const filterFields = useMemo(() => buildFilterFields(orgs), [orgs]);
  const hasFilters = !!keyword || !!quick || Object.keys(filterStateToParams(USER_FILTER_FIELDS, filterState)).length > 0;

  const manageUser = async (userId, action, value) => {
    let url = '/api/user/manage';
    let body = {};
    if (action === 'delete') body = { user_id: userId, action: 'delete' };
    else if (action === 'status') body = { user_id: userId, action: value === 1 ? 'enable' : 'disable' };
    else if (action === 'set_role') body = { user_id: userId, action: value === 1 ? 'demote' : value === 3 ? 'set_reliable' : 'promote' };
    else if (action === 'quota') {
      url = '/api/user/quota/' + userId;
      body = value;
    }
    try {
      const res = await API.post(url, body);
      if (res.data.success) {
        showSuccess(t('userPage.operationSuccess'));
        reload();
      } else {
        showError(res.data.message);
      }
      return res.data;
    } catch (e) {
      return undefined;
    }
  };

  // 批量启用 / 禁用 / 删除:后端无批量接口,逐条调用 manage 并汇总失败数。
  const runBatch = async (action) => {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    setBatchBusy(true);
    try {
      const results = await Promise.allSettled(ids.map((id) => API.post('/api/user/manage', { user_id: id, action })));
      const failed = results.filter((r) => r.status !== 'fulfilled' || !r.value.data?.success).length;
      if (failed > 0) showError(t('userPage.batchFailed', { count: failed }));
      if (failed < ids.length) showSuccess(t('userPage.operationSuccess'));
    } finally {
      setBatchBusy(false);
      setBatchDeleteOpen(false);
      reload();
    }
  };

  const openEdit = (id) => {
    setEditId(id);
    setSheetOpen(true);
  };

  const toggleSelect = useCallback((id) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const allSelected = users.length > 0 && users.every((u) => selectedIds.has(u.id));
  const someSelected = users.some((u) => selectedIds.has(u.id));
  const toggleAll = () => setSelectedIds(allSelected ? new Set() : new Set(users.map((u) => u.id)));

  const sortHeader = (id, label, alignEnd) => (
    <SortHeader id={id} label={label} order={order} orderBy={orderBy} onSort={handleSort} alignEnd={alignEnd} />
  );

  const columns = useMemo(
    () => [
      {
        id: 'select',
        header: () => (
          <Checkbox
            checked={allSelected}
            indeterminate={!allSelected && someSelected}
            onCheckedChange={toggleAll}
            aria-label={t('userPage.selectAll')}
          />
        ),
        meta: { className: 'w-[1%]' },
        cell: (c) => (
          <Checkbox
            checked={selectedIds.has(c.row.original.id)}
            onCheckedChange={() => toggleSelect(c.row.original.id)}
            aria-label={c.row.original.username}
          />
        )
      },
      {
        id: 'user',
        header: () => sortHeader('id', t('userPage.username')),
        meta: { className: 'max-w-60' },
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
      { id: 'org', header: t('userPage.org'), cell: (c) => <OrgCell user={c.row.original} /> },
      {
        id: 'quota',
        header: () => sortHeader('quota', t('userPage.balance'), true),
        meta: { className: 'text-right' },
        cell: (c) => <span className="whitespace-nowrap tabular-nums">{renderQuota(c.row.original.quota)}</span>
      },
      {
        id: 'used_quota',
        header: () => sortHeader('used_quota', t('userPage.usedShort'), true),
        meta: { className: 'text-right' },
        cell: (c) => <span className="whitespace-nowrap tabular-nums">{renderSpend(c.row.original.used_quota)}</span>
      },
      {
        id: 'request_count',
        header: () => sortHeader('request_count', t('userPage.requestCountShort'), true),
        meta: { className: 'text-right' },
        cell: (c) => <span className="tabular-nums">{renderNumber(c.row.original.request_count)}</span>
      },
      {
        id: 'last_login_time',
        header: () => sortHeader('last_login_time', t('userPage.lastLoginTime')),
        cell: (c) => {
          const u = c.row.original;
          if (!u.last_login_time) return <span className="whitespace-nowrap text-muted-foreground">{t('userPage.neverLoggedIn')}</span>;
          return (
            <div className="flex flex-col whitespace-nowrap leading-tight">
              <span>{timestamp2string(u.last_login_time)}</span>
              {u.last_login_ip && <span className="font-mono text-xs text-muted-foreground">{u.last_login_ip}</span>}
            </div>
          );
        }
      },
      { id: 'status', header: t('userPage.status'), cell: (c) => <StatusDot status={c.row.original.status} /> },
      {
        id: 'enabled',
        header: t('userPage.enableSwitch'),
        meta: { className: 'w-12 min-w-12 text-center' },
        cell: (c) => (
          <span className="flex justify-center">
            <StatusCell item={c.row.original} manageUser={manageUser} />
          </span>
        )
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('userPage.action')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => <ActionsCell item={c.row.original} manageUser={manageUser} onEdit={openEdit} />
      }
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, order, orderBy, selectedIds, allSelected, someSelected]
  );

  const table = useTable({
    features: appTableFeatures,
    data: users,
    columns,
    getRowId: (row) => String(row.id)
  });

  // 窄屏卡片:勾选 + 用户 + 状态;角色 / 分组 / 组织一行;底部余额、已用、启用开关与 ⋯ 菜单。
  const renderCard = (u) => (
    <>
      <div className="flex items-center gap-2">
        <Checkbox checked={selectedIds.has(u.id)} onCheckedChange={() => toggleSelect(u.id)} aria-label={u.username} />
        <div className="min-w-0 flex-1">
          <UserCell user={u} />
        </div>
        <StatusDot status={u.status} />
        <StatusCell item={u} manageUser={manageUser} />
        <ActionsCell item={u} manageUser={manageUser} onEdit={openEdit} />
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2 pl-6">
        {roleBadge(t, u.role)}
        <Badge variant="outline" className="whitespace-nowrap">
          {u.group || '-'}
        </Badge>
        {u.org_name && <OrgCell user={u} />}
      </div>
      <div className="mt-2 flex items-center gap-3 pl-6 text-xs">
        <span className="text-muted-foreground">{t('userPage.balance')}</span>
        <span className="font-semibold tabular-nums">{renderQuota(u.quota)}</span>
        <span className="text-muted-foreground">{t('userPage.usedShort')}</span>
        <span className="tabular-nums">{renderSpend(u.used_quota)}</span>
      </div>
    </>
  );

  const statChips = [
    { id: 'all', label: t('userPage.statsAll'), count: stats?.total, active: !quick, onClick: () => applyQuick('') },
    { id: 'enabled', label: t('userPage.statsEnabled'), count: stats?.enabled, tone: 'success' },
    { id: 'disabled', label: t('userPage.statsDisabled'), count: stats?.disabled, tone: 'destructive' },
    { id: 'admin', label: t('userPage.statsAdmin'), count: stats ? (stats.admin ?? 0) + (stats.root ?? 0) : undefined, tone: 'info' },
    ...(coverage ? [{ id: 'nooidc', label: t('userPage.statsNoOidc'), count: coverage.without_oidc ?? 0, tone: 'warning' }] : [])
  ].map((chip) =>
    chip.id === 'all' ? chip : { ...chip, active: quick === chip.id, onClick: () => applyQuick(quick === chip.id ? '' : chip.id) }
  );

  let body;
  if (loaded && !searching && users.length === 0) {
    body = hasFilters ? (
      <ListEmptyState variant="noMatch" onClearFilters={handleClearFilters} />
    ) : (
      <ListEmptyState
        title={t('userPage.emptyTitle')}
        description={t('userPage.emptyDescription')}
        action={
          <Button size="sm" onClick={() => openEdit(0)}>
            <Plus className="size-4" /> {t('userPage.createUser')}
          </Button>
        }
      />
    );
  } else {
    body = (
      <>
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id} className="hover:bg-transparent">
                  {hg.headers.map((h) => (
                    <TableHead key={h.id} className={cn('h-9 whitespace-nowrap', h.column.columnDef.meta?.className)}>
                      {flexRender(h.column.columnDef.header, h.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} data-state={selectedIds.has(row.original.id) ? 'selected' : undefined}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className={cn('py-1.5', cell.column.columnDef.meta?.className)}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <DataCards table={table} className="md:hidden" renderCard={renderCard} searching={searching} />
      </>
    );
  }

  const providers = Array.isArray(coverage?.by_provider) ? coverage.by_provider : [];

  return (
    <>
      {/* 标题行右侧操作:刷新(图标) → 新建。 */}
      <PageActions>
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  handleRefresh();
                  loadStats();
                }}
                aria-label={t('userPage.refresh')}
              >
                <RotateCcw className={searching ? 'size-4 animate-spin' : 'size-4'} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t('userPage.refresh')}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <ResponsiveToolbarButton primary size="sm" icon={Plus} label={t('userPage.createUser')} onClick={() => openEdit(0)} />
      </PageActions>

      <ListPage
        className="max-md:[&>[data-slot=list-page-footer]]:static"
        header={
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-1">
              {statChips.map(({ id, ...chip }) => (
                <StatChip key={id} {...chip} />
              ))}
            </div>
            {providers.length > 0 && (
              <p className="text-xs text-muted-foreground">
                {t('userPage.oidcCoverageByProvider')}:{' '}
                {providers.map((p) => (p.display_name || p.slug) + ' ' + renderNumber(p.bound_users ?? 0)).join(' · ')}
              </p>
            )}
          </div>
        }
        toolbar={
          <ListToolbar
            className="max-sm:[&>div:first-child]:w-auto max-sm:[&>div:first-child]:min-w-0 max-sm:[&>div:first-child]:flex-1"
            search={{ value: keywordInput, onChange: setKeywordInput, placeholder: t('userPage.searchShort') }}
            filters={{ fields: filterFields, state: filterState, onChange: handleFilterChange, onClearAll: handleClearFilters }}
          />
        }
        selectedCount={selectedIds.size}
        batchBar={
          <BatchBar
            count={selectedIds.size}
            onClear={() => setSelectedIds(new Set())}
            actions={
              <>
                <Button size="sm" variant="outline" disabled={batchBusy} onClick={() => runBatch('enable')}>
                  <UserCheck className="size-4" />
                  {t('userPage.batchEnable')}
                </Button>
                <Button size="sm" variant="outline" disabled={batchBusy} onClick={() => runBatch('disable')}>
                  <UserX className="size-4" />
                  {t('userPage.batchDisable')}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="text-destructive"
                  disabled={batchBusy}
                  onClick={() => setBatchDeleteOpen(true)}
                >
                  <Trash2 className="size-4" />
                  {t('common.delete')}
                </Button>
              </>
            }
          />
        }
        footer={
          <ListFooter
            total={listCount}
            page={page}
            pageSize={rowsPerPage}
            selectedCount={selectedIds.size}
            onPageChange={setPage}
            onPageSizeChange={onRowsPerPageChange}
          />
        }
      >
        {body}
      </ListPage>

      <UserSheet
        open={sheetOpen}
        userId={editId}
        onClose={() => setSheetOpen(false)}
        onSaved={() => {
          setSheetOpen(false);
          reload();
        }}
      />

      <Dialog open={batchDeleteOpen} onOpenChange={(o) => !o && setBatchDeleteOpen(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
            <DialogDescription>{t('userPage.batchDeleteConfirm', { count: selectedIds.size })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setBatchDeleteOpen(false)}>
              {t('userPage.cancel')}
            </Button>
            <Button variant="destructive" disabled={batchBusy} onClick={() => runBatch('delete')}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
