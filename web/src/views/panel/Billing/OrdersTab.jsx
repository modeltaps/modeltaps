import PropTypes from 'prop-types';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';

import { Card } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ListFooter, useListLayout } from '@/components/list-page';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { renderQuota, showError, timestamp2string, timestamp2stringCompact } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';

// ==============================|| PANEL — BILLING / TOP-UP ORDERS ||============================== //
// 用户自己的充值订单,数据来自 GET /api/user/order(会话强制 user_id)。固定按创建时间
// 倒序,不提供筛选/排序(个人订单量小,管理端 Payment/PaymentOrder 才需要筛选)。
// 渠道列:订单只记了 gateway_id,按 /api/user/payment 返回的 id 映射成渠道名,匹配
// 不到(渠道已删除等)时退化为「#<gateway_id>」。

// Status as quiet text; only a failed payment is tinted.
const STATUS_CLASSES = { pending: 'text-muted-foreground', success: '', failed: 'text-destructive', closed: 'text-muted-foreground' };

function StatusText({ t, status }) {
  return (
    <span className={cn('whitespace-nowrap', STATUS_CLASSES[status] ?? 'text-muted-foreground')}>
      {t(`billingPage.orders.statuses.${status}`, { defaultValue: status })}
    </span>
  );
}

StatusText.propTypes = { t: PropTypes.func.isRequired, status: PropTypes.string };

export default function OrdersTab({ payment = [], reloadToken = 0 }) {
  const { t } = useTranslation();

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('userOrder'));
  const [orders, setOrders] = useState([]);
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);

  const dataReqIdRef = useRef(0);

  const fetchData = useCallback(async (pageArg, size) => {
    const reqId = ++dataReqIdRef.current;
    setSearching(true);
    try {
      const res = await API.get('/api/user/order', { params: { page: pageArg + 1, size, order: '-created_at' } });
      if (reqId !== dataReqIdRef.current) return;
      const { success, message, data } = res.data;
      if (success) {
        setListCount(data.total_count);
        setOrders(data.data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      if (reqId !== dataReqIdRef.current) return;
      console.error(error);
    } finally {
      if (reqId === dataReqIdRef.current) setSearching(false);
    }
  }, []);

  // reloadToken 由宿主(账单页刷新按钮)递增,变化即重新拉取。
  useEffect(() => {
    fetchData(page, rowsPerPage);
  }, [page, rowsPerPage, reloadToken, fetchData]);

  const gatewayNames = useMemo(() => {
    const map = {};
    payment.forEach((p) => {
      if (p?.id) map[p.id] = p.name;
    });
    return map;
  }, [payment]);

  const columns = useMemo(
    () =>
      [
        {
          id: 'created_at',
          header: t('billingPage.orders.time'),
          cell: (o) => <span title={timestamp2string(o.created_at)}>{timestamp2stringCompact(o.created_at)}</span>
        },
        { id: 'trade_no', header: t('billingPage.orders.tradeNo'), cell: (o) => <span className="font-mono text-xs">{o.trade_no}</span> },
        {
          id: 'order_amount',
          header: t('billingPage.orders.payAmount'),
          cell: (o) => `${o.order_amount} ${o.order_currency || ''}`.trim()
        },
        { id: 'quota', header: t('billingPage.orders.quota'), cell: (o) => renderQuota(o.quota) },
        { id: 'gateway_id', header: t('billingPage.orders.gateway'), cell: (o) => gatewayNames[o.gateway_id] || `#${o.gateway_id}` },
        {
          id: 'status',
          header: t('billingPage.orders.status'),
          cell: (o) => <StatusText t={t} status={o.status} />
        }
      ].map((c) => ({
        id: c.id,
        header: () => c.header,
        cell: ({ row }) => c.cell(row.original)
      })),
    [t, gatewayNames]
  );

  const table = useTable({
    features: appTableFeatures,
    data: orders,
    columns
  });

  // Full table while it fits (the sidebar collapses first), 56px rows on phones.
  const { narrow } = useListLayout();
  const amount = (o) => `${o.order_amount} ${o.order_currency || ''}`.trim();

  let body;
  if (orders.length === 0) {
    body = <p className="px-4 py-10 text-center text-sm text-muted-foreground">{searching ? '…' : t('billingPage.orders.empty')}</p>;
  } else if (narrow) {
    // Phone row: amount paid and status, then the credited quota, time and order number.
    body = (
      <ul className="divide-y divide-border">
        {orders.map((o) => (
          <li key={o.id || o.trade_no} className="flex h-14 items-center gap-3 px-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="min-w-0 truncate text-sm font-medium tabular-nums">{amount(o)}</span>
                <span className="ml-auto shrink-0 text-xs">
                  <StatusText t={t} status={o.status} />
                </span>
              </div>
              <div className="mt-0.5 truncate text-xs text-muted-foreground">
                {renderQuota(o.quota)} · {timestamp2stringCompact(o.created_at)} · <span className="font-mono">{o.trade_no}</span>
              </div>
            </div>
          </li>
        ))}
      </ul>
    );
  } else {
    body = (
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((hg) => (
            <TableRow key={hg.id} className="hover:bg-transparent">
              {hg.headers.map((h) => (
                <TableHead key={h.id} className="whitespace-nowrap">
                  {flexRender(h.column.columnDef.header, h.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.map((row) => (
            <TableRow key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <TableCell key={cell.id} className="whitespace-nowrap text-sm">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    );
  }

  return (
    <div>
      <Card>
        {body}
        <div className="border-t border-border">
          <ListFooter
            total={listCount}
            page={page}
            pageSize={rowsPerPage}
            onPageChange={setPage}
            onPageSizeChange={(size) => {
              setPage(0);
              setRowsPerPage(size);
              savePageSize('userOrder', size);
            }}
          />
        </div>
      </Card>
    </div>
  );
}

OrdersTab.propTypes = {
  payment: PropTypes.array,
  reloadToken: PropTypes.number
};
