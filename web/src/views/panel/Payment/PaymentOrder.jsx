import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { ArrowDown, ArrowUp, RotateCcw } from 'lucide-react';

import { cn } from '@/lib/utils';
import { timestamp2string } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';
import { getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import PageActions from '@/components/chrome/PageActions';
import TimeRangeSelect, { detectPreset } from '@/components/TimeRangeSelect';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { FilterBar, filterStateToParams, isEntryActive, paramsToFilterState } from '@/components/filter-bar';
import {
  ColumnPicker,
  ListEmptyState,
  ListFooter,
  ListSkeleton,
  StateDot,
  StatusSegments,
  useColumnVisibility,
  useListLayout
} from '@/components/list-page';

import { ROW_OPENS, rowOpenProps, useObjectRoute } from '@/components/objects';
import { ORDER_STATUS, fetchOrders } from './paymentApi';
import OrderPanel from './OrderPanel';

// ==============================|| PAYMENT — ORDER LIST (ADMIN) ||============================== //
// Status segments (with counts from the list response) over one-line rows: when (relative, the full
// time on hover), who, the order number, the amount, what was paid and the status. The rest
// (gateway, gateway order no, fee, discount, rate, credited quota) is in the column picker. On
// phones the rows are 56px. A row opens the read-only order panel (?trade=<order no>).

// FilterBar 字段：文本(gateway_id/user_id/trade_no/gateway_no)；状态由上方分段承担，时间由共享
// TimeRangeSelect 承担（与 Log 一致，时间不入 chip）。后端均为单值标量参数。
const ORDER_FILTER_FIELDS = [
  { key: 'gateway_id', labelKey: 'orderlogPage.gatewayIdLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.gatewayId' },
  { key: 'user_id', labelKey: 'orderlogPage.userIdLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.userId' },
  { key: 'trade_no', labelKey: 'orderlogPage.tradeNoLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.tradeNo' },
  { key: 'gateway_no', labelKey: 'orderlogPage.gatewayNoLabel', type: 'text', placeholderKey: 'orderlogPage.placeholder.gatewayNo' }
];
const ORDER_FILTER_PARAM_KEYS = ['gateway_id', 'user_id', 'trade_no', 'gateway_no'];

const SEGMENTS = ['all', 'success', 'pending', 'failed', 'closed'];
const STATUS_TONE = { success: 'live', pending: 'warn', failed: 'bad', closed: 'off' };

export const ORDER_COLUMNS = [
  { id: 'created_at', labelKey: 'orderlogPage.tableHeaders.created_at', fixed: true },
  { id: 'user', labelKey: 'orderlogPage.tableHeaders.user_id' },
  { id: 'trade_no', labelKey: 'orderlogPage.tableHeaders.trade_no' },
  { id: 'gateway', labelKey: 'orderlogPage.tableHeaders.gateway_id', defaultHidden: true },
  { id: 'gateway_no', labelKey: 'orderlogPage.tableHeaders.gateway_no', defaultHidden: true },
  { id: 'amount', labelKey: 'orderlogPage.tableHeaders.amount' },
  { id: 'fee', labelKey: 'orderlogPage.tableHeaders.fee', defaultHidden: true },
  { id: 'discount', labelKey: 'orderlogPage.tableHeaders.discount', defaultHidden: true },
  { id: 'order_amount', labelKey: 'orderList.paid' },
  { id: 'exchange_rate', labelKey: 'orderlogPage.tableHeaders.exchange_rate', defaultHidden: true },
  { id: 'quota', labelKey: 'orderlogPage.tableHeaders.quota', defaultHidden: true },
  { id: 'status', labelKey: 'orderlogPage.tableHeaders.status', fixed: true }
];

const buildDefaultTimeRange = () => ({ start_timestamp: 0, end_timestamp: dayjs().unix() + 3600 });

const buildTimeRangeFromParams = (searchParams) => {
  const base = buildDefaultTimeRange();
  const s = searchParams.get('start_timestamp');
  if (s && /^[0-9]+$/.test(s)) base.start_timestamp = parseInt(s, 10);
  const e = searchParams.get('end_timestamp');
  if (e && /^[0-9]+$/.test(e)) base.end_timestamp = parseInt(e, 10);
  return base;
};

function OrderStatus({ status, t }) {
  const opt = ORDER_STATUS[status];
  return (
    <span className="whitespace-nowrap">
      <StateDot tone={STATUS_TONE[status] || 'off'} />
      {opt ? t(opt.labelKey) : t('common.unknown')}
    </span>
  );
}

const money = (value, currency) => `${Number(value || 0).toFixed(2)} ${currency || ''}`.trim();

export default function PaymentOrder() {
  const { t, i18n } = useTranslation();
  const route = useObjectRoute([], { param: 'trade' });
  const { narrow } = useListLayout();
  const columns = useColumnVisibility({ storageKey: 'payment-order-columns-v1', columns: ORDER_COLUMNS });
  const rtf = useMemo(
    () => new Intl.RelativeTimeFormat((i18n.language || 'zh_CN').replace('_', '-'), { numeric: 'auto' }),
    [i18n.language]
  );
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('paymentOrder'));
  const [order, setOrder] = useState('desc');
  const [listCount, setListCount] = useState(0);
  const [orderList, setOrderList] = useState([]);
  const [statusCounts, setStatusCounts] = useState(null);
  const [searching, setSearching] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [refreshFlag, setRefreshFlag] = useState(false);

  const [searchParams, setSearchParams] = useSearchParams();
  const [status, setStatus] = useState(() => (SEGMENTS.includes(searchParams.get('status')) ? searchParams.get('status') : 'all'));
  const [filterState, setFilterState] = useState(() => paramsToFilterState(ORDER_FILTER_FIELDS, searchParams));
  const [timeRange, setTimeRange] = useState(() => buildTimeRangeFromParams(searchParams));
  // 时间范围档位(触发器徽标显示用);默认区间为「不限起点」,反推不到日历档位时按自定义呈现。
  const [timePreset, setTimePreset] = useState(() => detectPreset(buildTimeRangeFromParams(searchParams)));

  // reqId 守卫(UX-13):快速切页/切筛选/切时间范围时丢弃过期响应。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  const fetchData = useCallback(() => {
    const keyword = {
      ...filterStateToParams(ORDER_FILTER_FIELDS, filterState),
      status: status === 'all' ? '' : status,
      start_timestamp: timeRange.start_timestamp,
      end_timestamp: timeRange.end_timestamp
    };
    return runGuardedFetch(guardRef.current, () => fetchOrders(page, rowsPerPage, keyword, order, 'created_at'), {
      onStart: () => setSearching(true),
      onResult: (data) => {
        if (data) {
          setListCount(data.total_count);
          setOrderList(data.data || []);
          if (data.status_counts) {
            const counts = { ...data.status_counts };
            counts.all = Object.values(data.status_counts).reduce((a, b) => a + b, 0);
            setStatusCounts(counts);
          }
        }
      },
      onFinally: () => {
        setSearching(false);
        setLoaded(true);
      }
    });
  }, [page, rowsPerPage, filterState, status, timeRange, order]);

  useEffect(() => {
    fetchData();
  }, [fetchData, refreshFlag]);

  // 已应用筛选(文本 + 状态 + 时间)同步到 URL(replace,刷新可恢复)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        [...ORDER_FILTER_PARAM_KEYS, 'status'].forEach((k) => params.delete(k));
        const q = filterStateToParams(ORDER_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        if (status !== 'all') params.set('status', status);
        params.set('start_timestamp', String(timeRange.start_timestamp));
        params.set('end_timestamp', String(timeRange.end_timestamp));
        return params;
      },
      { replace: true }
    );
  }, [filterState, status, timeRange, setSearchParams]);

  const doRefresh = () => {
    setOrder('desc');
    setFilterState({});
    setStatus('all');
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
  const handleApplyRange = (range, presetId) => {
    setPage(0);
    setTimePreset(presetId);
    setTimeRange(range);
  };
  const changeStatus = (next) => {
    setPage(0);
    setStatus(next);
  };

  const activeChipFields = useMemo(() => ORDER_FILTER_FIELDS.filter((f) => isEntryActive(f, filterState[f.key])), [filterState]);
  const segmentOptions = SEGMENTS.map((value) => ({ value, label: t(`orderList.segments.${value}`) }));
  const visible = ORDER_COLUMNS.filter((c) => c.fixed || columns.visibility[c.id]);
  const when = (row) => (
    <span title={timestamp2string(row.created_at)} className="whitespace-nowrap">
      {relativeTime(rtf, row.created_at)}
    </span>
  );
  const user = (row) => row.username || `#${row.user_id}`;

  const SortIcon = order === 'desc' ? ArrowDown : ArrowUp;
  const head = {
    created_at: (
      <button
        type="button"
        className="inline-flex items-center gap-1"
        onClick={() => {
          setOrder((o) => (o === 'desc' ? 'asc' : 'desc'));
          setPage(0);
        }}
      >
        {t('orderlogPage.tableHeaders.created_at')}
        <SortIcon className="size-3.5" />
      </button>
    )
  };
  const cell = {
    created_at: (row) => <TableCell className="text-muted-foreground">{when(row)}</TableCell>,
    user: (row) => <TableCell className="max-w-40 truncate">{user(row)}</TableCell>,
    trade_no: (row) => <TableCell className="font-mono text-xs">{row.trade_no}</TableCell>,
    gateway: (row) => <TableCell className="whitespace-nowrap">{row.gateway_name || `#${row.gateway_id}`}</TableCell>,
    gateway_no: (row) => <TableCell className="font-mono text-xs text-muted-foreground">{row.gateway_no || '—'}</TableCell>,
    amount: (row) => <TableCell className="text-right tabular-nums">${row.amount}</TableCell>,
    fee: (row) => <TableCell className="text-right tabular-nums text-muted-foreground">${Number(row.fee || 0).toFixed(2)}</TableCell>,
    discount: (row) => (
      <TableCell className="text-right tabular-nums text-muted-foreground">
        {row.discount > 0 ? money(row.discount, row.order_currency) : '—'}
      </TableCell>
    ),
    order_amount: (row) => (
      <TableCell className="whitespace-nowrap text-right tabular-nums">{money(row.order_amount, row.order_currency)}</TableCell>
    ),
    // 历史订单未记录汇率(0)时显示 —,避免误导
    exchange_rate: (row) => (
      <TableCell className="text-right tabular-nums text-muted-foreground">{row.exchange_rate > 0 ? row.exchange_rate : '—'}</TableCell>
    ),
    quota: (row) => (
      <TableCell className="text-right tabular-nums text-muted-foreground">{Number(row.quota || 0).toLocaleString()}</TableCell>
    ),
    status: (row) => (
      <TableCell>
        <OrderStatus status={row.status} t={t} />
      </TableCell>
    )
  };
  const RIGHT = new Set(['amount', 'fee', 'discount', 'order_amount', 'exchange_rate', 'quota']);

  let body;
  if (!loaded) {
    body = narrow ? <ListSkeleton layout="list" rows={6} /> : <ListSkeleton cells={[96, 160, 64, 80, 64]} checkbox={false} />;
  } else if (orderList.length === 0) {
    body = <ListEmptyState title={t('common.noData', { defaultValue: 'No data' })} />;
  } else if (narrow) {
    // Phone rows (56px): order number and amount on top; user · time and the status below.
    body = (
      <ul className="divide-y divide-border">
        {orderList.map((row) => (
          <li
            key={row.id}
            className={cn('flex h-14 flex-col justify-center px-4', ROW_OPENS, row.trade_no === route.openId && 'bg-muted/60')}
            {...rowOpenProps(() => route.open(row.trade_no))}
          >
            <div className="flex items-baseline gap-3">
              <span className="min-w-0 truncate font-mono text-xs">{row.trade_no}</span>
              <span className="ml-auto shrink-0 text-sm font-medium tabular-nums">${row.amount}</span>
            </div>
            <div className="mt-0.5 flex items-baseline gap-3 text-xs text-muted-foreground">
              <span className="min-w-0 truncate">
                {user(row)} · {relativeTime(rtf, row.created_at)}
              </span>
              <span className="ml-auto shrink-0">
                <OrderStatus status={row.status} t={t} />
              </span>
            </div>
          </li>
        ))}
      </ul>
    );
  } else {
    body = (
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            {visible.map((c) => (
              <TableHead key={c.id} className={cn('whitespace-nowrap', RIGHT.has(c.id) && 'text-right')}>
                {head[c.id] || t(c.labelKey)}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {orderList.map((row) => (
            <TableRow
              key={row.id}
              className={ROW_OPENS}
              data-state={row.trade_no === route.openId ? 'selected' : undefined}
              {...rowOpenProps(() => route.open(row.trade_no))}
            >
              {visible.map((c) => (
                <Fragment key={c.id}>{cell[c.id](row)}</Fragment>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    );
  }

  return (
    <div className="space-y-3">
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

      <div className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <StatusSegments chips={narrow} value={status} onChange={changeStatus} options={segmentOptions} counts={statusCounts} />
        </div>
        {!narrow && <ColumnPicker columns={columns} />}
      </div>

      {/* 全宽 chips 容器(chips/「+」/Clear);仅在有筛选条件时渲染。 */}
      {activeChipFields.length > 0 && (
        <FilterBar
          fields={ORDER_FILTER_FIELDS}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          triggerVariant="plus"
          clearAlignEnd
          t={t}
          className="w-full rounded-lg border border-input bg-field px-3 py-2"
        />
      )}

      <Card className="overflow-hidden">
        {body}
        <div className={cn('border-t border-border', searching && loaded && 'opacity-60')}>
          <ListFooter
            total={listCount}
            page={page}
            pageSize={rowsPerPage}
            onPageChange={setPage}
            onPageSizeChange={(n) => {
              setRowsPerPage(n);
              savePageSize('paymentOrder', n);
              setPage(0);
            }}
          />
        </div>
      </Card>

      {route.openId && <OrderPanel key={route.openId} tradeNo={route.openId} rows={orderList} onClose={route.close} />}
    </div>
  );
}
