import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Plus, RotateCcw, Search, Trash2, UserCheck, UserX } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { FilterSheet, describeEntry, filterStateToParams, isEntryActive, paramsToFilterState } from '@/components/filter-bar';
import {
  BatchBar,
  ColumnPicker,
  ConditionBar,
  ListEmptyState,
  ListFooter,
  ListSkeleton,
  PhoneListToolbar,
  PresetChips,
  StatusSegments,
  useColumnVisibility,
  useFilterPresets,
  useListLayout,
  useRowSelection
} from '@/components/list-page';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { API } from 'utils/api';
import { showError, showSuccess, renderNumber, useIsRoot } from 'utils/common';
import usePaginatedList from 'hooks/usePaginatedList';
import { useObjectRoute } from '@/components/objects';
import UserPanel from './UserPanel';
import NewUserSheet from './NewUserSheet';
import { USER_TAB_KEYS } from './userTabs';
import UserTable, { UserList } from './UserTable';

// Status segments (?status=): all, enabled (1) or disabled (2), counted by /api/user/stats.
const STATUS_SEGMENTS = [
  { value: 'all', labelKey: 'userPage.statsAll' },
  { value: 'enabled', labelKey: 'userPage.statsEnabled', param: 1 },
  { value: 'disabled', labelKey: 'userPage.statsDisabled', param: 2 }
];

// Conditions in the "Show users that" bar. Role and login method are not states of an account,
// so they are conditions rather than segments; the organization exists only for the root user
// (its options come from the root-only organization list).
const one = (value) => ({ op: 'in', values: [value] });
const buildFilterFields = (orgs) => [
  {
    key: 'min_role',
    labelKey: 'userPage.roleFilterLabel',
    single: true,
    supportsExclude: false,
    paramInclude: 'min_role',
    options: [
      { value: '10', labelKey: 'userPage.roleAdmins' },
      { value: '100', labelKey: 'userPage.superAdminRole' }
    ]
  },
  {
    key: 'login_method',
    labelKey: 'userPage.loginMethodFilterLabel',
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
          single: true,
          supportsExclude: false,
          paramInclude: 'org_id',
          options: orgs.map((o) => ({ value: String(o.id), label: o.name }))
        }
      ]
    : [])
];
// Parameter mapping depends only on key / type / single / paramInclude, so URL parsing can use a
// static description with a placeholder organization.
const USER_FILTER_FIELDS = buildFilterFields([{ id: 0, name: '' }]);
const USER_FILTER_PARAM_KEYS = ['keyword', 'status', 'min_role', 'login_method', 'org_id', 'quick'];

// Built-in presets; "No OIDC" matches the coverage count (enabled users only).
const BUILTIN_PRESETS = [
  { id: 'admins', labelKey: 'userPage.statsAdmin', status: 'all', conditions: { min_role: one('10') } },
  { id: 'nooidc', labelKey: 'userPage.statsNoOidc', status: 'enabled', conditions: { login_method: one('no_oidc') } }
];
const PRESET_STORAGE_KEY = 'user-filter-presets-v1';

// Old links used ?quick= for the header chips; map them onto the segment and conditions.
const QUICK_TO_STATE = {
  enabled: { status: 'enabled' },
  disabled: { status: 'disabled' },
  admin: { conditions: { min_role: one('10') } },
  nooidc: { status: 'enabled', conditions: { login_method: one('no_oidc') } }
};

// Columns: selection, user and actions are fixed; organization and call count are one click away.
const USER_COLUMNS = [
  { id: 'select', fixed: true },
  { id: 'user', labelKey: 'userPage.username', fixed: true },
  { id: 'role', labelKey: 'userPage.userRole' },
  { id: 'group', labelKey: 'userPage.group' },
  { id: 'org', labelKey: 'userPage.org', defaultHidden: true },
  { id: 'quota', labelKey: 'userPage.balance' },
  { id: 'used_quota', labelKey: 'userPage.usedShort' },
  { id: 'request_count', labelKey: 'userPage.requestCountShort', defaultHidden: true },
  { id: 'last_login_time', labelKey: 'userPage.lastLoginTime' },
  { id: 'status', labelKey: 'userPage.status' },
  { id: 'actions', fixed: true }
];

