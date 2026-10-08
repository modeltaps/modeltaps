import { useState } from 'react';
import {
  Pencil,
  FlaskConical,
  Copy,
  Trash2,
  DollarSign,
  Globe,
  ListChecks,
  HelpCircle,
  Loader2,
  AlertTriangle,
  PlusCircle,
  KeyRound,
  Box
} from 'lucide-react';

import { renderSpend, timestamp2string } from 'utils/common';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Checkbox } from '@/components/ui/checkbox';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { RowActions } from '@/components/ui/row-actions';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { statusLabel } from './channelApi';

// 渠道 models 字段为逗号分隔字符串;空项不计。
export const countModels = (models) =>
  (models || '')
    .split(',')
    .map((m) => m.trim())
    .filter(Boolean).length;

const STATUS_DOT = { 1: 'bg-success', 2: 'bg-muted-foreground', 3: 'bg-destructive' };

// 状态:小圆点 + 文字(1 启用 / 2 手动禁用 / 3 自动禁用)。
export function StatusDot({ t, status }) {
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-xs">
      <span className={cn('size-1.5 rounded-full', STATUS_DOT[status] || 'bg-muted-foreground')} />
      <span className={status === 3 ? 'text-destructive' : status === 1 ? 'text-foreground' : 'text-muted-foreground'}>
        {statusLabel(t, status)}
      </span>
    </span>
  );
}

// 模型数 chip;可点击时进入编辑抽屉的「模型」Tab。
export function ModelCountChip({ count, onClick, label }) {
  const className = cn(
    'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs tabular-nums',
    count > 0 ? 'border-border bg-card text-foreground' : 'border-dashed border-border text-muted-foreground',
    onClick && 'hover:bg-muted'
  );
  const content = (
    <>
      <Box className="size-3" />
      {count}
    </>
  );
  if (!onClick) return <span className={className}>{content}</span>;
  return (
    <button type="button" className={className} aria-label={label} title={label} onClick={onClick}>
      {content}
    </button>
  );
}

export function NumberCell({ value, onCommit }) {
  const [val, setVal] = useState(value);
  return (
    <Input
      type="number"
      value={val}
      onChange={(e) => setVal(e.target.value)}
      onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
      onBlur={() => val !== value && onCommit(val)}
      className="h-8 w-16 px-2"
    />
  );
}

export function ResponseTime({ ms }) {
  if (!ms) return <span className="text-xs text-muted-foreground">-</span>;
  const s = ms / 1000;
  // v1 ResponseTimeLabel tiers: ≤1s success, ≤3s primary, ≤5s secondary, else error.
  const color =
    ms <= 1000
      ? 'text-emerald-600 dark:text-emerald-400'
      : ms <= 3000
        ? 'text-foreground'
        : ms <= 5000
          ? 'text-amber-600 dark:text-amber-400'
          : 'text-destructive';
  return <span className={`text-xs ${color}`}>{s.toFixed(2)}s</span>;
}

export function PriorityWeightTip({ t }) {
  return (
    <div className="space-y-1">
      <p>{t('channel_index.priorityWeightExplanation')}</p>
      <p>{t('channel_index.description1')}</p>
      <p>{t('channel_index.description2')}</p>
      <p>{t('channel_index.description3')}</p>
      <p>{t('channel_index.description4')}</p>
    </div>
  );
}

