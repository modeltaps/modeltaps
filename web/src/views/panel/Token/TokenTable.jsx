import { useMemo } from 'react';
import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import {
  Eye,
  EyeOff,
  Copy,
  Pencil,
  Trash2,
  ChevronsUpDown,
  ArrowUp,
  ArrowDown,
  BarChart3,
  FileText,
  Terminal,
  Plug,
  MoreHorizontal
} from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { RowActions } from '@/components/ui/row-actions';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { renderQuota, SpendAmount, timestamp2string } from 'utils/common';
import { isPlaygroundToken } from 'utils/playgroundToken';

export function statusInfo(t, status) {
  switch (status) {
    case 1:
      return { label: t('common.enable'), variant: 'default' };
    case 2:
      return { label: t('common.disable'), variant: 'secondary' };
    case 3:
      return { label: t('common.expired'), variant: 'destructive' };
    case 4:
      return { label: t('common.exhaust'), variant: 'destructive' };
    default:
      return { label: t('common.unknown'), variant: 'outline' };
  }
}

// 相对时间（unix 秒）：本地化 Intl.RelativeTimeFormat，就近取最大单位
const RELATIVE_TIME_UNITS = [
  ['year', 31536000],
  ['month', 2592000],
  ['week', 604800],
  ['day', 86400],
  ['hour', 3600],
  ['minute', 60],
  ['second', 1]
];
export function relativeTime(rtf, tsSec) {
  const diffSec = tsSec - Math.floor(Date.now() / 1000);
  const absSec = Math.abs(diffSec);
  for (const [unit, secs] of RELATIVE_TIME_UNITS) {
    if (absSec >= secs || unit === 'second') return rtf.format(Math.round(diffSec / secs), unit);
  }
  return rtf.format(0, 'second');
}

