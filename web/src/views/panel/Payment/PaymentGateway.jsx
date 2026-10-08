import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Plus, RotateCcw, MoreHorizontal, Pencil, Trash2, ChevronLeft, ChevronRight } from 'lucide-react';

import { timestamp2string } from 'utils/common';
import { getPageSize, savePageSize, PAGE_SIZE_OPTIONS } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator
} from '@/components/ui/dropdown-menu';
import { toast } from '@/components/ui/sonner';
import { FilterBar, filterStateToParams, isEntryActive, paramsToFilterState } from '@/components/filter-bar';

import PaymentIcon from './PaymentIcon';
import PaymentSheet from './PaymentSheet';
import ConfirmDialog from './ConfirmDialog';
import { fetchGateways, managePayment, gatewayTypeLabel } from './paymentApi';
import { PaymentType } from './paymentConfig';

// FilterBar 字段:name/uuid 文本 + type 单选 enum(标量,后端按类型精确过滤)。
const GATEWAY_FILTER_FIELDS = [
  { key: 'name', labelKey: 'paymentGatewayPage.tableHeaders.name', type: 'text' },
  { key: 'uuid', labelKey: 'paymentGatewayPage.tableHeaders.uuid', type: 'text' },
  {
    key: 'type',
    labelKey: 'paymentGatewayPage.tableHeaders.type',
    type: 'enum',
    single: true,
    supportsExclude: false,
    paramInclude: 'type',
    options: Object.entries(PaymentType).map(([value, labelKey]) => ({ value, labelKey }))
  }
];
const GATEWAY_FILTER_PARAM_KEYS = ['name', 'uuid', 'type'];

function SortCell({ value, onCommit, t }) {
  const [val, setVal] = useState(value);
  return (
    <Input
      type="number"
      min="0"
      value={val}
      onChange={(e) => setVal(e.target.value)}
      onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
      onBlur={() => {
        const num = parseInt(val, 10);
        if (isNaN(num) || num === value) return;
        if (num < 0) {
          toast.error(t('payment_row.sortTip'));
          setVal(value);
          return;
        }
        onCommit(num);
      }}
      className="h-8 w-16 px-2"
    />
  );
}

