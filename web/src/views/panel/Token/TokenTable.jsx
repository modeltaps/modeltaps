import { useLayoutEffect, useMemo, useRef } from 'react';
import { useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import {
  Copy,
  Pencil,
  Trash2,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  BarChart3,
  FileText,
  Terminal,
  Plug,
  Power,
  PowerOff,
  MoreHorizontal
} from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Checkbox } from '@/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { renderQuota, SpendAmount, timestamp2string } from 'utils/common';
import { isPlaygroundToken } from 'utils/playgroundToken';

// The short form of a key shown next to its name: the prefix and the last four characters.
export function keySuffix(key) {
  const k = key || '';
  return k.length > 4 ? `sk-…${k.slice(-4)}` : `sk-${k}`;
}

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

// How much a key has spent against its limit: the period counter against a periodic cap, the
// total spend against a fixed total, or the total spend with no limit at all.
export function keyUsage(item) {
  const qr = item.setting?.quota_reset;
  if (qr?.period && qr.limit > 0) return { used: item.period_used || 0, limit: qr.limit, period: qr.period };
  const used = item.used_quota || 0;
  if (item.unlimited_quota) return { used, limit: null, period: null };
  return { used, limit: used + (item.remain_quota || 0), period: null };
}

// Usage column: "$used / $limit" with the share used and a thin bar under it. The tooltip adds
// today's spend, the lifetime spend of a periodic key and the next reset.
function UsageCell({ item, todayUsage, resetTimezone, resetWeekStart }) {
  const { t } = useTranslation();
  const { used, limit, period } = keyUsage(item);
  const ratio = limit ? used / limit : 0;
  const pct = Math.min(100, Math.round(ratio * 100));
  const hints = [];
  if (todayUsage) hints.push(`${t('token_index.usageToday')}: ${renderQuota(todayUsage[item.name] || 0)}`);
  if (period) {
    hints.push(`${t('apiKey.panel.used')}: ${renderQuota(item.used_quota || 0)}`);
    const next = nextResetTime(period, resetTimezone, resetWeekStart);
    if (next) hints.push(`${t('token_index.quotaResetNext')}: ${timestamp2string(next)}`);
  }
  return (
    <div className="flex w-48 flex-col gap-1" title={hints.join('\n') || undefined}>
      <div className="flex items-baseline justify-between gap-2 whitespace-nowrap">
        <span className="text-[14px] tabular-nums">
          <SpendAmount quota={used} className="font-medium" />
          {limit != null ? (
            <span className="text-muted-foreground"> / {renderQuota(limit)}</span>
          ) : (
            <span className="ml-1.5 text-xs text-muted-foreground">{t('token_index.noLimit')}</span>
          )}
          {period && <span className="ml-1.5 text-xs text-muted-foreground">{t(`token_index.limitPeriod_${period}`)}</span>}
        </span>
        {limit != null && <span className="text-xs tabular-nums text-muted-foreground">{pct}%</span>}
      </div>
      <LimitBar ratio={ratio} />
    </div>
  );
}

// Builds one key's actions for the row ⋯ menu (table and mobile card alike). `group` splits the
// menu into sections: manage, connect, inspect, then the destructive delete.
export function buildTokenActions({ item, t, navigate, onEdit, onDelete, onCopyKey, onShowUsage, onConnectOpenClaw, onToggleStatus }) {
  // 按 API Key 名预过滤跳转:用量页用 tokens 参数,日志页用 token_name 参数(与 T50b URL 过滤对齐)
  const goUsage = () => navigate(`/panel/usage?tokens=${encodeURIComponent(item.name)}`);
  const goLog = () => navigate(`/panel/log?token_name=${encodeURIComponent(item.name)}`);
  // 试用区专用 Key 由系统托管:不给复制明文与编辑入口,删除仍保留。
  const isPlayground = isPlaygroundToken(item);
  return [
    !isPlayground && { key: 'edit', group: 'manage', label: t('common.edit'), icon: Pencil, onClick: () => onEdit(item.id) },
    !isPlayground && { key: 'copy', group: 'manage', label: t('token_index.copy'), icon: Copy, onClick: () => onCopyKey(item) },
    // Enable / disable appears in the menu where the row has no switch (medium and narrow layouts).
    onToggleStatus && {
      key: 'toggle',
      group: 'manage',
      label: item.status === 1 ? t('apiKey.list.batchDisable') : t('apiKey.list.batchEnable'),
      icon: item.status === 1 ? PowerOff : Power,
      onClick: () => onToggleStatus(item)
    },
    { key: 'test', group: 'connect', label: t('token_index.testCommand'), icon: Terminal, onClick: () => onShowUsage(item) },
    { key: 'openclaw', group: 'connect', label: t('token_index.openclawConnect'), icon: Plug, onClick: () => onConnectOpenClaw(item) },
    { key: 'usage', group: 'inspect', label: t('token_index.usage'), icon: BarChart3, disabled: !item.name, onClick: goUsage },
    { key: 'log', group: 'inspect', label: t('token_index.viewLog'), icon: FileText, disabled: !item.name, onClick: goLog },
    { key: 'delete', group: 'danger', label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => onDelete(item) }
  ];
}