// 目标时区某 UTC 时刻的偏移分钟数（东为正）：用 Intl 求，勿手写偏移表。
function tzOffsetMinutes(timeZone, date) {
  const dtf = new Intl.DateTimeFormat('en-US', {
    timeZone,
    hour12: false,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
  const map = {};
  for (const p of dtf.formatToParts(date)) map[p.type] = p.value;
  let hour = parseInt(map.hour, 10);
  if (hour === 24) hour = 0;
  const asUTC = Date.UTC(+map.year, +map.month - 1, +map.day, hour, +map.minute, +map.second);
  return (asUTC - date.getTime()) / 60000;
}

// 目标时区某日 0 点对应的 unix 秒（含 DST 二次校正）。month 为 0-based。
function zonedMidnightUnix(timeZone, year, month, day) {
  const wall = Date.UTC(year, month, day, 0, 0, 0);
  const offset = tzOffsetMinutes(timeZone, new Date(wall));
  let ts = wall - offset * 60000;
  const offset2 = tzOffsetMinutes(timeZone, new Date(ts));
  if (offset2 !== offset) ts = wall - offset2 * 60000;
  return Math.floor(ts / 1000);
}

// 目标时区「当前」的日历年月日（1-based month）。
function zonedYMD(timeZone, date) {
  const dtf = new Intl.DateTimeFormat('en-US', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' });
  const map = {};
  for (const p of dtf.formatToParts(date)) map[p.type] = p.value;
  return { y: +map.year, m: +map.month, d: +map.day };
}

// 下次周期重置时间（unix 秒）：按配置时区求当日 0 点边界，周起点可配置，与后端对齐。
// 缺 key 时回退 UTC / 周一，兼容旧后端。
export function nextResetTime(period, timeZone, weekStart) {
  const tz = timeZone || 'UTC';
  try {
    const { y, m, d } = zonedYMD(tz, new Date());
    if (period === 'daily') return zonedMidnightUnix(tz, y, m - 1, d + 1);
    if (period === 'weekly') {
      const dow = new Date(Date.UTC(y, m - 1, d)).getUTCDay(); // 0=周日..6=周六
      const startDow = weekStart === 'sunday' ? 0 : 1;
      let delta = (startDow - dow + 7) % 7;
      if (delta === 0) delta = 7; // 下次重置严格在未来
      return zonedMidnightUnix(tz, y, m - 1, d + delta);
    }
    if (period === 'monthly') return zonedMidnightUnix(tz, y, m, 1);
  } catch {
    /* 无效时区名等异常时不显示下次重置 */
  }
  return null;
}

export function GroupBadge({ item, userGroup }) {
  const { t } = useTranslation();
  if (item.group === '') return <Badge variant="secondary">{t('token_index.followUser')}</Badge>;
  const g = userGroup[item.group];
  if (!g) return <Badge variant="destructive">{`${item.group} (${t('token_index.groupNotExist')})`}</Badge>;
  if (g.inaccessible) return <Badge variant="destructive">{`${g.name} (${t('token_index.groupUnavailable')})`}</Badge>;
  return <Badge variant="outline">{g.name}</Badge>;
}

// OpenRouter 式细进度条:ratio ∈ [0,1];≥0.8 转警示色。无 shadcn Progress 组件,用细 div 实现。
export function LimitBar({ ratio }) {
  const pct = Math.min(100, Math.max(0, (Number(ratio) || 0) * 100));
  return (
    <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
      <div className={cn('h-full rounded-full', pct >= 80 ? 'bg-destructive' : 'bg-primary')} style={{ width: `${pct}%` }} />
    </div>
  );
}

// 构建单个 API Key 的动作列表(表格行 RowActions 与移动端卡片 … 菜单共用),避免逻辑重复。
export function buildTokenActions({ item, t, navigate, onEdit, onDelete, onCopyKey, onShowUsage, onConnectOpenClaw }) {
  // 按 API Key 名预过滤跳转:用量页用 tokens 参数,日志页用 token_name 参数(与 T50b URL 过滤对齐)
  const goUsage = () => navigate(`/panel/usage?tokens=${encodeURIComponent(item.name)}`);
  const goLog = () => navigate(`/panel/log?token_name=${encodeURIComponent(item.name)}`);
  // 试用区专用 Key 由系统托管:不给复制明文与编辑入口,删除仍保留。
  const isPlayground = isPlaygroundToken(item);
  return [
    !isPlayground && { key: 'copy', label: t('token_index.copy'), icon: Copy, primary: true, onClick: () => onCopyKey(item) },
    { key: 'test', label: t('token_index.testCommand'), icon: Terminal, overflow: true, onClick: () => onShowUsage(item) },
    { key: 'openclaw', label: t('token_index.openclawConnect'), icon: Plug, overflow: true, onClick: () => onConnectOpenClaw(item) },
    { key: 'usage', label: t('token_index.usage'), icon: BarChart3, overflow: true, disabled: !item.name, onClick: goUsage },
    { key: 'log', label: t('token_index.viewLog'), icon: FileText, overflow: true, disabled: !item.name, onClick: goLog },
    !isPlayground && { key: 'edit', label: t('common.edit'), icon: Pencil, primary: true, onClick: () => onEdit(item.id) },
    { key: 'delete', label: t('common.delete'), icon: Trash2, overflow: true, destructive: true, onClick: () => onDelete(item) }
  ];
}

function TokenRowActions({ item, onEdit, onDelete, onCopyKey, onShowUsage, onConnectOpenClaw }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const actions = buildTokenActions({
    item,
    t,
    navigate,
    onEdit,
    onDelete,
    onCopyKey,
    onShowUsage,
    onConnectOpenClaw
  });
  return <RowActions actions={actions} />;
}

// 移动端卡片右上角 … 菜单:把 API Key 全部动作平铺进单个 DropdownMenu(不铺底部操作条)。
// 带 items 的动作渲染为带标签的分组;destructive 项前置分隔线。
function TokenCardMenu({ actions, t, onTriggerClick, onTriggerKeyDown }) {
  const items = (actions || []).filter(Boolean);
  const renderItem = (a, i) => {
    const prevDestructive = i > 0 && items[i - 1].destructive;
    const separator = a.destructive && i > 0 && !prevDestructive ? <DropdownMenuSeparator /> : null;
    if (a.items) {
      return (
        <div key={a.key || a.label}>
          {separator}
          <DropdownMenuLabel>{a.label}</DropdownMenuLabel>
          {a.items.map((it) => {
            const ItIcon = it.icon;
            return (
              <DropdownMenuItem key={it.key || it.label} disabled={it.disabled} onClick={it.onClick}>
                {ItIcon ? <ItIcon /> : null} {it.label}
              </DropdownMenuItem>
            );
          })}
        </div>
      );
    }
    const Icon = a.icon;
    return (
      <div key={a.key || a.label}>
        {separator}
        <DropdownMenuItem disabled={a.disabled} className={a.destructive ? 'text-destructive' : undefined} onClick={a.onClick}>
          <Icon /> {a.label}
        </DropdownMenuItem>
      </div>
    );
  };
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild onClick={onTriggerClick} onKeyDown={onTriggerKeyDown}>
        <Button variant="ghost" size="icon" className="size-8" aria-label={t('token_index.actions')}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">{items.map(renderItem)}</DropdownMenuContent>
    </DropdownMenu>
  );
}

// 移动端(<md)精简卡片:名称+状态开关+…菜单 / 掩码 key+眼睛复制 / 紧凑 meta 行(用量·今日·最近使用),
// 仅限额(周期或总额)时追加限额 Badge + 细进度条,未限额不渲染任何限额元素。
// 分组/过期/创建时间/计费标签/creator 不在此呈现(后续 TC3 详情 Sheet 承接)。
function TokenCard({
  item,
  t,
  rtf,
  revealed,
  todayUsage,
  quotaResetTimezone,
  quotaResetWeekStart,
  onToggleReveal,
  onCopyKey,
  onToggleStatus,
  actions
}) {
  const shown = revealed.has(item.id);
  const k = item.key || '';
  const masked = k.length > 6 ? `sk-${k.slice(0, 3)}…${k.slice(-3)}` : `sk-${k}`;
  const info = statusInfo(t, item.status);
  const isAbnormal = item.status === 3 || item.status === 4;
  const accessed = item.accessed_time;
  const isPlayground = isPlaygroundToken(item);

  const qr = item.setting?.quota_reset;
  let limitEl = null;
  if (qr?.period && qr.limit > 0) {
    const used = item.period_used || 0;
    const next = nextResetTime(qr.period, quotaResetTimezone, quotaResetWeekStart);
    limitEl = (
      <div className="flex flex-col gap-1" title={next ? `${t('token_index.quotaResetNext')}: ${timestamp2string(next)}` : undefined}>
        <div className="flex items-center justify-between gap-1.5">
          <span className="font-mono text-[13px] tabular-nums">{renderQuota(qr.limit)}</span>
          <Badge variant="secondary" className="text-[11px]">
            {t(`token_index.limitPeriod_${qr.period}`)}
          </Badge>
        </div>
        <LimitBar ratio={used / qr.limit} />
      </div>
    );
  } else if (!item.unlimited_quota) {
    const used = item.used_quota || 0;
    const total = used + (item.remain_quota || 0);
    limitEl = (
      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between gap-1.5">
          <span className="font-mono text-[13px] tabular-nums">{renderQuota(total)}</span>
          <Badge variant="outline" className="text-[11px]">
            {t('token_index.limitTotal')}
          </Badge>
        </div>
        <LimitBar ratio={total > 0 ? used / total : 0} />
      </div>
    );
  }

  // 卡片主体点击打开详情 Sheet;卡片上的开关/眼睛/复制/… 菜单等交互元素需拦住冒泡(含键盘
  // Space/Enter),避免误触发 DataCards 的 onRowClick。
  const stop = (e) => e.stopPropagation();

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-start justify-between gap-2">
        <span className="min-w-0 flex-1 truncate font-medium">{item.name}</span>
        <div className="flex shrink-0 items-center gap-1.5">
          {isAbnormal && (
            <Badge variant="destructive" className="text-[11px]">
              {info.label}
            </Badge>
          )}
          {/* Switch/菜单为原生 button:onClick 覆盖内部 toggle,故在此直接 stop+切换;onKeyDown 拦
              键盘冒泡,均为防止误触发整卡片的 onRowClick。 */}
          <Switch
            checked={item.status === 1}
            onClick={(e) => {
              stop(e);
              onToggleStatus(item);
            }}
            onKeyDown={stop}
            title={info.label}
            aria-label={info.label}
          />
          <TokenCardMenu actions={actions} t={t} onTriggerClick={stop} onTriggerKeyDown={stop} />
        </div>
      </div>
      {isPlayground ? (
        <Badge variant="secondary" className="w-fit text-[11px]">
          {t('token_index.playgroundManaged')}
        </Badge>
      ) : (
        <div className="flex items-center gap-1">
          <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-muted-foreground">{shown ? `sk-${item.key}` : masked}</code>
          <Button
            variant="ghost"
            size="icon"
            className="size-6"
            onClick={(e) => {
              stop(e);
              onToggleReveal(item.id);
            }}
            onKeyDown={stop}
            aria-label="reveal"
          >
            {shown ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="size-6"
            onClick={(e) => {
              stop(e);
              onCopyKey(item);
            }}
            onKeyDown={stop}
            aria-label="copy"
          >
            <Copy className="size-3.5" />
          </Button>
        </div>
      )}
      <div className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-xs text-muted-foreground">
        <SpendAmount quota={item.used_quota} />
        {todayUsage && (
          <>
            <span>·</span>
            <span className="inline-flex items-center gap-1">
              {t('token_index.usageToday')} <SpendAmount quota={todayUsage[item.name] || 0} />
            </span>
          </>
        )}
        <span>·</span>
        <span title={accessed > 0 ? timestamp2string(accessed) : t('token_index.neverUsed')}>
          {accessed > 0 ? relativeTime(rtf, accessed) : '—'}
        </span>
      </div>
      {limitEl}
    </div>
  );
}

export default function TokenTable({
  tokens,
  todayUsage = null,
  orgMemberNames = null,
  userGroup,
  userIsReliable,
  revealed,
  onToggleReveal,
  onCopyKey,
  onEdit,
  onDelete,
  onToggleStatus,
  onShowUsage,
  onConnectOpenClaw,
  onCardClick,
  order,
  orderBy,
  onSort,
  columnVisibility = {},
  quotaResetTimezone,
  quotaResetWeekStart
}) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const rtfLocale = (i18n.language || 'zh_CN').replace('_', '-');
  const cardRtf = useMemo(() => new Intl.RelativeTimeFormat(rtfLocale, { numeric: 'auto' }), [rtfLocale]);

  const columns = useMemo(() => {
    const rtf = new Intl.RelativeTimeFormat(rtfLocale, { numeric: 'auto' });
    const cols = [
      // API Key:合并原「名称」+「密钥」。第一行名称,第二行 mono 掩码 key;hover 显示眼睛/复制
      // (opacity 切换,不额外占列;移动端 max-md 常显以便触屏操作)。排序仍按 name。
      {
        accessorKey: 'name',
        header: t('token_index.token'),
        meta: { sortable: true },
        cell: (c) => {
          const item = c.row.original;
          const shown = revealed.has(item.id);
          const k = item.key || '';
          const masked = k.length > 6 ? `sk-${k.slice(0, 3)}…${k.slice(-3)}` : `sk-${k}`;
          // 试用区专用 Key:不显示掩码与眼睛/复制,仅标注由系统托管。
          return (
            <div className="group/token flex flex-col gap-0.5">
              <span className="font-medium text-[15px]">{item.name}</span>
              {isPlaygroundToken(item) ? (
                <Badge variant="secondary" className="w-fit text-[11px]">
                  {t('token_index.playgroundManaged')}
                </Badge>
              ) : (
                <div className="flex items-center gap-1">
                  <code className="font-mono text-[13px] text-muted-foreground">{shown ? `sk-${item.key}` : masked}</code>
                  <div className="flex items-center opacity-0 transition-opacity focus-within:opacity-100 group-hover/token:opacity-100 max-md:opacity-100">
                    <Button variant="ghost" size="icon" className="size-6" onClick={() => onToggleReveal(item.id)} aria-label="reveal">
                      {shown ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
                    </Button>
                    <Button variant="ghost" size="icon" className="size-6" onClick={() => onCopyKey(item)} aria-label="copy">
                      <Copy className="size-3.5" />
                    </Button>
                  </div>
                </div>
              )}
            </div>
          );
        }
      },
      // 组织上下文:创建者列(orgMemberNames 非空时启用;created_by=0 为历史数据)
      ...(orgMemberNames
        ? [
            {
              id: 'creator',
              header: t('org.creator'),
              cell: (c) => {
                const createdBy = c.row.original.created_by;
                if (!createdBy) return '-';
                return <Badge variant="outline">{orgMemberNames[createdBy] || `#${createdBy}`}</Badge>;
              }
            }
          ]
        : []),
      {
        id: 'group',
        header: t('token_index.userGroup'),
        meta: { sortable: true },
        cell: (c) => <GroupBadge item={c.row.original} userGroup={userGroup} />
      }
    ];
    if (userIsReliable) {
      cols.push({
        id: 'billing_tag',
        header: t('token_index.billingTag'),
        cell: (c) => <Badge variant="outline">{userGroup[c.row.original.setting?.billing_tag]?.name || '-'}</Badge>
      });
    }
    cols.push(
      // 过期:相对时间(永不 / 30 天后 / 已过期红字);title 提示绝对时间。排序仍按 expired_time。
      {
        accessorKey: 'expired_time',
        header: t('token_index.expiryTime'),
        meta: { sortable: true },
        cell: (c) => {
          const v = c.getValue();
          if (v === -1) return <span className="text-[13px] text-muted-foreground">{t('token_index.expiryNever')}</span>;
          const expired = v <= Math.floor(Date.now() / 1000);
          return (
            <span className={cn('text-[13px]', expired && 'text-destructive')} title={timestamp2string(v)}>
              {expired ? t('token_index.expiryExpired') : relativeTime(rtf, v)}
            </span>
          );
        }
      },
      {
        accessorKey: 'accessed_time',
        header: t('token_index.lastUsed'),
        cell: (c) => {
          const v = c.getValue();
          if (!v || v <= 0)
            return (
              <span className="text-[13px] text-muted-foreground" title={t('token_index.neverUsed')}>
                —
              </span>
            );
          return (
            <span className="text-[13px]" title={timestamp2string(v)}>
              {relativeTime(rtf, v)}
            </span>
          );
        }
      },
      // 用量:合并原「已用额度」+「今日消费」。第一行累计消费,第二行小字今日(todayUsage 为 null 时不显示)。
      {
        accessorKey: 'used_quota',
        header: t('token_index.usageColumn'),
        meta: { sortable: true },
        cell: (c) => {
          const item = c.row.original;
          return (
            <div className="flex flex-col gap-0.5">
              <SpendAmount quota={c.getValue()} className="font-mono text-[15px] tabular-nums" />
              {todayUsage && (
                <span className="text-[13px] text-muted-foreground">
                  {t('token_index.usageToday')} <SpendAmount quota={todayUsage[item.name] || 0} />
                </span>
              )}
            </div>
          );
        }
      },
      // 限额:合并原「剩余额度」+「周期额度重置」,OpenRouter Limit 列风格。三形态,按判断顺序:
      //   周期(有 quota_reset,unlimited_quota 通常为 true):$limit + 周期 Badge + 进度条(period_used/limit),title 提示下次重置。
      //   无限制(无 reset 且 unlimited_quota):muted 文本,无进度条。
      //   总额(非无限):$total + 「总额」Badge + 进度条(used/(used+remain))。排序仍按 remain_quota。
      {
        accessorKey: 'remain_quota',
        header: t('token_index.limitColumn'),
        meta: { sortable: true },
        cell: (c) => {
          const item = c.row.original;
          const qr = item.setting?.quota_reset;
          if (qr?.period && qr.limit > 0) {
            const used = item.period_used || 0;
            const next = nextResetTime(qr.period, quotaResetTimezone, quotaResetWeekStart);
            return (
              <div
                className="flex min-w-[7rem] flex-col gap-1"
                title={next ? `${t('token_index.quotaResetNext')}: ${timestamp2string(next)}` : undefined}
              >
                <div className="flex items-center justify-between gap-1.5">
                  <span className="font-mono text-[15px] tabular-nums">{renderQuota(qr.limit)}</span>
                  <Badge variant="secondary" className="text-[11px]">
                    {t(`token_index.limitPeriod_${qr.period}`)}
                  </Badge>
                </div>
                <LimitBar ratio={used / qr.limit} />
              </div>
            );
          }
          if (item.unlimited_quota)
            return (
              <div className="flex min-w-[7rem] flex-col gap-1">
                <span className="text-muted-foreground">{t('token_index.noLimit')}</span>
                <LimitBar ratio={0} />
              </div>
            );
          const used = item.used_quota || 0;
          const total = used + (item.remain_quota || 0);
          return (
            <div className="flex min-w-[7rem] flex-col gap-1">
              <div className="flex items-center justify-between gap-1.5">
                <span className="font-mono text-[15px] tabular-nums">{renderQuota(total)}</span>
                <Badge variant="outline" className="text-[11px]">
                  {t('token_index.limitTotal')}
                </Badge>
              </div>
              <LimitBar ratio={total > 0 ? used / total : 0} />
            </div>
          );
        }
      },
      // 创建时间:默认隐藏(通过 columnVisibility),可在列设置中开启。
      {
        accessorKey: 'created_time',
        header: t('token_index.createdTime'),
        meta: { sortable: true },
        cell: (c) => timestamp2string(c.getValue())
      },
      // 状态:紧邻操作列左侧。启停开关 + 过期/耗尽异常态 Badge。
      {
        id: 'status',
        header: t('token_index.status'),
        meta: { sortable: true },
        cell: (c) => {
          const item = c.row.original;
          const info = statusInfo(t, item.status);
          const isAbnormal = item.status === 3 || item.status === 4;
          return (
            <div className="flex items-center gap-2">
              <Switch checked={item.status === 1} onCheckedChange={() => onToggleStatus(item)} title={info.label} aria-label={info.label} />
              {isAbnormal && (
                <Badge variant="destructive" className="text-[11px]">
                  {info.label}
                </Badge>
              )}
            </div>
          );
        }
      },
      {
        id: 'actions',
        header: t('token_index.actions'),
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <TokenRowActions
            item={c.row.original}
            onEdit={onEdit}
            onDelete={onDelete}
            onCopyKey={onCopyKey}
            onShowUsage={onShowUsage}
            onConnectOpenClaw={onConnectOpenClaw}
          />
        )
      }
    );
    return cols;
  }, [
    t,
    userGroup,
    userIsReliable,
    revealed,
    onToggleReveal,
    onCopyKey,
    onEdit,
    onDelete,
    onToggleStatus,
    onShowUsage,
    onConnectOpenClaw,
    orgMemberNames,
    todayUsage,
    rtfLocale,
    quotaResetTimezone,
    quotaResetWeekStart
  ]);

  const table = useTable({
    features: appTableFeatures,
    data: tokens,
    columns,
    state: { columnVisibility }
  });

  const SortIcon = ({ id }) => {
    if (orderBy !== id) return <ChevronsUpDown className="size-3.5 opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />;
  };

  return (
    <>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id} className="hover:bg-transparent">
                {hg.headers.map((header) => (
                  <TableHead key={header.id} className={`whitespace-nowrap ${header.column.columnDef.meta?.className ?? ''}`}>
                    {header.column.columnDef.meta?.sortable && onSort ? (
                      <button
                        type="button"
                        className="inline-flex items-center gap-1 hover:text-foreground"
                        onClick={() => onSort(header.column.id)}
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        <SortIcon id={header.column.id} />
                      </button>
                    ) : (
                      flexRender(header.column.columnDef.header, header.getContext())
                    )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
                    {flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <DataCards
        table={table}
        className="md:hidden"
        onRowClick={onCardClick}
        renderCard={(item) => (
          <TokenCard
            item={item}
            t={t}
            rtf={cardRtf}
            revealed={revealed}
            todayUsage={todayUsage}
            quotaResetTimezone={quotaResetTimezone}
            quotaResetWeekStart={quotaResetWeekStart}
            onToggleReveal={onToggleReveal}
            onCopyKey={onCopyKey}
            onToggleStatus={onToggleStatus}
            actions={buildTokenActions({
              item,
              t,
              navigate,
              onEdit,
              onDelete,
              onCopyKey,
              onShowUsage,
              onConnectOpenClaw
            })}
          />
        )}
      />
    </>
  );
}
