import PropTypes from 'prop-types';
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { ChevronLeft, Filter, Plus } from 'lucide-react';

import { cn } from '@/lib/utils';
import FilterChip from './FilterChip';
import FilterMenu from './FilterMenu';
import ValuePanel from './ValuePanel';
import { countActiveFilters, emptyEnumEntry, isEntryActive } from './utils';

// ==============================|| FILTER BAR — REUSABLE CASCADING FILTER (OpenRouter 风格) ||============================== //
// 字段定义驱动、受控组件:state 为 { [field.key]: entry },任何改动通过 onChange(nextState) 回传。
// 左侧渲染已选 chips,右侧圆形 filter 按钮(带计数徽标)打开级联菜单:一级分类列表常驻,
// 悬停(或点击)某分类即在右侧展开二级值面板(flyout);flyout 依视口空间右展/左翻/极窄降级覆盖。
// specials 为非 chip 的附加入口(如 ID 查找),点击回调 onSpecial(special)。

// 一级列表宽度(与 FilterMenu 的 w-64 对齐)、flyout 估算宽度(含 gap)、视口边距 —— 用于放置决策。
const L1_WIDTH = 256;
const FLYOUT_WIDTH = 296;
const VIEWPORT_MARGIN = 8;
const HOVER_DELAY = 150;

