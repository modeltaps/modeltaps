import PropTypes from 'prop-types';
import { useMemo } from 'react';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ChevronsUpDown, HelpCircle } from 'lucide-react';
import { Link } from 'react-router';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import ModelIcon from '@/components/brand/ModelIcon';
import ProviderIcon from '@/components/brand/ProviderIcon';
import AppFavicon from './AppFavicon';
import { cn } from '@/lib/utils';
import { SpendAmount, timestamp2string, timestamp2stringCompact } from 'utils/common';
import {
  badgeClass,
  calculateOriginalQuota,
  deriveDuration,
  finishReasonMeta,
  logTypeMeta,
  relayModeMeta,
  requestTimeColor,
  requestTsColor
} from './logHelpers';

const Badge = ({ color, children }) => <span className={badgeClass(color)}>{children}</span>;
Badge.propTypes = { color: PropTypes.string, children: PropTypes.node };

// 字段超链接:保留 Badge 外观,hover 加下划线/前景色;onClick stopPropagation 防止触发整行详情;
// 用 react-router Link 保持 cmd/ctrl+click 新标签能力。
const LinkBadge = ({ to, color, children }) => (
  <Link
    to={to}
    onClick={(e) => e.stopPropagation()}
    className={cn(badgeClass(color), 'transition-colors hover:text-foreground hover:underline')}
  >
    {children}
  </Link>
);
LinkBadge.propTypes = { to: PropTypes.string, color: PropTypes.string, children: PropTypes.node };

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
        <SpendAmount quota={quota} className="font-mono tabular-nums font-medium text-emerald-600 dark:text-emerald-400" />
      </div>
    );
  }
  return <SpendAmount quota={quota} className="font-mono tabular-nums font-medium text-emerald-600 dark:text-emerald-400" />;
}
QuotaCell.propTypes = { item: PropTypes.object };

function DurationCell({ item }) {
  const { requestTime, requestTimeStr, firstTimeStr, requestTs, requestTsStr } = deriveDuration(item);
  return (
    <div className="flex items-center justify-end gap-1 whitespace-nowrap tabular-nums">
      <Badge color={requestTimeColor(requestTime)}>
        {item.request_time === 0 ? '—' : requestTimeStr}
        {firstTimeStr ? ` / ${firstTimeStr}` : ''}
      </Badge>
      {requestTsStr && <Badge color={requestTsColor(requestTs)}>{requestTsStr}</Badge>}
    </div>
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
  return (
    <span className="whitespace-nowrap font-mono tabular-nums">
      {value}
      <span className="text-muted-foreground"> tok</span>
    </span>
  );
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
  emptyInfo
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
              to={i.channel?.name ? `/panel/channel?name=${encodeURIComponent(i.channel.name)}` : '/panel/channel'}
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
        cell: (i) =>
          i.username ? (
            <LinkBadge to={`/panel/user?keyword=${encodeURIComponent(i.username)}`} color="default">
              {i.username}
            </LinkBadge>
          ) : (
            ''
          )
      });
    }
    if (isOrgContext) {
      // 组织上下文:成员列(后端已把 username 改写为实际成员用户名)
      cols.push({
        id: 'member',
        header: t('logPage.memberLabel'),
        cell: (i) => (i.username ? <Badge color="default">{i.username}</Badge> : '')
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
        return userIsAdmin ? (
          <LinkBadge to="/panel/user_group" color="default">
            {name}
          </LinkBadge>
        ) : (
          <Badge color="default">{name}</Badge>
        );
      }
    });
    cols.push({
      id: 'token_name',
      header: t('logPage.tokenLabel'),
      cell: (i) =>
        i.token_name ? (
          <LinkBadge to={`/panel/token?keyword=${encodeURIComponent(i.token_name)}`} color="default">
            {i.token_name}
          </LinkBadge>
        ) : (
          ''
        )
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
          <Badge color="primary">
            <ModelIcon model={i.model_name} className="mr-1" />
            {i.model_name}
          </Badge>
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

  const SortIcon = ({ id }) => {
    if (orderBy !== id) return <ChevronsUpDown className="size-3.5 opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />;
  };
  SortIcon.propTypes = { id: PropTypes.string };

  return (
    <>
      <div className="hidden md:block">
        <Table className="min-w-[800px]">
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((h) => {
                  const sortable = h.column.columnDef.meta?.sortable;
                  return (
                    <TableHead key={h.id} className={cn('h-[29px] py-0', h.column.columnDef.meta?.className)}>
                      {sortable ? (
                        <button
                          type="button"
                          className="inline-flex items-center gap-1 hover:text-foreground"
                          onClick={() => onSort(h.column.id)}
                        >
                          {flexRender(h.column.columnDef.header, h.getContext())}
                          <SortIcon id={h.column.id} />
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
                  {searching ? t('logPage.searching') : emptyInfo || t('logPage.searchLogsInfo', { defaultValue: 'No logs found' })}
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className={cn(canViewLogIO && 'cursor-pointer hover:bg-muted/50')}
                  onClick={canViewLogIO ? () => onRowDetail(row.original) : undefined}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className={cn('py-1.5', cell.column.columnDef.meta?.className)}>
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
        onRowClick={canViewLogIO ? onRowDetail : undefined}
        empty={emptyInfo || t('logPage.searchLogsInfo', { defaultValue: 'No logs found' })}
        searching={searching}
      />
    </>
  );
}

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
  emptyInfo: PropTypes.node
};
