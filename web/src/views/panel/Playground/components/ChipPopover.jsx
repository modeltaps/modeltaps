import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';
import ChipButton from './ChipButton';
import { clipBounds, resolveAlign } from './ModelPicker';

const useIsoLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;

// ==============================|| CONSOLE — CHIP POPOVER ||============================== //
// ChipButton + 弹出的浮层容器（输入框贴底，所以默认朝上；side="bottom" 给页面中部的
// 编辑卡用）。与 ui/dialog、ui/sheet 一样不引 Radix：Escape 与浮层外点击关闭。
// open/onOpenChange 可省，省略时自己持有开合状态，这样装配层只想要一颗 chip 时不必额外管 state。

export default function ChipPopover({
  icon,
  label,
  open,
  onOpenChange,
  disabled = false,
  align = 'start',
  side = 'top',
  className,
  contentClassName,
  chipClassName,
  children,
  ...chipProps
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const expanded = open ?? internalOpen;
  const wrapper = useRef(null);
  const content = useRef(null);
  const [box, setBox] = useState({ align, offset: 0, maxWidth: null });

  const setOpen = (next) => {
    if (open === undefined) setInternalOpen(next);
    onOpenChange?.(next);
  };

  // 与模型菜单同一套横向收边：按浮层实际宽度量一次，挤出边界就翻面 / 平移 / 收窄。
  useIsoLayoutEffect(() => {
    if (!expanded) {
      setBox({ align, offset: 0, maxWidth: null });
      return;
    }
    const rect = wrapper.current?.getBoundingClientRect?.();
    const width = content.current?.offsetWidth;
    if (!rect || !width || typeof window === 'undefined') return;
    const bounds = clipBounds(wrapper.current);
    setBox(
      resolveAlign({
        preferred: align,
        triggerLeft: rect.left,
        triggerRight: rect.right,
        boundsLeft: Math.max(0, bounds?.left ?? 0),
        boundsRight: Math.min(window.innerWidth, bounds?.right ?? window.innerWidth),
        width
      })
    );
  }, [expanded, align]);

  useEffect(() => {
    if (!expanded) return undefined;
    const onKey = (event) => {
      if (event.key === 'Escape') setOpen(false);
    };
    const onPointerDown = (event) => {
      if (!wrapper.current?.contains(event.target)) setOpen(false);
    };
    document.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onPointerDown);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onPointerDown);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- setOpen 每次渲染都是新函数，只需跟随开合状态重挂
  }, [expanded]);

  return (
    <div ref={wrapper} className={cn('relative inline-flex min-w-0', className)}>
      <ChipButton
        icon={icon}
        expanded={expanded}
        disabled={disabled}
        className={chipClassName}
        onClick={() => setOpen(!expanded)}
        {...chipProps}
      >
        {label}
      </ChipButton>
      {expanded && (
        <div
          ref={content}
          role="dialog"
          aria-label={typeof label === 'string' ? label : undefined}
          style={{
            transform: box.offset ? `translateX(${box.offset}px)` : undefined,
            maxWidth: box.maxWidth ?? undefined,
            minWidth: box.maxWidth ?? undefined
          }}
          className={cn(
            'absolute z-30 min-w-[240px] rounded-xl border border-border bg-card p-3 shadow-xl',
            side === 'bottom' ? 'top-full mt-2' : 'bottom-full mb-2',
            box.align === 'end' ? 'right-0' : 'left-0',
            contentClassName
          )}
        >
          {children}
        </div>
      )}
    </div>
  );
}

ChipPopover.propTypes = {
  icon: PropTypes.node,
  label: PropTypes.node,
  open: PropTypes.bool,
  onOpenChange: PropTypes.func,
  disabled: PropTypes.bool,
  align: PropTypes.oneOf(['start', 'end']),
  side: PropTypes.oneOf(['top', 'bottom']),
  className: PropTypes.string,
  contentClassName: PropTypes.string,
  chipClassName: PropTypes.string,
  children: PropTypes.node
};
