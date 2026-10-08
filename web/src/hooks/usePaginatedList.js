import { useCallback, useEffect, useRef, useState } from 'react';

import { getPageSize, savePageSize } from 'constants';
import { showError } from 'utils/common';
import { createRequestGuard, runGuardedFetch } from './paginatedListGuard';

// ==============================|| 分页列表共享 hook(UX-13 竞态根治) ||============================== //
// 封装 page/rowsPerPage/order/orderBy/refreshFlag/searching 状态 + 守卫化 fetch 生命周期。
// fetcher 由调用方注入(保留各页 URL/参数清洗差异),须返回后端响应体 res.data
// ({ success, message, data: { total_count, data }, ... });成功时 hook 统一落
// listCount/rows,额外字段(如 today_usage)经 onSuccess 回调交回调用方,且仅对最新请求触发。

export default function usePaginatedList({
  pageSizeKey,
  fetcher,
  onSuccess,
  defaultOrder = 'desc',
  defaultOrderBy = 'id',
  resetPageOnSort = false,
  resetSortOnRefresh = true,
  resetPageOnRefresh = false,
  deps = []
}) {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize(pageSizeKey));
  const [order, setOrder] = useState(defaultOrder);
  const [orderBy, setOrderBy] = useState(defaultOrderBy);
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);
  const [rows, setRows] = useState([]);
  const [refreshFlag, setRefreshFlag] = useState(false);

  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  // fetcher/onSuccess 走 ref:调用方通常内联定义,identity 每轮变化;真正的重拉时机由
  // page/rowsPerPage/order/orderBy/refreshFlag/deps 显式驱动(与迁移前各页 useEffect 依赖一致)。
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;
  const onSuccessRef = useRef(onSuccess);
  onSuccessRef.current = onSuccess;

  const fetchData = useCallback(
    () =>
      runGuardedFetch(guardRef.current, () => fetcherRef.current({ page, rowsPerPage, order, orderBy }), {
        onStart: () => setSearching(true),
        onResult: (payload) => {
          const { success, message, data } = payload;
          if (success) {
            setListCount(data.total_count);
            setRows(data.data ?? []);
            onSuccessRef.current?.(payload);
          } else {
            showError(message);
          }
        },
        onError: (error) => console.error(error),
        onFinally: () => setSearching(false)
      }),
    [page, rowsPerPage, order, orderBy]
  );

  useEffect(() => {
    fetchData();
    // deps 为调用方声明的额外重拉输入(如 filterState/currentOrgId),长度在各页恒定。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchData, refreshFlag, ...deps]);

  // 仅触发重拉,不动分页/排序(供保存回调、行操作后局部刷新使用)。
  const refresh = () => setRefreshFlag((f) => !f);

  const handleRefresh = () => {
    if (resetSortOnRefresh) {
      setOrder(defaultOrder);
      setOrderBy(defaultOrderBy);
    }
    if (resetPageOnRefresh) setPage(0);
    refresh();
  };

  const handleSort = (id) => {
    if (!id) return;
    const isAsc = orderBy === id && order === 'asc';
    setOrder(isAsc ? 'desc' : 'asc');
    setOrderBy(id);
    if (resetPageOnSort) setPage(0);
  };

  const onRowsPerPageChange = (n) => {
    setRowsPerPage(n);
    setPage(0);
    savePageSize(pageSizeKey, n);
  };

  return {
    page,
    setPage,
    rowsPerPage,
    order,
    orderBy,
    listCount,
    searching,
    rows,
    setRows,
    refresh,
    handleRefresh,
    handleSort,
    onRowsPerPageChange
  };
}
