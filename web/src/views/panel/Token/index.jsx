import { useState, useEffect, useContext, useMemo, useRef } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Plus, RotateCcw, KeyRound, Copy, Columns3, Check, Minus } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import { FilterBar, filterStateToParams, paramsToFilterState, isEntryActive } from '@/components/filter-bar';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import PageTitleExtra from '@/components/chrome/PageTitleExtra';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { showError, useIsReliable, useIsAdmin } from 'utils/common';
import { resolveServerAddress } from 'utils/serverAddress';
import { PAGE_SIZE_OPTIONS } from 'constants';
import { UserContext } from 'contexts/UserContext';
import { useOrg } from 'contexts/OrgContext';
import usePaginatedList from 'hooks/usePaginatedList';
import TokenTable from './TokenTable';
import TokenSheet from './TokenSheet';
import TokenDetailSheet from './TokenDetailSheet';
import TokenUsageDialog from './TokenUsageDialog';
import OpenClawConnectDialog from './OpenClawConnectDialog';

// Column registry for the show/hide menu. Ids match TokenTable column ids.
// OpenRouter 式高密度列表:API Key(合并名称+密钥)、分组、状态、过期、最后使用、用量
// (合并已用+今日)、限额(合并剩余额度+周期重置)默认可见;创建时间默认隐藏,可在
// 菜单中开启。orgOnly 列仅组织上下文存在;reliableOnly 列仅可信用户存在(否则不入表)。
const TOKEN_COLUMNS = [
  { id: 'name', labelKey: 'token_index.token' },
  { id: 'creator', labelKey: 'org.creator', orgOnly: true },
  { id: 'group', labelKey: 'token_index.userGroup' },
  { id: 'billing_tag', labelKey: 'token_index.billingTag', reliableOnly: true, defaultHidden: true },
  { id: 'expired_time', labelKey: 'token_index.expiryTime' },
  { id: 'accessed_time', labelKey: 'token_index.lastUsed' },
  { id: 'used_quota', labelKey: 'token_index.usageColumn' },
  { id: 'remain_quota', labelKey: 'token_index.limitColumn' },
  { id: 'created_time', labelKey: 'token_index.createdTime', defaultHidden: true },
  { id: 'status', labelKey: 'token_index.status' },
  { id: 'actions', labelKey: 'token_index.actions' }
];

// v2:列注册表默认值升级(TK1 起过期/最后使用/用量/限额默认可见)。旧键 token-column-visibility
// 会用陈旧的 false 覆盖新默认列,导致老访客看不到新列,故 v2 不存在时忽略旧键、直接用默认值。
const COLUMN_VISIBILITY_STORAGE_KEY = 'token-column-visibility-v2';
const LEGACY_COLUMN_VISIBILITY_STORAGE_KEY = 'token-column-visibility';

const buildDefaultColumnVisibility = () => Object.fromEntries(TOKEN_COLUMNS.map((c) => [c.id, !c.defaultHidden]));

// localStorage persistence; unknown/missing keys fall back to the default.
const loadColumnVisibility = () => {
  const defaults = buildDefaultColumnVisibility();
  try {
    localStorage.removeItem(LEGACY_COLUMN_VISIBILITY_STORAGE_KEY);
    const saved = JSON.parse(localStorage.getItem(COLUMN_VISIBILITY_STORAGE_KEY));
    if (!saved || typeof saved !== 'object') return defaults;
    for (const id of Object.keys(defaults)) {
      if (typeof saved[id] === 'boolean') defaults[id] = saved[id];
    }
  } catch {
    /* corrupted storage -> defaults */
  }
  return defaults;
};

