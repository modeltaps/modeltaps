import PropTypes from 'prop-types';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Navigate } from 'react-router';
import dayjs from 'dayjs';

import { Card, CardContent } from '@/components/ui/card';
import { API } from 'utils/api';
import { showError, useIsAdmin } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import { useOrg } from 'contexts/OrgContext';
import LogTable from '../Log/LogTable';
import LogPagination from '../Log/LogPagination';
import LogDetailDialog from '../Log/LogDetailDialog';
import { BILLING_LOG_TYPES } from '../Log/logHelpers';

// ==============================|| PANEL — LEDGER (账户流水) ||============================== //
// 账务记录专页:从 Log 的「账务记录」视图抽出(类型 1/3/4),只读复用 Log 子组件。
// 作为账单页「最近记录」的一个 Tab 使用:只有排序与分页,筛选/时间范围/导出去 /panel/log。
// 刷新由宿主通过 reloadToken 递增驱动。
// 仅个人/站点语义,无组织上下文;组织态直访重定向回 /panel/log。后端零改动。

// 账务类型(充值/管理/系统);'0' 是「全部」哨兵,不参与逐类查询。
const BILLING_TYPE_VALUES = BILLING_LOG_TYPES.map((o) => o.value).filter((v) => v !== '0');

// LogTable 列 id 全集;账务页固定精简列(时间/类型/额度/详情,+admin 用户列),
// 其余与 API 请求相关的列一律隐藏。
const ALL_COLUMN_IDS = [
  'created_at',
  'channel_id',
  'user_id',
  'member',
  'group',
  'token_name',
  'type',
  'model_name',
  'duration',
  'prompt_tokens',
  'completion_tokens',
  'quota',
  'source_ip',
  'detail'
];
const BILLING_COLUMN_IDS = new Set(['created_at', 'user_id', 'type', 'quota', 'detail']);
const BILLING_COLUMN_VISIBILITY = Object.fromEntries(ALL_COLUMN_IDS.map((id) => [id, BILLING_COLUMN_IDS.has(id)]));

// 固定时间窗口:当天零点 ~ now+1h 缓冲(对齐旧账户流水默认档位「今天」)。
const buildDefaultTimeRange = () => ({
  start_timestamp: dayjs().startOf('day').unix(),
  end_timestamp: dayjs().unix() + 3600
});

export default function Ledger({ reloadToken = 0 }) {
  const { t } = useTranslation();
  const userIsAdmin = useIsAdmin();
  const { userGroup } = useSelector((state) => state.account);

  // 账户流水仅个人/站点语义;组织上下文不暴露此页(nav hideInOrganization),
  // 直访亦重定向回 /panel/log(见末尾的早退守卫)。
  const { currentOrgId } = useOrg();
  const isOrgContext = Boolean(currentOrgId);

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('ledger'));
  const [order, setOrder] = useState('desc');
  const [orderBy, setOrderBy] = useState('created_at');

  const [logs, setLogs] = useState([]);
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);
  const [detailItem, setDetailItem] = useState(null);

  const dataReqIdRef = useRef(0);

  const fetchData = useCallback(
    async (pageArg, size, ord, ordBy) => {
      const reqId = ++dataReqIdRef.current;
      setSearching(true);
      try {
        const sortParam = ordBy ? (ord === 'desc' ? `-${ordBy}` : ordBy) : ordBy;
        const url = userIsAdmin ? '/api/log/' : '/api/log/self/';
        // 全部账务:后端 log_type 仅支持单值(0 会混入消费),按 1/3/4 各查一次后
        // 客户端归并。size 受后端 MaxRecentItems(100)限制,翻页越深窗口越窄。
        const base = buildDefaultTimeRange();
        const need = Math.min((pageArg + 1) * size, 100);
        const responses = await Promise.all(
          BILLING_TYPE_VALUES.map((tp) => API.get(url, { params: { page: 1, size: need, order: sortParam, ...base, log_type: tp } }))
        );
        if (reqId !== dataReqIdRef.current) return;
        const failed = responses.find((r) => !r.data?.success);
        if (failed) {
          showError(failed.data?.message);
        } else {
          const dir = ord === 'asc' ? 1 : -1;
          const field = ordBy || 'created_at';
          const merged = responses
            .flatMap((r) => r.data.data?.data || [])
            .sort((a, b) => ((a?.[field] ?? 0) > (b?.[field] ?? 0) ? dir : -dir));
          setListCount(responses.reduce((sum, r) => sum + (r.data.data?.total_count || 0), 0));
          setLogs(merged.slice(pageArg * size, pageArg * size + size));
        }
      } catch (error) {
        if (reqId !== dataReqIdRef.current) return;
        console.error(error);
      } finally {
        if (reqId === dataReqIdRef.current) setSearching(false);
      }
    },
    [userIsAdmin]
  );

  // 组织上下文不拉取数据(将被下方 Navigate 立即卸载,避免无谓请求)。
  // reloadToken 由宿主(账单页刷新按钮)递增,变化即重新拉取。
  useEffect(() => {
    if (isOrgContext) return;
    fetchData(page, rowsPerPage, order, orderBy);
  }, [isOrgContext, page, rowsPerPage, order, orderBy, fetchData, reloadToken]);

  const handleSort = (id) => {
    if (!id) return;
    const isAsc = orderBy === id && order === 'asc';
    setOrder(isAsc ? 'desc' : 'asc');
    setOrderBy(id);
  };

  // 组织上下文不提供账户流水,直访重定向回日志页(hooks 已在上方全部声明)。
  if (isOrgContext) return <Navigate to="/panel/log" replace />;

  return (
    <div className="space-y-6">
      <Card>
        <CardContent className="p-0">
          <LogTable
            t={t}
            data={logs}
            userIsAdmin={userIsAdmin}
            isOrgContext={false}
            userGroup={userGroup}
            order={order}
            orderBy={orderBy}
            onSort={handleSort}
            onRowDetail={setDetailItem}
            searching={searching}
            columnVisibility={BILLING_COLUMN_VISIBILITY}
            emptyInfo={t('logPage.ledgerEmptyInfo')}
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
                savePageSize('ledger', size);
              }}
            />
          </div>
        </CardContent>
      </Card>

      {detailItem && <LogDetailDialog item={detailItem} userGroup={userGroup} t={t} onClose={() => setDetailItem(null)} />}
    </div>
  );
}

Ledger.propTypes = {
  reloadToken: PropTypes.number
};
