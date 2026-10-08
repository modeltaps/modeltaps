import { Fragment, useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowLeftRight, Copy, DollarSign, Lock, MoreHorizontal, Network, Pencil, Trash2 } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { cn } from '@/lib/utils';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { ValueFormatter, copy, timestamp2string } from 'utils/common';
import { statusLabel } from '../Channel/channelApi';
import { enabledChannels, isVisibleToUsers, modelStatus } from '../Pricing/modelCatalog';
import {
  MODE_LABEL_KEYS,
  CapabilityBadges,
  EndpointBadges,
  ModalityBadges,
  ModelStatusBadge,
  TagBadges,
  VendorIcon,
  safeJsonArray
} from './modelInfoHelpers';

// 同步来源 → 文案 key;空值(历史数据)显示「未知」。
const SOURCE_LABEL_KEYS = {
  manual: 'modelInfoPage.sourceManual',
  'models.dev': 'modelInfoPage.sourceModelsDev',
  openrouter: 'modelInfoPage.sourceOpenRouter'
};

const MAX_CHIP_ICONS = 2;
const CHANNEL_STATUS_ENABLED = 1;

// 渠道页按渠道名筛选并通过 channel_id 直接打开该渠道的编辑抽屉。
export function channelEditPath(channel) {
  return `/panel/channel?name=${encodeURIComponent(channel.name || '')}&channel_id=${channel.id}`;
}

// 单个渠道的提示内容:渠道名、类型名称、状态,以及有值时的优先级 / 权重。
export function describeChannel(t, channel) {
  const lines = [
    channel.name,
    CHANNEL_OPTIONS[channel.type]?.text || `#${channel.type}`,
    statusLabel(t, channel.status ?? CHANNEL_STATUS_ENABLED)
  ];
  if (channel.priority != null) lines.push(`${t('channel_index.priority')}: ${channel.priority}`);
  if (channel.weight != null) lines.push(`${t('channel_index.weight')}: ${channel.weight}`);
  return lines;
}

const isChannelEnabled = (channel) => (channel.status ?? CHANNEL_STATUS_ENABLED) === CHANNEL_STATUS_ENABLED;

export function ChannelGlyph({ channel }) {
  return (
    <span className={cn('inline-flex shrink-0', !isChannelEnabled(channel) && 'opacity-40 grayscale')}>
      <ProviderIcon type={channel.type} name={channel.name} baseUrl={channel.base_url} className="size-4" />
    </span>
  );
}

ChannelGlyph.propTypes = { channel: PropTypes.object.isRequired };

// 渠道计数 chip:「N 个渠道」+ 前两个启用渠道图标,只计启用渠道;没有启用渠道显示警示色「未绑定」。
// 点击打开抽屉的「渠道与分组」Tab。
export function ChannelsCell({ channels, onOpen }) {
  const { t } = useTranslation();
  const enabled = enabledChannels({ bound_channels: Array.isArray(channels) ? channels : [] });
  const label = enabled.length > 0 ? t('modelsPage.channelCount', { count: enabled.length }) : t('modelsPage.unbound');
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={label}
      data-zero={enabled.length === 0 ? '' : undefined}
      className={cn(
        'inline-flex h-6 items-center gap-1 whitespace-nowrap rounded-md px-1.5 text-xs transition-colors',
        enabled.length > 0 ? 'bg-muted text-foreground hover:bg-muted/70' : 'bg-warning/15 text-warning hover:bg-warning/25'
      )}
    >
      {enabled.slice(0, MAX_CHIP_ICONS).map((channel) => (
        <ChannelGlyph key={channel.id} channel={channel} />
      ))}
      <span className="tabular-nums">{label}</span>
    </button>
  );
}

ChannelsCell.propTypes = { channels: PropTypes.array, onOpen: PropTypes.func };

function VendorCell({ vendor }) {
  const { t } = useTranslation();
  if (!vendor) return <span className="text-muted-foreground">{t('modelInfoPage.vendorUnknown')}</span>;
  return (
    <span className="flex max-w-[9rem] items-center gap-1.5 whitespace-nowrap">
      <VendorIcon vendor={vendor} />
      <span className="truncate">{vendor.name}</span>
    </span>
  );
}

VendorCell.propTypes = { vendor: PropTypes.object };