export default function FilterBar({
  fields,
  state,
  onChange,
  specials = [],
  onSpecial,
  onClearAll,
  t,
  leading,
  className,
  hideChips = false,
  triggerVariant = 'funnel',
  clearAlignEnd = false
}) {
  const [open, setOpen] = useState(false);
  const [activeKey, setActiveKey] = useState(null);
  // flyout 放置:menuAlign 一级菜单贴左/右缘;flyoutSide 二级面板右展/左翻/覆盖。
  const [placement, setPlacement] = useState({ menuAlign: 'left', flyoutSide: 'right' });
  const rootRef = useRef(null);
  const hoverTimer = useRef(null);
  // 镜像最新 state:供外点/Esc 监听(闭包按 open 订阅、易过期)与 pruneEmpty 的“是否需清理”判定读取最新值。
  const stateRef = useRef(state);
  stateRef.current = state;

  useEffect(() => {
    if (!open) return undefined;
    const onDown = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) closeMenu();
    };
    const onKey = (e) => {
      if (e.key === 'Escape') closeMenu();
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // 依据筛选按钮相对视口的位置决定放置:一级菜单溢出右缘则贴右缘收敛;
  // flyout 优先右展,右侧放不下翻转到左侧,两侧都放不下则覆盖在一级列表之上。
  const measure = useCallback(() => {
    const rootEl = rootRef.current;
    if (!rootEl) return;
    const btn = rootEl.getBoundingClientRect();
    const vw = window.innerWidth;
    const overflowRight = btn.left + L1_WIDTH > vw - VIEWPORT_MARGIN;
    const menuAlign = overflowRight ? 'right' : 'left';
    const menuLeft = menuAlign === 'right' ? btn.right - L1_WIDTH : btn.left;
    const menuRight = menuLeft + L1_WIDTH;
    let flyoutSide = 'right';
    if (menuRight + FLYOUT_WIDTH > vw - VIEWPORT_MARGIN) {
      flyoutSide = menuLeft - FLYOUT_WIDTH >= VIEWPORT_MARGIN ? 'left' : 'overlay';
    }
    setPlacement({ menuAlign, flyoutSide });
  }, []);

  useLayoutEffect(() => {
    if (open) measure();
  }, [open, activeKey, measure]);

  useEffect(() => {
    if (!open) return undefined;
    const onResize = () => measure();
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [open, measure]);

  useEffect(() => () => clearHoverTimer(), []);

  const count = countActiveFilters(fields, state);
  const activeFields = fields.filter((f) => isEntryActive(f, state[f.key]));

  // 一律走函数式更新:同一事件内的多次写(如 text「应用」= 提交值 + 关闭时 pruneEmpty)
  // 基于最新 state 依次合并,不再互相全量覆盖;也不受外点/Esc 监听闭包过期影响。
  const setEntry = (key, entry) => onChange((prev) => ({ ...prev, [key]: entry }));
  const removeField = (key) =>
    onChange((prev) => {
      const next = { ...prev };
      delete next[key];
      return next;
    });

  // 关闭菜单时清理未产生有效值的空条目,保持 state 干净(不影响已生效的 chips)。
  // 依最新 state(stateRef)判定是否确有可清理项:无则直接返回,避免无谓触发消费端 setPage(0)
  // (仅开合菜单不应重置分页);有则走函数式更新,与同一事件内的「提交值」写入按序合并、不互相覆盖。
  const pruneEmpty = () => {
    const cur = stateRef.current;
    if (!fields.some((f) => cur[f.key] && !isEntryActive(f, cur[f.key]))) return;
    onChange((prev) => {
      let changed = false;
      const next = { ...prev };
      for (const f of fields) {
        if (next[f.key] && !isEntryActive(f, next[f.key])) {
          delete next[f.key];
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  };

  const clearHoverTimer = () => {
    if (hoverTimer.current) {
      clearTimeout(hoverTimer.current);
      hoverTimer.current = null;
    }
  };

  const closeMenu = () => {
    clearHoverTimer();
    setOpen(false);
    setActiveKey(null);
    pruneEmpty();
  };

  const activeField = fields.find((f) => f.key === activeKey);

  // 展开某分类的 flyout:首次进入时补建空条目(关闭时 pruneEmpty 清理)。点击或悬停计时到点均走此。
  const openField = (f) => {
    clearHoverTimer();
    if (!state[f.key]) setEntry(f.key, f.type === 'number' || f.type === 'text' ? { value: '' } : emptyEnumEntry());
    setActiveKey(f.key);
  };

  // 悬停意图:延迟 ~150ms 展开/切换,避免扫过时误触发;离开行(onHoverEnd)取消挂起,不关闭已展开面板。
  const scheduleOpen = (f) => {
    if (activeKey === f.key) return;
    clearHoverTimer();
    hoverTimer.current = setTimeout(() => openField(f), HOVER_DELAY);
  };

  return (
    <div className={cn('flex flex-wrap items-center gap-2', className)}>
      {leading}
      {!hideChips &&
        activeFields.map((f) => (
          <FilterChip
            key={f.key}
            field={f}
            entry={state[f.key]}
            onChange={(e) => setEntry(f.key, e)}
            onRemove={() => removeField(f.key)}
            t={t}
          />
        ))}
      <div ref={rootRef} className="relative inline-flex">
        <button
          type="button"
          onClick={() => (open ? closeMenu() : setOpen(true))}
          aria-label={t('filterBar.addFilter')}
          aria-expanded={open}
          className={cn(
            triggerVariant === 'plus'
              ? 'flex size-7 items-center justify-center rounded-md border border-dashed border-input text-muted-foreground transition-colors hover:bg-muted hover:text-foreground'
              : cn(
                  'relative flex size-9 items-center justify-center rounded-full border border-input bg-background text-muted-foreground transition-colors hover:bg-muted hover:text-foreground',
                  count > 0 && 'border-foreground/40 text-foreground'
                )
          )}
        >
          {triggerVariant === 'plus' ? <Plus className="size-4" /> : <Filter className="size-4" />}
          {triggerVariant !== 'plus' && count > 0 && (
            <span className="absolute -right-1 -top-1 flex min-w-[18px] items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-semibold leading-4 text-background">
              {count}
            </span>
          )}
        </button>
        {open && (
          <div className={cn('absolute top-full z-50 mt-1', placement.menuAlign === 'right' ? 'right-0' : 'left-0')}>
            <div className="relative flex items-start">
              {/* 一级分类列表 —— 常驻,展开 flyout 不改变其位置/宽度(flyout 为绝对定位)。 */}
              <div className="rounded-md border border-border bg-card text-card-foreground shadow-md">
                <FilterMenu
                  fields={fields}
                  state={state}
                  specials={specials}
                  activeKey={activeKey}
                  onPick={openField}
                  onHover={scheduleOpen}
                  onHoverEnd={clearHoverTimer}
                  onSpecial={(s) => {
                    closeMenu();
                    onSpecial?.(s);
                  }}
                  t={t}
                />
              </div>
              {activeField && (
                <div
                  onMouseEnter={clearHoverTimer}
                  className={cn(
                    'absolute top-0 z-10 rounded-md border border-border bg-card text-card-foreground shadow-md',
                    placement.flyoutSide === 'right' && 'left-full ml-1',
                    placement.flyoutSide === 'left' && 'right-full mr-1',
                    placement.flyoutSide === 'overlay' && 'right-0'
                  )}
                >
                  {/* 极窄视口下 flyout 覆盖在一级列表之上,保留返回入口回到列表。 */}
                  {placement.flyoutSide === 'overlay' && (
                    <button
                      type="button"
                      onClick={() => setActiveKey(null)}
                      className="flex w-full items-center gap-1 border-b border-border px-2 py-1.5 text-left text-sm font-medium hover:bg-muted"
                    >
                      <ChevronLeft className="size-4" />
                      <span className="truncate">{t(activeField.labelKey)}</span>
                    </button>
                  )}
                  <ValuePanel
                    key={activeField.key}
                    field={activeField}
                    entry={state[activeKey]}
                    onChange={(e) => setEntry(activeKey, e)}
                    onConfirm={closeMenu}
                    t={t}
                  />
                </div>
              )}
            </div>
          </div>
        )}
      </div>
      {!hideChips && count > 0 && onClearAll && (
        <button
          type="button"
          onClick={onClearAll}
          className={cn(
            'text-sm text-muted-foreground underline-offset-2 hover:text-foreground hover:underline',
            clearAlignEnd && 'ml-auto'
          )}
        >
          {t('filterBar.clearAll')}
        </button>
      )}
    </div>
  );
}

FilterBar.propTypes = {
  fields: PropTypes.array.isRequired,
  state: PropTypes.object.isRequired,
  onChange: PropTypes.func.isRequired,
  specials: PropTypes.array,
  onSpecial: PropTypes.func,
  onClearAll: PropTypes.func,
  t: PropTypes.func.isRequired,
  leading: PropTypes.node,
  className: PropTypes.string,
  hideChips: PropTypes.bool,
  triggerVariant: PropTypes.oneOf(['funnel', 'plus']),
  clearAlignEnd: PropTypes.bool
};
