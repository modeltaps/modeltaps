import PropTypes from 'prop-types';
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import ModelRow, { ROW_COLUMNS } from './ModelRow';
import { buildProviders, collectCapabilities, filterRows, nextActiveIndex } from '../shared/modelIndex';

// ==============================|| PLAYGROUND — MODEL PICKER ||============================== //
// 目录式模型选择器：贴触发器弹出的菜单（与参数 chip 的浮层同一形态，参考用量页的筛选浮层），
// 不再是居中对话框。触发器作为 children 传进来与浮层同处一个 relative 容器里，外部点击与
// Escape 的判定才能把触发器算作「内部」。
// 结构自上而下：搜索 +「隐藏不可用」→ 能力 chip → 供应商 tab → 列标题（含价格口径）→ 模型行。
// 只吃归一后的行（见 shared/modelIndex），自己不取数，便于外壳复用与单测直接喂夹具。
// mode 预留多选：'single' 回调单行并关闭菜单，'multi' 行内 checkbox 切换、回调 id 数组且不关闭。

// 浮层高度上限（max-h-[60vh] 的像素兜底），翻转判断按它算可用余量。
export const MENU_MAX_HEIGHT = 420;
// 浮层宽度（w-[420px]），横向翻转与收边按它算。
export const MENU_WIDTH = 420;
// 贴边时留一点缝，避免浮层压在容器边框上。
const EDGE_GUTTER = 8;
// 能力 chip 与供应商 tab 共用同一种高度与圆角。
const CHIP_CLASS = 'flex h-6 shrink-0 items-center gap-1 rounded-full px-2.5 text-xs transition-colors';
const chipState = (selected) =>
  selected ? 'bg-primary/10 font-medium text-foreground ring-1 ring-inset ring-primary/40' : 'text-muted-foreground hover:bg-muted';

// 组件也跑在 renderToStaticMarkup 的单测里，服务端没有布局可量，避免 useLayoutEffect 警告。
const useIsoLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;

// 键盘语义集中在一处：浮层里的按键与文档级 Escape 走同一张表。
export function menuKeyAction(key) {
  if (key === 'ArrowDown') return 'next';
  if (key === 'ArrowUp') return 'prev';
  if (key === 'Enter') return 'pick';
  if (key === 'Escape') return 'close';
  return null;
}

// 选中语义：单选回调整行并关闭，多选切换 id 数组且保持打开（对比页要连续勾选）。
export function applyPick({ multi = false, selected = [], row }) {
  if (!multi) return { value: row, close: true };
  return { value: selected.includes(row.id) ? selected.filter((id) => id !== row.id) : [...selected, row.id], close: false };
}

// 首选方向放不下就翻面：两边都放不下时留在余量大的一侧。
export function resolvePlacement({ preferred = 'top', spaceAbove = 0, spaceBelow = 0, needed = MENU_MAX_HEIGHT }) {
  const [first, second] = preferred === 'bottom' ? [spaceBelow, spaceAbove] : [spaceAbove, spaceBelow];
  const flipped = preferred === 'bottom' ? 'top' : 'bottom';
  return first >= needed || first >= second ? preferred : flipped;
}

// 横向同理：首选对齐方式把浮层挤出边界（最近的滚动祖先或视口）就翻到另一侧，
// 两侧都放不下时把浮层 clamp 进边界，并在边界比浮层还窄时收窄宽度（平移按收窄后的实际宽度算）。
// offset 是相对首选对齐位置要平移的像素，交给 translateX，避免和 right-0 / left-0 打架。
export function resolveAlign({
  preferred = 'end',
  triggerLeft = 0,
  triggerRight = 0,
  boundsLeft = 0,
  boundsRight = 0,
  width: preferredWidth = MENU_WIDTH,
  gutter = EDGE_GUTTER
}) {
  const min = boundsLeft + gutter;
  const max = boundsRight - gutter;
  const width = Math.max(0, Math.min(preferredWidth, max - min));
  const leftFor = (side) => (side === 'end' ? triggerRight - width : triggerLeft);
  const fits = (left) => left >= min && left + width <= max;
  const other = preferred === 'end' ? 'start' : 'end';

  const align = fits(leftFor(preferred)) || !fits(leftFor(other)) ? preferred : other;
  const anchored = leftFor(align);
  const clamped = Math.max(min, Math.min(anchored, max - width));
  return {
    align,
    offset: Math.round(clamped - anchored),
    maxWidth: width < preferredWidth ? Math.round(width) : null
  };
}

// 最近的横向可滚动 / 裁切祖先，浮层不能越出它；没有就按视口算。
export function clipBounds(node) {
  if (typeof window === 'undefined') return null;
  let current = node?.parentElement;
  while (current && current !== document.body) {
    const style = window.getComputedStyle(current);
    if (/(auto|scroll|hidden|clip)/.test(`${style.overflowX} ${style.overflowY}`)) {
      const rect = current.getBoundingClientRect();
      return { left: rect.left, right: rect.right };
    }
    current = current.parentElement;
  }
  return null;
}