// Sort choices of the phone list, which has no headers to click.
const SORT_OPTIONS = [
  { id: 'id', dir: 'desc', labelKey: 'userPage.sortNewest' },
  { id: 'quota', dir: 'desc', labelKey: 'userPage.sortBalance' },
  { id: 'used_quota', dir: 'desc', labelKey: 'userPage.sortUsed' },
  { id: 'last_login_time', dir: 'desc', labelKey: 'userPage.sortLastLogin' }
];

const KEYWORD_DEBOUNCE_MS = 300;

export default function Users() {
  const { t, i18n } = useTranslation();
  const isRoot = useIsRoot();
  const [searchParams, setSearchParams] = useSearchParams();
  const quickInit = QUICK_TO_STATE[searchParams.get('quick')];
  const [filterState, setFilterState] = useState(() => ({
    ...paramsToFilterState(USER_FILTER_FIELDS, searchParams),
    ...(quickInit?.conditions || {})
  }));
  const [status, setStatus] = useState(() => {
    if (quickInit?.status) return quickInit.status;
    const s = searchParams.get('status');
    return STATUS_SEGMENTS.some((x) => x.value === s) ? s : 'all';
  });
  const [keywordInput, setKeywordInput] = useState(() => searchParams.get('keyword') || '');
  const [keyword, setKeyword] = useState(() => (searchParams.get('keyword') || '').trim());
  const route = useObjectRoute(USER_TAB_KEYS);
  const [groups, setGroups] = useState([]);
  // Bumped after a row or menu action so an open user panel reloads the stored user.
  const [reloadKey, setReloadKey] = useState(0);
  const [stats, setStats] = useState(null);
  const [coverage, setCoverage] = useState(null);
  const [orgs, setOrgs] = useState([]);
  const [batchBusy, setBatchBusy] = useState(false);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [filterSheetOpen, setFilterSheetOpen] = useState(false);

  // Segment counts (/api/user/stats) and OIDC coverage do not depend on the page or conditions;
  // they reload with the refresh button and after row or bulk actions.
  const loadStats = useCallback(async () => {
    try {
      const [sres, cres] = await Promise.all([API.get('/api/user/stats'), API.get('/api/user/oidc_coverage')]);
      if (sres.data?.success) setStats(sres.data.data);
      if (cres.data?.success) setCoverage(cres.data.data);
    } catch (e) {
      // the global interceptor already reported it
    }
  }, []);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  // Group choices for the user panel.
  useEffect(() => {
    API.get('/api/group/')
      .then((res) => {
        const list = Array.isArray(res.data?.data) ? res.data.data.map((g) => (typeof g === 'string' ? g : g.symbol)) : [];
        setGroups(list.sort((a, b) => String(a).localeCompare(String(b))));
      })
      .catch(() => {});
  }, []);

  // Organization condition options: the organization list is root-only.
  useEffect(() => {
    if (!isRoot) return;
    API.get('/api/organization/', { params: { page: 1, size: 100 } })
      .then((res) => {
        if (res.data?.success) setOrgs(res.data.data?.data || []);
      })
      .catch(() => {});
  }, [isRoot]);

  const filterFields = useMemo(() => buildFilterFields(orgs), [orgs]);
  const statusParam = STATUS_SEGMENTS.find((x) => x.value === status)?.param;

  const {
    page,
    setPage,
    rowsPerPage,
    order,
    orderBy,
    listCount,
    searching,
    loaded,
    rows: users,
    refresh,
    handleRefresh,
    handleSort,
    setSort,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'user',
    fetcher: async ({ page, rowsPerPage, order, orderBy }) => {
      const sortOrder = orderBy ? (order === 'desc' ? '-' + orderBy : orderBy) : '-id';
      const params = {
        page: page + 1,
        size: rowsPerPage,
        order: sortOrder,
        ...(keyword ? { keyword } : {}),
        ...(statusParam ? { status: statusParam } : {}),
        ...filterStateToParams(USER_FILTER_FIELDS, filterState)
      };
      const res = await API.get('/api/user/', { params });
      return res.data;
    },
    resetPageOnSort: true,
    resetSortOnRefresh: false,
    resetPageOnRefresh: true,
    deps: [filterState, keyword, status]
  });

  const reload = () => {
    refresh();
    loadStats();
  };

  // The search box applies after a short pause in typing.
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

  // Applied filters live in the URL (replace), so a reload restores them; ?quick= is rewritten.
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        USER_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(USER_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        if (keyword) params.set('keyword', keyword);
        if (status !== 'all') params.set('status', status);
        return params;
      },
      { replace: true }
    );
  }, [filterState, keyword, status, setSearchParams]);

  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };
  const clearEverything = () => {
    handleClearFilters();
    setStatus('all');
    setKeywordInput('');
    setKeyword('');
  };
  const changeStatus = (next) => {
    setPage(0);
    setStatus(next);
  };

  const hasFilters = !!keyword || status !== 'all' || Object.keys(filterStateToParams(USER_FILTER_FIELDS, filterState)).length > 0;

  const columns = useColumnVisibility({ storageKey: 'user-column-visibility-v1', columns: USER_COLUMNS });
  const selection = useRowSelection(users, [page, rowsPerPage, keyword, status, filterState]);
  // Full table while it fits (the sidebar collapses first), phone list otherwise.
  const { layout, narrow } = useListLayout();

  const presets = useFilterPresets({
    storageKey: PRESET_STORAGE_KEY,
    builtin: BUILTIN_PRESETS,
    fields: filterFields,
    state: filterState,
    status,
    statusLabel: (value) => t(STATUS_SEGMENTS.find((x) => x.value === value)?.labelKey || ''),
    apply: (preset) => {
      setPage(0);
      setStatus(preset.status || 'all');
      setFilterState({ ...preset.conditions });
    }
  });
  const activeConditionFields = filterFields.filter((f) => isEntryActive(f, filterState[f.key]));
  const removeCondition = (key) =>
    handleFilterChange((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });

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
        if (action === 'delete' && userId === route.openId) route.close();
        else setReloadKey((k) => k + 1);
        reload();
      } else {
        showError(res.data.message);
      }
      return res.data;
    } catch (e) {
      return undefined;
    }
  };

  // Bulk enable / disable / delete: there is no batch endpoint, so each user goes through manage
  // and failures are counted.
  const runBatch = async (action) => {
    const ids = [...selection.selectedIds];
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
      selection.clear();
      reload();
    }
  };

  const batchActions = (
    <>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('enable')}>
        <UserCheck className="size-3.5" />
        {t('userPage.batchEnable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('disable')}>
        <UserX className="size-3.5" />
        {t('userPage.batchDisable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7 text-destructive" disabled={batchBusy} onClick={() => setBatchDeleteOpen(true)}>
        <Trash2 className="size-3.5" />
        {t('common.delete')}
      </Button>
    </>
  );

  const segmentOptions = STATUS_SEGMENTS.map((x) => ({ value: x.value, label: t(x.labelKey) }));
  const segmentCounts = stats ? { all: stats.total, enabled: stats.enabled, disabled: stats.disabled } : null;
  const providers = Array.isArray(coverage?.by_provider) ? coverage.by_provider : [];
  const rtf = useMemo(
    () => new Intl.RelativeTimeFormat((i18n.language || 'zh_CN').replace('_', '-'), { numeric: 'auto' }),
    [i18n.language]
  );

  let body;
  if (!loaded) {
    body = <ListSkeleton layout={layout} cells={[80, 64, 72, 72, 32]} />;
  } else if (!searching && users.length === 0) {
    body = hasFilters ? (
      <ListEmptyState variant="noMatch" onClearFilters={clearEverything} />
    ) : (
      <ListEmptyState
        title={t('userPage.emptyTitle')}
        description={t('userPage.emptyDescription')}
        action={
          <Button size="sm" onClick={route.openNew}>
            <Plus className="size-4" /> {t('userPage.createUser')}
          </Button>
        }
      />
    );
  } else if (narrow) {
    body = (
      <UserList
        users={users}
        rtf={rtf}
        selectMode={selection.selectMode}
        selectedIds={selection.selectedIds}
        onToggleSelect={selection.toggle}
        openId={route.openId}
        onOpen={(id) => route.open(id)}
        manageUser={manageUser}
      />
    );
  } else {
    body = (
      <UserTable
        users={users}
        columnVisibility={columns.visibility}
        selectedIds={selection.selectedIds}
        onToggleSelect={selection.toggle}
        onToggleSelectAll={selection.toggleAll}
        openId={route.openId}
        onOpen={(id) => route.open(id)}
        order={order}
        orderBy={orderBy}
        onSort={handleSort}
        manageUser={manageUser}
      />
    );
  }

  return (
    <>
      {/* Title row: refresh (icon), then create. */}
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
        <ResponsiveToolbarButton primary size="sm" icon={Plus} label={t('userPage.createUser')} onClick={route.openNew} />
      </PageActions>

      <div className={cn('space-y-3', narrow && selection.selectMode && 'pb-20')}>
        <div className="space-y-2">
          {narrow ? (
            <>
              <StatusSegments chips value={status} onChange={changeStatus} options={segmentOptions} counts={segmentCounts} />
              <PhoneListToolbar
                search={{ value: keywordInput, onChange: setKeywordInput, placeholder: t('userPage.searchShort') }}
                filter={{ count: activeConditionFields.length, onOpen: () => setFilterSheetOpen(true) }}
                sort={{
                  options: SORT_OPTIONS.map((o) => ({
                    key: o.labelKey,
                    label: t(o.labelKey),
                    active: orderBy === o.id && order === o.dir,
                    onSelect: () => setSort(o.id, o.dir)
                  }))
                }}
                select={{
                  active: selection.selectMode,
                  onToggle: () => (selection.selectMode ? selection.exitSelectMode() : selection.enterSelectMode())
                }}
                chips={activeConditionFields.map((f) => ({
                  key: f.key,
                  label: describeEntry(t, f, filterState[f.key]),
                  onRemove: () => removeCondition(f.key)
                }))}
              />
            </>
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <StatusSegments value={status} onChange={changeStatus} options={segmentOptions} counts={segmentCounts} />
                <div className="relative min-w-[12rem] flex-1">
                  <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={keywordInput}
                    onChange={(e) => setKeywordInput(e.target.value)}
                    placeholder={t('userPage.searchShort')}
                    aria-label={t('userPage.searchShort')}
                    className="h-8 pl-8"
                  />
                </div>
                <ColumnPicker columns={columns} />
              </div>
              {selection.count > 0 ? (
                <BatchBar
                  count={selection.count}
                  onClear={selection.clear}
                  className="min-h-10 rounded-lg border border-primary/30 bg-primary/[0.04] px-3 py-1.5"
                  actions={batchActions}
                />
              ) : (
                <ConditionBar
                  fields={filterFields}
                  state={filterState}
                  onChange={handleFilterChange}
                  onClearAll={handleClearFilters}
                  leadingLabel={t('userPage.showUsersThat')}
                  presets={presets}
                />
              )}
            </>
          )}
          {providers.length > 0 && (
            <p className="text-xs text-muted-foreground">
              {t('userPage.oidcCoverageByProvider')}:{' '}
              {providers.map((p) => (p.display_name || p.slug) + ' ' + renderNumber(p.bound_users ?? 0)).join(' · ')}
            </p>
          )}
        </div>

        <Card>
          {body}
          <div className="border-t border-border">
            <ListFooter
              total={listCount}
              page={page}
              pageSize={rowsPerPage}
              selectedCount={selection.count}
              onPageChange={setPage}
              onPageSizeChange={onRowsPerPageChange}
            />
          </div>
        </Card>
      </div>

      {narrow && selection.selectMode && (
        <BatchBar count={selection.count} onClear={selection.exitSelectMode} floating actions={batchActions} />
      )}
      {narrow && (
        <FilterSheet
          open={filterSheetOpen}
          onOpenChange={setFilterSheetOpen}
          title={t('listPage.filter')}
          fields={filterFields}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          t={t}
          top={<PresetChips presets={presets} onPick={() => setFilterSheetOpen(false)} />}
        />
      )}

      {route.openId > 0 && (
        <UserPanel
          key={route.openId}
          userId={route.openId}
          tab={route.tab}
          onTabChange={route.setTab}
          onClose={route.close}
          onChanged={reload}
          manageUser={manageUser}
          groups={groups}
          isRoot={isRoot}
          reloadKey={reloadKey}
        />
      )}

      {route.isNew && (
        <NewUserSheet
          onClose={route.close}
          onCreated={(username) => {
            showSuccess(t('userPage.new.created', { username }));
            route.close();
            setKeywordInput(username);
            reload();
          }}
        />
      )}

      <Dialog open={batchDeleteOpen} onOpenChange={(o) => !o && setBatchDeleteOpen(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
            <DialogDescription>{t('userPage.batchDeleteConfirm', { count: selection.count })}</DialogDescription>
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
