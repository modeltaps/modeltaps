import PropTypes from 'prop-types';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Navigate } from 'react-router';
import dayjs from 'dayjs';

import { Card } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ROW_OPENS, rowOpenProps } from '@/components/objects';
import { ListFooter, useListLayout } from '@/components/list-page';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { calculateQuota, showError, timestamp2string, timestamp2stringCompact, useIsAdmin } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import { useOrg } from 'contexts/OrgContext';
import LogDetailDialog from '../Log/LogDetailDialog';
import { BILLING_LOG_TYPES, logTypeMeta } from '../Log/logHelpers';

// ==============================|| PANEL — LEDGER (账户流水) ||============================== //
// 账务记录专页:从 Log 的「账务记录」视图抽出(类型 1/3/4),只读复用 Log 子组件。
// 作为账单页「最近记录」的一个 Tab 使用:只有排序与分页,筛选/时间范围/导出去 /panel/log。
// 刷新由宿主通过 reloadToken 递增驱动。
// 仅个人/站点语义,无组织上下文;组织态直访重定向回 /panel/log。后端零改动。

// 账务类型(充值/管理/系统);'0' 是「全部」哨兵,不参与逐类查询。
const BILLING_TYPE_VALUES = BILLING_LOG_TYPES.map((o) => o.value).filter((v) => v !== '0');

// Ledger columns, listed explicitly (time, type, description, amount; plus the user for a site
// admin), so columns added to the request log never leak in here.

// Signed amount in one format: "+$10.00" in, "−$2.00" out, two decimals. Points mode shows the
// raw quota with its sign.
export function signedAmount(quota) {
  const q = Number(quota) || 0;
  const sign = q < 0 ? '−' : '+';
  if (localStorage.getItem('display_in_currency') !== 'true') return sign + Math.abs(q).toLocaleString();
  return sign + '$' + Math.abs(Number(calculateQuota(q, 6))).toFixed(2);
}

function TypeLabel({ t, type }) {
  const meta = logTypeMeta(type);
  return <span className="whitespace-nowrap text-muted-foreground">{meta ? t(meta.labelKey) : '—'}</span>;
}

TypeLabel.propTypes = { t: PropTypes.func.isRequired, type: PropTypes.number };

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
  // Newest first; the ledger has no sorting of its own.
  const order = 'desc';
  const orderBy = 'created_at';

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

  // 组织上下文不提供账户流水,直访重定向回日志页(hooks 已在上方全部声明)。
  if (isOrgContext) return <Navigate to="/panel/log" replace />;

  return (
    <LedgerView
      t={t}
      logs={logs}
      searching={searching}
      userIsAdmin={userIsAdmin}
      page={page}
      rowsPerPage={rowsPerPage}
      listCount={listCount}
      onPageChange={setPage}
      onRowsPerPageChange={(size) => {
        setPage(0);
        setRowsPerPage(size);
        savePageSize('ledger', size);
      }}
      onOpen={setDetailItem}
      detail={detailItem && <LogDetailDialog item={detailItem} userGroup={userGroup} t={t} onClose={() => setDetailItem(null)} />}
    />
  );
}

// The ledger itself: a four-column table while it fits, 56px rows on phones (description and
// amount, then type and time). A row opens the entry's detail.
function LedgerView({ t, logs, searching, userIsAdmin, page, rowsPerPage, listCount, onPageChange, onRowsPerPageChange, onOpen, detail }) {
  const { narrow } = useListLayout();
  const empty = <p className="px-4 py-10 text-center text-sm text-muted-foreground">{searching ? '…' : t('logPage.ledgerEmptyInfo')}</p>;
  const amountClass = (q) => cn('whitespace-nowrap font-mono tabular-nums', Number(q) < 0 && 'text-muted-foreground');

  let body;
  if (logs.length === 0) body = empty;
  else if (narrow) {
    body = (
      <ul className="divide-y divide-border">
        {logs.map((i) => (
          <li key={i.id} className={cn('flex h-14 items-center gap-3 px-4', ROW_OPENS)} {...rowOpenProps(() => onOpen(i))}>
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="min-w-0 truncate text-sm">{i.content || '—'}</span>
                <span className={cn('ml-auto shrink-0 text-sm', amountClass(i.quota))}>{signedAmount(i.quota)}</span>
              </div>
              <div className="mt-0.5 truncate text-xs text-muted-foreground">
                <TypeLabel t={t} type={i.type} /> · {timestamp2stringCompact(i.created_at)}
                {userIsAdmin && i.username ? ` · ${i.username}` : ''}
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
          <TableRow className="hover:bg-transparent">
            <TableHead className="whitespace-nowrap">{t('logPage.timeLabel')}</TableHead>
            <TableHead className="whitespace-nowrap">{t('logPage.typeLabel')}</TableHead>
            <TableHead className="w-full whitespace-nowrap">{t('ledgerList.description')}</TableHead>
            {userIsAdmin && <TableHead className="whitespace-nowrap">{t('logPage.userLabel')}</TableHead>}
            <TableHead className="whitespace-nowrap text-right">{t('ledgerList.amount')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.map((i) => (
            <TableRow key={i.id} className={ROW_OPENS} {...rowOpenProps(() => onOpen(i))}>
              <TableCell className="whitespace-nowrap" title={timestamp2string(i.created_at)}>
                {timestamp2stringCompact(i.created_at)}
              </TableCell>
              <TableCell>
                <TypeLabel t={t} type={i.type} />
              </TableCell>
              <TableCell className="max-w-0">
                <span className="block truncate">{i.content || '—'}</span>
              </TableCell>
              {userIsAdmin && <TableCell className="whitespace-nowrap">{i.username || '—'}</TableCell>}
              <TableCell className={cn('text-right', amountClass(i.quota))}>{signedAmount(i.quota)}</TableCell>
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
            onPageChange={onPageChange}
            onPageSizeChange={onRowsPerPageChange}
          />
        </div>
      </Card>
      {detail}
    </div>
  );
}

LedgerView.propTypes = {
  t: PropTypes.func.isRequired,
  logs: PropTypes.array.isRequired,
  searching: PropTypes.bool,
  userIsAdmin: PropTypes.bool,
  page: PropTypes.number.isRequired,
  rowsPerPage: PropTypes.number.isRequired,
  listCount: PropTypes.number.isRequired,
  onPageChange: PropTypes.func.isRequired,
  onRowsPerPageChange: PropTypes.func.isRequired,
  onOpen: PropTypes.func.isRequired,
  detail: PropTypes.node
};

Ledger.propTypes = {
  reloadToken: PropTypes.number
};
