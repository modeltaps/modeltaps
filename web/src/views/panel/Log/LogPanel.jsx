import PropTypes from 'prop-types';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSelector } from 'react-redux';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Check, Columns3, Download, Loader2, Minus, MoreVertical, RotateCcw, Search } from 'lucide-react';

import PageActions from '@/components/chrome/PageActions';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { showError, showSuccess, useIsAdmin } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import { useOrg } from 'contexts/OrgContext';
import { FilterBar, filterStateToParams, isEntryActive, paramsToFilterState, toSearchParams } from '@/components/filter-bar';
import { buildLogFilterFields, LOG_FILTER_PARAM_KEYS } from './logFilterFields';
import LogIdLookup from './LogIdLookup';
import TimeRangeSelect, { detectPreset } from '@/components/TimeRangeSelect';
import LogHistogram from './LogHistogram';
import LogTable from './LogTable';
import LogPagination from './LogPagination';
import LogDetailDialog from './LogDetailDialog';
import { exportLogs } from './logExport';

// ==============================|| PANEL — LOG VIEWER ||============================== //
// shadcn/Tailwind port of views/Log. Server-side pagination + filters + CSV export.
// 仅 API 请求明细(log_type 固定 2);账务记录(充值/管理/系统)已迁出到账户流水页。

// 默认时间范围:最近 1 天(now-24h ~ now+1h 缓冲),对齐 OpenRouter 默认档位。log_type 固定为 '2'(仅请求明细)。
const buildDefaultTimeRange = () => ({
  start_timestamp: dayjs().subtract(1, 'day').unix(),
  end_timestamp: dayjs().unix() + 3600
});

// 从 URL 恢复时间范围(深链/刷新);非法/缺失回退默认。筛选条件的恢复见 paramsToFilterState。
const buildTimeRangeFromParams = (searchParams) => {
  const base = buildDefaultTimeRange();
  const start = searchParams.get('start_timestamp');
  if (start && /^[0-9]+$/.test(start)) base.start_timestamp = parseInt(start, 10);
  const end = searchParams.get('end_timestamp');
  if (end && /^[0-9]+$/.test(end)) base.end_timestamp = parseInt(end, 10);
  return base;
};

// Column registry for the show/hide menu. Ids match LogTable column ids; v1
// (135c469) had the same 13 columns all visible by default. adminOnly columns
// never appear in the menu for regular users (they are not rendered either).
// orgOnly columns only exist in an organization context; hideInOrg columns are
// site-internal info excluded from the org view (aligned with the org CSV export).
const LOG_COLUMNS = [
  { id: 'created_at', labelKey: 'logPage.timeLabel' },
  { id: 'channel_id', labelKey: 'logPage.channelLabel', adminOnly: true },
  { id: 'user_id', labelKey: 'logPage.userLabel', adminOnly: true },
  { id: 'member', labelKey: 'logPage.memberLabel', orgOnly: true },
  { id: 'group', labelKey: 'logPage.groupLabel' },
  { id: 'token_name', labelKey: 'logPage.tokenLabel' },
  { id: 'app', labelKey: 'logPage.appLabel' },
  { id: 'type', labelKey: 'logPage.typeLabel' },
  { id: 'model_name', labelKey: 'logPage.modelLabel' },
  { id: 'relay_mode', labelKey: 'logPage.relayMode.columnLabel' },
  { id: 'finish_reason', labelKey: 'logPage.finishReason.columnLabel' },
  { id: 'duration', labelKey: 'logPage.durationLabel' },
  { id: 'throughput', labelKey: 'logPage.throughputLabel' },
  { id: 'prompt_tokens', labelKey: 'logPage.inputLabel' },
  { id: 'completion_tokens', labelKey: 'logPage.outputLabel' },
  { id: 'quota', labelKey: 'logPage.quotaLabel' },
  { id: 'source_ip', labelKey: 'logPage.sourceIp', hideInOrg: true }
];

const COLUMN_VISIBILITY_STORAGE_KEY = 'log-column-visibility';

const buildDefaultColumnVisibility = () => Object.fromEntries(LOG_COLUMNS.map((c) => [c.id, true]));