export default function ModelPicker({
  open,
  onOpenChange,
  items = [],
  loading = false,
  error = false,
  value = '',
  values = [],
  mode = 'single',
  placement: preferred = 'top',
  align = 'end',
  className,
  children,
  onSelect
}) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const [provider, setProvider] = useState('');
  const [caps, setCaps] = useState([]);
  const [hideUnavailable, setHideUnavailable] = useState(true);
  const [active, setActive] = useState(-1);
  const [placement, setPlacement] = useState(preferred);
  const [box, setBox] = useState({ align, offset: 0, maxWidth: null });
  const wrapper = useRef(null);

  const providers = useMemo(() => buildProviders(items), [items]);
  const capabilities = useMemo(() => collectCapabilities(items), [items]);
  const visible = useMemo(
    () => filterRows(items, { search, provider, capabilities: caps, hideUnavailable }),
    [items, search, provider, caps, hideUnavailable]
  );

  const multi = mode === 'multi';
  const selected = multi ? values : [value].filter(Boolean);

  // 每次打开都从干净的搜索与高亮开始，避免上次的关键词挡住列表。
  useEffect(() => {
    if (!open) return;
    setSearch('');
    setActive(-1);
  }, [open]);

  // 触发器贴在输入框底部时向上弹，视口不够再翻到下方；横向同理，触发器靠容器左侧时
  // 右对齐会把浮层顶出滚动容器。打开瞬间量一次，打开期间窗口缩放 / 滚动时按同一套算法重量。
  useIsoLayoutEffect(() => {
    if (!open || typeof window === 'undefined') return undefined;
    const measure = () => {
      const rect = wrapper.current?.getBoundingClientRect?.();
      if (!rect) return;
      setPlacement(
        resolvePlacement({
          preferred,
          spaceAbove: rect.top,
          spaceBelow: window.innerHeight - rect.bottom,
          needed: Math.min(MENU_MAX_HEIGHT, window.innerHeight * 0.6)
        })
      );
      const bounds = clipBounds(wrapper.current);
      setBox(
        resolveAlign({
          preferred: align,
          triggerLeft: rect.left,
          triggerRight: rect.right,
          boundsLeft: Math.max(0, bounds?.left ?? 0),
          boundsRight: Math.min(window.innerWidth, bounds?.right ?? window.innerWidth)
        })
      );
    };
    measure();
    let frame = 0;
    const schedule = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(() => {
        frame = 0;
        measure();
      });
    };
    window.addEventListener('resize', schedule);
    window.addEventListener('scroll', schedule, true);
    return () => {
      window.removeEventListener('resize', schedule);
      window.removeEventListener('scroll', schedule, true);
      if (frame) window.cancelAnimationFrame(frame);
    };
  }, [open, preferred, align]);

  // Escape 与浮层外点击关闭：与 ChipPopover 同一套，不引 Radix。
  useEffect(() => {
    if (!open) return undefined;
    const onKey = (event) => {
      if (menuKeyAction(event.key) === 'close') onOpenChange?.(false);
    };
    const onPointerDown = (event) => {
      if (!wrapper.current?.contains(event.target)) onOpenChange?.(false);
    };
    document.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onPointerDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onPointerDown);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- onOpenChange 每次渲染都是新函数，只需跟随开合状态重挂
  }, [open]);

  const pick = (row) => {
    const { value: next, close } = applyPick({ multi, selected, row });
    onSelect?.(next, row);
    if (close) onOpenChange?.(false);
  };

  // 上下键在可见行之间移动高亮，回车选中；Escape 走上面的文档级监听。
  const onKeyDown = (event) => {
    const action = menuKeyAction(event.key);
    if (action === 'next' || action === 'prev') {
      event.preventDefault();
      setActive((current) => nextActiveIndex(current, action === 'next' ? 1 : -1, visible.length));
      return;
    }
    if (action === 'pick' && visible[active]) {
      event.preventDefault();
      pick(visible[active]);
    }
  };

  const toggleCap = (cap) => setCaps((list) => (list.includes(cap) ? list.filter((c) => c !== cap) : [...list, cap]));

  return (
    <div ref={wrapper} className={cn('relative inline-flex min-w-0', className)}>
      {children}
      {open && (
        // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- 键盘导航挂在浮层上，行本身是 button
        <div
          role="dialog"
          aria-label={t('playground.picker.title')}
          onKeyDown={onKeyDown}
          style={{
            transform: box.offset ? `translateX(${box.offset}px)` : undefined,
            maxWidth: box.maxWidth ?? undefined
          }}
          className={cn(
            'absolute z-30 flex max-h-[60vh] w-[420px] flex-col overflow-hidden rounded-xl border border-border bg-card shadow-xl',
            placement === 'top' ? 'bottom-full mb-2' : 'top-full mt-2',
            box.align === 'end' ? 'right-0' : 'left-0'
          )}
        >
          <div className="shrink-0 space-y-1.5 border-b border-border p-2">
            <div className="flex items-center gap-2">
              <Input
                autoFocus
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setActive(-1);
                }}
                placeholder={t('playground.picker.searchPlaceholder')}
                aria-label={t('playground.picker.searchPlaceholder')}
                className="h-8 min-w-0 flex-1"
              />
              <label className="flex shrink-0 items-center gap-1.5 whitespace-nowrap text-xs text-muted-foreground">
                <Switch checked={hideUnavailable} onCheckedChange={setHideUnavailable} className="h-5 w-9" />
                {t('playground.picker.hideUnavailable')}
              </label>
            </div>
            {capabilities.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {capabilities.map((cap) => (
                  <button
                    key={cap}
                    type="button"
                    aria-pressed={caps.includes(cap)}
                    onClick={() => toggleCap(cap)}
                    className={cn(CHIP_CLASS, chipState(caps.includes(cap)))}
                  >
                    {t(`modelpricePage.capability.${cap}`, { defaultValue: cap })}
                  </button>
                ))}
              </div>
            )}
            <div className="flex gap-1 overflow-x-auto" role="tablist">
              <ProviderTab
                label={t('playground.picker.allProviders')}
                count={items.length}
                selected={!provider}
                onClick={() => {
                  setProvider('');
                  setActive(-1);
                }}
              />
              {providers.map((item) => (
                <ProviderTab
                  key={item.id || 'unknown'}
                  label={item.id || t('playground.picker.unknownProvider')}
                  count={item.count}
                  selected={provider === item.id}
                  onClick={() => {
                    setProvider(item.id);
                    setActive(-1);
                  }}
                />
              ))}
            </div>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-1.5" role="listbox" aria-label={t('playground.picker.title')}>
            {!loading && !error && visible.length > 0 && (
              <div
                aria-hidden="true"
                title={t('playground.picker.priceNote')}
                className="sticky top-0 z-10 flex gap-2 bg-card px-3 pb-1 pt-2 text-[0.6875rem] text-muted-foreground"
              >
                {multi && <span className="size-4 shrink-0" />}
                <div className={cn(ROW_COLUMNS, 'min-w-0 flex-1')}>
                  <span className="truncate">{t('playground.picker.columns.unit')}</span>
                  <span className="text-right">{t('playground.picker.columns.context')}</span>
                  <span className="text-right">{t('playground.picker.columns.input')}</span>
                  <span className="text-right">{t('playground.picker.columns.output')}</span>
                </div>
              </div>
            )}
            {loading && <p className="p-4 text-sm text-muted-foreground">{t('common.loading')}</p>}
            {!loading && error && <p className="p-4 text-sm text-muted-foreground">{t('playground.picker.loadFailed')}</p>}
            {!loading && !error && visible.length === 0 && (
              <p className="p-4 text-sm text-muted-foreground">{t('playground.picker.empty')}</p>
            )}
            {visible.map((row, index) => (
              <ModelRow
                key={row.id}
                model={row}
                selected={selected.includes(row.id)}
                active={index === active}
                multi={multi}
                onPick={pick}
              />
            ))}
          </div>

          {multi && (
            <div className="flex shrink-0 justify-end border-t border-border px-3 py-2 text-[0.6875rem] text-muted-foreground">
              <span className="tabular-nums">{t('playground.picker.selectedCount', { count: selected.length })}</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function ProviderTab({ label, count, selected, onClick }) {
  return (
    <button type="button" onClick={onClick} aria-selected={selected} role="tab" className={cn(CHIP_CLASS, chipState(selected))}>
      <span className="max-w-[10rem] truncate">{label}</span>
      <span className="tabular-nums opacity-70">{count}</span>
    </button>
  );
}

ProviderTab.propTypes = { label: PropTypes.node, count: PropTypes.number, selected: PropTypes.bool, onClick: PropTypes.func };

ModelPicker.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func,
  items: PropTypes.array,
  loading: PropTypes.bool,
  error: PropTypes.bool,
  value: PropTypes.string,
  values: PropTypes.arrayOf(PropTypes.string),
  mode: PropTypes.oneOf(['single', 'multi']),
  placement: PropTypes.oneOf(['top', 'bottom']),
  align: PropTypes.oneOf(['start', 'end']),
  className: PropTypes.string,
  children: PropTypes.node,
  onSelect: PropTypes.func
};