// One status label in place of the switch and expiry columns (medium and narrow layouts):
// disabled, expired, used up, expiring within a week (with when), or enabled.
const SOON_SECONDS = 7 * 86400;
export function keyState(item, nowSec = Math.floor(Date.now() / 1000)) {
  if (item.status === 2) return { key: 'disabled', tone: 'muted' };
  if (item.status === 4) return { key: 'exhausted', tone: 'danger' };
  if (item.status === 3 || (item.expired_time !== -1 && item.expired_time <= nowSec)) return { key: 'expired', tone: 'danger' };
  if (item.expired_time !== -1 && item.expired_time - nowSec <= SOON_SECONDS) return { key: 'expiring', tone: 'warn' };
  return { key: 'enabled', tone: 'ok' };
}

const TONE_DOT = { ok: 'bg-success', warn: 'bg-amber-500', danger: 'bg-destructive', muted: 'bg-muted-foreground/40' };

export function KeyStatus({ item, rtf, dotOnly = false }) {
  const { t } = useTranslation();
  const state = keyState(item);
  const label =
    state.key === 'expiring'
      ? t('apiKey.list.stateExpiring', { when: relativeTime(rtf, item.expired_time) })
      : t(`apiKey.list.state_${state.key}`);
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-[13px]" title={label}>
      <span className={cn('size-1.5 shrink-0 rounded-full', TONE_DOT[state.tone])} aria-hidden="true" />
      {dotOnly ? (
        <span className="sr-only">{label}</span>
      ) : (
        <span className={cn(state.tone === 'muted' && 'text-muted-foreground')}>{label}</span>
      )}
    </span>
  );
}

