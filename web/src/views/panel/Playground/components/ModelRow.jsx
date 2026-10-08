import { ChevronDown } from 'lucide-react';
import PropTypes from 'prop-types';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Checkbox } from '@/components/ui/checkbox';
import { cn } from '@/lib/utils';
import { ValueFormatter } from 'utils/common';
import { AVAILABILITY } from '../shared/modelIndex';
import StatusDot from './StatusDot';

// ==============================|| PLAYGROUND — MODEL ROW ||============================== //
// 选择器右列的一行模型：状态点 / 名称 / 上下文 / 输入 / 输出单价，名称下方一行小字放渠道数与展开入口。
// 缺失字段留空——目录没给的东西不编。
// multi 模式在行首补一个 checkbox，与整行点击同一个 onPick 切换。
// 行上可带可选的 badge 文案（如图像页的「对话出图」），由上层注入。

// 名称 / 上下文 / 输入 / 输出四列，列表顶部的列标题共用同一份模板，数字才能跨行纵向对齐。
export const ROW_COLUMNS = 'grid grid-cols-[minmax(0,1fr)_3rem_3.75rem_3.75rem] gap-x-2';

export const formatContext = (length) => {
  if (!length) return null;
  if (length >= 1000000) return `${Math.round(length / 100000) / 10}M`;
  if (length >= 1000) return `${Math.round(length / 1000)}K`;
  return String(length);
};

// 按量计费给「输入 / 输出」两个单价（每 1M tokens），按次计费只有一个单次价。
export const formatPricing = (pricing, t) => {
  if (!pricing) return null;
  const one = (value) => {
    if (value === undefined) return '—';
    if (value === 0) return t('modelpricePage.free');
    return ValueFormatter(value, true, pricing.type !== 'times');
  };
  return pricing.type === 'times' ? one(pricing.input) : `${one(pricing.input)} / ${one(pricing.output)}`;
};

// 行内的两列价格：按量计费拆成输入 / 输出，按次计费只占输入列、输出列标「按次」。
function PriceCells({ pricing }) {
  const { t } = useTranslation();
  if (!pricing) {
    return (
      <span className="col-span-2 truncate text-right text-xs leading-5 text-muted-foreground">{t('playground.picker.priceUnknown')}</span>
    );
  }
  if (pricing.type === 'times') {
    return (
      <>
        <span className="text-right text-xs font-medium leading-5 tabular-nums">{formatPricing(pricing, t)}</span>
        <span className="text-right text-xs leading-5 text-muted-foreground">{t('playground.picker.perCall')}</span>
      </>
    );
  }
  const [input, output] = formatPricing(pricing, t).split(' / ');
  return (
    <>
      <span className="text-right text-xs font-medium leading-5 tabular-nums">{input}</span>
      <span className="text-right text-xs font-medium leading-5 tabular-nums">{output}</span>
    </>
  );
}

PriceCells.propTypes = { pricing: PropTypes.object };

export default function ModelRow({ model, selected = false, active = false, multi = false, onPick }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);

  const unavailable = model.availability === AVAILABILITY.unavailable;
  const context = formatContext(model.contextLength);
  const channels = model.channels || [];
  const extra = channels.slice(1);

  // 名称下方固定一行小字：不可用提示 / 渠道数 / 多渠道展开入口共用这一处，行高不随内容跳动。
  let meta = null;
  if (unavailable) {
    meta = t('playground.picker.noChannel');
  } else if (extra.length > 0) {
    meta = (
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        aria-expanded={expanded}
        className="inline-flex items-center gap-0.5 hover:text-foreground hover:underline"
      >
        {t('playground.picker.channels', { count: channels.length })}
        <ChevronDown className={cn('size-3 transition-transform', expanded && 'rotate-180')} />
      </button>
    );
  } else if (channels.length > 0) {
    meta = t('playground.picker.channels', { count: channels.length });
  }

  return (
    <div
      className={cn(
        'rounded-lg border border-transparent px-3 py-1.5 transition-colors',
        active && 'border-border bg-muted/60',
        selected && 'border-primary/40 bg-primary/5',
        unavailable && 'opacity-60'
      )}
      data-active={active || undefined}
    >
      <div className="flex items-start gap-2">
        {multi && (
          <Checkbox
            checked={selected}
            onCheckedChange={() => onPick?.(model)}
            aria-label={model.id}
            tabIndex={-1}
            className="mt-0.5 shrink-0"
          />
        )}
        <div className="min-w-0 flex-1">
          <button
            type="button"
            role="option"
            aria-selected={selected}
            onClick={() => onPick?.(model)}
            className={cn(ROW_COLUMNS, 'w-full items-start text-left')}
          >
            <span className="flex min-w-0 items-center gap-2">
              <StatusDot availability={model.availability} />
              <span className="truncate font-mono text-sm font-medium leading-5" title={model.id}>
                {model.id}
              </span>
              {model.badge && (
                <span className="shrink-0 rounded border border-primary/30 px-1.5 text-[0.6875rem] leading-4 text-primary">
                  {model.badge}
                </span>
              )}
            </span>
            <span className="text-right text-xs leading-5 tabular-nums text-muted-foreground">{context || ''}</span>
            <PriceCells pricing={model.pricing} />
          </button>

          <div className="h-4 truncate pl-4 text-[0.6875rem] leading-4 text-muted-foreground">{meta}</div>

          {expanded && (
            <ul className="mt-1 space-y-0.5 pl-4">
              {extra.map((channel, index) => (
                <li key={channel.id ?? channel.name ?? index} className="flex justify-between text-[0.6875rem] text-muted-foreground">
                  <span className="truncate">{channel.name || channel.id}</span>
                  <span className="tabular-nums">{formatPricing(channel.pricing, t) || t('playground.picker.priceUnknown')}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}

ModelRow.propTypes = {
  model: PropTypes.object.isRequired,
  selected: PropTypes.bool,
  active: PropTypes.bool,
  multi: PropTypes.bool,
  onPick: PropTypes.func
};
