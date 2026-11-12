import PropTypes from 'prop-types';
import { useMemo } from 'react';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { AlertTriangle, ArrowDown, ArrowUp, ArrowUpDown, HelpCircle } from 'lucide-react';
import { Link } from 'react-router';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import ModelIcon from '@/components/brand/ModelIcon';
import ProviderIcon from '@/components/brand/ProviderIcon';
import AppFavicon from './AppFavicon';
import { cn } from '@/lib/utils';
import { ROW_OPENS, rowOpenProps } from '@/components/objects';
import { useNaturalWidth } from '@/components/list-page';
import { SpendAmount, timestamp2string, timestamp2stringCompact } from 'utils/common';
import { badgeClass, calculateOriginalQuota, deriveDuration, finishReasonMeta, logTypeMeta, relayModeMeta } from './logHelpers';

const Badge = ({ color, children }) => <span className={badgeClass(color)}>{children}</span>;
Badge.propTypes = { color: PropTypes.string, children: PropTypes.node };

// A plain text link (user, key, group): underline on hover; stopPropagation so it does not open
// the row's detail; a react-router Link keeps cmd/ctrl+click for a new tab.
const TextLink = ({ to, children }) => (
  <Link to={to} onClick={(e) => e.stopPropagation()} className="whitespace-nowrap underline-offset-2 hover:underline">
    {children}
  </Link>
);
TextLink.propTypes = { to: PropTypes.string, children: PropTypes.node };

// Typical column widths, for the layout estimate before the table has been measured.
const COLUMN_WIDTHS = {
  created_at: 104,
  channel_id: 160,
  user_id: 110,
  member: 120,
  group: 96,
  token_name: 130,
  app: 120,
  type: 72,
  model_name: 210,
  relay_mode: 80,
  finish_reason: 90,
  duration: 110,
  throughput: 96,
  prompt_tokens: 80,
  completion_tokens: 80,
  quota: 90,
  source_ip: 120
};
export const estimateLogTableWidth = (columnIds, visibility) =>
  columnIds.filter((id) => visibility[id] !== false).reduce((sum, id) => sum + (COLUMN_WIDTHS[id] || 100), 0);

function QuotaCell({ item }) {
  if (item.type !== 2)
    return item.quota ? (
      <SpendAmount quota={item.quota} className="font-mono tabular-nums" />
    ) : (
      <span className="font-mono tabular-nums">$0</span>
    );
  const groupRatio = item.metadata?.group_ratio || 1;
  const quota = item.quota || 0;
  if (groupRatio < 1) {
    return (
      <div className="flex flex-col items-end leading-tight">
        <SpendAmount quota={calculateOriginalQuota(item)} className="font-mono tabular-nums text-xs text-muted-foreground line-through" />
        <SpendAmount quota={quota} className="font-mono tabular-nums font-medium text-foreground" />
      </div>
    );
  }
  return <SpendAmount quota={quota} className="font-mono tabular-nums font-medium text-foreground" />;
}
QuotaCell.propTypes = { item: PropTypes.object };

// Duration as text: total, then the time to first token muted.
function DurationCell({ item }) {
  const { requestTimeStr, firstTimeStr } = deriveDuration(item);
  if (item.request_time === 0) return <span className="text-muted-foreground">—</span>;
  return (
    <span className="whitespace-nowrap tabular-nums">
      {requestTimeStr}
      {firstTimeStr && <span className="text-muted-foreground"> / {firstTimeStr}</span>}
    </span>
  );
}
DurationCell.propTypes = { item: PropTypes.object };

// App attribution cell (metadata.app_name/app_domain, populated by W9-B). No attribution
// → a muted em dash. app_name is X-Title (or Referer domain); domain drives the favicon.
function AppCell({ item }) {
  const m = item.metadata || {};
  const name = m.app_name;
  const domain = m.app_domain;
  if (!name && !domain) return <span className="text-muted-foreground/40">—</span>;
  return (
    <span className="inline-flex items-center gap-1.5">
      <AppFavicon name={name} domain={domain} />
      <span className="whitespace-nowrap">{name || domain}</span>
    </span>
  );
}
AppCell.propTypes = { item: PropTypes.object };

