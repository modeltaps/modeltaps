import PropTypes from 'prop-types';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';

import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { API } from 'utils/api';
import { renderQuota, showError, timestamp2string } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import Pagination from '../components/Pagination';

// ==============================|| PANEL — BILLING / TOP-UP ORDERS ||============================== //
// 用户自己的充值订单,数据来自 GET /api/user/order(会话强制 user_id)。固定按创建时间
// 倒序,不提供筛选/排序(个人订单量小,管理端 Payment/PaymentOrder 才需要筛选)。
// 渠道列:订单只记了 gateway_id,按 /api/user/payment 返回的 id 映射成渠道名,匹配
// 不到(渠道已删除等)时退化为「#<gateway_id>」。

const STATUS_VARIANTS = {
  pending: 'outline',
  success: 'default',
  failed: 'destructive',
  closed: 'secondary'
};

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
        { id: 'created_at', header: t('billingPage.orders.time'), cell: (o) => timestamp2string(o.created_at) },
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
          cell: (o) => (
            <Badge variant={STATUS_VARIANTS[o.status] || 'secondary'}>
              {t(`billingPage.orders.statuses.${o.status}`, { defaultValue: o.status })}
            </Badge>
          )
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

  return (
    <Card>
      <CardContent className="p-0">
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id}>
                  {hg.headers.map((h) => (
                    <TableHead key={h.id} className="whitespace-nowrap">
                      {flexRender(h.column.columnDef.header, h.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="h-24 text-center text-sm text-muted-foreground">
                    {searching ? '…' : t('billingPage.orders.empty')}
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id} className="whitespace-nowrap text-sm">
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        <DataCards
          table={table}
          className="md:hidden"
          primaryColumnId="trade_no"
          empty={t('billingPage.orders.empty')}
          searching={searching}
        />

        <Pagination
          page={page}
          rowsPerPage={rowsPerPage}
          count={listCount}
          onPageChange={setPage}
          onRowsPerPageChange={(size) => {
            setPage(0);
            setRowsPerPage(size);
            savePageSize('userOrder', size);
          }}
        />
      </CardContent>
    </Card>
  );
}

OrdersTab.propTypes = {
  payment: PropTypes.array,
  reloadToken: PropTypes.number
};
