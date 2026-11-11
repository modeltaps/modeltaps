import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowLeftRight, Copy, DollarSign, Eye, EyeOff, Layers, Lock, MoreHorizontal, Network, Pencil, Trash2 } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
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
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { estimateWidth, useNaturalWidth } from '@/components/list-page';
import { cn } from '@/lib/utils';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { ValueFormatter, copy, timestamp2string } from 'utils/common';
import { statusLabel } from '../Channel/channelApi';
import { enabledChannels, followsStatus, isVisibleToUsers, modelStatus } from '../Pricing/modelCatalog';
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
import { ModelStatusPill } from './supply/supplyParts';
import { isPublicModelRow, normalizeSupplyMode } from './supply/supplyHelpers';

// 同步来源 → 文案 key;空值(历史数据)显示「未知」。
const SOURCE_LABEL_KEYS = {
  manual: 'modelInfoPage.sourceManual',
  'models.dev': 'modelInfoPage.sourceModelsDev',
  openrouter: 'modelInfoPage.sourceOpenRouter'
};

const MAX_CHIP_ICONS = 2;
const CHANNEL_STATUS_ENABLED = 1;

// The channel's own page.
export function channelEditPath(channel) {
  return `/panel/channel/${channel.id}`;
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

// Public status and number of supplies of a public model on one line (the supply mode is in the
// tooltip); opens the Supplies tab. Aliases and routing policies have no supplies of their own.
export function SupplyCell({ row, onOpen }) {
  const { t } = useTranslation();
  if (!isPublicModelRow(row)) return <span className="text-muted-foreground">—</span>;
  const counts = row.supplyCounts || { total: 0, usable: 0 };
  return (
    <button
      type="button"
      onClick={onOpen}
      title={t(`modelSupply.mode.${normalizeSupplyMode(row.supply_mode)}`)}
      className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-md text-left hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <ModelStatusPill status={row.status} />
      <span className={cn('text-xs text-muted-foreground', counts.total > 0 && counts.usable === 0 && 'text-warning')}>
        · {t('modelSupply.count', { count: counts.total })}
      </span>
    </button>
  );
}

SupplyCell.propTypes = { row: PropTypes.object.isRequired, onOpen: PropTypes.func };

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

// Price on one line: per token "input / output" in the chosen unit (the header names it), per call
// the single price; a dash without a price.
export function PriceCell({ price, unit, hideUnit = false }) {
  const { t } = useTranslation();
  if (!price) return <span className="text-muted-foreground">—</span>;
  const isM = unit === 'M';
  const lock = price.locked && <Lock className="size-3 shrink-0 text-warning" aria-label={t('pricing_edit.locked')} />;
  if (price.type !== 'tokens') {
    return (
      <span className="inline-flex items-center gap-1 whitespace-nowrap tabular-nums">
        {ValueFormatter(price.input, true, false)}
        <span className="text-xs text-muted-foreground">/ {t('modelpricePage.times')}</span>
        {lock}
      </span>
    );
  }
  return (
    <span
      className="inline-flex items-center gap-1 whitespace-nowrap tabular-nums"
      title={`${t('modelsPage.priceIn')} ${ValueFormatter(price.input, true, isM)} · ${t('modelsPage.priceOut')} ${ValueFormatter(price.output, true, isM)}`}
    >
      {ValueFormatter(price.input, true, isM)}
      <span className="text-muted-foreground">/</span>
      {ValueFormatter(price.output, true, isM)}
      {!hideUnit && <span className="text-xs text-muted-foreground">· 1{unit}</span>}
      {lock}
    </span>
  );
}

PriceCell.propTypes = { price: PropTypes.object, unit: PropTypes.string, hideUnit: PropTypes.bool };

// Second part of the model cell: not in the catalog / alias target / display name · aliases · tags.
function modelSubtitle(t, row) {
  if (!row.catalogId) return <span className="text-warning">{t('modelInfoPage.notInCatalog')}</span>;
  if (row.alias_of) return <span className="text-info">{t('modelsPage.aliasTo', { model: row.alias_of })}</span>;
  const parts = [row.name && row.name !== row.model ? row.name : null];
  if (row.alias_count > 0) parts.push(t('modelInfoPage.aliasCount', { count: row.alias_count }));
  parts.push(...safeJsonArray(row.tags));
  return parts.filter(Boolean).join(' · ') || null;
}

// One line: vendor icon, the model name in mono (click copies it), then the subtitle, muted.
function ModelCell({ row, vendor }) {
  const { t } = useTranslation();
  const sub = modelSubtitle(t, row);
  return (
    <div className="flex min-w-0 max-w-[18rem] items-center gap-2 whitespace-nowrap">
      <VendorIcon vendor={vendor} model={row.model} />
      {/* The name keeps its width (up to a cap); the subtitle takes what is left and truncates. */}
      <RowControl className="min-w-0 max-w-[15rem] shrink-0">
        <button
          type="button"
          className="max-w-full truncate font-mono text-[13px] font-medium hover:underline"
          title={row.model}
          onClick={() => copy(row.model, t('modelInfoPage.model'))}
        >
          {row.model}
        </button>
      </RowControl>
      {row.locked && <Lock className="size-3 shrink-0 text-warning" aria-label={t('modelInfoPage.locked')} />}
      {sub && <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">{sub}</span>}
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

// Row menu: price, profile, channels, supplies, copy the name; with `onToggleHidden` (the phone
// list, which has no switch) also show / hide; then the deletes, destructive and last.
export function RowActionsCell({ row, onOpen, onDeleteCatalog, onDeletePrice, onToggleHidden }) {
  const { t } = useTranslation();
  const canToggle = onToggleHidden && row.catalogId > 0 && !followsStatus(row);
  const visible = isVisibleToUsers(row);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('modelsPage.moreActions')}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuItem onClick={() => onOpen(row, 'pricing')}>
          <DollarSign className="size-4" />
          {t('modelsPage.setPrice')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onOpen(row, 'profile')}>
          <Pencil className="size-4" />
          {t('modelsPage.editProfile')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onOpen(row, 'channels')}>
          <Network className="size-4" />
          {t('modelsPage.channelsTab')}
        </DropdownMenuItem>
        {isPublicModelRow(row) && (
          <DropdownMenuItem onClick={() => onOpen(row, 'supplies')}>
            <Layers className="size-4" />
            {t('modelSupply.tab')}
          </DropdownMenuItem>
        )}
        {canToggle && (
          <DropdownMenuItem onClick={() => onToggleHidden(row, visible)}>
            {visible ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            {visible ? t('modelsPage.hideFromUsers') : t('modelsPage.showToUsers')}
          </DropdownMenuItem>
        )}
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
  );
}

RowActionsCell.propTypes = {
  row: PropTypes.object.isRequired,
  onOpen: PropTypes.func.isRequired,
  onDeleteCatalog: PropTypes.func.isRequired,
  onDeletePrice: PropTypes.func.isRequired,
  onToggleHidden: PropTypes.func
};

// Typical column widths, for the layout estimate before the table has been measured.
const COLUMN_WIDTHS = {
  select: 40,
  model: 288,
  endpoints: 180,
  channels: 120,
  supply: 150,
  price: 120,
  state: 96,
  visible: 96,
  vendor: 140,
  actions: 52
};
export const estimateModelTableWidth = (columnIds, visibility) => estimateWidth(columnIds, visibility, COLUMN_WIDTHS);

function VisibleSwitch({ row, onToggleHidden }) {
  const { t } = useTranslation();
  let title = t('modelsPage.visibleToUsers');
  if (followsStatus(row)) title = t('modelsPage.visibleFollowsStatus');
  else if (!(row.catalogId > 0)) title = t('modelsPage.visibleNeedsCatalog');
  return (
    <Switch
      size="sm"
      checked={isVisibleToUsers(row)}
      disabled={!(row.catalogId > 0) || followsStatus(row)}
      onCheckedChange={(next) => onToggleHidden?.(row, !next)}
      aria-label={t('modelsPage.visibleToUsers')}
      title={title}
    />
  );
}

VisibleSwitch.propTypes = { row: PropTypes.object.isRequired, onToggleHidden: PropTypes.func };

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
  onDeletePrice,
  onNaturalWidth
}) {
  const { t } = useTranslation();

  const allSelected = items.length > 0 && items.every((item) => selectedKeys?.has(item.key));
  const someSelected = items.some((item) => selectedKeys?.has(item.key));

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
        meta: { className: 'w-[1%] pr-0' },
        cell: (c) => (
          <RowControl className="flex">
            <Checkbox
              checked={!!selectedKeys?.has(c.row.original.key)}
              onCheckedChange={() => onToggleSelect?.(c.row.original.key)}
              aria-label={c.row.original.model}
            />
          </RowControl>
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
        cell: (c) => (
          <span className="flex flex-nowrap">
            <EndpointBadges json={c.row.original.endpoints} />
          </span>
        )
      },
      {
        id: 'channels',
        header: t('modelInfoPage.channels'),
        cell: (c) => (
          <RowControl>
            <ChannelsCell channels={c.row.original.bound_channels} onOpen={() => onOpen(c.row.original, 'channels')} />
          </RowControl>
        )
      },
      {
        id: 'supply',
        header: t('modelSupply.column'),
        cell: (c) => (
          <RowControl>
            <SupplyCell row={c.row.original} onOpen={() => onOpen(c.row.original, 'supplies')} />
          </RowControl>
        )
      },
      {
        id: 'price',
        header: () => <PriceHeader unit={unit} onUnitChange={onUnitChange} />,
        meta: { className: 'text-right' },
        cell: (c) => <PriceCell price={c.row.original.price} unit={unit} hideUnit />
      },
      {
        id: 'state',
        header: t('modelInfoPage.state'),
        cell: (c) => <ModelStatusBadge status={modelStatus(c.row.original)} />
      },
      {
        id: 'visible',
        header: t('modelsPage.visibleToUsers'),
        cell: (c) => (
          <RowControl className="flex">
            <VisibleSwitch row={c.row.original} onToggleHidden={onToggleHidden} />
          </RowControl>
        )
      },
      { id: 'vendor', header: t('modelInfoPage.vendor'), cell: (c) => <VendorCell vendor={vendors[c.row.original.vendor_id]} /> },
      {
        id: 'context_length',
        header: t('modelInfoPage.contextLength'),
        accessorKey: 'context_length',
        meta: { className: 'text-right' },
        cell: numberCell
      },
      {
        id: 'max_tokens',
        header: t('modelInfoPage.maxTokens'),
        accessorKey: 'max_tokens',
        meta: { className: 'text-right' },
        cell: numberCell
      },
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
          if (row.alias_of) return <span className="text-muted-foreground">→ {row.alias_of}</span>;
          if (row.alias_count > 0) {
            return (
              <Badge variant="secondary" className="px-1.5 py-0 font-normal">
                {t('modelInfoPage.aliasCount', { count: row.alias_count })}
              </Badge>
            );
          }
          return <span className="text-muted-foreground">—</span>;
        }
      },
      {
        id: 'source',
        header: t('modelInfoPage.source'),
        cell: (c) => (c.row.original.catalogId ? t(SOURCE_LABEL_KEYS[c.row.original.source] || 'modelInfoPage.sourceUnknown') : '—')
      },
      {
        id: 'synced_at',
        header: t('modelInfoPage.syncedAt'),
        cell: (c) => <span className="tabular-nums">{c.row.original.synced_at ? timestamp2string(c.row.original.synced_at) : '—'}</span>
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">{t('common.actions')}</span>,
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => (
          <RowControl className="justify-end">
            <RowActionsCell row={c.row.original} onOpen={onOpen} onDeleteCatalog={onDeleteCatalog} onDeletePrice={onDeletePrice} />
          </RowControl>
        )
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

  const tableRef = useNaturalWidth(onNaturalWidth);
  const table = useTable({
    features: appTableFeatures,
    data: items,
    columns,
    getRowId: (row) => row.key,
    state: { columnVisibility }
  });

  return (
    <TooltipProvider delayDuration={150}>
      <Table ref={tableRef}>
        <TableHeader>
          {table.getHeaderGroups().map((hg) => (
            <TableRow key={hg.id} className="hover:bg-transparent">
              {hg.headers.map((header) => (
                <TableHead key={header.id} className={cn('whitespace-nowrap', header.column.columnDef.meta?.className)}>
                  {flexRender(header.column.columnDef.header, header.getContext())}
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
              data-state={selectedKeys?.has(row.original.key) ? 'selected' : undefined}
              {...rowOpenProps(() => onOpen(row.original, 'profile'))}
            >
              {row.getVisibleCells().map((cell) => (
                <TableCell key={cell.id} className={cn('whitespace-nowrap', cell.column.columnDef.meta?.className)}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
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
  onDeletePrice: PropTypes.func.isRequired,
  onNaturalWidth: PropTypes.func
};

const STATE_DOT = { ok: 'bg-success', nochannel: 'bg-warning', unpriced: 'bg-warning' };

// Phone list: one 56px row per model. The state dot leads (a checkbox in select mode); the first
// line is the model name with the price on the right, the second the subtitle (display name,
// aliases, tags) with the state on the right. A tap opens the model; ⋯ holds the row actions,
// including show / hide since there is no switch.
export function ModelList({
  items,
  unit = 'M',
  selectMode,
  selectedKeys,
  onToggleSelect,
  onToggleHidden,
  onOpen,
  onDeleteCatalog,
  onDeletePrice
}) {
  const { t } = useTranslation();
  return (
    <ul className="divide-y divide-border">
      {items.map((row) => {
        const selected = selectedKeys.has(row.key);
        const status = modelStatus(row);
        const sub = modelSubtitle(t, row);
        return (
          <li
            key={row.key}
            className={cn('flex h-14 items-center gap-3 px-4', ROW_OPENS, selected && 'bg-muted/60')}
            {...rowOpenProps(() => (selectMode ? onToggleSelect(row.key) : onOpen(row, 'profile')))}
          >
            {selectMode ? (
              <RowControl>
                <Checkbox checked={selected} onCheckedChange={() => onToggleSelect(row.key)} aria-label={row.model} />
              </RowControl>
            ) : (
              <span className={cn('size-2 shrink-0 rounded-full', STATE_DOT[status])} aria-hidden="true" />
            )}
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span
                  className={cn('min-w-0 truncate font-mono text-[13px] font-medium', !isVisibleToUsers(row) && 'text-muted-foreground')}
                >
                  {row.model}
                </span>
                <span className="ml-auto shrink-0 text-[13px]">
                  <PriceCell price={row.price} unit={unit} hideUnit />
                </span>
              </div>
              <div className="mt-0.5 flex items-baseline gap-3 text-xs text-muted-foreground">
                <span className="min-w-0 truncate">{sub}</span>
                <span className="ml-auto shrink-0">
                  {t(`modelsPage.status.${status}`)}
                  {!isVisibleToUsers(row) && ` · ${t('modelsPage.stats.hidden')}`}
                </span>
              </div>
            </div>
            {!selectMode && (
              <RowControl className="-mr-2 shrink-0">
                <RowActionsCell
                  row={row}
                  onOpen={onOpen}
                  onDeleteCatalog={onDeleteCatalog}
                  onDeletePrice={onDeletePrice}
                  onToggleHidden={onToggleHidden}
                />
              </RowControl>
            )}
          </li>
        );
      })}
    </ul>
  );
}

ModelList.propTypes = {
  items: PropTypes.array.isRequired,
  unit: PropTypes.string,
  selectMode: PropTypes.bool,
  selectedKeys: PropTypes.instanceOf(Set).isRequired,
  onToggleSelect: PropTypes.func.isRequired,
  onToggleHidden: PropTypes.func,
  onOpen: PropTypes.func.isRequired,
  onDeleteCatalog: PropTypes.func.isRequired,
  onDeletePrice: PropTypes.func.isRequired
};