// 价格:按量为「入 / 出」两行 + 单位(K/M 即时换算),按次为单次价格;无价格显示「—」。
export function PriceCell({ price, unit, hideUnit = false }) {
  const { t } = useTranslation();
  if (!price) return <span className="text-muted-foreground">—</span>;
  const isM = unit === 'M';
  const lock = price.locked && <Lock className="size-3 shrink-0 text-warning" aria-label={t('pricing_edit.locked')} />;
  if (price.type !== 'tokens') {
    return (
      <span className="flex items-center gap-1 whitespace-nowrap tabular-nums">
        {ValueFormatter(price.input, true, false)}
        <span className="text-xs text-muted-foreground">/ {t('modelpricePage.times')}</span>
        {lock}
      </span>
    );
  }
  return (
    <span className="flex items-center gap-1.5 whitespace-nowrap text-xs tabular-nums">
      <span className="flex flex-col leading-4">
        <span>
          <span className="mr-1 text-muted-foreground">{t('modelsPage.priceIn')}</span>
          {ValueFormatter(price.input, true, isM)}
        </span>
        <span>
          <span className="mr-1 text-muted-foreground">{t('modelsPage.priceOut')}</span>
          {ValueFormatter(price.output, true, isM)}
        </span>
      </span>
      {!hideUnit && <span className="text-muted-foreground">/ 1{unit}</span>}
      {lock}
    </span>
  );
}

PriceCell.propTypes = { price: PropTypes.object, unit: PropTypes.string, hideUnit: PropTypes.bool };

// 厂商图标 + 主名(点击复制)两行:次行为 未入目录 / 别名指向 / 显示名 · N 别名 · 标签。
function ModelCell({ row, vendor }) {
  const { t } = useTranslation();
  let sub;
  if (!row.catalogId) {
    sub = <span className="text-warning">{t('modelInfoPage.notInCatalog')}</span>;
  } else if (row.alias_of) {
    sub = <span className="text-info">{t('modelsPage.aliasTo', { model: row.alias_of })}</span>;
  } else {
    const parts = [row.name && row.name !== row.model ? row.name : null];
    if (row.alias_count > 0) parts.push(t('modelInfoPage.aliasCount', { count: row.alias_count }));
    parts.push(...safeJsonArray(row.tags));
    sub = parts.filter(Boolean).join(' · ');
  }
  return (
    <div className="flex min-w-0 max-w-[20rem] items-center gap-2">
      <VendorIcon vendor={vendor} model={row.model} />
      <div className="flex min-w-0 flex-col">
        <span className="flex min-w-0 items-center gap-1">
          <button
            type="button"
            className="truncate font-mono text-[13px] font-medium hover:underline"
            title={row.model}
            onClick={() => copy(row.model, t('modelInfoPage.model'))}
          >
            {row.model}
          </button>
          {row.locked && <Lock className="size-3 shrink-0 text-warning" aria-label={t('modelInfoPage.locked')} />}
        </span>
        {sub && <span className="truncate text-xs text-muted-foreground">{sub}</span>}
      </div>
    </div>
  );
}

ModelCell.propTypes = { row: PropTypes.object, vendor: PropTypes.object };

const numberCell = (c) => <span className="tabular-nums">{c.getValue() || '-'}</span>;

// 价格列表头:点击在 1K / 1M tokens 之间切换单位,tooltip 提示切换目标。
export function PriceHeader({ unit, onUnitChange }) {
  const { t } = useTranslation();
  const label = t('modelsPage.priceHeader', { unit });
  if (!onUnitChange) return label;
  const nextUnit = unit === 'M' ? 'K' : 'M';
  const hint = t('modelsPage.priceUnitToggle', { unit: nextUnit });
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={() => onUnitChange(nextUnit)}
          aria-label={`${label} · ${hint}`}
          className="inline-flex items-center gap-1 whitespace-nowrap rounded-sm hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {label}
          <ArrowLeftRight className="size-3 opacity-60" />
        </button>
      </TooltipTrigger>
      <TooltipContent>{hint}</TooltipContent>
    </Tooltip>
  );
}

PriceHeader.propTypes = { unit: PropTypes.string, onUnitChange: PropTypes.func };

// 行尾操作:「定价」图标 + ⋯ 菜单(编辑 / 复制名称 / 删除价格 / 删除目录)。
function RowActionsCell({ row, onOpen, onDeleteCatalog, onDeletePrice }) {
  const { t } = useTranslation();
  return (
    <span className="flex items-center justify-end gap-0.5">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7"
        aria-label={t('modelsPage.setPrice')}
        title={t('modelsPage.setPrice')}
        onClick={() => onOpen(row, 'pricing')}
      >
        <DollarSign className="size-3.5" />
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="ghost" size="icon" className="size-7" aria-label={t('modelsPage.moreActions')}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem onClick={() => onOpen(row, 'profile')}>
            <Pencil className="size-4" />
            {t('modelsPage.editProfile')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => onOpen(row, 'channels')}>
            <Network className="size-4" />
            {t('modelsPage.channelsTab')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => copy(row.model, t('modelInfoPage.model'))}>
            <Copy className="size-4" />
            {t('modelsPage.copyName')}
          </DropdownMenuItem>
          {(row.price || row.catalogId > 0) && <DropdownMenuSeparator />}
          {row.price && (
            <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDeletePrice(row)}>
              <Trash2 className="size-4" />
              {t('modelInfoPage.deletePrice')}
            </DropdownMenuItem>
          )}
          {row.catalogId > 0 && (
            <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDeleteCatalog(row)}>
              <Trash2 className="size-4" />
              {t('modelInfoPage.deleteCatalog')}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    </span>
  );
}

