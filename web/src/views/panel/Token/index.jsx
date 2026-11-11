import { useState, useEffect, useContext, useMemo, useRef } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import {
  Plus,
  RotateCcw,
  KeyRound,
  Columns3,
  Check,
  Minus,
  Search,
  ListFilter,
  Link2,
  Sparkles,
  X,
  BookmarkPlus,
  Power,
  PowerOff,
  Trash2,
  ArrowDownUp,
  ListChecks
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import { FilterBar, FilterSheet, filterStateToParams, paramsToFilterState, isEntryActive, optionLabel } from '@/components/filter-bar';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { useContentMinWidth } from '@/components/chrome/ContentWidth';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { showError, useIsReliable, useIsAdmin } from 'utils/common';
import { UserContext } from 'contexts/UserContext';
import { useOrg } from 'contexts/OrgContext';
import { isOrgAdminRole } from '@/components/chrome/nav-config';
import usePaginatedList from 'hooks/usePaginatedList';
import useElementWidth from 'hooks/useElementWidth';
import { BatchBar, ListFooter } from '@/components/list-page';
import { Segmented, useObjectRoute } from '@/components/objects';
import ConnectInfo, { useApiAddresses } from './ConnectInfo';
import TokenTable, { estimateTableWidth, TokenListSkeleton } from './TokenTable';
import TokenPanel from './TokenPanel';
import NewTokenDialog from './NewTokenDialog';
import { TOKEN_TAB_KEYS } from './tokenTabs';
import TokenUsageDialog from './TokenUsageDialog';
import OpenClawConnectDialog from './OpenClawConnectDialog';

// Column registry for the show/hide menu. Ids match TokenTable column ids. The name and
// actions columns are always shown and stay out of the menu (fixed). Usage carries spend, the
// limit and its bar in one cell; the creation date is hidden by default. orgOnly columns exist
// only in an organization, reliableOnly columns only for reliable users.
const TOKEN_COLUMNS = [
  { id: 'select', labelKey: 'apiKey.list.selectAll', fixed: true },
  { id: 'name', labelKey: 'token_index.token', fixed: true },
  { id: 'creator', labelKey: 'org.creator', orgOnly: true },
  { id: 'group', labelKey: 'token_index.userGroup', defaultHidden: true },
  { id: 'billing_tag', labelKey: 'token_index.billingTag', reliableOnly: true, defaultHidden: true },
  { id: 'expired_time', labelKey: 'token_index.expiryTime' },
  { id: 'accessed_time', labelKey: 'token_index.lastUsed' },
  { id: 'used_quota', labelKey: 'token_index.usageColumn' },
  { id: 'created_time', labelKey: 'token_index.createdTime', defaultHidden: true },
  { id: 'status', labelKey: 'token_index.status' },
  { id: 'actions', labelKey: 'token_index.actions', fixed: true }
];

// v5: the default list is name, creator (organizations only), usage, last used, expiry and status;
// group, billing tag and creation date are one click away in the column menu. Older saved states
// are dropped so the new defaults apply.
const COLUMN_VISIBILITY_STORAGE_KEY = 'token-column-visibility-v5';
const LEGACY_COLUMN_VISIBILITY_STORAGE_KEYS = [
  'token-column-visibility',
  'token-column-visibility-v2',
  'token-column-visibility-v3',
  'token-column-visibility-v4'
];

const buildDefaultColumnVisibility = () => Object.fromEntries(TOKEN_COLUMNS.map((c) => [c.id, !c.defaultHidden]));

// localStorage persistence; unknown/missing keys fall back to the default.
const loadColumnVisibility = () => {
  const defaults = buildDefaultColumnVisibility();
  try {
    LEGACY_COLUMN_VISIBILITY_STORAGE_KEYS.forEach((k) => localStorage.removeItem(k));
    const saved = JSON.parse(localStorage.getItem(COLUMN_VISIBILITY_STORAGE_KEY));
    if (!saved || typeof saved !== 'object') return defaults;
    for (const c of TOKEN_COLUMNS) {
      if (!c.fixed && typeof saved[c.id] === 'boolean') defaults[c.id] = saved[c.id];
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

// List segments (?status=). Counts come back with the list as status_counts.
const STATUS_SEGMENTS = ['all', 'enabled', 'disabled', 'invalid'];

// Conditions in the "Show keys that" bar, sent as same-named parameters and shown in sections
// like the reference gateway's menu: expiry, spend, usage, other. Group options are filled in from
// the account's groups.
const SEC_EXPIRY = 'apiKey.list.secExpiry';
const SEC_SPEND = 'apiKey.list.secSpend';
const SEC_USAGE = 'apiKey.list.secUsage';
const SEC_OTHER = 'apiKey.list.secOther';
const CONDITION_FIELDS = [
  {
    key: 'expiry',
    sectionKey: SEC_EXPIRY,
    labelKey: 'apiKey.list.expiry',
    single: true,
    paramInclude: 'expiry',
    options: [
      { value: 'never', labelKey: 'apiKey.list.expiryNever' },
      { value: 'expired', labelKey: 'apiKey.list.expiryExpired' },
      { value: 'soon', labelKey: 'apiKey.list.expirySoon' }
    ]
  },
  {
    key: 'limit',
    sectionKey: SEC_SPEND,
    labelKey: 'apiKey.list.limit',
    single: true,
    paramInclude: 'limit',
    options: [
      { value: 'none', labelKey: 'apiKey.list.limitNone' },
      { value: 'any', labelKey: 'apiKey.list.limitAny' }
    ]
  },
  { key: 'spent_min', sectionKey: SEC_SPEND, labelKey: 'apiKey.list.spentMin', type: 'number', placeholderKey: 'apiKey.list.spentMinHint' },
  {
    key: 'budget_min',
    sectionKey: SEC_SPEND,
    labelKey: 'apiKey.list.budgetMin',
    type: 'number',
    placeholderKey: 'apiKey.list.budgetMinHint'
  },
  {
    key: 'activity',
    sectionKey: SEC_USAGE,
    labelKey: 'apiKey.list.activity',
    single: true,
    paramInclude: 'activity',
    options: [
      { value: 'never', labelKey: 'apiKey.list.activityNever' },
      { value: 'idle', labelKey: 'apiKey.list.activityIdle' }
    ]
  },
  { key: 'group', sectionKey: SEC_OTHER, labelKey: 'token_index.userGroup', single: true, paramInclude: 'group', options: [] }
];
// Organization owners and admins can also narrow the list to one member's keys; the options are
// filled in from the member list.
const CREATOR_FIELD = {
  key: 'created_by',
  sectionKey: SEC_OTHER,
  labelKey: 'org.creator',
  single: true,
  paramInclude: 'created_by',
  options: []
};
const TOKEN_FILTER_FIELDS = [...CONDITION_FIELDS, CREATOR_FIELD];
const TOKEN_FILTER_PARAM_KEYS = ['keyword', 'status', ...CONDITION_FIELDS.map((f) => f.key), 'created_by'];

// Condition presets: a status segment plus conditions, applied in one click. The built-in ones
// cover the usual clean-up questions; users can save the current combination as their own.
const one = (value) => ({ op: 'in', values: [value] });
const BUILTIN_PRESETS = [
  { id: 'expiring', labelKey: 'apiKey.list.presetExpiring', status: 'all', conditions: { expiry: one('soon') } },
  { id: 'invalid', labelKey: 'apiKey.list.presetInvalid', status: 'invalid', conditions: {} },
  { id: 'unused', labelKey: 'apiKey.list.presetUnused', status: 'all', conditions: { activity: one('never') } },
  { id: 'idle', labelKey: 'apiKey.list.presetIdle', status: 'enabled', conditions: { activity: one('idle') } },
  { id: 'budget', labelKey: 'apiKey.list.presetBudget', status: 'all', conditions: { budget_min: { value: '80' } } },
  { id: 'nolimit', labelKey: 'apiKey.list.presetNoLimit', status: 'enabled', conditions: { limit: one('none') } }
];
// The list shows the full table whenever its natural width fits the list area; the page first
// asks the shell to collapse the sidebar to make room. Only when even that is not enough does it
// switch to the phone list. `fits` is the table's natural width plus the card border.
export const listLayout = (areaWidth, tableWidth) => (areaWidth > 0 && areaWidth < tableWidth + 2 ? 'list' : 'table');

// Sort choices of the narrow layout, which has no table headers to click.
const SORT_OPTIONS = [
  { id: 'id', dir: 'desc', labelKey: 'apiKey.list.sortNewest' },
  { id: 'name', dir: 'asc', labelKey: 'apiKey.list.sortName' },
  { id: 'used_quota', dir: 'desc', labelKey: 'apiKey.list.sortUsage' },
  { id: 'expired_time', dir: 'asc', labelKey: 'apiKey.list.sortExpiry' }
];

const PRESET_STORAGE_KEY = 'token-filter-presets-v1';
const loadSavedPresets = () => {
  try {
    const saved = JSON.parse(localStorage.getItem(PRESET_STORAGE_KEY));
    return Array.isArray(saved) ? saved.filter((p) => p && typeof p.name === 'string' && p.conditions) : [];
  } catch {
    return [];
  }
};

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
  const [keyword, setKeyword] = useState(() => searchParams.get('keyword') || '');
  const [searchText, setSearchText] = useState(keyword);
  const [status, setStatus] = useState(() => (STATUS_SEGMENTS.includes(searchParams.get('status')) ? searchParams.get('status') : 'all'));
  const [statusCounts, setStatusCounts] = useState(null);
  const [orgMemberNames, setOrgMemberNames] = useState({});
  // 当日(TZ 零点起)按 API Key 名聚合的已用额度映射(token_name -> quota);管理员搜索接口不返回时为 null
  const [todayUsage, setTodayUsage] = useState(null);

  // The key open in the right-side panel (?id=, &tab=) and the create dialog (?new=1).
  const route = useObjectRoute(TOKEN_TAB_KEYS);
  const [reloadKey, setReloadKey] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [usageTarget, setUsageTarget] = useState(null);
  const [openClawTarget, setOpenClawTarget] = useState(null);

  const { loadUserGroup } = useContext(UserContext);
  const siteInfo = useSelector((state) => state.siteInfo);
  const { userGroup } = useSelector((state) => state.account);
  const userIsReliable = useIsReliable();
  const userIsAdmin = useIsAdmin();
  // 组织上下文:列表/增删改经 orgScope 改写走 /api/org/:id/token*;站点管理员搜索是站点维度功能,隐藏
  const { currentOrgId, orgRole } = useOrg();
  const isOrgContext = Boolean(currentOrgId);
  const canFilterByCreator = isOrgContext && isOrgAdminRole(orgRole);

  // 管理员搜索字段仅站点管理员、非组织上下文可见;FilterBar 只展示当前上下文允许的字段。
  const showAdminFilters = userIsAdmin && !isOrgContext;
  const filterFields = useMemo(() => {
    const creator = canFilterByCreator
      ? [
          {
            ...CREATOR_FIELD,
            options: Object.entries(orgMemberNames).map(([id, name]) => ({ value: id, label: name }))
          }
        ]
      : [];
    const groupOptions = [
      { value: '_default', labelKey: 'token_index.followUser' },
      ...Object.entries(userGroup || {}).map(([symbol, g]) => ({ value: symbol, label: g.name || symbol }))
    ];
    const conditions = CONDITION_FIELDS.map((f) => (f.key === 'group' ? { ...f, options: groupOptions } : f));
    return [...conditions, ...creator, ...(showAdminFilters ? ADMIN_FILTER_FIELDS : [])];
  }, [showAdminFilters, canFilterByCreator, orgMemberNames, userGroup]);
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
    loaded,
    rows: tokens,
    setRows: setTokens,
    handleRefresh,
    handleSort,
    setSort,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'token',
    fetcher: async ({ page, rowsPerPage, order, orderBy }) => {
      const ob = order === 'desc' ? '-' + orderBy : orderBy;
      // Conditions the current context offers (the creator only for organization owners/admins).
      const conditions = filterStateToParams(
        filterFields.filter((f) => !ADMIN_FILTER_PARAM_KEYS.includes(f.key)),
        filterState
      );
      const listParams = { keyword, order: ob, status: status === 'all' ? undefined : status, ...conditions };
      const res = isAdminSearch
        ? await API.get('/api/token/admin/search', {
            params: {
              page: page + 1,
              size: rowsPerPage,
              ...listParams,
              user_id: adminSearchParams.user_id ? parseInt(adminSearchParams.user_id, 10) : undefined,
              token_id: adminSearchParams.token_id ? parseInt(adminSearchParams.token_id, 10) : undefined,
              key: adminSearchParams.key || undefined
            }
          })
        : await API.get('/api/token/', { params: { page: page + 1, size: rowsPerPage, ...listParams } });
      return res.data;
    },
    onSuccess: (payload) => {
      setTodayUsage(payload.today_usage ?? null);
      // The admin search returns no segment counts; the segments then show without numbers.
      setStatusCounts(payload.status_counts ?? null);
    },
    // 筛选/管理员搜索/组织上下文变化时重拉(与迁移前 fetchData 依赖一致;组织上下文经 orgScope 改写 URL)
    deps: [filterState, filterFields, keyword, status, isAdminSearch, adminSearchParams, currentOrgId]
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
    () => TOKEN_COLUMNS.filter((c) => !c.fixed && (!c.reliableOnly || userIsReliable) && (!c.orgOnly || isOrgContext)),
    [userIsReliable, isOrgContext]
  );
  const allMenuColumnsVisible = menuColumns.every((c) => columnVisibility[c.id]);
  const someMenuColumnsVisible = menuColumns.some((c) => columnVisibility[c.id]);
  const visibleMenuColumnCount = menuColumns.filter((c) => columnVisibility[c.id]).length;
  const resetColumns = () => setColumnVisibility(buildDefaultColumnVisibility());

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
        if (keyword) params.set('keyword', keyword);
        if (status !== 'all') params.set('status', status);
        return params;
      },
      { replace: true }
    );
  }, [filterState, filterFields, keyword, status, setSearchParams]);

  // The search box applies after a short pause in typing.
  useEffect(() => {
    const next = searchText.trim();
    if (next === keyword) return undefined;
    const timer = setTimeout(() => {
      setPage(0);
      setKeyword(next);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchText, keyword, setPage]);

  // Bulk selection (ids on the current page). It is cleared whenever the rows can change under it:
  // another page, page size, search, segment, conditions or organization.
  const [selectedIds, setSelectedIds] = useState(() => new Set());
  const [batchBusy, setBatchBusy] = useState(false);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  useEffect(() => {
    setSelectedIds(new Set());
  }, [page, rowsPerPage, keyword, status, filterState, currentOrgId]);
  const toggleSelect = (id) =>
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const toggleSelectAll = (all) => setSelectedIds(all ? new Set(tokens.map((x) => x.id)) : new Set());

  const [pageRef, pageWidth] = useElementWidth();
  // The table's natural width: measured while the table shows, estimated from the column set
  // otherwise (and after the columns change, until it is measured again).
  const [measuredTableWidth, setMeasuredTableWidth] = useState(null);
  const visibleColumnIds = useMemo(
    () => TOKEN_COLUMNS.filter((c) => (!c.reliableOnly || userIsReliable) && (!c.orgOnly || isOrgContext)).map((c) => c.id),
    [userIsReliable, isOrgContext]
  );
  useEffect(() => {
    setMeasuredTableWidth(null);
  }, [columnVisibility, visibleColumnIds]);
  const tableWidth = measuredTableWidth ?? estimateTableWidth(visibleColumnIds, columnVisibility);
  // Ask the shell for this much content width; it collapses the sidebar when that helps.
  useContentMinWidth(tableWidth + 2);
  const layout = listLayout(pageWidth, tableWidth);
  const narrow = layout === 'list';
  // Narrow layout: checkboxes only in select mode; conditions in a bottom sheet.
  const [selectMode, setSelectMode] = useState(false);
  const [filterSheetOpen, setFilterSheetOpen] = useState(false);
  const exitSelectMode = () => {
    setSelectMode(false);
    setSelectedIds(new Set());
  };

  const changeStatus = (next) => {
    setPage(0);
    setStatus(next);
  };

  // Presets. Admin search fields are never part of one: they switch the list to another endpoint.
  const [savedPresets, setSavedPresets] = useState(loadSavedPresets);
  useEffect(() => {
    try {
      localStorage.setItem(PRESET_STORAGE_KEY, JSON.stringify(savedPresets));
    } catch {
      /* storage unavailable -> keep in-memory only */
    }
  }, [savedPresets]);
  const conditionFields = filterFields.filter((f) => !ADMIN_FILTER_PARAM_KEYS.includes(f.key));
  const currentConditions = Object.fromEntries(
    conditionFields.filter((f) => isEntryActive(f, filterState[f.key])).map((f) => [f.key, filterState[f.key]])
  );
  const hasCurrentConditions = status !== 'all' || Object.keys(currentConditions).length > 0;
  // A readable name for a combination, e.g. "Disabled · Expiry: Expire within 14 days".
  const describePreset = (preset) =>
    [
      preset.status !== 'all' ? t(`apiKey.list.status_${preset.status}`) : null,
      ...conditionFields
        .filter((f) => preset.conditions[f.key])
        .map((f) => {
          const entry = preset.conditions[f.key];
          if (f.type === 'number') return `${t(f.labelKey)} ≥ ${entry.value}`;
          const v = entry.values?.[0];
          const opt = (f.options || []).find((o) => String(o.value) === String(v));
          return `${t(f.labelKey)}: ${opt ? optionLabel(t, opt) : v}`;
        })
    ]
      .filter(Boolean)
      .join(' · ');
  // One active condition as text ("Expiry: Never expire", "Spent at least ≥ 10").
  const describeEntry = (f, entry) => {
    if (f.type === 'number' || f.type === 'text') return `${t(f.labelKey)} ≥ ${entry.value}`;
    const v = entry.values?.[0];
    const opt = (f.options || []).find((o) => String(o.value) === String(v));
    return `${t(f.labelKey)}: ${opt ? optionLabel(t, opt) : v}`;
  };
  const activeConditionFields = filterFields.filter((f) => isEntryActive(f, filterState[f.key]));
  const removeCondition = (key) =>
    handleFilterChange((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });

  const applyPreset = (preset) => {
    setPage(0);
    setStatus(preset.status || 'all');
    setFilterState((prev) => {
      const next = Object.fromEntries(Object.entries(prev).filter(([k]) => ADMIN_FILTER_PARAM_KEYS.includes(k)));
      return { ...next, ...preset.conditions };
    });
  };
  const saveCurrentPreset = () => {
    const preset = { id: `saved-${Date.now()}`, status, conditions: currentConditions };
    preset.name = describePreset(preset);
    setSavedPresets((list) => [...list.filter((p) => p.name !== preset.name), preset]);
  };
  const removePreset = (id) => setSavedPresets((list) => list.filter((p) => p.id !== id));

  // 级联筛选受控回调:任何改动即时应用并回到第一页;清空同理。
  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };

  useEffect(() => {
    loadUserGroup();
  }, [loadUserGroup]);

  // 切换上下文后回到第一页(列表 fetch 依赖 currentOrgId,自动重新拉取)
  useEffect(() => {
    setPage(0);
  }, [currentOrgId, setPage]);

  // 组织上下文:拉取成员列表,用于 API Key"创建者"列与创建者条件的 user_id -> username 映射
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
        if (action === 'delete') {
          if (id === route.openId) route.close();
          handleRefresh();
        } else setReloadKey((k) => k + 1);
      }
      return res.data;
    } catch (error) {
      showError(error);
    }
  };

  // Bulk enable / disable / delete. There is no batch endpoint: each key goes through the same
  // call as its own row (so permissions and the organization scope apply), and failures are counted.
  const runBatch = async (action) => {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    setBatchBusy(true);
    try {
      const results = await Promise.allSettled(
        ids.map((id) => {
          if (action === 'delete') return API.delete((isAdminSearch ? '/api/token/admin/' : '/api/token/') + id);
          const body = { id, status: action === 'enable' ? 1 : 2 };
          return API.put(isAdminSearch ? '/api/token/admin?status_only=true' : '/api/token/?status_only=true', body);
        })
      );
      const failed = results.filter((r) => r.status !== 'fulfilled' || !r.value.data?.success).length;
      if (failed > 0) toast.error(t('apiKey.list.batchFailed', { count: failed }));
      if (failed < ids.length) toast.success(t('common.operationSuccess'));
      if (action === 'delete' && selectedIds.has(route.openId)) route.close();
    } finally {
      setBatchBusy(false);
      setBatchDeleteOpen(false);
      setSelectedIds(new Set());
      setReloadKey((k) => k + 1);
      handleRefresh();
    }
  };

  const onToggleStatus = async (item) => {
    const next = item.status === 1 ? 2 : 1;
    const r = await manageToken(item.id, 'status', next);
    if (r?.success) setTokens((prev) => prev.map((x) => (x.id === item.id ? { ...x, status: next } : x)));
  };

  const copyText = (text, label) => {
    try {
      navigator.clipboard.writeText(text);
      toast.success(`${t('token_index.copy')} ${label} ✓`);
    } catch (e) {
      toast.error(`${label}: ${text}`);
    }
  };

  const onCopyKey = (item) => copyText(`sk-${item.key}`, t('token_index.token'));

  const openCreate = route.openNew;
  const openEdit = (id) => route.open(id, 'basics');

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

  const apiAddresses = useApiAddresses();

  // Preset menu items, shared by the condition bar's Presets menu.
  const presetMenuItems = (
    <>
      <DropdownMenuLabel>{t('apiKey.list.presetsBuiltin')}</DropdownMenuLabel>
      {BUILTIN_PRESETS.map((p) => (
        <DropdownMenuItem key={p.id} onClick={() => applyPreset(p)} className="flex-col items-start gap-0">
          <span>{t(p.labelKey)}</span>
          <span className="text-xs text-muted-foreground">{describePreset(p)}</span>
        </DropdownMenuItem>
      ))}
      {savedPresets.length > 0 && (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuLabel>{t('apiKey.list.presetsSaved')}</DropdownMenuLabel>
          {savedPresets.map((p) => (
            <div key={p.id} className="group/preset flex items-center gap-1">
              <DropdownMenuItem onClick={() => applyPreset(p)} className="min-w-0 flex-1">
                <span className="truncate">{p.name}</span>
              </DropdownMenuItem>
              <button
                type="button"
                aria-label={t('apiKey.list.presetRemove')}
                title={t('apiKey.list.presetRemove')}
                onClick={() => removePreset(p.id)}
                className="flex size-6 shrink-0 items-center justify-center rounded text-muted-foreground opacity-0 transition-opacity hover:bg-muted hover:text-foreground focus-visible:opacity-100 group-hover/preset:opacity-100"
              >
                <X className="size-3.5" />
              </button>
            </div>
          ))}
        </>
      )}
      <DropdownMenuSeparator />
      <DropdownMenuItem disabled={!hasCurrentConditions} onClick={saveCurrentPreset}>
        <BookmarkPlus className="size-4" />
        {t('apiKey.list.presetSave')}
      </DropdownMenuItem>
    </>
  );

  // Batch action buttons, in the bar above the table or pinned to the bottom on phones.
  const batchActions = (
    <>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('enable')}>
        <Power className="size-3.5" />
        {t('apiKey.list.batchEnable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('disable')}>
        <PowerOff className="size-3.5" />
        {t('apiKey.list.batchDisable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7 text-destructive" disabled={batchBusy} onClick={() => setBatchDeleteOpen(true)}>
        <Trash2 className="size-3.5" />
        {t('common.delete')}
      </Button>
    </>
  );

  return (
    <>
      <div ref={pageRef} className={cn('space-y-3', narrow && selectMode && 'pb-20')}>
        {/* Title row: refresh, the Connect popover (where to send requests), then create. */}
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
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm" className="gap-1.5">
                <Link2 className="size-4" />
                <span className="max-sm:hidden">{t('apiKey.list.connect')}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-80 p-3">
              <div className="mb-2 text-sm font-medium">{t('token_index.apiAddress')}</div>
              <ConnectInfo addresses={apiAddresses} onCopy={copyText} />
            </DropdownMenuContent>
          </DropdownMenu>
          <ResponsiveToolbarButton primary icon={Plus} label={t('token_index.createToken')} onClick={openCreate} />
        </PageActions>

        <div className="space-y-2">
          {narrow ? (
            <>
              {/* Phone list: segments are chips that scroll sideways instead of wrapping; one row with search and the
                  filter, sort and select buttons; active conditions as removable chips. */}
              <div
                role="radiogroup"
                aria-label={t('apiKey.list.segments')}
                className="-mx-4 flex gap-1.5 overflow-x-auto px-4 no-scrollbar"
              >
                {STATUS_SEGMENTS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    role="radio"
                    aria-checked={status === value}
                    onClick={() => changeStatus(value)}
                    className={cn(
                      'inline-flex h-8 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border px-3 text-[13px] transition-colors',
                      status === value
                        ? 'border-foreground bg-foreground text-background'
                        : 'border-input bg-field text-foreground hover:bg-muted'
                    )}
                  >
                    {t(`apiKey.list.status_${value}`)}
                    {statusCounts && (
                      <span className={cn('tabular-nums', status === value ? 'text-background/70' : 'text-muted-foreground')}>
                        {statusCounts[value] ?? 0}
                      </span>
                    )}
                  </button>
                ))}
              </div>
              <div className="flex items-center gap-2">
                <div className="relative min-w-0 flex-1">
                  <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={searchText}
                    onChange={(e) => setSearchText(e.target.value)}
                    placeholder={t('token_index.searchTokenName')}
                    aria-label={t('token_index.searchTokenName')}
                    className="h-8 pl-8"
                  />
                </div>
                <Button
                  variant="outline"
                  size="icon"
                  className="relative size-8 shrink-0"
                  aria-label={t('apiKey.list.filter')}
                  onClick={() => setFilterSheetOpen(true)}
                >
                  <ListFilter className="size-4" />
                  {activeConditionFields.length > 0 && (
                    <span className="absolute -right-1 -top-1 flex min-w-[16px] items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-medium leading-4 text-background">
                      {activeConditionFields.length}
                    </span>
                  )}
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="icon" className="size-8 shrink-0" aria-label={t('apiKey.list.sort')}>
                      <ArrowDownUp className="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-44">
                    <DropdownMenuLabel>{t('apiKey.list.sort')}</DropdownMenuLabel>
                    {SORT_OPTIONS.map((o) => (
                      <DropdownMenuItem key={o.labelKey} onClick={() => setSort(o.id, o.dir)} className="justify-between">
                        {t(o.labelKey)}
                        {orderBy === o.id && order === o.dir && <Check className="size-4" />}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
                <Button
                  variant={selectMode ? 'secondary' : 'outline'}
                  size="icon"
                  className="size-8 shrink-0"
                  aria-label={t('apiKey.list.selectMode')}
                  aria-pressed={selectMode}
                  onClick={() => (selectMode ? exitSelectMode() : setSelectMode(true))}
                >
                  <ListChecks className="size-4" />
                </Button>
              </div>
              {activeConditionFields.length > 0 && (
                <div className="-mx-1 flex gap-1.5 overflow-x-auto px-1 no-scrollbar">
                  {activeConditionFields.map((f) => (
                    <button
                      key={f.key}
                      type="button"
                      onClick={() => removeCondition(f.key)}
                      aria-label={`${t('filterBar.remove')} ${describeEntry(f, filterState[f.key])}`}
                      className="inline-flex h-7 shrink-0 items-center gap-1 rounded-full border border-input bg-field pl-2.5 pr-1.5 text-xs"
                    >
                      {describeEntry(f, filterState[f.key])}
                      <X className="size-3.5 text-muted-foreground" />
                    </button>
                  ))}
                </div>
              )}
            </>
          ) : (
            <>
              {/* Toolbar: status segments with counts, name search, column picker. */}
              <div className="flex flex-wrap items-center gap-2">
                <Segmented
                  label={t('apiKey.list.segments')}
                  value={status}
                  onChange={changeStatus}
                  options={STATUS_SEGMENTS.map((value) => ({
                    value,
                    label: (
                      <span className="inline-flex items-center gap-1.5">
                        {t(`apiKey.list.status_${value}`)}
                        {statusCounts && (
                          <span className="min-w-[1.25rem] rounded-[5px] bg-foreground/[0.07] px-1 text-[11px] tabular-nums leading-[18px]">
                            {statusCounts[value] ?? 0}
                          </span>
                        )}
                      </span>
                    )
                  }))}
                />
                <div className="relative min-w-[12rem] flex-1">
                  <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={searchText}
                    onChange={(e) => setSearchText(e.target.value)}
                    placeholder={t('token_index.searchTokenName')}
                    aria-label={t('token_index.searchTokenName')}
                    className="h-8 pl-8"
                  />
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-8 gap-1.5"
                      onKeyDown={(e) => {
                        if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && columnMenuRef.current) {
                          e.preventDefault();
                          focusColumnMenuItem(columnMenuRef.current, e.key);
                        }
                      }}
                    >
                      <Columns3 className="size-4" />
                      {t('apiKey.list.columns')}
                      <span className="text-xs tabular-nums text-muted-foreground">
                        {visibleMenuColumnCount}/{menuColumns.length}
                      </span>
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    ref={columnMenuRef}
                    align="end"
                    className="max-h-96 w-52 overflow-y-auto"
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
                      <ColumnToggleItem
                        key={c.id}
                        checked={!!columnVisibility[c.id]}
                        label={t(c.labelKey)}
                        onToggle={() => toggleColumn(c.id)}
                      />
                    ))}
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={resetColumns}>{t('apiKey.list.resetColumns')}</DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>

              {/* Condition bar: "Show keys that" + chips + add condition, always present so the
                  conditions are discoverable. Selected rows swap it for the bulk actions. */}
              {selectedIds.size > 0 ? (
                <BatchBar
                  count={selectedIds.size}
                  onClear={() => setSelectedIds(new Set())}
                  className="min-h-10 rounded-lg border border-primary/30 bg-primary/[0.04] px-3 py-1.5"
                  actions={
                    <>
                      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('enable')}>
                        <Power className="size-3.5" />
                        {t('apiKey.list.batchEnable')}
                      </Button>
                      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('disable')}>
                        <PowerOff className="size-3.5" />
                        {t('apiKey.list.batchDisable')}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 text-destructive"
                        disabled={batchBusy}
                        onClick={() => setBatchDeleteOpen(true)}
                      >
                        <Trash2 className="size-3.5" />
                        {t('common.delete')}
                      </Button>
                    </>
                  }
                />
              ) : (
                <div className="flex min-h-10 w-full items-center gap-2 rounded-lg border border-border bg-card px-3 py-1.5">
                  <FilterBar
                    fields={filterFields}
                    state={filterState}
                    onChange={handleFilterChange}
                    onClearAll={handleClearFilters}
                    triggerVariant="plus"
                    triggerLabel={t('apiKey.list.addCondition')}
                    clearAlignEnd
                    t={t}
                    leading={
                      <span className="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground">
                        <ListFilter className="size-3.5" />
                        {t('apiKey.list.showKeysThat')}
                      </span>
                    }
                    className="min-w-0 flex-1"
                  />
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" size="sm" className="h-7 shrink-0 gap-1.5 px-2 text-[13px]">
                        <Sparkles className="size-3.5" />
                        {t('apiKey.list.presets')}
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-72">
                      {presetMenuItems}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              )}
            </>
          )}
        </div>

        {narrow && selectMode && (
          <BatchBar count={Math.max(selectedIds.size, 0) || 0} onClear={exitSelectMode} floating actions={batchActions} />
        )}
        {narrow && (
          <FilterSheet
            open={filterSheetOpen}
            onOpenChange={setFilterSheetOpen}
            title={t('apiKey.list.filter')}
            fields={filterFields}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            t={t}
            top={
              <div className="space-y-1.5">
                <div className="text-[11px] font-medium text-muted-foreground">{t('apiKey.list.presets')}</div>
                <div className="flex flex-wrap gap-1.5">
                  {[...BUILTIN_PRESETS, ...savedPresets].map((p) => (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => {
                        applyPreset(p);
                        setFilterSheetOpen(false);
                      }}
                      className="h-8 rounded-full border border-input bg-field px-3 text-[13px] hover:bg-muted"
                    >
                      {p.labelKey ? t(p.labelKey) : p.name}
                    </button>
                  ))}
                </div>
              </div>
            }
          />
        )}

        <Card>
          {!loaded ? (
            <TokenListSkeleton layout={layout} />
          ) : tokens.length === 0 && !searching ? (
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
              onCopyKey={onCopyKey}
              onEdit={openEdit}
              onDelete={setDeleteTarget}
              onToggleStatus={onToggleStatus}
              onShowUsage={setUsageTarget}
              onConnectOpenClaw={setOpenClawTarget}
              onOpen={(item) => route.open(item.id)}
              openId={route.openId}
              selectedIds={selectedIds}
              onToggleSelect={toggleSelect}
              onToggleSelectAll={toggleSelectAll}
              layout={layout}
              onNaturalWidth={setMeasuredTableWidth}
              selectMode={selectMode}
              order={order}
              orderBy={orderBy}
              onSort={handleSort}
              columnVisibility={columnVisibility}
              quotaResetTimezone={siteInfo?.quota_reset_timezone}
              quotaResetWeekStart={siteInfo?.quota_reset_week_start}
            />
          )}

          <div className="border-t border-border">
            <ListFooter
              total={listCount}
              page={page}
              pageSize={rowsPerPage}
              selectedCount={selectedIds.size}
              onPageChange={setPage}
              onPageSizeChange={onRowsPerPageChange}
            />
          </div>
        </Card>

        {/* The panel comes first so the dialogs it opens (delete, test command, client setup) stack above it. */}
        {route.openId > 0 && (
          <TokenPanel
            key={route.openId}
            tokenId={route.openId}
            tab={route.tab}
            onTabChange={route.setTab}
            onClose={route.close}
            onChanged={handleRefresh}
            adminMode={isAdminSearch}
            reloadKey={reloadKey}
            userGroup={userGroup}
            groupOptions={userGroupOptions}
            userIsReliable={userIsReliable}
            orgMemberNames={isOrgContext ? orgMemberNames : null}
            todayUsage={todayUsage}
            resetTimezone={siteInfo?.quota_reset_timezone}
            resetWeekStart={siteInfo?.quota_reset_week_start}
            onToggleStatus={onToggleStatus}
            onCopyKey={onCopyKey}
            onCopyText={copyText}
            onDelete={setDeleteTarget}
            onShowUsage={setUsageTarget}
            onConnectOpenClaw={setOpenClawTarget}
          />
        )}

        {route.isNew && (
          <NewTokenDialog
            onClose={route.close}
            onCreated={handleRefresh}
            onOpen={(id) => route.open(id)}
            baseUrl={apiAddresses[0]?.url}
            onCopy={copyText}
          />
        )}

        <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('common.delete')}</DialogTitle>
              <DialogDescription>
                {t('common.deleteConfirm', { title: `${t('token_index.token')} "${deleteTarget?.name}"` })}
              </DialogDescription>
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

        <Dialog open={batchDeleteOpen} onOpenChange={(o) => !o && setBatchDeleteOpen(false)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('common.delete')}</DialogTitle>
              <DialogDescription>{t('apiKey.list.batchDeleteConfirm', { count: selectedIds.size })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setBatchDeleteOpen(false)}>
                {t('token_index.cancel')}
              </Button>
              <Button variant="destructive" onClick={() => runBatch('delete')} disabled={batchBusy}>
                {t('token_index.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

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