// 时间列:紧凑 MM-DD HH:mm,悬浮 Tooltip 显示完整 YYYY-MM-DD HH:mm:ss。
function TimeCell({ ts }) {
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="whitespace-nowrap">{timestamp2stringCompact(ts)}</span>
        </TooltipTrigger>
        <TooltipContent className="text-xs">{timestamp2string(ts)}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
TimeCell.propTypes = { ts: PropTypes.number };

// 输入/输出 token 数:数字 + 弱化色 " tok" 单位;空值(null/undefined)保持空,
// 绝不渲染成 "undefined tok" 或 " tok"。
function TokenCell({ value }) {
  if (value === null || value === undefined || value === '') return '';
  return <span className="whitespace-nowrap font-mono tabular-nums">{Number(value).toLocaleString()}</span>;
}
TokenCell.propTypes = { value: PropTypes.oneOfType([PropTypes.number, PropTypes.string]) };

// 吞吐(tok/s):口径与 LogDetailDialog 的 requestTs 完全一致(复用 deriveDuration)。
// 数据不足(无首字时延或无输出 token)时 requestTs=0 → 显示空;并防御非有限值,杜绝 Infinity/NaN。
function ThroughputCell({ item }) {
  const { requestTs } = deriveDuration(item);
  if (!(requestTs > 0) || !Number.isFinite(requestTs)) return '';
  return (
    <span className="whitespace-nowrap font-mono tabular-nums">
      {requestTs.toFixed(2)}
      <span className="text-muted-foreground"> tok/s</span>
    </span>
  );
}
ThroughputCell.propTypes = { item: PropTypes.object };

export default function LogTable({
  t,
  data,
  userIsAdmin,
  isOrgContext,
  canViewLogIO = true,
  userGroup,
  order,
  orderBy,
  onSort,
  onRowDetail,
  searching,
  columnVisibility,
  emptyInfo,
  layout = 'table',
  onNaturalWidth
}) {
  const columns = useMemo(() => {
    const cols = [
      {
        id: 'created_at',
        sortable: true,
        header: t('logPage.timeLabel'),
        cell: (i) => <TimeCell ts={i.created_at} />
      }
    ];
    if (userIsAdmin) {
      cols.push({
        id: 'channel_id',
        header: t('logPage.channelLabel'),
        // admin 列表预载 channel 的 id+name+type+base_url(见 model.GetLogsList Preload):
        // ProviderIcon 三级回退——厂牌图标 → 本站缓存的 base_url favicon → 首字母占位。
        cell: (i) =>
          i.channel_id ? (
            <Link
              to={`/panel/channel/${i.channel_id}`}
              onClick={(e) => e.stopPropagation()}
              className="inline-flex items-center gap-1.5 transition-colors hover:text-foreground hover:underline"
            >
              <ProviderIcon type={i.channel?.type} name={i.channel?.name} baseUrl={i.channel?.base_url} />
              <span className="whitespace-nowrap">
                {i.channel_id}
                {i.channel?.name ? ` (${i.channel.name})` : ''}
              </span>
            </Link>
          ) : (
            ''
          )
      });
      cols.push({
        id: 'user_id',
        header: t('logPage.userLabel'),
        cell: (i) => (i.username ? <TextLink to={`/panel/user?keyword=${encodeURIComponent(i.username)}`}>{i.username}</TextLink> : '')
      });
    }
    if (isOrgContext) {
      // 组织上下文:成员列(后端已把 username 改写为实际成员用户名)
      cols.push({
        id: 'member',
        header: t('logPage.memberLabel'),
        cell: (i) => (i.username ? <span className="whitespace-nowrap">{i.username}</span> : '')
      });
    }
    cols.push({
      id: 'group',
      header: t('logPage.groupLabel'),
      cell: (i) => {
        const m = i.metadata || {};
        const name = userGroup?.[m.group_name || m.backup_group_name]?.name;
        if (!name) return '';
        // 分组页暂不支持按名称过滤,仅 admin 渲染为跳转链接;非 admin 保持纯 Badge。
        return userIsAdmin ? <TextLink to="/panel/user_group">{name}</TextLink> : <span className="whitespace-nowrap">{name}</span>;
      }
    });
    cols.push({
      id: 'token_name',
      header: t('logPage.tokenLabel'),
      cell: (i) => (i.token_name ? <TextLink to={`/panel/token?keyword=${encodeURIComponent(i.token_name)}`}>{i.token_name}</TextLink> : '')
    });
    cols.push({
      id: 'app',
      // 带 icon 的列(Model/Provider/App)统一左对齐才协调(参照 OpenRouter),故 App 列不设
      // className,走默认左对齐;Model/Provider 列本就默认左对齐,不动。
      header: (
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex items-center gap-1">
                {t('logPage.appLabel')}
                <HelpCircle className="size-3.5 text-muted-foreground" />
              </span>
            </TooltipTrigger>
            <TooltipContent className="max-w-xs whitespace-normal text-xs">{t('logPage.appTooltip')}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      ),
      cell: (i) => <AppCell item={i} />
    });
    cols.push({
      id: 'type',
      header: t('logPage.typeLabel'),
      cell: (i) => {
        const meta = logTypeMeta(i.type);
        return meta ? (
          <Badge color={meta.color}>{t(meta.labelKey)}</Badge>
        ) : (
          <Badge color="error">{t('logPage.unknown', { defaultValue: 'Unknown' })}</Badge>
        );
      }
    });
    cols.push({
      id: 'model_name',
      header: t('logPage.modelLabel'),
      cell: (i) =>
        i.model_name ? (
          <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
            <ModelIcon model={i.model_name} />
            <span className="font-mono text-[13px]">{i.model_name}</span>
            {i.metadata?.response_model_mismatch && (
              <TooltipProvider delayDuration={150}>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <AlertTriangle className="size-3.5 shrink-0 text-amber-500" aria-label={t('logPage.responseModel.mismatchShort')} />
                  </TooltipTrigger>
                  <TooltipContent className="max-w-xs whitespace-normal text-xs">
                    {t('logPage.responseModel.mismatchTooltip', { model: i.metadata.response_model })}
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            )}
          </span>
        ) : (
          ''
        )
    });
    cols.push({
      id: 'relay_mode',
      header: t('logPage.relayMode.columnLabel'),
      // 旧日志/判不出模态无 metadata.relay_mode:relayModeMeta 返回 null → 显示占位符(容错)。
      cell: (i) => {
        const meta = relayModeMeta(i.metadata?.relay_mode);
        return meta ? <Badge color={meta.color}>{t(meta.labelKey)}</Badge> : <span className="text-muted-foreground/40">—</span>;
      }
    });
    cols.push({
      id: 'finish_reason',
      header: t('logPage.finishReason.columnLabel'),
      // 旧日志/非兼容 provider 无 finish_reason:finishReasonMeta 返回 null → 不渲染 badge(容错)。
      cell: (i) => {
        const meta = finishReasonMeta(i.metadata?.finish_reason);
        return meta ? <Badge color={meta.color}>{t(meta.labelKey)}</Badge> : '';
      }
    });
    cols.push({
      id: 'duration',
      className: 'text-right',
      header: (
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex items-center gap-1">
                {t('logPage.durationLabel')}
                <HelpCircle className="size-3.5 text-muted-foreground" />
              </span>
            </TooltipTrigger>
            <TooltipContent className="max-w-xs whitespace-normal text-xs">{t('logPage.durationTooltip')}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      ),
      cell: (i) => <DurationCell item={i} />
    });
    // 吞吐列:耗时列之后,等宽数字右对齐;口径复用 deriveDuration.requestTs(与详情弹窗一致)。
    cols.push({
      id: 'throughput',
      className: 'text-right',
      header: t('logPage.throughputLabel'),
      cell: (i) => <ThroughputCell item={i} />
    });
    // 数值列(Input/Output/Cost)右对齐(参照 OpenRouter),走既有 meta.className 通道,表头与单元格同步。
    cols.push({
      id: 'prompt_tokens',
      className: 'text-right',
      header: t('logPage.inputLabel'),
      cell: (i) => <TokenCell value={i.prompt_tokens} />
    });
    cols.push({
      id: 'completion_tokens',
      className: 'text-right',
      header: t('logPage.outputLabel'),
      cell: (i) => <TokenCell value={i.completion_tokens} />
    });
    cols.push({ id: 'quota', className: 'text-right', header: t('logPage.quotaLabel'), cell: (i) => <QuotaCell item={i} /> });
    cols.push({
      id: 'source_ip',
      header: t('logPage.sourceIp'),
      cell: (i) => (i.source_ip ? <span className="font-mono tabular-nums">{i.source_ip}</span> : '')
    });
    return cols.map((c) => ({
      id: c.id,
      header: () => c.header,
      cell: ({ row }) => c.cell(row.original),
      meta: { sortable: c.sortable, className: c.className }
    }));
  }, [t, userIsAdmin, isOrgContext, userGroup]);

  const table = useTable({
    features: appTableFeatures,
    data: data || [],
    columns,
    manualSorting: true,
    state: { columnVisibility: columnVisibility ?? {} }
  });
  const tableRef = useNaturalWidth(layout === 'table' ? onNaturalWidth : null);

  // Sort arrows: the active column shows its direction; others show a faint arrow on hover only.
  const sortIcon = (id) => {
    if (orderBy !== id) return <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };
  const emptyText = searching ? t('logPage.searching') : emptyInfo || t('logPage.searchLogsInfo', { defaultValue: 'No logs found' });

  if (layout === 'list') {
    if (!data?.length) return <p className="px-4 py-10 text-center text-sm text-muted-foreground">{emptyText}</p>;
    return <LogList t={t} data={data} isOrgContext={isOrgContext} onRowDetail={canViewLogIO ? onRowDetail : undefined} />;
  }

  return (
    <Table ref={tableRef}>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id} className="hover:bg-transparent">
            {hg.headers.map((h) => {
              const sortable = h.column.columnDef.meta?.sortable;
              return (
                <TableHead key={h.id} className={cn('whitespace-nowrap', h.column.columnDef.meta?.className)}>
                  {sortable ? (
                    <button
                      type="button"
                      className={cn(
                        'group/sort inline-flex items-center gap-1 hover:text-foreground',
                        orderBy === h.column.id && 'text-foreground'
                      )}
                      onClick={() => onSort(h.column.id)}
                    >
                      {flexRender(h.column.columnDef.header, h.getContext())}
                      {sortIcon(h.column.id)}
                    </button>
                  ) : (
                    flexRender(h.column.columnDef.header, h.getContext())
                  )}
                </TableHead>
              );
            })}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={table.getVisibleLeafColumns().length || 1} className="h-24 text-center text-sm text-muted-foreground">
              {emptyText}
            </TableCell>
          </TableRow>
        ) : (
          table.getRowModel().rows.map((row) => (
            <TableRow
              key={row.id}
              className={cn(canViewLogIO && ROW_OPENS)}
              {...(canViewLogIO ? rowOpenProps(() => onRowDetail(row.original)) : {})}
            >
              {row.getVisibleCells().map((cell) => (
                <TableCell key={cell.id} className={cn('whitespace-nowrap', cell.column.columnDef.meta?.className)}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}

// Phone list: one 56px row per request. Model (with its icon) and cost on the first line; time,
// duration, tokens and the key (the member in an organization) on the second. A tap opens the
// detail.
function LogList({ t, data, isOrgContext, onRowDetail }) {
  return (
    <ul className="divide-y divide-border">
      {data.map((i) => {
        const { requestTimeStr } = deriveDuration(i);
        const who = isOrgContext ? i.username : i.token_name;
        const tokens =
          i.prompt_tokens || i.completion_tokens
            ? `${(i.prompt_tokens || 0).toLocaleString()} → ${(i.completion_tokens || 0).toLocaleString()}`
            : null;
        return (
          <li
            key={i.id}
            className={cn('flex h-14 items-center gap-3 px-4', onRowDetail && ROW_OPENS)}
            {...(onRowDetail ? rowOpenProps(() => onRowDetail(i)) : {})}
          >
            <span className="flex size-7 shrink-0 items-center justify-center rounded-lg border border-border bg-field">
              <ModelIcon model={i.model_name} />
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="min-w-0 truncate font-mono text-[13px] font-medium">
                  {i.model_name || (logTypeMeta(i.type) ? t(logTypeMeta(i.type).labelKey) : '—')}
                </span>
                <span className="ml-auto shrink-0 text-[13px]">
                  <QuotaCell item={i} />
                </span>
              </div>
              <div className="mt-0.5 truncate text-xs tabular-nums text-muted-foreground">
                {[timestamp2stringCompact(i.created_at), i.request_time ? requestTimeStr : null, tokens, who].filter(Boolean).join(' · ')}
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

LogList.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array.isRequired,
  isOrgContext: PropTypes.bool,
  onRowDetail: PropTypes.func
};

LogTable.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array,
  userIsAdmin: PropTypes.bool,
  isOrgContext: PropTypes.bool,
  canViewLogIO: PropTypes.bool,
  userGroup: PropTypes.object,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  onRowDetail: PropTypes.func.isRequired,
  searching: PropTypes.bool,
  columnVisibility: PropTypes.object,
  emptyInfo: PropTypes.node,
  layout: PropTypes.oneOf(['table', 'list']),
  onNaturalWidth: PropTypes.func
};