RowActionsCell.propTypes = {
  row: PropTypes.object.isRequired,
  onOpen: PropTypes.func.isRequired,
  onDeleteCatalog: PropTypes.func.isRequired,
  onDeletePrice: PropTypes.func.isRequired
};

export default function ModelTable({
  items,
  vendors = {},
  unit = 'M',
  onUnitChange,
  columnVisibility = {},
  selectedKeys,
  onToggleSelect,
  onToggleAll,
  onToggleHidden,
  onOpen,
  onDeleteCatalog,
  onDeletePrice
}) {
  const { t } = useTranslation();

  const allSelected = items.length > 0 && items.every((item) => selectedKeys?.has(item.key));
  const someSelected = items.some((item) => selectedKeys?.has(item.key));

  const renderVisibleSwitch = (row) => (
    <Switch
      checked={isVisibleToUsers(row)}
      disabled={!(row.catalogId > 0)}
      onCheckedChange={(next) => onToggleHidden?.(row, !next)}
      aria-label={t('modelsPage.visibleToUsers')}
      title={row.catalogId > 0 ? t('modelsPage.visibleToUsers') : t('modelsPage.visibleNeedsCatalog')}
    />
  );

  const columns = useMemo(
    () => [
      {
        id: 'select',
        header: () => (
          <Checkbox
            checked={allSelected}
            indeterminate={!allSelected && someSelected}
            onCheckedChange={() => onToggleAll?.(!allSelected)}
            aria-label={t('modelInfoPage.selectAll')}
          />
        ),
        meta: { className: 'w-[1%]' },
        cell: (c) => (
          <Checkbox
            checked={!!selectedKeys?.has(c.row.original.key)}
            onCheckedChange={() => onToggleSelect?.(c.row.original.key)}
            aria-label={c.row.original.model}
          />
        )
      },
      {
        id: 'model',
        header: t('modelInfoPage.model'),
        cell: (c) => <ModelCell row={c.row.original} vendor={vendors[c.row.original.vendor_id]} />
      },
      {
        id: 'endpoints',
        header: t('modelInfoPage.endpoints'),
        cell: (c) => <EndpointBadges json={c.row.original.endpoints} />,
        meta: { cardFull: true }
      },
      {
        id: 'channels',
        header: t('modelInfoPage.channels'),
        cell: (c) => <ChannelsCell channels={c.row.original.bound_channels} onOpen={() => onOpen(c.row.original, 'channels')} />
      },
      {
        id: 'price',
        header: () => <PriceHeader unit={unit} onUnitChange={onUnitChange} />,
        cell: (c) => <PriceCell price={c.row.original.price} unit={unit} hideUnit />
      },
      {
        id: 'state',
        header: t('modelInfoPage.state'),
        meta: { className: 'w-24' },
        cell: (c) => <ModelStatusBadge status={modelStatus(c.row.original)} />
      },
      {
        id: 'visible',
        header: t('modelsPage.visibleToUsers'),
        meta: { className: 'w-28 min-w-28 whitespace-nowrap text-center' },
        cell: (c) => <span className="flex justify-center">{renderVisibleSwitch(c.row.original)}</span>
      },
      { id: 'vendor', header: t('modelInfoPage.vendor'), cell: (c) => <VendorCell vendor={vendors[c.row.original.vendor_id]} /> },
      { id: 'context_length', header: t('modelInfoPage.contextLength'), accessorKey: 'context_length', cell: numberCell },
      { id: 'max_tokens', header: t('modelInfoPage.maxTokens'), accessorKey: 'max_tokens', cell: numberCell },
      {
        id: 'modalities',
        header: t('modelInfoPage.modalities'),
        cell: (c) => <ModalityBadges input={c.row.original.input_modalities} output={c.row.original.output_modalities} />
      },
      { id: 'tags', header: t('modelInfoPage.tags'), cell: (c) => <TagBadges json={c.row.original.tags} /> },
      { id: 'capabilities', header: t('modelInfoPage.capabilities'), cell: (c) => <CapabilityBadges json={c.row.original.capabilities} /> },
      {
        id: 'mode',
        header: t('modelInfoPage.mode'),
        cell: (c) => {
          const value = c.row.original.mode || '';
          return (
            <span className={value ? '' : 'text-muted-foreground'}>{t(MODE_LABEL_KEYS[value] || 'modelInfoPage.modeOptions.unset')}</span>
          );
        }
      },
      {
        id: 'alias_of',
        header: t('modelInfoPage.aliasOf'),
        cell: (c) => {
          const row = c.row.original;
          if (row.alias_of) return <span className="whitespace-nowrap text-sky-600 dark:text-sky-400">→ {row.alias_of}</span>;
          if (row.alias_count > 0) {
            return (
              <Badge variant="secondary" className="whitespace-nowrap px-1.5 py-0 font-normal">
                {t('modelInfoPage.aliasCount', { count: row.alias_count })}
              </Badge>
            );
          }
          return <span className="text-muted-foreground">-</span>;
        }
      },
      {
        id: 'source',
        header: t('modelInfoPage.source'),
        cell: (c) => (c.row.original.catalogId ? t(SOURCE_LABEL_KEYS[c.row.original.source] || 'modelInfoPage.sourceUnknown') : '-')
      },
      {
        id: 'synced_at',
        header: t('modelInfoPage.syncedAt'),
        cell: (c) => <span className="tabular-nums">{c.row.original.synced_at ? timestamp2string(c.row.original.synced_at) : '-'}</span>
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('common.actions')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => <RowActionsCell row={c.row.original} onOpen={onOpen} onDeleteCatalog={onDeleteCatalog} onDeletePrice={onDeletePrice} />
      }
    ],
    [
      t,
      vendors,
      unit,
      onUnitChange,
      selectedKeys,
      allSelected,
      someSelected,
      onToggleAll,
      onToggleSelect,
      onToggleHidden,
      onOpen,
      onDeleteCatalog,
      onDeletePrice
    ]
  );

  const table = useTable({
    features: appTableFeatures,
    data: items,
    columns,
    getRowId: (row) => row.key,
    state: { columnVisibility }
  });

  // 窄屏卡片:左上选择框 + 模型名,右上操作;其余可见列按「标签 | 值」单列纵向排列。
  const cardCell = {
    price: (row) => <PriceCell price={row.price} unit={unit} hideUnit />,
    visible: (row) => renderVisibleSwitch(row)
  };
  const renderCard = (original) => {
    const row = table.getRow(original.key);
    const fields = row.getVisibleCells().filter((c) => !['select', 'model', 'actions'].includes(c.column.id));
    return (
      <>
        <div className="flex items-center gap-2">
          <Checkbox
            checked={!!selectedKeys?.has(original.key)}
            onCheckedChange={() => onToggleSelect?.(original.key)}
            aria-label={original.model}
          />
          <div className="min-w-0 flex-1">
            <ModelCell row={original} vendor={vendors[original.vendor_id]} />
          </div>
          <RowActionsCell row={original} onOpen={onOpen} onDeleteCatalog={onDeleteCatalog} onDeletePrice={onDeletePrice} />
        </div>
        {fields.length > 0 && (
          <dl className="mt-2 grid grid-cols-[6.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-1.5 pl-6">
            {fields.map((cell) => (
              <Fragment key={cell.id}>
                <dt className="text-xs font-medium text-muted-foreground">{flexRender(cell.column.columnDef.header, cell.getContext())}</dt>
                <dd className="flex min-w-0 flex-wrap items-center gap-1 text-sm">
                  {cardCell[cell.column.id]
                    ? cardCell[cell.column.id](original)
                    : flexRender(cell.column.columnDef.cell, cell.getContext())}
                </dd>
              </Fragment>
            ))}
          </dl>
        )}
      </>
    );
  };

  return (
    <TooltipProvider delayDuration={150}>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id} className="hover:bg-transparent">
                {hg.headers.map((header) => (
                  <TableHead key={header.id} className={`h-9 ${header.column.columnDef.meta?.className ?? ''}`}>
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow key={row.id} data-state={selectedKeys?.has(row.original.key) ? 'selected' : undefined}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id} className={`py-1.5 ${cell.column.columnDef.meta?.className ?? ''}`}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <DataCards table={table} className="md:hidden" renderCard={renderCard} />
    </TooltipProvider>
  );
}

ModelTable.propTypes = {
  items: PropTypes.array.isRequired,
  vendors: PropTypes.object,
  unit: PropTypes.string,
  onUnitChange: PropTypes.func,
  columnVisibility: PropTypes.object,
  selectedKeys: PropTypes.instanceOf(Set),
  onToggleSelect: PropTypes.func,
  onToggleAll: PropTypes.func,
  onToggleHidden: PropTypes.func,
  onOpen: PropTypes.func.isRequired,
  onDeleteCatalog: PropTypes.func.isRequired,
  onDeletePrice: PropTypes.func.isRequired
};
