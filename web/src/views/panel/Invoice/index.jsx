import PropTypes from 'prop-types';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Alert } from '@/components/ui/alert';
import { Card, CardContent } from '@/components/ui/card';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import Pagination from '../components/Pagination';
import InvoiceTable from './InvoiceTable';
import InvoiceDetailSheet from './InvoiceDetailSheet';

// ==============================|| PANEL — MONTHLY BILLING (INVOICE) ||============================== //
// shadcn/Tailwind port of views/Invoice. Server-side pagination + sorting.
// 作为账单页「最近记录」的一个 Tab 使用;刷新由宿主通过 reloadToken 递增驱动。

export default function Invoice({ reloadToken = 0 }) {
  const { t } = useTranslation();

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('invoice'));
  const [order, setOrder] = useState('desc');
  const [orderBy, setOrderBy] = useState('date');

  const [invoices, setInvoices] = useState([]);
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);
  const [detailDate, setDetailDate] = useState(null);

  const dataReqIdRef = useRef(0);

  const fetchData = useCallback(async (pageArg, size, ord, ordBy) => {
    const reqId = ++dataReqIdRef.current;
    setSearching(true);
    try {
      const sortParam = ordBy ? (ord === 'desc' ? `-${ordBy}` : ordBy) : ordBy;
      const res = await API.get('/api/user/invoice', { params: { page: pageArg + 1, size, order: sortParam } });
      if (reqId !== dataReqIdRef.current) return;
      const { success, message, data } = res.data;
      if (success) {
        setListCount(data.total_count);
        setInvoices(data.data || []);
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

  useEffect(() => {
    fetchData(page, rowsPerPage, order, orderBy);
  }, [page, rowsPerPage, order, orderBy, reloadToken, fetchData]);

  const handleSort = (id) => {
    if (!id) return;
    const isAsc = orderBy === id && order === 'asc';
    setOrder(isAsc ? 'desc' : 'asc');
    setOrderBy(id);
  };

  const handleView = (date) => {
    if (date) setDetailDate(date.substring(0, 7));
  };

  return (
    <div className="space-y-6">
      <Alert variant="info">{t('invoice_index.alert')}</Alert>

      <Card>
        <CardContent className="p-0">
          <InvoiceTable
            t={t}
            data={invoices}
            order={order}
            orderBy={orderBy}
            onSort={handleSort}
            onView={handleView}
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
              savePageSize('invoice', size);
            }}
          />
        </CardContent>
      </Card>

      {detailDate && <InvoiceDetailSheet date={detailDate} t={t} onClose={() => setDetailDate(null)} />}
    </div>
  );
}

Invoice.propTypes = {
  reloadToken: PropTypes.number
};