// Menu row with a checkbox glyph. A plain DropdownMenuItem closes the menu on
// click, which breaks multi-toggle; the glyph is a span (not the Checkbox
// component) to avoid nesting a button inside a button.
function ColumnToggleItem({ checked, indeterminate = false, label, onToggle }) {
  return (
    <button
      type="button"
      role="menuitemcheckbox"
      aria-checked={indeterminate ? 'mixed' : checked}
      onClick={onToggle}
      className="flex w-full cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted focus:bg-muted"
    >
      <span
        className={cn(
          'flex size-4 shrink-0 items-center justify-center rounded-sm border border-input',
          (checked || indeterminate) && 'border-primary bg-primary text-primary-foreground'
        )}
      >
        {indeterminate ? <Minus className="size-3.5" /> : checked ? <Check className="size-3.5" /> : null}
      </span>
      <span className="truncate">{label}</span>
    </button>
  );
}

ColumnToggleItem.propTypes = {
  checked: PropTypes.bool,
  indeterminate: PropTypes.bool,
  label: PropTypes.node,
  onToggle: PropTypes.func.isRequired
};

// ↑/↓/Home/End roving focus for the column menu items.
const focusColumnMenuItem = (root, key) => {
  const items = Array.from(root?.querySelectorAll('[role="menuitemcheckbox"]') ?? []);
  if (!items.length) return;
  const idx = items.indexOf(document.activeElement);
  let next;
  if (key === 'Home') next = 0;
  else if (key === 'End') next = items.length - 1;
  else if (key === 'ArrowDown') next = idx < 0 ? 0 : (idx + 1) % items.length;
  else next = idx < 0 ? items.length - 1 : (idx - 1 + items.length) % items.length;
  items[next].focus();
};

// FilterBar 字段:单个名称文本 chip(后端 keyword 单值,按 API Key 名称模糊匹配)。
const TOKEN_FILTER_FIELDS = [{ key: 'keyword', labelKey: 'token_index.name', type: 'text', placeholderKey: 'token_index.searchTokenName' }];
const TOKEN_FILTER_PARAM_KEYS = ['keyword'];

// 管理员搜索字段:仅站点管理员、非组织上下文时注册进 FilterBar;标量值直接落同名参数,
// 触发任一字段即切到 /api/token/admin/search(见 fetchData 的 isAdminSearch 分支)。
const ADMIN_FILTER_FIELDS = [
  { key: 'user_id', labelKey: 'token_index.userId', type: 'number', placeholderKey: 'token_index.userIdPlaceholder' },
  { key: 'token_id', labelKey: 'token_index.tokenId', type: 'number', placeholderKey: 'token_index.tokenIdPlaceholder' },
  { key: 'key', labelKey: 'token_index.tokenKeySearch', type: 'text', placeholderKey: 'token_index.tokenKeySearchPlaceholder' }
];
const ADMIN_FILTER_PARAM_KEYS = ['user_id', 'token_id', 'key'];
// URL 恢复始终用全集解析(URL 是静态串,不依赖 redux 里 user 是否已就绪),再按上下文决定展示/发送。
const ALL_FILTER_FIELDS = [...TOKEN_FILTER_FIELDS, ...ADMIN_FILTER_FIELDS];
const ALL_FILTER_PARAM_KEYS = [...TOKEN_FILTER_PARAM_KEYS, ...ADMIN_FILTER_PARAM_KEYS];

