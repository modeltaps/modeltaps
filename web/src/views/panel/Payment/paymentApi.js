import dayjs from 'dayjs';

import { API } from 'utils/api';
import { trims } from 'utils/common';
import { toast } from '@/components/ui/sonner';
import { PaymentType } from './paymentConfig';

// Data + mutation helpers for the Payment admin page. Ported from the v1
// `views/Payment/Gateway.jsx` and `views/Payment/Order.jsx` so the API
// contract stays identical.

export const GATEWAY_KEYWORD = {
  type: '',
  name: '',
  uuid: '',
  currency: ''
};

export const ORDER_KEYWORD = () => ({
  user_id: '',
  trade_no: '',
  status: '',
  gateway_id: '',
  gateway_no: '',
  start_timestamp: 0,
  end_timestamp: dayjs().unix() + 3600
});

// Order status metadata. Ported from v1 component/OrderTableRow.jsx StatusType.
export const ORDER_STATUS = {
  pending: { labelKey: 'billingPage.orders.statuses.pending', value: 'pending', variant: 'outline' },
  success: { labelKey: 'billingPage.orders.statuses.success', value: 'success', variant: 'default' },
  failed: { labelKey: 'billingPage.orders.statuses.failed', value: 'failed', variant: 'destructive' },
  closed: { labelKey: 'billingPage.orders.statuses.closed', value: 'closed', variant: 'secondary' }
};

export function gatewayTypeLabel(type, t) {
  return PaymentType?.[type] ? t(PaymentType[type]) : type || '';
}

export async function fetchGateways(page, rowsPerPage, keyword, order, orderBy) {
  try {
    const params = { page: page + 1, size: rowsPerPage, ...trims(keyword) };
    if (orderBy) params.order = order === 'desc' ? '-' + orderBy : orderBy;
    delete params._timestamp;
    const res = await API.get('/api/payment/', { params });
    const { success, message, data } = res.data;
    if (success) return data;
    toast.error(message);
  } catch (error) {
    console.error(error);
  }
  return false;
}

// Mirrors v1 managePayment for the subset of actions the page needs.
export async function managePayment(id, action, value) {
  const url = '/api/payment/';
  const data = { id };
  let res;
  try {
    switch (action) {
      case 'delete':
        res = await API.delete(url + id);
        break;
      case 'status':
        res = await API.put(url, { ...data, enable: value });
        break;
      case 'sort':
        res = await API.put(url, { ...data, sort: value });
        break;
      default:
        return { success: false, message: 'invalid action' };
    }
    return res.data;
  } catch (error) {
    return { success: false, message: error.message };
  }
}

export async function fetchOrders(page, rowsPerPage, keyword, order, orderBy) {
  try {
    const params = { page: page + 1, size: rowsPerPage, ...trims(keyword) };
    if (orderBy) params.order = order === 'desc' ? '-' + orderBy : orderBy;
    delete params._timestamp;
    const res = await API.get('/api/payment/order', { params });
    const { success, message, data } = res.data;
    if (success) return data;
    toast.error(message);
  } catch (error) {
    console.error(error);
  }
  return false;
}
