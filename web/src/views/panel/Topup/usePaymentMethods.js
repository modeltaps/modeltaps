import { useCallback, useEffect, useState } from 'react';

import { API } from 'utils/api';

// 账单页余额卡与充值弹层共用同一份支付渠道列表:列表为空(管理端一个网关都没配)
// 时余额卡不显示「充值」按钮,改为展示「暂未开放在线充值」说明。
export default function usePaymentMethods() {
  const [payment, setPayment] = useState([]);
  const [selectedPayment, setSelectedPayment] = useState(null);
  const [loaded, setLoaded] = useState(false);

  const reload = useCallback(async () => {
    try {
      const res = await API.get('/api/user/payment');
      const { success, data } = res.data;
      if (success && Array.isArray(data) && data.length > 0) {
        const list = [...data].sort((a, b) => b.sort - a.sort);
        setPayment(list);
        setSelectedPayment(list[0]);
      } else {
        setPayment([]);
        setSelectedPayment(null);
      }
    } catch (error) {
      // handled by interceptor
    } finally {
      setLoaded(true);
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  return { payment, selectedPayment, setSelectedPayment, loaded, reload };
}