export default function PaymentGateway() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('paymentGateway'));
  const [order, setOrder] = useState('desc');
  const [orderBy, setOrderBy] = useState('created_at');
  const [listCount, setListCount] = useState(0);
  const [payment, setPayment] = useState([]);
  const [loading, setLoading] = useState(false);
  const [refreshFlag, setRefreshFlag] = useState(false);

  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(GATEWAY_FILTER_FIELDS, searchParams));

  const [sheetOpen, setSheetOpen] = useState(false);
  const [editId, setEditId] = useState(0);
  const [confirm, setConfirm] = useState({ open: false, title: '', content: '', onConfirm: null });

  // reqId 守卫(UX-13):快速切页/切筛选时丢弃过期响应;paymentApi.js 结构保留,不迁 usePaginatedList。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  const fetchData = useCallback(
    () =>
      runGuardedFetch(
        guardRef.current,
        () => fetchGateways(page, rowsPerPage, filterStateToParams(GATEWAY_FILTER_FIELDS, filterState), order, orderBy),
        {
          onStart: () => setLoading(true),
          onResult: (data) => {
            if (data) {
              setListCount(data.total_count);
              setPayment(data.data || []);
            }
          },
          onFinally: () => setLoading(false)
        }
      ),
    [page, rowsPerPage, filterState, order, orderBy]
  );

  useEffect(() => {
    fetchData();
  }, [fetchData, refreshFlag]);

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        GATEWAY_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(GATEWAY_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        return params;
      },
      { replace: true }
    );
  }, [filterState, setSearchParams]);

  const doRefresh = () => {
    setOrder('desc');
    setOrderBy('created_at');
    setFilterState({});
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

  // 已生效的筛选字段(仅用于判断是否渲染标题行下方的 chips 容器),对齐 Log / PaymentOrder 页。
  const activeChipFields = useMemo(() => GATEWAY_FILTER_FIELDS.filter((f) => isEntryActive(f, filterState[f.key])), [filterState]);

  const onSortClick = (colId) => {
    const isAsc = orderBy === colId && order === 'asc';
    setOrder(isAsc ? 'desc' : 'asc');
    setOrderBy(colId);
    setPage(0);
  };

  const runManage = async (id, action, value) => {
    const { success, message } = await managePayment(id, action, value);
    if (success) {
      toast.success(t('userPage.operationSuccess'));
      setRefreshFlag((f) => !f);
    } else if (message) {
      toast.error(message);
    }
    return success;
  };

  const onDelete = (row) => {
    setConfirm({
      open: true,
      title: t('payment_row.delPayment'),
      content: `${t('payment_row.delPaymentTip')} ${row.name}？`,
      onConfirm: () => runManage(row.id, 'delete')
    });
  };

  const totalPages = Math.max(1, Math.ceil(listCount / rowsPerPage));

  const sortableCols = ['id', 'uuid', 'type', 'sort', 'enable', 'created_at'];
  const headers = [
    { id: 'id', label: t('paymentGatewayPage.tableHeaders.id') },
    { id: 'uuid', label: t('paymentGatewayPage.tableHeaders.uuid') },
    { id: 'name', label: t('paymentGatewayPage.tableHeaders.name') },
    { id: 'type', label: t('paymentGatewayPage.tableHeaders.type') },
    { id: 'icon', label: t('paymentGatewayPage.tableHeaders.icon') },
    { id: 'fixed_fee', label: t('paymentGatewayPage.tableHeaders.fixedFee') },
    { id: 'percent_fee', label: t('paymentGatewayPage.tableHeaders.percentFee') },
    { id: 'sort', label: t('paymentGatewayPage.tableHeaders.sort') },
    { id: 'enable', label: t('paymentGatewayPage.tableHeaders.enable') },
    { id: 'created_at', label: t('paymentGatewayPage.tableHeaders.createdAt') },
    { id: 'action', label: t('paymentGatewayPage.tableHeaders.action') }
  ];

  return (
    <div className="space-y-4">
      {/* Setting 页内嵌章节:本地标题行复刻头部版式——左标题,右侧刷新(图标) → 筛选 → 新建。 */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-xl font-semibold">{t('paymentGatewayPage.title')}</h2>
        <div className="flex flex-wrap items-center gap-2">
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="sm" onClick={doRefresh} aria-label={t('paymentGatewayPage.refreshClear')}>
                  <RotateCcw className={loading ? 'size-4 animate-spin' : 'size-4'} />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('paymentGatewayPage.refreshClear')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <FilterBar
            fields={GATEWAY_FILTER_FIELDS}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            hideChips
            t={t}
          />
          <Button
            onClick={() => {
              setEditId(0);
              setSheetOpen(true);
            }}
          >
            <Plus className="size-4" /> {t('paymentGatewayPage.createPayment')}
          </Button>
        </div>
      </div>

      {/* 标题行下方:全宽 chips 容器(chips/「+」/Clear);仅在有筛选条件时渲染。 */}
      {activeChipFields.length > 0 && (
        <FilterBar
          fields={GATEWAY_FILTER_FIELDS}
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
                      className={`whitespace-nowrap ${canSort ? 'cursor-pointer select-none' : ''} ${h.id === 'action' ? 'sticky right-0 bg-card z-10 w-[1%]' : ''}`}
                    >
                      {h.label}
                      {orderBy === h.id ? (order === 'desc' ? ' ↓' : ' ↑') : ''}
                    </TableHead>
                  );
                })}
              </TableRow>
            </TableHeader>
            <TableBody>
              {payment.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={headers.length} className="h-24 text-center text-muted-foreground">
                    {loading ? '…' : t('common.noData', { defaultValue: 'No data' })}
                  </TableCell>
                </TableRow>
              ) : (
                payment.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="font-mono text-xs">{row.id}</TableCell>
                    <TableCell className="font-mono text-xs">{row.uuid}</TableCell>
                    <TableCell className="font-medium">{row.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{gatewayTypeLabel(row.type, t) || t('common.unknown')}</Badge>
                    </TableCell>
                    <TableCell>
                      <PaymentIcon icon={row.icon} size={24} />
                    </TableCell>
                    <TableCell>{row.fixed_fee}</TableCell>
                    <TableCell>{row.percent_fee}</TableCell>
                    <TableCell>
                      <SortCell value={row.sort} onCommit={(v) => runManage(row.id, 'sort', v)} t={t} />
                    </TableCell>
                    <TableCell>
                      <Switch checked={!!row.enable} onCheckedChange={() => runManage(row.id, 'status', !row.enable)} />
                    </TableCell>
                    <TableCell className="whitespace-nowrap text-sm">{timestamp2string(row.created_at)}</TableCell>
                    <TableCell className="sticky right-0 bg-card z-10">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" className="size-8" aria-label={t('common.actions')}>
                            <MoreHorizontal className="size-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent>
                          <DropdownMenuItem
                            onClick={() => {
                              setEditId(row.id);
                              setSheetOpen(true);
                            }}
                          >
                            <Pencil /> {t('common.edit')}
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem className="text-destructive" onClick={() => onDelete(row)}>
                            <Trash2 /> {t('common.delete')}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        {payment.length === 0 ? (
          <div className="md:hidden rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
            {loading ? '…' : t('common.noData', { defaultValue: 'No data' })}
          </div>
        ) : (
          <div className="md:hidden flex flex-col gap-3 p-3">
            {payment.map((row) => (
              <div key={row.id} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                <div className="mb-2 text-sm font-medium">{row.name}</div>
                <dl className="flex flex-col divide-y divide-border/60">
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.id')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono text-xs">{row.id}</dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.uuid')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono text-xs">{row.uuid}</dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.type')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <Badge variant="outline">{gatewayTypeLabel(row.type, t) || t('common.unknown')}</Badge>
                    </dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.icon')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <PaymentIcon icon={row.icon} size={24} />
                    </dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.fixedFee')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{row.fixed_fee}</dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.percentFee')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{row.percent_fee}</dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.sort')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <SortCell value={row.sort} onCommit={(v) => runManage(row.id, 'sort', v)} t={t} />
                    </dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.enable')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <Switch checked={!!row.enable} onCheckedChange={() => runManage(row.id, 'status', !row.enable)} />
                    </dd>
                  </div>
                  <div className="flex items-center justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      {t('paymentGatewayPage.tableHeaders.createdAt')}
                    </dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{timestamp2string(row.created_at)}</dd>
                  </div>
                </dl>
                <div className="mt-3 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-3">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="size-8" aria-label={t('common.actions')}>
                        <MoreHorizontal className="size-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent>
                      <DropdownMenuItem
                        onClick={() => {
                          setEditId(row.id);
                          setSheetOpen(true);
                        }}
                      >
                        <Pencil /> {t('common.edit')}
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem className="text-destructive" onClick={() => onDelete(row)}>
                        <Trash2 /> {t('common.delete')}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
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
                savePageSize('paymentGateway', Number(v));
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

      <PaymentSheet
        open={sheetOpen}
        paymentId={editId || 0}
        onClose={() => setSheetOpen(false)}
        onSaved={() => {
          setSheetOpen(false);
          setRefreshFlag((f) => !f);
        }}
      />

      <ConfirmDialog
        open={confirm.open}
        title={confirm.title}
        content={confirm.content}
        onCancel={() => setConfirm((c) => ({ ...c, open: false }))}
        onConfirm={() => {
          confirm.onConfirm?.();
          setConfirm((c) => ({ ...c, open: false }));
        }}
      />
    </div>
  );
}
