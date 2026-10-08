import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { RotateCcw, ChevronLeft, ChevronRight } from 'lucide-react';

import { timestamp2string } from 'utils/common';
import { getPageSize, savePageSize, PAGE_SIZE_OPTIONS } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import PageActions from '@/components/chrome/PageActions';
import TimeRangeSelect, { detectPreset } from '@/components/TimeRangeSelect';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { FilterBar, filterStateToParams, isEntryActive, paramsToFilterState } from '@/components/filter-bar';

import { ORDER_STATUS, fetchOrders } from './paymentApi';

// FilterBar 字段:文本(gateway_id/user_id/trade_no/gateway_no)+ status 单选 enum;
// 时间范围由共享 TimeRangeSelect 承担(与 Log 一致,时间不入 chip)。后端均为单值标量参数。
const ORDER_FILTER_FIELDS = [
  { key: 'gateway_id', labelKey: 'orderlogPage.gatewayIdLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.gatewayId' },
  { key: 'user_id', labelKey: 'orderlogPage.userIdLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.userId' },
  { key: 'trade_no', labelKey: 'orderlogPage.tradeNoLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.tradeNo' },
  { key: 'gateway_no', labelKey: 'orderlogPage.gatewayNoLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.gatewayNo' },
  {
    key: 'status',
    labelKey: 'orderlogPage.statusLabel',
    type: 'enum',
    single: true,
    supportsExclude: false,
    paramInclude: 'status',
    options: Object.values(ORDER_STATUS).map((o) => ({ value: o.value, labelKey: o.labelKey }))
  }
];
const ORDER_FILTER_PARAM_KEYS = ['gateway_id', 'user_id', 'trade_no', 'gateway_no', 'status'];

const buildDefaultTimeRange = () => ({ start_timestamp: 0, end_timestamp: dayjs().unix() + 3600 });

const buildTimeRangeFromParams = (searchParams) => {
  const base = buildDefaultTimeRange();
  const s = searchParams.get('start_timestamp');
  if (s && /^[0-9]+$/.test(s)) base.start_timestamp = parseInt(s, 10);
  const e = searchParams.get('end_timestamp');
  if (e && /^[0-9]+$/.test(e)) base.end_timestamp = parseInt(e, 10);
  return base;
};

function statusBadge(status, t) {
  const opt = ORDER_STATUS[status];
  return <Badge variant={opt?.variant || 'secondary'}>{opt ? t(opt.labelKey) : t('common.unknown')}</Badge>;
}