// Phone list: one 56px row per key. The status dot leads; the first line is the name with the
// spend (against the limit) on the right; the second line is the key suffix with the state or last
// use, and the share of the limit used. A tap opens the key; the ⋯ menu at the end holds the same
// actions as the table row, plus enable / disable.
function KeyListRow({ item, rtf, selectMode, selected, onToggleSelect, onOpen, active, actions }) {
  const { t } = useTranslation();
  const { used, limit } = keyUsage(item);
  const state = keyState(item);
  const pct = limit ? Math.min(100, Math.round((used / limit) * 100)) : null;
  const stateLabel =
    state.key === 'enabled'
      ? null
      : state.key === 'expiring'
        ? t('apiKey.list.stateExpiring', { when: relativeTime(rtf, item.expired_time) })
        : t(`apiKey.list.state_${state.key}`);
  const lastUse = item.accessed_time > 0 ? relativeTime(rtf, item.accessed_time) : t('token_index.neverUsed');
  return (
    <li
      className={cn('flex h-14 items-center gap-3 px-4', ROW_OPENS, (active || selected) && 'bg-muted/60')}
      {...rowOpenProps(() => (selectMode ? onToggleSelect(item.id) : onOpen(item)))}
    >
      {selectMode ? (
        <RowControl>
          <Checkbox
            checked={selected}
            onCheckedChange={() => onToggleSelect(item.id)}
            aria-label={t('apiKey.list.selectRow', { name: item.name })}
          />
        </RowControl>
      ) : (
        <span
          className={cn('size-2 shrink-0 rounded-full', TONE_DOT[state.tone])}
          title={stateLabel || t('apiKey.list.state_enabled')}
          aria-hidden="true"
        />
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-3">
          <span className="min-w-0 truncate text-sm font-medium">{item.name}</span>
          <span className="ml-auto shrink-0 text-sm tabular-nums">
            <SpendAmount quota={used} />
            {limit ? <span className="text-muted-foreground"> / {renderQuota(limit)}</span> : null}
          </span>
        </div>
        <div className="mt-0.5 flex items-baseline gap-3 text-xs text-muted-foreground">
          <span className="min-w-0 truncate">
            {!isPlaygroundToken(item) && <span className="font-mono">{keySuffix(item.key)}</span>}
            {!isPlaygroundToken(item) && ' · '}
            <span
              className={cn(state.tone === 'danger' && 'text-destructive', state.tone === 'warn' && 'text-amber-600 dark:text-amber-500')}
            >
              {stateLabel || lastUse}
            </span>
          </span>
          <span className={cn('ml-auto shrink-0 tabular-nums', pct >= 80 && 'text-destructive')}>
            {pct == null ? t('token_index.noLimit') : `${pct}%`}
          </span>
        </div>
      </div>
      {!selectMode && (
        <RowControl className="-mr-2 shrink-0">
          <TokenActionMenu t={t} actions={actions} />
        </RowControl>
      )}
    </li>
  );
}

// The ⋯ menu of a key (table row and mobile card): every action in one dropdown, with a
// separator wherever the action group changes; actions with `items` render as a labelled group.
function TokenActionMenu({ actions, t, onTriggerClick, onTriggerKeyDown }) {
  const items = (actions || []).filter(Boolean);
  const renderItem = (a, i) => {
    const separator = i > 0 && a.group !== items[i - 1].group ? <DropdownMenuSeparator /> : null;
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
      <DropdownMenuContent align="end" className="min-w-44">
        {items.map(renderItem)}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// Typical rendered widths (px) of each column (measured, plus a little room for longer names),
// used to decide what fits on one line.
const COLUMN_WIDTHS = {
  select: 32,
  name: 180,
  creator: 100,
  group: 110,
  billing_tag: 100,
  expired_time: 96,
  accessed_time: 90,
  used_quota: 220,
  created_time: 100,
  status: 70,
  actions: 56
};
// Estimated natural width of the table with these columns, used before the table has been
// measured (for example while the phone list is showing).
export function estimateTableWidth(columnIds, visibility) {
  return columnIds.filter((id) => visibility[id] !== false).reduce((sum, id) => sum + (COLUMN_WIDTHS[id] || 120), 0);
}

export default function TokenTable({
  tokens,
  todayUsage = null,
  orgMemberNames = null,
  userGroup,
  userIsReliable,
  onCopyKey,
  onEdit,
  onDelete,
  onToggleStatus,
  onShowUsage,
  onConnectOpenClaw,
  onOpen,
  openId,
  selectedIds = null,
  onToggleSelect,
  onToggleSelectAll,
  layout = 'table',
  onNaturalWidth,
  selectMode = false,
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
  // Medium and narrow layouts use the short relative time ("3h ago").
  const listRtf = useMemo(() => new Intl.RelativeTimeFormat(rtfLocale, { numeric: 'auto', style: 'narrow' }), [rtfLocale]);

  const columns = useMemo(() => {
    const rtf = new Intl.RelativeTimeFormat(rtfLocale, { numeric: 'auto' });
    const allSelected = !!selectedIds && tokens.length > 0 && tokens.every((x) => selectedIds.has(x.id));
    const someSelected = !!selectedIds && tokens.some((x) => selectedIds.has(x.id));
    const cols = [
      // Row selection for the bulk actions; the header box selects the whole page.
      ...(selectedIds
        ? [
            {
              id: 'select',
              header: () => (
                <Checkbox
                  checked={allSelected}
                  indeterminate={!allSelected && someSelected}
                  onCheckedChange={() => onToggleSelectAll(!allSelected)}
                  aria-label={t('apiKey.list.selectAll')}
                />
              ),
              meta: { className: 'w-[1%] pr-0' },
              // A block-level flex wrapper: an inline one sits on the text baseline, a few px above
              // the vertical center of the row.
              cell: (c) => (
                <RowControl className="flex">
                  <Checkbox
                    checked={selectedIds.has(c.row.original.id)}
                    onCheckedChange={() => onToggleSelect(c.row.original.id)}
                    aria-label={t('apiKey.list.selectRow', { name: c.row.original.name })}
                  />
                </RowControl>
              )
            }
          ]
        : []),
      // API Key: one line, the name followed by the key's last characters ("sk-…Rc6g") so a key can
      // be told apart without a second row; copy appears on hover (always on touch screens). The
      // full key is shown in the key's panel. Sorting is by name.
      {
        accessorKey: 'name',
        header: t('token_index.token'),
        meta: { sortable: true },
        cell: (c) => {
          const item = c.row.original;
          // Keys the playground manages show a badge instead of the suffix and copy button.
          return (
            <div className="group/token flex min-w-0 items-center gap-2 whitespace-nowrap">
              <span className="truncate font-medium">{item.name}</span>
              {isPlaygroundToken(item) ? (
                <Badge variant="secondary" className="shrink-0 text-[11px]">
                  {t('token_index.playgroundManaged')}
                </Badge>
              ) : (
                <>
                  <code className="shrink-0 font-mono text-xs text-muted-foreground">{keySuffix(item.key)}</code>
                  <RowControl className="-ml-1 shrink-0 opacity-0 transition-opacity focus-within:opacity-100 group-hover/token:opacity-100 max-md:opacity-100">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-6"
                      onClick={() => onCopyKey(item)}
                      aria-label={t('token_index.copy')}
                      title={t('token_index.copy')}
                    >
                      <Copy className="size-3.5" />
                    </Button>
                  </RowControl>
                </>
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
          if (!v || v <= 0) return <span className="text-[13px] text-amber-600 dark:text-amber-500">{t('token_index.neverUsed')}</span>;
          return (
            <span className="text-[13px]" title={timestamp2string(v)}>
              {relativeTime(rtf, v)}
            </span>
          );
        }
      },
      // Usage: spend against the limit with a progress bar (sorted by total spend).
      {
        accessorKey: 'used_quota',
        header: t('token_index.usageColumn'),
        meta: { sortable: true },
        cell: (c) => (
          <UsageCell
            item={c.row.original}
            todayUsage={todayUsage}
            resetTimezone={quotaResetTimezone}
            resetWeekStart={quotaResetWeekStart}
          />
        )
      },
      // 创建时间:默认隐藏(通过 columnVisibility),可在列设置中开启。
      {
        accessorKey: 'created_time',
        header: t('token_index.createdTime'),
        meta: { sortable: true },
        cell: (c) => (
          <span className="text-[13px]" title={timestamp2string(c.getValue())}>
            {timestamp2string(c.getValue()).slice(0, 10)}
          </span>
        )
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
            <RowControl className="flex gap-2">
              <Switch
                size="sm"
                checked={item.status === 1}
                onCheckedChange={() => onToggleStatus(item)}
                title={info.label}
                aria-label={info.label}
              />
              {isAbnormal && (
                <Badge variant="destructive" className="text-[11px]">
                  {info.label}
                </Badge>
              )}
            </RowControl>
          );
        }
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('token_index.actions')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <RowControl className="justify-end">
            <TokenActionMenu
              t={t}
              actions={buildTokenActions({
                item: c.row.original,
                t,
                navigate,
                onEdit,
                onDelete,
                onCopyKey,
                onShowUsage,
                onConnectOpenClaw
              })}
            />
          </RowControl>
        )
      }
    );
    return cols;
  }, [
    t,
    userGroup,
    userIsReliable,
    onCopyKey,
    onEdit,
    onDelete,
    onToggleStatus,
    onShowUsage,
    onConnectOpenClaw,
    navigate,
    tokens,
    selectedIds,
    onToggleSelect,
    onToggleSelectAll,
    orgMemberNames,
    todayUsage,
    rtfLocale,
    quotaResetTimezone,
    quotaResetWeekStart
  ]);

  // The table never drops columns. It reports its natural (max-content) width so the page can
  // collapse the sidebar, or switch to the phone list, when that width does not fit.
  const tableRef = useRef(null);
  useLayoutEffect(() => {
    const el = tableRef.current;
    if (!el || !onNaturalWidth) return;
    const prev = el.style.width;
    el.style.width = 'max-content';
    const natural = Math.ceil(el.getBoundingClientRect().width);
    el.style.width = prev;
    onNaturalWidth(natural);
  });

  const table = useTable({
    features: appTableFeatures,
    data: tokens,
    columns,
    state: { columnVisibility }
  });

  // Sort arrows: the active column shows its direction; others reveal a faint arrow only on hover,
  // so sortable headers do not look like dropdowns.
  const SortIcon = ({ id }) => {
    if (orderBy !== id) return <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  if (layout === 'list') {
    return (
      <ul className="divide-y divide-border">
        {tokens.map((item) => (
          <KeyListRow
            key={item.id}
            item={item}
            rtf={listRtf}
            selectMode={selectMode}
            selected={!!selectedIds?.has(item.id)}
            onToggleSelect={onToggleSelect}
            onOpen={onOpen}
            active={item.id === openId}
            actions={buildTokenActions({ item, t, navigate, onEdit, onDelete, onCopyKey, onShowUsage, onConnectOpenClaw, onToggleStatus })}
          />
        ))}
      </ul>
    );
  }

  return (
    <div>
      <Table ref={tableRef}>
        <TableHeader>
          {table.getHeaderGroups().map((hg) => (
            <TableRow key={hg.id} className="hover:bg-transparent">
              {hg.headers.map((header) => (
                <TableHead key={header.id} className={`whitespace-nowrap ${header.column.columnDef.meta?.className ?? ''}`}>
                  {header.column.columnDef.meta?.sortable && onSort ? (
                    <button
                      type="button"
                      className="group/sort inline-flex items-center gap-1 hover:text-foreground"
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
            <TableRow
              key={row.id}
              className={ROW_OPENS}
              data-state={row.original.id === openId || selectedIds?.has(row.original.id) ? 'selected' : undefined}
              {...rowOpenProps(() => onOpen(row.original))}
            >
              {row.getVisibleCells().map((cell) => (
                <TableCell key={cell.id} className={cn('whitespace-nowrap', cell.column.columnDef.meta?.className)}>
                  {flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