function HeaderWithTip({ label, tip }) {
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex items-center gap-1">
            {label}
            <HelpCircle className="size-3.5 text-muted-foreground" />
          </span>
        </TooltipTrigger>
        <TooltipContent className="max-w-sm whitespace-normal text-xs">{tip}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function renderBalance(type, balance) {
  switch (type) {
    case 28: // Deepseek
      return <>¥{balance}</>;
    case 45: // Deepseek
      return <>¥{balance}</>;
    default:
      return <>${balance.toFixed(2)}</>;
  }
}

// Amber warning shown next to the channel name when the last drift check found
// models the channel still advertises but the upstream no longer returns
// (`row.model_drift = { checked_at, missing_models, ok }`, brought out by
// MODEL-1a). Three states: never checked (no `model_drift`) or clean
// (`ok`/empty) render nothing; drifted renders the badge + tooltip listing the
// missing models and the check time.
function DriftBadge({ t, drift }) {
  const missing = drift?.missing_models;
  if (!missing || missing.length === 0) return null;
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge variant="outline" className="gap-1 border-amber-500/50 text-amber-600 dark:text-amber-400">
            <AlertTriangle className="size-3" />
            {t('channel_row.driftMissing', { count: missing.length })}
          </Badge>
        </TooltipTrigger>
        <TooltipContent className="max-w-xs whitespace-normal text-xs">
          <div className="space-y-1">
            <p className="font-medium">{t('channel_row.driftMissingTitle')}</p>
            <p>{missing.join(', ')}</p>
            {drift.checked_at ? (
              <p className="text-muted-foreground">
                {t('channel_row.driftCheckedAt')}: {timestamp2string(drift.checked_at)}
              </p>
            ) : null}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

// Sky info badge shown next to the channel name when the last drift check found
// models the upstream provider newly offers but the channel does not carry yet
// (`row.model_drift.new_models`, MODEL-3). Empty/absent renders nothing. The
// tooltip lists at most `NEW_MODELS_TOOLTIP_LIMIT` names plus a "…and N more"
// line, the check time, and a dismiss button clearing the badge for that row.
const NEW_MODELS_TOOLTIP_LIMIT = 20;

function NewModelsBadge({ t, drift, onDismiss, dismissing }) {
  const newModels = drift?.new_models;
  if (!newModels || newModels.length === 0) return null;
  const shown = newModels.slice(0, NEW_MODELS_TOOLTIP_LIMIT);
  const rest = newModels.length - shown.length;
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge variant="outline" className="gap-1 border-sky-500/50 text-sky-600 dark:text-sky-400">
            <PlusCircle className="size-3" />
            {t('channel_row.newModels', { count: newModels.length })}
          </Badge>
        </TooltipTrigger>
        <TooltipContent className="max-w-xs whitespace-normal text-xs">
          <div className="space-y-1">
            <p className="font-medium">{t('channel_row.newModelsTitle')}</p>
            <p>{shown.join(', ')}</p>
            {rest > 0 ? <p className="text-muted-foreground">{t('channel_row.newModelsMore', { count: rest })}</p> : null}
            {drift.checked_at ? (
              <p className="text-muted-foreground">
                {t('channel_row.driftCheckedAt')}: {timestamp2string(drift.checked_at)}
              </p>
            ) : null}
            <Button variant="ghost" size="sm" className="h-6 w-full px-2 text-xs" disabled={dismissing} onClick={onDismiss}>
              {t('channel_row.newModelsDismiss')}
            </Button>
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

// Amber warning shown next to the channel name when the stored key is empty
// (`row.key_status.configured === false`). Rows without `key_status` render nothing.
export function KeyMissingBadge({ t, keyStatus }) {
  if (!keyStatus || keyStatus.configured) return null;
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge variant="outline" className="gap-1 border-amber-500/50 text-amber-600 dark:text-amber-400">
            <KeyRound className="size-3" />
            {t('channel_row.keyMissing')}
          </Badge>
        </TooltipTrigger>
        <TooltipContent className="max-w-xs whitespace-normal text-xs">{t('channel_row.keyMissingTip')}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

export function getColumns({ t, handlers, testingIds, driftCheckingIds, dismissingIds }) {
  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllRowsSelected()}
          indeterminate={table.getIsSomeRowsSelected() && !table.getIsAllRowsSelected()}
          onCheckedChange={(v) => table.toggleAllRowsSelected(!!v)}
        />
      ),
      cell: ({ row }) => <Checkbox checked={row.getIsSelected()} onCheckedChange={(v) => row.toggleSelected(!!v)} />,
      enableSorting: false
    },
    {
      accessorKey: 'id',
      header: 'ID',
      cell: ({ row }) => <span className="font-mono text-xs text-muted-foreground">{row.original.id}</span>
    },
    {
      accessorKey: 'name',
      header: t('channel_index.name'),
      cell: ({ row }) => {
        const c = row.original;
        const opt = CHANNEL_OPTIONS[c.type];
        return (
          <div className="flex min-w-0 items-center gap-2.5">
            <ProviderIcon type={c.type} name={c.name} baseUrl={c.base_url} className="size-6 flex-none" />
            <div className="flex min-w-0 flex-col">
              <span className="max-w-[16rem] truncate font-medium" title={c.name}>
                {c.name}
              </span>
              <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
                <span>{opt ? opt.text : t('common.unknown')}</span>
                <KeyMissingBadge t={t} keyStatus={c.key_status} />
                <DriftBadge t={t} drift={c.model_drift} />
                <NewModelsBadge
                  t={t}
                  drift={c.model_drift}
                  dismissing={dismissingIds?.has(c.id)}
                  onDismiss={() => handlers.onDismissNewModels(c)}
                />
              </div>
            </div>
          </div>
        );
      }
    },
    {
      accessorKey: 'group',
      header: t('channel_index.group'),
      enableSorting: false,
      meta: { cardFull: true },
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1">
          {(row.original.group || '')
            .split(',')
            .filter(Boolean)
            .map((g) => (
              <Badge key={g} variant="secondary">
                {g}
              </Badge>
            ))}
        </div>
      )
    },
    {
      id: 'models',
      header: t('channel_index.model'),
      enableSorting: false,
      cell: ({ row }) => (
        <ModelCountChip
          count={countModels(row.original.models)}
          label={t('channelPage.openModels')}
          onClick={() => handlers.onEdit(row.original, 'model')}
        />
      )
    },
    {
      accessorKey: 'priority',
      header: () => <HeaderWithTip label={t('channel_index.priority')} tip={<PriorityWeightTip t={t} />} />,
      cell: ({ row }) => <NumberCell value={row.original.priority} onCommit={(v) => handlers.onUpdatePriority(row.original, v)} />
    },
    {
      accessorKey: 'weight',
      header: () => <HeaderWithTip label={t('channel_index.weight')} tip={<PriorityWeightTip t={t} />} />,
      cell: ({ row }) => <NumberCell value={row.original.weight} onCommit={(v) => handlers.onUpdateWeight(row.original, v)} />
    },
    {
      accessorKey: 'response_time',
      header: t('channel_index.responseTime'),
      cell: ({ row }) =>
        testingIds?.has(row.original.id) ? (
          <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
            <Loader2 className="size-3 animate-spin" />
            {t('channel_row.testing', { defaultValue: 'Testing…' })}
          </span>
        ) : (
          <ResponseTime ms={row.original.response_time} />
        )
    },
    {
      id: 'used',
      header: t('channel_index.usedBalance'),
      enableSorting: false,
      cell: ({ row }) => (
        <div className="flex flex-col gap-0.5 text-sm tabular-nums">
          <span>{renderSpend(row.original.used_quota)}</span>
          <span className="text-xs text-muted-foreground">{renderBalance(row.original.type, Number(row.original.balance || 0))}</span>
        </div>
      )
    },
    {
      accessorKey: 'status',
      header: t('channel_index.status'),
      cell: ({ row }) => <StatusDot t={t} status={row.original.status} />
    },
    {
      id: 'enabled',
      header: t('channelPage.enabledColumn'),
      enableSorting: false,
      meta: { className: 'w-16' },
      cell: ({ row }) => (
        <Switch
          checked={row.original.status === 1}
          aria-label={t('channelPage.enabledColumn')}
          onCheckedChange={() => handlers.onToggleStatus(row.original)}
        />
      )
    },
    {
      id: 'actions',
      header: <span className="sr-only">{t('channel_index.actions')}</span>,
      enableSorting: false,
      meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
      cell: ({ row }) => {
        const webUrl = CHANNEL_OPTIONS[row.original.type]?.url;
        const testing = testingIds?.has(row.original.id);
        // 行内只留「测试」,其余(编辑 / 排查 / 漂移检查 / 余额 / 官网 / 复制 / 删除)收进 ⋯ 菜单。
        const actions = [
          {
            label: t('channel_index.test', { defaultValue: 'Test' }),
            icon: FlaskConical,
            primary: true,
            disabled: testing,
            onClick: () => handlers.onTest(row.original)
          },
          {
            label: t('common.edit'),
            icon: Pencil,
            onClick: () => handlers.onEdit(row.original)
          },
          {
            label: t('channel_row.check'),
            icon: ListChecks,
            onClick: () => handlers.onCheck(row.original)
          },
          {
            label: t('channel_row.driftCheck'),
            icon: AlertTriangle,
            disabled: driftCheckingIds?.has(row.original.id),
            onClick: () => handlers.onCheckDrift(row.original)
          },
          {
            label: t('channel_index.updateBalance', { defaultValue: 'Update Balance' }),
            icon: DollarSign,
            onClick: () => handlers.onRefreshBalance(row.original)
          },
          {
            label: t('channel_row.channelWeb'),
            icon: Globe,
            disabled: !webUrl,
            onClick: () => webUrl && window.open(webUrl, '_blank', 'noopener,noreferrer')
          },
          {
            label: t('common.copy', { defaultValue: 'Copy' }),
            icon: Copy,
            onClick: () => handlers.onCopy(row.original)
          },
          {
            label: t('common.delete'),
            icon: Trash2,
            destructive: true,
            onClick: () => handlers.onDelete(row.original)
          }
        ].map((a) => (a.primary ? a : { ...a, overflow: true }));
        return <RowActions actions={actions} />;
      }
    }
  ];
}