export default function PaymentOrder() {
  const { t, i18n } = useTranslation();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('paymentOrder'));
  const [order, setOrder] = useState('desc');
  const [orderBy, setOrderBy] = useState('created_at');
  const [listCount, setListCount] = useState(0);
  const [orderList, setOrderList] = useState([]);
  const [searching, setSearching] = useState(false);
  const [refreshFlag, setRefreshFlag] = useState(false);

  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(ORDER_FILTER_FIELDS, searchParams));
  const [timeRange, setTimeRange] = useState(() => buildTimeRangeFromParams(searchParams));
  // 时间范围档位(触发器徽标显示用);默认区间为「不限起点」,反推不到日历档位时按自定义呈现。
  const [timePreset, setTimePreset] = useState(() => detectPreset(buildTimeRangeFromParams(searchParams)));

  // reqId 守卫(UX-13):快速切页/切筛选/切时间范围时丢弃过期响应;paymentApi.js 结构保留,不迁 usePaginatedList。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  const fetchData = useCallback(() => {
    const keyword = {
      ...filterStateToParams(ORDER_FILTER_FIELDS, filterState),
      start_timestamp: timeRange.start_timestamp,
      end_timestamp: timeRange.end_timestamp
    };
    return runGuardedFetch(guardRef.current, () => fetchOrders(page, rowsPerPage, keyword, order, orderBy), {
      onStart: () => setSearching(true),
      onResult: (data) => {
        if (data) {
          setListCount(data.total_count);
          setOrderList(data.data || []);
        }
      },
      onFinally: () => setSearching(false)
    });
  }, [page, rowsPerPage, filterState, timeRange, order, orderBy]);

  useEffect(() => {
    fetchData();
  }, [fetchData, refreshFlag]);

  // 已应用筛选(文本 + status + 时间)同步到 URL(replace,刷新可恢复)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        ORDER_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(ORDER_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        params.set('start_timestamp', String(timeRange.start_timestamp));
        params.set('end_timestamp', String(timeRange.end_timestamp));
        return params;
      },
      { replace: true }
    );
  }, [filterState, timeRange, setSearchParams]);

  const doRefresh = () => {
    setOrder('desc');
    setOrderBy('created_at');
    setFilterState({});
    setTimeRange(buildDefaultTimeRange());
    setTimePreset(detectPreset(buildDefaultTimeRange()));
    setPage(0);
    setRefreshFlag((f) => !f);
  };

  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };
  // 时间范围档位应用:范围原子写入(联动列表/URL),记录档位供触发器显示。
  const handleApplyRange = (range, presetId) => {
    setPage(0);
    setTimePreset(presetId);
    setTimeRange(range);
  };

  // 已生效的筛选字段(仅用于判断是否渲染下方 chips 容器),对齐 Log 页。
  const activeChipFields = useMemo(() => ORDER_FILTER_FIELDS.filter((f) => isEntryActive(f, filterState[f.key])), [filterState]);

  const onSortClick = (colId) => {
    const isAsc = orderBy === colId && order === 'asc';
    setOrder(isAsc ? 'desc' : 'asc');
    setOrderBy(colId);
    setPage(0);
  };

  const totalPages = Math.max(1, Math.ceil(listCount / rowsPerPage));

  const sortableCols = ['created_at', 'gateway_id', 'user_id', 'status'];
  const headers = [
    { id: 'created_at', label: t('orderlogPage.tableHeaders.created_at') },
    { id: 'gateway_id', label: t('orderlogPage.tableHeaders.gateway_id') },
    { id: 'user_id', label: t('orderlogPage.tableHeaders.user_id') },
    { id: 'trade_no', label: t('orderlogPage.tableHeaders.trade_no') },
    { id: 'gateway_no', label: t('orderlogPage.tableHeaders.gateway_no') },
    { id: 'amount', label: t('orderlogPage.tableHeaders.amount') },
    { id: 'fee', label: t('orderlogPage.tableHeaders.fee') },
    { id: 'discount', label: t('orderlogPage.tableHeaders.discount') },
    { id: 'order_amount', label: t('orderlogPage.tableHeaders.order_amount') },
    { id: 'exchange_rate', label: t('orderlogPage.tableHeaders.exchange_rate') },
    { id: 'quota', label: t('orderlogPage.tableHeaders.quota') },
    { id: 'status', label: t('orderlogPage.tableHeaders.status') }
  ];

  return (
    <div className="space-y-4">
      {/* 标题行右侧操作:刷新 → 筛选 → 时间范围,对齐 Log / Ledger 的 PageActions 结构。 */}
      <PageActions>
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" size="sm" onClick={doRefresh} aria-label={t('orderlogPage.refreshClear')}>
                <RotateCcw className="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t('orderlogPage.refreshClear')}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <FilterBar
          fields={ORDER_FILTER_FIELDS}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          hideChips
          t={t}
        />
        <TimeRangeSelect value={timeRange} preset={timePreset} onApply={handleApplyRange} t={t} locale={i18n.language} />
      </PageActions>

      {/* 标题行下方:全宽 chips 容器(chips/「+」/Clear);仅在有筛选条件时渲染。 */}
      {activeChipFields.length > 0 && (
        <FilterBar
          fields={ORDER_FILTER_FIELDS}
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
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              <TableRow>
                {headers.map((h) => {
                  const canSort = sortableCols.includes(h.id);
                  return (
                    <TableHead
                      key={h.id}
                      onClick={canSort ? () => onSortClick(h.id) : undefined}
                      className={`whitespace-nowrap ${canSort ? 'cursor-pointer select-none' : ''}`}
                    >
                      {h.label}
                      {orderBy === h.id ? (order === 'desc' ? ' ↓' : ' ↑') : ''}
                    </TableHead>
                  );
                })}
              </TableRow>
            </TableHeader>
            <TableBody>
              {orderList.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={headers.length} className="h-24 text-center text-muted-foreground">
                    {searching ? '…' : t('common.noData', { defaultValue: 'No data' })}
                  </TableCell>
                </TableRow>
              ) : (
                orderList.map((row, index) => (
                  <TableRow key={`${row.id}_${index}`}>
                    <TableCell className="whitespace-nowrap text-sm">{timestamp2string(row.created_at)}</TableCell>
                    <TableCell>{row.gateway_id}</TableCell>
                    <TableCell>{row.user_id}</TableCell>
                    <TableCell className="font-mono text-xs">{row.trade_no}</TableCell>
                    <TableCell className="font-mono text-xs">{row.gateway_no}</TableCell>
                    <TableCell>${row.amount}</TableCell>
                    <TableCell>${row.fee}</TableCell>
                    <TableCell>
                      {row.discount} {row.order_currency}
                    </TableCell>
                    <TableCell>
                      {row.order_amount} {row.order_currency}
                    </TableCell>
                    {/* 历史订单未记录汇率(0)时显示 -,避免误导 */}
                    <TableCell>{row.exchange_rate > 0 ? row.exchange_rate : '-'}</TableCell>
                    <TableCell>{row.quota}</TableCell>
                    <TableCell>{statusBadge(row.status, t)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        {orderList.length === 0 ? (
          <div className="md:hidden rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
            {searching ? '…' : t('common.noData', { defaultValue: 'No data' })}
          </div>
        ) : (
          <div className="md:hidden flex flex-col gap-3 p-3">
            {orderList.map((row, index) => (
              <div key={`${row.id}_${index}`} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                <div className="mb-2 text-sm font-medium font-mono text-xs">{row.trade_no}</div>
                <dl className="flex flex-col divide-y divide-border/60">
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.created_at')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{timestamp2string(row.created_at)}</dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.gateway_id')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{row.gateway_id}</dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.user_id')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{row.user_id}</dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.gateway_no')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono text-xs">{row.gateway_no}</dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.amount')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">${row.amount}</dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.fee')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">${row.fee}</dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.discount')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      {row.discount} {row.order_currency}
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.order_amount')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      {row.order_amount} {row.order_currency}
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.exchange_rate')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      {row.exchange_rate > 0 ? row.exchange_rate : '-'}
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.quota')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{row.quota}</dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('orderlogPage.tableHeaders.status')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{statusBadge(row.status, t)}</dd>
                  </div>
                </dl>
              </div>
            ))}
          </div>
        )}

        <div className="flex items-center justify-between gap-4 border-t border-border px-4 py-3 text-sm">
          <span className="text-muted-foreground">{listCount} total</span>
          <div className="flex items-center gap-3">
            <Select
              value={String(rowsPerPage)}
              onValueChange={(v) => {
                setRowsPerPage(Number(v));
                savePageSize('paymentOrder', Number(v));
                setPage(0);
              }}
            >
              <SelectTrigger className="h-8 w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PAGE_SIZE_OPTIONS.map((s) => (
                  <SelectItem key={s} value={String(s)}>
                    {s} / page
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span className="text-muted-foreground">
              {page + 1} / {totalPages}
            </span>
            <Button variant="outline" size="icon" className="size-8" disabled={page <= 0} onClick={() => setPage((p) => p - 1)}>
              <ChevronLeft className="size-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="size-8"
              disabled={page + 1 >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}