export default function Token() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(ALL_FILTER_FIELDS, searchParams));
  // 当日(TZ 零点起)按 API Key 名聚合的已用额度映射(token_name -> quota);管理员搜索接口不返回时为 null
  const [todayUsage, setTodayUsage] = useState(null);
  const [revealed, setRevealed] = useState(new Set());

  const [sheetOpen, setSheetOpen] = useState(false);
  const [editTokenId, setEditTokenId] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [usageTarget, setUsageTarget] = useState(null);
  const [openClawTarget, setOpenClawTarget] = useState(null);
  // TC3:移动端点卡片弹出的详情 Sheet 只存 API Key id,实时从 tokens 派生,状态开关等改动即时反映。
  const [detailId, setDetailId] = useState(null);
  const [apiType, setApiType] = useState('openai');

  const { loadUserGroup } = useContext(UserContext);
  const siteInfo = useSelector((state) => state.siteInfo);
  const { userGroup } = useSelector((state) => state.account);
  const userIsReliable = useIsReliable();
  const userIsAdmin = useIsAdmin();
  // 组织上下文:列表/增删改经 orgScope 改写走 /api/org/:id/token*;站点管理员搜索是站点维度功能,隐藏
  const { currentOrgId } = useOrg();
  const isOrgContext = Boolean(currentOrgId);
  const [orgMemberNames, setOrgMemberNames] = useState({});

  // 管理员搜索字段仅站点管理员、非组织上下文可见;FilterBar 只展示当前上下文允许的字段。
  const showAdminFilters = userIsAdmin && !isOrgContext;
  const filterFields = useMemo(() => (showAdminFilters ? ALL_FILTER_FIELDS : TOKEN_FILTER_FIELDS), [showAdminFilters]);
  // 从筛选状态提取管理员参数;非管理员/组织上下文一律空,保证不会误走管理员端点。
  const adminSearchParams = useMemo(
    () => (showAdminFilters ? filterStateToParams(ADMIN_FILTER_FIELDS, filterState) : {}),
    [showAdminFilters, filterState]
  );
  const isAdminSearch = Boolean(adminSearchParams.user_id || adminSearchParams.token_id || adminSearchParams.key);

  // 分页列表状态 + 守卫化 fetch 生命周期(UX-13);fetcher 保留原有 URL/参数构造逻辑。
  const {
    page,
    setPage,
    rowsPerPage,
    order,
    orderBy,
    listCount,
    searching,
    rows: tokens,
    setRows: setTokens,
    handleRefresh,
    handleSort,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'token',
    fetcher: async ({ page, rowsPerPage, order, orderBy }) => {
      const keyword = filterStateToParams(TOKEN_FILTER_FIELDS, filterState).keyword || '';
      const ob = order === 'desc' ? '-' + orderBy : orderBy;
      const res = isAdminSearch
        ? await API.get('/api/token/admin/search', {
            params: {
              page: page + 1,
              size: rowsPerPage,
              keyword,
              order: ob,
              user_id: adminSearchParams.user_id ? parseInt(adminSearchParams.user_id, 10) : undefined,
              token_id: adminSearchParams.token_id ? parseInt(adminSearchParams.token_id, 10) : undefined,
              key: adminSearchParams.key || undefined
            }
          })
        : await API.get('/api/token/', { params: { page: page + 1, size: rowsPerPage, keyword, order: ob } });
      return res.data;
    },
    onSuccess: (payload) => setTodayUsage(payload.today_usage ?? null),
    // 筛选/管理员搜索/组织上下文变化时重拉(与迁移前 fetchData 依赖一致;组织上下文经 orgScope 改写 URL)
    deps: [filterState, isAdminSearch, adminSearchParams, currentOrgId]
  });

  const [columnVisibility, setColumnVisibility] = useState(loadColumnVisibility);
  const columnMenuRef = useRef(null);

  useEffect(() => {
    try {
      localStorage.setItem(COLUMN_VISIBILITY_STORAGE_KEY, JSON.stringify(columnVisibility));
    } catch {
      /* storage unavailable -> keep in-memory only */
    }
  }, [columnVisibility]);

  // Columns the current user may toggle; reliable-only columns appear only for
  // reliable users, org-only columns only in an organization context (matching
  // which columns TokenTable actually renders).
  const menuColumns = useMemo(
    () => TOKEN_COLUMNS.filter((c) => (!c.reliableOnly || userIsReliable) && (!c.orgOnly || isOrgContext)),
    [userIsReliable, isOrgContext]
  );
  const allMenuColumnsVisible = menuColumns.every((c) => columnVisibility[c.id]);
  const someMenuColumnsVisible = menuColumns.some((c) => columnVisibility[c.id]);

  const toggleColumn = (id) => setColumnVisibility((v) => ({ ...v, [id]: !v[id] }));

  const toggleAllColumns = () =>
    setColumnVisibility((v) => {
      const next = { ...v };
      menuColumns.forEach((c) => {
        next[c.id] = !allMenuColumnsVisible;
      });
      return next;
    });

  const userGroupOptions = useMemo(
    () =>
      Object.values(userGroup || {})
        .filter((item) => !item.inaccessible)
        .sort((a, b) => (a.ratio ?? 0) - (b.ratio ?? 0))
        .map((item) => ({ label: `${item.name} (${t('token_index.groupRatio')}: ${item.ratio})`, value: item.symbol })),
    [userGroup, t]
  );

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        ALL_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(filterFields, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        return params;
      },
      { replace: true }
    );
  }, [filterState, filterFields, setSearchParams]);

  // 级联筛选受控回调:任何改动即时应用并回到第一页;清空同理。
  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };

  // 已生效的筛选字段(仅用于判断是否渲染标题行下方的 chips 容器,对齐 Log 页)。
  const activeChipFields = useMemo(() => filterFields.filter((f) => isEntryActive(f, filterState[f.key])), [filterFields, filterState]);

  useEffect(() => {
    loadUserGroup();
  }, [loadUserGroup]);

  // 切换上下文后回到第一页(列表 fetch 依赖 currentOrgId,自动重新拉取)
  useEffect(() => {
    setPage(0);
  }, [currentOrgId, setPage]);

  // 组织上下文:拉取成员列表,用于 API Key"创建者"列的 user_id -> username 映射
  useEffect(() => {
    if (!isOrgContext) {
      setOrgMemberNames({});
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const res = await API.get(`/api/org/${currentOrgId}/members`, { params: { page: 1, size: 100 } });
        const { success, data } = res.data;
        if (!cancelled && success) {
          const names = {};
          (data?.data || []).forEach((m) => {
            names[m.user_id] = m.username;
          });
          setOrgMemberNames(names);
        }
      } catch (error) {
        // 错误已由全局拦截器提示
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isOrgContext, currentOrgId]);

  const manageToken = async (id, action, value) => {
    try {
      let res;
      if (action === 'delete') res = await API.delete((isAdminSearch ? '/api/token/admin/' : '/api/token/') + id);
      else if (isAdminSearch) res = await API.put('/api/token/admin?status_only=true', { id, status: value });
      else res = await API.put('/api/token/?status_only=true', { id, status: value });
      const { success } = res.data;
      if (success) {
        toast.success(t('common.operationSuccess'));
        if (action === 'delete') handleRefresh();
      }
      return res.data;
    } catch (error) {
      showError(error);
    }
  };

  const onToggleStatus = async (item) => {
    const next = item.status === 1 ? 2 : 1;
    const r = await manageToken(item.id, 'status', next);
    if (r?.success) setTokens((prev) => prev.map((x) => (x.id === item.id ? { ...x, status: next } : x)));
  };

  const onToggleReveal = (id) =>
    setRevealed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const copyText = (text, label) => {
    try {
      navigator.clipboard.writeText(text);
      toast.success(`${t('token_index.copy')} ${label} ✓`);
    } catch (e) {
      toast.error(`${label}: ${text}`);
    }
  };

  const onCopyKey = (item) => copyText(`sk-${item.key}`, t('token_index.token'));

  const openCreate = () => {
    setEditTokenId(0);
    setSheetOpen(true);
  };
  const openEdit = (id) => {
    setEditTokenId(id);
    setSheetOpen(true);
  };
  const onSaved = () => {
    setSheetOpen(false);
    setEditTokenId(0);
    handleRefresh();
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await manageToken(deleteTarget.id, 'delete', '');
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const resolvedServerAddress = resolveServerAddress(siteInfo.server_address);
  const apiAddress = {
    openai: `${resolvedServerAddress}/v1`,
    gemini: `${resolvedServerAddress}/gemini`,
    claude: `${resolvedServerAddress}/claude`
  }[apiType];
  // 仅当有额外供应商(Gemini/Claude)可选时才显示切换器,否则只剩 openai 无需下拉。
  const hasExtraApiProviders = Boolean(siteInfo.GeminiAPIEnabled || siteInfo.ClaudeAPIEnabled);

  const totalPages = Math.max(1, Math.ceil(listCount / rowsPerPage));

  // 从 tokens 派生当前详情 API Key(找不到则视为已关闭),保证开关/删除后 Sheet 内容与列表同步。
  const detailToken = useMemo(() => (detailId == null ? null : tokens.find((x) => x.id === detailId) || null), [detailId, tokens]);

  return (
    <>
      <div className="space-y-6">
        {/* Base URL 组内联到标题右侧、左对齐贴标题(供应商切换 + Base URL 复制);URL 变长仅向右
            截断,不移动左边缘。窄屏随标题行 flex-wrap 降级换行。 */}
        <PageTitleExtra>
          <div className="flex min-w-0 items-center gap-2">
            {hasExtraApiProviders && (
              <Select value={apiType} onValueChange={setApiType}>
                <SelectTrigger className="h-9 w-[120px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="openai">{t('token_index.openaiApi')}</SelectItem>
                  {siteInfo.ClaudeAPIEnabled && <SelectItem value="claude">{t('token_index.claudeApi')}</SelectItem>}
                  {siteInfo.GeminiAPIEnabled && <SelectItem value="gemini">{t('token_index.geminiApi')}</SelectItem>}
                </SelectContent>
              </Select>
            )}
            <button
              type="button"
              onClick={() => copyText(apiAddress, t('token_index.apiAddress'))}
              className="inline-flex h-9 min-w-0 max-w-full items-center gap-1.5 rounded-md border border-input bg-muted/50 px-2.5 text-sm transition-colors hover:bg-muted"
            >
              <span className="shrink-0 text-muted-foreground">{t('token_index.apiAddress')}</span>
              <span className="truncate font-medium">{apiAddress}</span>
              <Copy className="size-3.5 shrink-0" />
            </button>
          </div>
        </PageTitleExtra>
        {/* 标题行右侧操作:刷新 → 筛选 → 列设置 → 新建(primary),对齐 Log 页。 */}
        <PageActions>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="sm" onClick={handleRefresh} aria-label={t('token_index.refresh')}>
                  <RotateCcw className="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('token_index.refresh')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <FilterBar
            fields={filterFields}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            hideChips
            t={t}
          />
          <DropdownMenu>
            <TooltipProvider delayDuration={150}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      aria-label={t('token_index.columnSettings')}
                      onKeyDown={(e) => {
                        if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && columnMenuRef.current) {
                          e.preventDefault();
                          focusColumnMenuItem(columnMenuRef.current, e.key);
                        }
                      }}
                    >
                      <Columns3 className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent>{t('token_index.columnSettings')}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
            <DropdownMenuContent
              ref={columnMenuRef}
              align="end"
              className="max-h-80 w-48 overflow-y-auto"
              onKeyDown={(e) => {
                if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Home' || e.key === 'End') {
                  e.preventDefault();
                  focusColumnMenuItem(e.currentTarget, e.key);
                }
              }}
            >
              <DropdownMenuLabel>{t('token_index.selectColumns')}</DropdownMenuLabel>
              <ColumnToggleItem
                checked={allMenuColumnsVisible}
                indeterminate={!allMenuColumnsVisible && someMenuColumnsVisible}
                label={t('token_index.columnSelectAll')}
                onToggle={toggleAllColumns}
              />
              <DropdownMenuSeparator />
              {menuColumns.map((c) => (
                <ColumnToggleItem key={c.id} checked={!!columnVisibility[c.id]} label={t(c.labelKey)} onToggle={() => toggleColumn(c.id)} />
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <ResponsiveToolbarButton primary icon={Plus} label={t('token_index.createToken')} onClick={openCreate} />
        </PageActions>

        {/* 标题行下方:OpenRouter 式全宽 chips 容器;仅在有激活筛选时渲染,无筛选不占空间。 */}
        {activeChipFields.length > 0 && (
          <FilterBar
            fields={filterFields}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            triggerVariant="plus"
            clearAlignEnd
            t={t}
            className="w-full rounded-lg border border-input bg-background px-3 py-2"
          />
        )}

        <Card>
          {tokens.length === 0 && !searching ? (
            <div className="flex flex-col items-center justify-center gap-3 p-12 text-center">
              <KeyRound className="size-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                {t('token_index.token')} — {t('dashboard_index.no_data', { defaultValue: 'No tokens yet' })}
              </p>
              <Button onClick={openCreate}>
                <Plus className="size-4" /> {t('token_index.createToken')}
              </Button>
            </div>
          ) : (
            <TokenTable
              tokens={tokens}
              todayUsage={todayUsage}
              orgMemberNames={isOrgContext ? orgMemberNames : null}
              userGroup={userGroup}
              userIsReliable={userIsReliable}
              revealed={revealed}
              onToggleReveal={onToggleReveal}
              onCopyKey={onCopyKey}
              onEdit={openEdit}
              onDelete={setDeleteTarget}
              onToggleStatus={onToggleStatus}
              onShowUsage={setUsageTarget}
              onConnectOpenClaw={setOpenClawTarget}
              onCardClick={(item) => setDetailId(item.id)}
              order={order}
              orderBy={orderBy}
              onSort={handleSort}
              columnVisibility={columnVisibility}
              quotaResetTimezone={siteInfo?.quota_reset_timezone}
              quotaResetWeekStart={siteInfo?.quota_reset_week_start}
            />
          )}

          <div className="flex items-center justify-between gap-4 border-t border-border px-4 py-3 text-sm">
            <span className="text-muted-foreground">{t('pagination.total', { count: listCount })}</span>
            <div className="flex items-center gap-2">
              <select
                className="h-9 rounded-lg border border-input bg-background px-2 text-sm"
                value={rowsPerPage}
                onChange={(e) => onRowsPerPageChange(parseInt(e.target.value, 10))}
              >
                {PAGE_SIZE_OPTIONS.map((n) => (
                  <option key={n} value={n}>
                    {t('pagination.perPage', { count: n })}
                  </option>
                ))}
              </select>
              <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
                ‹
              </Button>
              <span className="tabular-nums">
                {page + 1} / {totalPages}
              </span>
              <Button variant="outline" size="sm" disabled={page + 1 >= totalPages} onClick={() => setPage((p) => p + 1)}>
                ›
              </Button>
            </div>
          </div>
        </Card>

        <TokenSheet
          open={sheetOpen}
          onOpenChange={setSheetOpen}
          tokenId={editTokenId}
          adminMode={!!isAdminSearch}
          userGroupOptions={userGroupOptions}
          userGroup={userGroup}
          userIsReliable={userIsReliable}
          isOrgContext={isOrgContext}
          onSaved={onSaved}
        />

        <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('common.delete')}</DialogTitle>
              <DialogDescription>{t('common.deleteConfirm', { title: `Token "${deleteTarget?.name}"` })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteTarget(null)}>
                {t('token_index.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmDelete} disabled={deleting}>
                {t('token_index.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <TokenDetailSheet
          open={!!detailToken}
          onOpenChange={(o) => !o && setDetailId(null)}
          token={detailToken}
          userGroup={userGroup}
          userIsReliable={userIsReliable}
          orgMemberNames={isOrgContext ? orgMemberNames : null}
          todayUsage={todayUsage}
          quotaResetTimezone={siteInfo?.quota_reset_timezone}
          quotaResetWeekStart={siteInfo?.quota_reset_week_start}
          onToggleStatus={onToggleStatus}
          onCopyKey={onCopyKey}
          onEdit={openEdit}
          onDelete={setDeleteTarget}
          onShowUsage={setUsageTarget}
          onConnectOpenClaw={setOpenClawTarget}
        />

        <TokenUsageDialog
          open={!!usageTarget}
          onOpenChange={(o) => !o && setUsageTarget(null)}
          token={usageTarget}
          serverAddress={siteInfo?.server_address}
        />

        <OpenClawConnectDialog
          open={!!openClawTarget}
          onOpenChange={(o) => !o && setOpenClawTarget(null)}
          token={openClawTarget}
          serverAddress={siteInfo?.server_address}
        />
      </div>
    </>
  );
}
