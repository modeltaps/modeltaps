import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { RotateCcw } from 'lucide-react';

import PageActions from '@/components/chrome/PageActions';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import TimeRangeSelect, { detectPreset } from '@/components/TimeRangeSelect';
import { API } from 'utils/api';
import { showError, useIsAdmin } from 'utils/common';
import { FilterBar, filterStateToParams, isEntryActive, paramsToFilterState } from '@/components/filter-bar';
import { getPageSize, savePageSize } from 'constants';
import MidjourneyTable from './MidjourneyTable';
import LogPagination from '../Log/LogPagination';

// ==============================|| PANEL — MIDJOURNEY RECORDS ||============================== //
// shadcn/Tailwind port of views/Midjourney. Server-side pagination + FilterBar chips + time range.
// 筛选与时间范围收进标题行右侧 PageActions(共享 TimeRangeSelect),结构对齐 Log。

// 文本筛选字段(channel_id 仅管理员);后端 /api/mj[/self] 接受单值标量参数。
const buildMidjourneyFilterFields = (userIsAdmin) => [
  { key: 'mj_id', labelKey: 'tableToolBar.taskId', type: 'text' },
  ...(userIsAdmin ? [{ key: 'channel_id', labelKey: 'tableToolBar.channelId', type: 'text' }] : [])
];
const MJ_FILTER_PARAM_KEYS = ['mj_id', 'channel_id'];

// 内部时间范围统一用 unix 秒(与共享 TimeRangeSelect 一致);默认最近 1 天(now-24h ~ now+1h 缓冲),对齐 Log。
// v1 MJ API 时间戳为毫秒,仅在组装请求参数时 ×1000 换算。
const buildDefaultTimeRange = () => ({
  start_timestamp: dayjs().subtract(1, 'day').unix(),
  end_timestamp: dayjs().unix() + 3600
});

const buildTimeRangeFromParams = (searchParams) => {
  const base = buildDefaultTimeRange();
  const s = searchParams.get('start_timestamp');
  if (s && /^[0-9]+$/.test(s)) base.start_timestamp = parseInt(s, 10);
  const e = searchParams.get('end_timestamp');
  if (e && /^[0-9]+$/.test(e)) base.end_timestamp = parseInt(e, 10);
  return base;
};

export default function Midjourney() {
  const { t, i18n } = useTranslation();
  const userIsAdmin = useIsAdmin();

  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('midjourney'));
  const [order, setOrder] = useState('desc');
  const [orderBy, setOrderBy] = useState('id');
  const filterFields = useMemo(() => buildMidjourneyFilterFields(userIsAdmin), [userIsAdmin]);
  const [filterState, setFilterState] = useState(() => paramsToFilterState(buildMidjourneyFilterFields(true), searchParams));
  const [timeRange, setTimeRange] = useState(() => buildTimeRangeFromParams(searchParams));
  // 时间范围档位(触发器徽标显示用):URL 带时间参数走 detectPreset,否则默认最近 1 天。
  const [timePreset, setTimePreset] = useState(() => {
    const hasTimeParams = searchParams.get('start_timestamp') || searchParams.get('end_timestamp');
    return hasTimeParams ? detectPreset(buildTimeRangeFromParams(searchParams)) : '1d';
  });
  const [refreshFlag, setRefreshFlag] = useState(false);

  const [logs, setLogs] = useState([]);
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);

  const dataReqIdRef = useRef(0);

  const fetchData = useCallback(async () => {
    const reqId = ++dataReqIdRef.current;
    setSearching(true);
    try {
      const sortParam = orderBy ? (order === 'desc' ? `-${orderBy}` : orderBy) : orderBy;
      const url = userIsAdmin ? '/api/mj/' : '/api/mj/self/';
      const params = {
        page: page + 1,
        size: rowsPerPage,
        order: sortParam,
        ...filterStateToParams(filterFields, filterState),
        start_timestamp: timeRange.start_timestamp * 1000,
        end_timestamp: timeRange.end_timestamp * 1000
      };
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
  }, [userIsAdmin, page, rowsPerPage, order, orderBy, filterFields, filterState, timeRange]);

  useEffect(() => {
    fetchData();
  }, [fetchData, refreshFlag]);

  // 已应用筛选(文本 + 时间)同步到 URL(replace,刷新可恢复)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        MJ_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(filterFields, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        params.set('start_timestamp', String(timeRange.start_timestamp));
        params.set('end_timestamp', String(timeRange.end_timestamp));
        return params;
      },
      { replace: true }
    );
  }, [filterFields, filterState, timeRange, setSearchParams]);

  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };

  // 已生效的筛选字段(仅用于判断是否渲染下方筛选容器)。chips/「+」/Clear 由容器内 FilterBar 承担,对齐 Log。
  const activeChipFields = useMemo(() => filterFields.filter((f) => isEntryActive(f, filterState[f.key])), [filterFields, filterState]);

  const handleReset = () => {
    setOrder('desc');
    setOrderBy('id');
    setPage(0);
    setFilterState({});
    setTimeRange(buildDefaultTimeRange());
    setTimePreset('1d');
    setRefreshFlag((v) => !v);
  };

  // 时间范围档位应用:计算/自定义范围原子写入 timeRange(联动列表/URL),并记录档位供触发器显示。
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

  return (
    <div className="space-y-4">
      {/* 标题行右侧操作:刷新 → 筛选 → 时间范围,对齐 Log。 */}
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
          hideChips
          t={t}
        />
        <TimeRangeSelect value={timeRange} preset={timePreset} onApply={handleApplyRange} t={t} locale={i18n.language} />
      </PageActions>

      {/* 标题行下方:OpenRouter 式全宽输入框容器(chips/「+」/Clear);仅在有筛选条件时渲染。 */}
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
        <CardContent className="p-0">
          <MidjourneyTable
            t={t}
            data={logs}
            userIsAdmin={userIsAdmin}
            order={order}
            orderBy={orderBy}
            onSort={handleSort}
            searching={searching}
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
                savePageSize('midjourney', size);
              }}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