// localStorage persistence is an addition over v1 (which kept the selection in
// memory only); unknown/missing keys fall back to the visible default.
const loadColumnVisibility = () => {
  const defaults = buildDefaultColumnVisibility();
  try {
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

// ↑/↓/Home/End roving focus for the column menu items. The custom DropdownMenu
// has no built-in arrow-key navigation; items stay plain buttons so mouse/Tab
// behavior is unchanged.
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

export default function Log() {
  const { t, i18n } = useTranslation();
  const userIsAdmin = useIsAdmin();
  const { userGroup } = useSelector((state) => state.account);
  // 站点级明细留存总闸门(/api/status 的 log_io_enabled);读不到时按 false 保守处理。
  const siteLogIOEnabled = useSelector((state) => Boolean(state.siteInfo?.log_io_enabled));
  // 时间范围下拉的最早可选点 = now - max_log_lookback_days;0/未设表示不限。
  const maxLogLookbackDays = useSelector((state) => state.siteInfo.max_log_lookback_days);
  const minTimestamp = useMemo(
    () => (maxLogLookbackDays > 0 ? dayjs().subtract(maxLogLookbackDays, 'day').unix() : 0),
    [maxLogLookbackDays]
  );
  const [searchParams, setSearchParams] = useSearchParams();

  // 组织上下文:/api/log/self* 由 orgScope 改写到 /api/org/:id/logs*;站点管理员
  // 维度的接口与筛选(/api/log/、username/channel_id)在组织上下文一律停用。
  const { currentOrgId, orgRole } = useOrg();
  const isOrgContext = Boolean(currentOrgId);
  const effectiveAdmin = userIsAdmin && !isOrgContext;
  // 仅管理员可见(D2=b):组织成员看不到 LogIO 详情入口;个人上下文(仅见己方日志)、
  // 组织 Owner/Admin、平台管理员可见,与后端 CanViewTokenLogIO 收紧一致。
  const orgIsAdmin = orgRole === 'owner' || orgRole === 'admin';
  const canViewLogIO = !isOrgContext || orgIsAdmin || userIsAdmin;
  const [orgMembers, setOrgMembers] = useState([]);
  // 枚举字段预置勾选项(真实数据源);拉取失败静默降级为空列表 + freeText,不阻塞列表首屏。
  const [modelOptions, setModelOptions] = useState([]);
  const [tokenOptions, setTokenOptions] = useState([]);
  const [appOptions, setAppOptions] = useState([]);
  const [channelOptions, setChannelOptions] = useState([]);

  // FilterBar 字段定义(级联筛选驱动),按上下文裁剪。
  const filterFields = useMemo(
    () => buildLogFilterFields({ effectiveAdmin, isOrgContext, orgMembers, modelOptions, tokenOptions, appOptions, channelOptions }),
    [effectiveAdmin, isOrgContext, orgMembers, modelOptions, tokenOptions, appOptions, channelOptions]
  );

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('log'));
  const [order, setOrder] = useState('desc');
  const [orderBy, setOrderBy] = useState('created_at');
  // 级联筛选受控状态(enum/number);从 URL 恢复(param 键与后端 form tag 对齐,与上下文无关)。
  const [filterState, setFilterState] = useState(() =>
    paramsToFilterState(
      buildLogFilterFields({ effectiveAdmin: userIsAdmin && !currentOrgId, isOrgContext: Boolean(currentOrgId), orgMembers: [] }),
      searchParams
    )
  );
  const [timeRange, setTimeRange] = useState(() => buildTimeRangeFromParams(searchParams));
  // 当前时间范围档位(触发器徽标显示用)。首次进入(URL 无时间参数)默认最近 1 天;
  // URL 带时间参数时走 detectPreset(仅识别起点稳定的日历区间,相对区间按自定义呈现)。
  const [timePreset, setTimePreset] = useState(() => {
    const hasTimeParams = searchParams.get('start_timestamp') || searchParams.get('end_timestamp');
    return hasTimeParams ? detectPreset(buildTimeRangeFromParams(searchParams)) : '1d';
  });
  const [refreshFlag, setRefreshFlag] = useState(false);

  const [logs, setLogs] = useState([]);
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [detailItem, setDetailItem] = useState(null);
  const [columnVisibility, setColumnVisibility] = useState(loadColumnVisibility);
  // ID 查找弹窗开关(FilterBar 菜单里的附加入口)。
  const [idLookupOpen, setIdLookupOpen] = useState(false);
  const columnMenuRef = useRef(null);

  // FilterBar 附加入口(非 chip):ID 查找。点击打开弹窗。
  const filterSpecials = useMemo(() => [{ key: 'id_lookup', labelKey: 'logIdLookup.menuLabel', icon: Search }], []);

  const dataReqIdRef = useRef(0);

  useEffect(() => {
    try {
      localStorage.setItem(COLUMN_VISIBILITY_STORAGE_KEY, JSON.stringify(columnVisibility));
    } catch {
      /* storage unavailable -> keep in-memory only */
    }
  }, [columnVisibility]);

  // Columns the current user may toggle; admin-only columns are excluded from
  // the menu (and from the table) for regular users, org-only columns appear
  // only in an organization context (and vice versa for hideInOrg).
  const menuColumns = useMemo(
    () =>
      LOG_COLUMNS.filter(
        (c) =>
          (!c.adminOnly || effectiveAdmin) &&
          (!c.orgOnly || isOrgContext) &&
          !(c.hideInOrg && isOrgContext) &&
          (!c.logIOOnly || canViewLogIO)
      ),
    [effectiveAdmin, isOrgContext, canViewLogIO]
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

  // 组织上下文隐藏 source_ip(站点内部信息),其余沿用用户保存的列偏好。
  const effectiveColumnVisibility = useMemo(
    () => (isOrgContext ? { ...columnVisibility, source_ip: false } : columnVisibility),
    [isOrgContext, columnVisibility]
  );

  // 组装筛选查询对象:log_type 固定 '2' + 时间范围 + 级联筛选参数(enum 多值数组 / number 标量)。
  // 上下文裁剪已由 filterFields 保证(org 只含 member,admin 才含 channel/username/source_ip)。
  const buildFilterQuery = useCallback(() => {
    const q = { log_type: '2', start_timestamp: timeRange.start_timestamp, end_timestamp: timeRange.end_timestamp };
    Object.assign(q, filterStateToParams(filterFields, filterState));
    return q;
  }, [timeRange, filterFields, filterState]);

  // 直方图与列表共用同一份筛选口径(时间窗/模型/API Key 等),随筛选联动。
  const filterQuery = useMemo(() => buildFilterQuery(), [buildFilterQuery]);

  const fetchData = useCallback(
    async (pageArg, size, ord, ordBy) => {
      const reqId = ++dataReqIdRef.current;
      setSearching(true);
      try {
        const sortParam = ordBy ? (ord === 'desc' ? `-${ordBy}` : ordBy) : ordBy;
        // 组织上下文固定走 self 路径,由 orgScope 改写到 /api/org/:id/logs
        const url = effectiveAdmin ? '/api/log/' : '/api/log/self/';
        const params = toSearchParams({ page: pageArg + 1, size, order: sortParam, ...buildFilterQuery() });
        const res = await API.get(url, { params });
        if (reqId !== dataReqIdRef.current) return;
        const { success, message, data } = res.data;
        if (success) {
          setListCount(data.total_count);
          setLogs(data.data);
        } else {
          showError(message);
        }
      } catch (error) {
        if (reqId !== dataReqIdRef.current) return;
        console.error(error);
      } finally {
        if (reqId === dataReqIdRef.current) setSearching(false);
      }
    },
    [effectiveAdmin, buildFilterQuery]
  );

  useEffect(() => {
    fetchData(page, rowsPerPage, order, orderBy);
  }, [page, rowsPerPage, order, orderBy, fetchData, refreshFlag]);

  // 把已应用筛选同步到 URL(replace,避免污染历史)。先清空所有可能的筛选键再按当前 state 重写,
  // 保证切换上下文后残留的键(如 admin 专属)被清掉;orgScope 行为不变。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        LOG_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(filterFields, filterState);
        for (const [k, v] of Object.entries(q)) {
          if (Array.isArray(v)) v.forEach((x) => params.append(k, String(x)));
          else params.set(k, String(v));
        }
        params.set('start_timestamp', String(timeRange.start_timestamp));
        params.set('end_timestamp', String(timeRange.end_timestamp));
        return params;
      },
      { replace: true }
    );
  }, [filterState, filterFields, timeRange, setSearchParams]);

  // T8 衔接点:切换组织/个人上下文后重置筛选并重新拉取(member_id 等过滤跨组织无意义)。
  // 首次挂载跳过,保持个人上下文初始行为与现网一致。
  const prevOrgIdRef = useRef(currentOrgId);
  useEffect(() => {
    if (prevOrgIdRef.current === currentOrgId) return;
    prevOrgIdRef.current = currentOrgId;
    setOrder('desc');
    setOrderBy('created_at');
    setPage(0);
    setFilterState({});
    setTimeRange(buildDefaultTimeRange());
    setTimePreset('1d');
  }, [currentOrgId]);

  // 组织上下文:成员列表(成员筛选下拉用)
  useEffect(() => {
    if (!isOrgContext) {
      setOrgMembers([]);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const res = await API.get(`/api/org/${currentOrgId}/members`, { params: { page: 1, size: 100 } });
        const { success, data } = res.data;
        if (!cancelled && success) setOrgMembers(data?.data || []);
      } catch (error) {
        // 错误已由全局拦截器提示
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isOrgContext, currentOrgId]);

  // 枚举筛选预置项:可用模型 + API Key 名(去重)。挂载/上下文切换拉取,静默降级,不阻塞首屏。
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await API.get('/api/available_model');
        const { success, data } = res.data;
        if (!cancelled && success && data && typeof data === 'object') setModelOptions(Object.keys(data).sort());
      } catch {
        /* 静默降级为空列表 + freeText */
      }
    })();
    (async () => {
      try {
        const res = await API.get('/api/token/', { params: { page: 1, size: 100 } });
        const list = res.data?.data?.data;
        if (!cancelled && res.data?.success && Array.isArray(list)) {
          setTokenOptions([...new Set(list.filter((tk) => tk.name).map((tk) => tk.name))]);
        }
      } catch {
        /* 静默降级为空列表 + freeText */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isOrgContext, currentOrgId]);

  // App 归因预置项:来自用量分析 group_by=app 聚合(每行含 app_name 及代表性 app_domain)。
  // 组织上下文走 /api/org/:id/analytics;个人与平台管理员上下文均走 /api/user/self/analytics
  // (best-effort:预置来自可见归因数据,覆盖不全由 freeText 兜底,与 token_name/model_name 一致)。
  // 全时段口径(不传时间范围),仅作候选建议;按 name 去重(取首个非空 domain);拉取失败静默降级。
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const url = isOrgContext ? `/api/org/${currentOrgId}/analytics` : '/api/user/self/analytics';
        const res = await API.get(url, { params: { group_by: 'app' } });
        const { success, data } = res.data;
        if (!cancelled && success && Array.isArray(data)) {
          const seen = new Map();
          for (const r of data) {
            if (r.app_name && !seen.has(r.app_name)) seen.set(r.app_name, r.app_domain || '');
          }
          setAppOptions(Array.from(seen, ([name, domain]) => ({ name, domain })));
        }
      } catch {
        /* 静默降级为空列表 + freeText */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isOrgContext, currentOrgId]);

  // 渠道选项仅平台管理员上下文可用(GET /api/channel/);非管理员清空,静默降级。
  useEffect(() => {
    if (!effectiveAdmin) {
      setChannelOptions([]);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const res = await API.get('/api/channel/', { params: { page: 1, size: 100 } });
        const { success, data } = res.data;
        const list = data?.data;
        if (!cancelled && success && Array.isArray(list)) setChannelOptions(list.map((c) => ({ id: c.id, name: c.name })));
      } catch {
        /* 静默降级为空列表 + freeText */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [effectiveAdmin]);

  // 级联筛选受控回调:任何改动即时应用并回到第一页;清空同理。
  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };

  // 已生效的筛选字段(仅用于判断是否渲染下方筛选容器)。chips/「+」/Clear 由容器内 FilterBar 承担,对齐 OpenRouter。
  const activeChipFields = useMemo(() => filterFields.filter((f) => isEntryActive(f, filterState[f.key])), [filterFields, filterState]);

  const handleReset = () => {
    setOrder('desc');
    setOrderBy('created_at');
    setPage(0);
    setFilterState({});
    setTimeRange(buildDefaultTimeRange());
    setTimePreset('1d');
    setRefreshFlag((v) => !v);
  };

  // 时间范围档位应用:计算/自定义范围原子写入 timeRange(联动列表/直方图/URL),并记录档位供触发器显示。
  const handleApplyRange = (range, presetId) => {
    setPage(0);
    setTimePreset(presetId);
    setTimeRange(range);
  };

  const handleSort = (id) => {
    if (!id) return;
    const isAsc = orderBy === id && order === 'asc';
    setOrder(isAsc ? 'desc' : 'asc');
    setOrderBy(id);
  };

  const handleExport = useCallback(async () => {
    if (exporting) return;
    setExporting(true);
    try {
      await exportLogs({ params: buildFilterQuery(), order, orderBy, userIsAdmin: effectiveAdmin });
      showSuccess(t('logPage.exportSuccess'));
    } catch (error) {
      showError(`${t('logPage.exportError')}: ${error.message}`);
    } finally {
      setExporting(false);
    }
  }, [exporting, buildFilterQuery, order, orderBy, effectiveAdmin, t]);

  return (
    <div className="space-y-4">
      {/* 标题行右侧操作:刷新 → 筛选 → 时间范围 → 列选择 → 更多(导出),对齐 OpenRouter。 */}
      <PageActions>
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" size="sm" onClick={handleReset} aria-label={t('logPage.refreshButton')}>
                <RotateCcw className="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t('logPage.refreshButton')}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <FilterBar
          fields={filterFields}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          specials={filterSpecials}
          onSpecial={() => setIdLookupOpen(true)}
          hideChips
          t={t}
        />
        <TimeRangeSelect
          value={timeRange}
          preset={timePreset}
          minTimestamp={minTimestamp}
          onApply={handleApplyRange}
          t={t}
          locale={i18n.language}
        />
        {/* 列选择:独立图标按钮,点开列勾选菜单(含 localStorage 持久化)。 */}
        <DropdownMenu>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    aria-label={t('logPage.selectColumns')}
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
              <TooltipContent>{t('logPage.selectColumns')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <DropdownMenuContent
            ref={columnMenuRef}
            align="end"
            className="max-h-[70vh] w-52 overflow-y-auto"
            onKeyDown={(e) => {
              if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Home' || e.key === 'End') {
                e.preventDefault();
                focusColumnMenuItem(e.currentTarget, e.key);
              }
            }}
          >
            <DropdownMenuLabel>{t('logPage.selectColumns')}</DropdownMenuLabel>
            <ColumnToggleItem
              checked={allMenuColumnsVisible}
              indeterminate={!allMenuColumnsVisible && someMenuColumnsVisible}
              label={t('logPage.columnSelectAll')}
              onToggle={toggleAllColumns}
            />
            <DropdownMenuSeparator />
            {menuColumns.map((c) => (
              <ColumnToggleItem key={c.id} checked={!!columnVisibility[c.id]} label={t(c.labelKey)} onToggle={() => toggleColumn(c.id)} />
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        {/* 更多:导出 CSV。 */}
        <DropdownMenu>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" aria-label={t('logPage.moreActions')}>
                    <MoreVertical className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
              </TooltipTrigger>
              <TooltipContent>{t('logPage.moreActions')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={handleExport} disabled={exporting}>
              {exporting ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
              <span>{exporting ? t('logPage.exporting') : t('logPage.exportButton')}</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </PageActions>

      {/* 标题行下方:OpenRouter 式全宽输入框容器(chips/「+」/Clear);仅在有筛选条件时渲染。
          自定义时间范围输入已移入时间范围下拉的弹层,不再有独立悬浮块。 */}
      {activeChipFields.length > 0 && (
        <FilterBar
          fields={filterFields}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          specials={filterSpecials}
          onSpecial={() => setIdLookupOpen(true)}
          triggerVariant="plus"
          clearAlignEnd
          t={t}
          className="w-full rounded-lg border border-input bg-background px-3 py-2"
        />
      )}

      <LogHistogram filterQuery={filterQuery} effectiveAdmin={effectiveAdmin} t={t} />

      <Card>
        <CardContent className="p-0">
          <LogTable
            t={t}
            data={logs}
            userIsAdmin={effectiveAdmin}
            isOrgContext={isOrgContext}
            canViewLogIO={canViewLogIO}
            userGroup={userGroup}
            order={order}
            orderBy={orderBy}
            onSort={handleSort}
            onRowDetail={setDetailItem}
            searching={searching}
            columnVisibility={effectiveColumnVisibility}
          />
          <div className="border-t border-border px-4">
            <LogPagination
              page={page}
              rowsPerPage={rowsPerPage}
              count={listCount}
              onPageChange={setPage}
              onRowsPerPageChange={(size) => {
                setPage(0);
                setRowsPerPage(size);
                savePageSize('log', size);
              }}
            />
          </div>
        </CardContent>
      </Card>

      <LogIdLookup open={idLookupOpen} onOpenChange={setIdLookupOpen} effectiveAdmin={effectiveAdmin} onFound={setDetailItem} t={t} />

      {detailItem && (
        <LogDetailDialog
          item={detailItem}
          userGroup={userGroup}
          userIsAdmin={effectiveAdmin}
          siteLogIOEnabled={siteLogIOEnabled}
          platformIsAdmin={userIsAdmin}
          isOrgContext={isOrgContext}
          orgIsAdmin={orgIsAdmin}
          t={t}
          onClose={() => setDetailItem(null)}
        />
      )}
    </div>
  );
}
