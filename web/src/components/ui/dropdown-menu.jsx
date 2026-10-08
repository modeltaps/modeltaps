import * as React from 'react';
import { createPortal } from 'react-dom';
import { Slot } from '@radix-ui/react-slot';
import { Check, ChevronRight } from 'lucide-react';

import { cn } from '@/lib/utils';

// Lightweight, dependency-free dropdown menu (no Radix popper). The content is
// rendered through a portal with fixed positioning anchored to the trigger so it
// is never clipped by ancestors that use overflow (e.g. table scroll wrappers).
// Because the portal escapes its parent stacking context, the content must sit
// at the same layer as dialog / sheet (z-[1300]) so it also stays above the
// mobile sidebar drawer (z-[1200]); a lower value renders behind the drawer.
const DropdownContext = React.createContext({ open: false, setOpen: () => {} });

function DropdownMenu({ className, children }) {
  const [open, setOpen] = React.useState(false);
  const triggerRef = React.useRef(null);
  const contentRef = React.useRef(null);
  // 子菜单内容渲染在独立 portal 中,注册到这里才不会被判定为「外部点击」
  const surfacesRef = React.useRef(new Set());

  React.useEffect(() => {
    if (!open) return undefined;
    const onDown = (e) => {
      if (triggerRef.current?.contains(e.target)) return;
      if (contentRef.current?.contains(e.target)) return;
      for (const node of surfacesRef.current) if (node?.contains(e.target)) return;
      setOpen(false);
    };
    const onKey = (e) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  return (
    <DropdownContext.Provider value={{ open, setOpen, triggerRef, contentRef, surfacesRef }}>
      <div ref={triggerRef} className={cn('relative inline-block text-left', className)}>
        {children}
      </div>
    </DropdownContext.Provider>
  );
}

const DropdownMenuTrigger = React.forwardRef(({ asChild, onClick, ...props }, ref) => {
  const { open, setOpen } = React.useContext(DropdownContext);
  const Comp = asChild ? Slot : 'button';
  return (
    <Comp
      ref={ref}
      aria-haspopup="menu"
      aria-expanded={open}
      onClick={(e) => {
        onClick?.(e);
        setOpen((v) => !v);
      }}
      {...props}
    />
  );
});
DropdownMenuTrigger.displayName = 'DropdownMenuTrigger';

const DropdownMenuContent = React.forwardRef(
  ({ className, align = 'end', side = 'bottom', sideOffset = 4, matchTriggerWidth = false, anchorRef = null, ...props }, ref) => {
    const { open, triggerRef, contentRef } = React.useContext(DropdownContext);
    const [style, setStyle] = React.useState(null);

    // First render while open is unpositioned (visibility:hidden) so the content
    // can be measured; the layout effect then places it with viewport collision
    // handling: `side="bottom"` opens below the trigger and flips above it when it
    // would overflow the bottom edge, `side="top"` opens above the trigger and
    // flips below it when it would overflow the top edge, `side="right"` opens
    // beside the trigger (`align="end"` bottom-aligns the two) and flips to its
    // left when it would overflow the right edge. `top` / `bottom` take the
    // horizontal edge from `align` (`start` = trigger's left edge, `end` = its
    // right edge). Every side clamps to the viewport.
    React.useLayoutEffect(() => {
      if (!open) {
        setStyle(null);
        return undefined;
      }
      const update = () => {
        // anchorRef 指定时按该元素(如整条页脚)定位与取宽,菜单可以比触发按钮宽得多
        const trigger = anchorRef?.current || triggerRef.current;
        const content = contentRef.current;
        if (!trigger || !content) return;
        const r = trigger.getBoundingClientRect();
        const w = matchTriggerWidth ? Math.max(content.offsetWidth, r.width) : content.offsetWidth;
        const h = content.offsetHeight;
        const margin = 4;

        let top;
        let left;
        if (side === 'right') {
          left = r.right + sideOffset;
          if (left + w > window.innerWidth - margin && r.left - sideOffset - w >= margin) {
            left = r.left - sideOffset - w;
          }
          top = align === 'end' ? r.bottom - h : r.top;
        } else if (side === 'top') {
          top = r.top - sideOffset - h;
          if (top < margin && r.bottom + sideOffset + h <= window.innerHeight - margin) {
            top = r.bottom + sideOffset;
          }
          left = align === 'end' ? r.right - w : r.left;
        } else {
          top = r.bottom + sideOffset;
          if (top + h > window.innerHeight - margin && r.top - sideOffset - h >= margin) {
            top = r.top - sideOffset - h;
          }
          left = align === 'end' ? r.right - w : r.left;
        }
        top = Math.min(Math.max(top, margin), Math.max(window.innerHeight - h - margin, margin));
        left = Math.min(Math.max(left, margin), Math.max(window.innerWidth - w - margin, margin));

        setStyle({ position: 'fixed', top, left, ...(matchTriggerWidth ? { minWidth: r.width } : null) });
      };
      update();
      window.addEventListener('scroll', update, true);
      window.addEventListener('resize', update);
      return () => {
        window.removeEventListener('scroll', update, true);
        window.removeEventListener('resize', update);
      };
    }, [open, align, side, sideOffset, matchTriggerWidth, anchorRef, triggerRef, contentRef]);

    if (!open) return null;

    const setRefs = (node) => {
      contentRef.current = node;
      if (typeof ref === 'function') ref(node);
      else if (ref) ref.current = node;
    };

    return createPortal(
      <div
        ref={setRefs}
        role="menu"
        style={style ?? { position: 'fixed', top: 0, left: 0, visibility: 'hidden' }}
        className={cn(
          'z-[1300] min-w-[10rem] overflow-hidden rounded-lg border border-border bg-card p-1 text-card-foreground shadow-md',
          className
        )}
        {...props}
      />,
      document.body
    );
  }
);
DropdownMenuContent.displayName = 'DropdownMenuContent';

const DropdownMenuItem = React.forwardRef(({ className, onClick, disabled, ...props }, ref) => {
  const { setOpen } = React.useContext(DropdownContext);
  return (
    <button
      ref={ref}
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={(e) => {
        onClick?.(e);
        setOpen(false);
      }}
      className={cn(
        'relative flex w-full cursor-pointer select-none items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted focus:bg-muted disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0',
        className
      )}
      {...props}
    />
  );
});
DropdownMenuItem.displayName = 'DropdownMenuItem';

function DropdownMenuSeparator({ className }) {
  return <div role="separator" className={cn('-mx-1 my-1 h-px bg-border', className)} />;
}

function DropdownMenuLabel({ className, ...props }) {
  return <div className={cn('px-2 py-1.5 text-xs font-semibold text-muted-foreground', className)} {...props} />;
}

// Sub-menus reuse the portal + fixed positioning of the root content so they are
// never clipped by the parent panel; they open on hover and on ArrowRight/click.
const SubContext = React.createContext(null);

function DropdownMenuSub({ children }) {
  const [open, setOpen] = React.useState(false);
  const triggerRef = React.useRef(null);
  const contentRef = React.useRef(null);
  const closeTimer = React.useRef(null);

  const cancelClose = React.useCallback(() => clearTimeout(closeTimer.current), []);
  const scheduleClose = React.useCallback(() => {
    clearTimeout(closeTimer.current);
    closeTimer.current = setTimeout(() => setOpen(false), 150);
  }, []);
  React.useEffect(() => () => clearTimeout(closeTimer.current), []);

  return (
    <SubContext.Provider value={{ open, setOpen, triggerRef, contentRef, cancelClose, scheduleClose }}>
      <div
        onMouseEnter={() => {
          cancelClose();
          setOpen(true);
        }}
        onMouseLeave={scheduleClose}
      >
        {children}
      </div>
    </SubContext.Provider>
  );
}

const DropdownMenuSubTrigger = React.forwardRef(({ className, children, onKeyDown, ...props }, ref) => {
  const { open, setOpen, triggerRef, contentRef } = React.useContext(SubContext);

  const setRefs = (node) => {
    triggerRef.current = node;
    if (typeof ref === 'function') ref(node);
    else if (ref) ref.current = node;
  };

  return (
    <button
      ref={setRefs}
      type="button"
      role="menuitem"
      aria-haspopup="menu"
      aria-expanded={open}
      onClick={() => setOpen((v) => !v)}
      onKeyDown={(e) => {
        onKeyDown?.(e);
        if (e.key === 'ArrowRight' || e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          e.stopPropagation();
          setOpen(true);
          // 内容在下一帧挂载后再把焦点移入子菜单
          requestAnimationFrame(() => contentRef.current?.querySelector('[role^="menuitem"]')?.focus());
        } else if (e.key === 'ArrowLeft') {
          setOpen(false);
        }
      }}
      className={cn(
        'relative flex w-full cursor-pointer select-none items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted focus:bg-muted [&_svg]:size-4 [&_svg]:shrink-0',
        className
      )}
      {...props}
    >
      {children}
      <ChevronRight className="ml-auto opacity-60" />
    </button>
  );
});
DropdownMenuSubTrigger.displayName = 'DropdownMenuSubTrigger';

const DropdownMenuSubContent = React.forwardRef(({ className, onKeyDown, ...props }, ref) => {
  const { open, setOpen, triggerRef, contentRef, cancelClose, scheduleClose } = React.useContext(SubContext);
  const { surfacesRef } = React.useContext(DropdownContext);
  const [style, setStyle] = React.useState(null);

  React.useLayoutEffect(() => {
    if (!open) {
      setStyle(null);
      return undefined;
    }
    const update = () => {
      const trigger = triggerRef.current;
      const content = contentRef.current;
      if (!trigger || !content) return;
      const r = trigger.getBoundingClientRect();
      const { offsetWidth: w, offsetHeight: h } = content;
      const margin = 4;

      let left = r.right + margin;
      if (left + w > window.innerWidth - margin) left = r.left - margin - w;
      left = Math.min(Math.max(left, margin), Math.max(window.innerWidth - w - margin, margin));

      let top = r.top;
      top = Math.min(Math.max(top, margin), Math.max(window.innerHeight - h - margin, margin));

      setStyle({ position: 'fixed', top, left });
    };
    update();
    window.addEventListener('scroll', update, true);
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update, true);
      window.removeEventListener('resize', update);
    };
  }, [open, triggerRef, contentRef]);

  React.useEffect(() => {
    const node = open ? contentRef.current : null;
    const surfaces = surfacesRef?.current;
    if (!node || !surfaces) return undefined;
    surfaces.add(node);
    return () => surfaces.delete(node);
  }, [open, contentRef, surfacesRef]);

  if (!open) return null;

  const setRefs = (node) => {
    contentRef.current = node;
    if (typeof ref === 'function') ref(node);
    else if (ref) ref.current = node;
  };

  return createPortal(
    <div
      ref={setRefs}
      role="menu"
      tabIndex={-1}
      style={style ?? { position: 'fixed', top: 0, left: 0, visibility: 'hidden' }}
      onMouseEnter={cancelClose}
      onMouseLeave={scheduleClose}
      onKeyDown={(e) => {
        onKeyDown?.(e);
        if (e.key === 'ArrowLeft') {
          e.preventDefault();
          setOpen(false);
          triggerRef.current?.focus();
          return;
        }
        if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
        e.preventDefault();
        e.stopPropagation();
        const items = Array.from(e.currentTarget.querySelectorAll('[role^="menuitem"]'));
        if (items.length === 0) return;
        const idx = items.indexOf(document.activeElement);
        const next = e.key === 'ArrowDown' ? (idx + 1) % items.length : (idx - 1 + items.length) % items.length;
        items[next].focus();
      }}
      className={cn(
        'z-[1300] min-w-[10rem] overflow-hidden rounded-lg border border-border bg-card p-1 text-card-foreground shadow-md',
        className
      )}
      {...props}
    />,
    document.body
  );
});
DropdownMenuSubContent.displayName = 'DropdownMenuSubContent';

const RadioGroupContext = React.createContext({ value: undefined, onValueChange: undefined });

function DropdownMenuRadioGroup({ value, onValueChange, ...props }) {
  return (
    <RadioGroupContext.Provider value={{ value, onValueChange }}>
      <div role="group" {...props} />
    </RadioGroupContext.Provider>
  );
}

const DropdownMenuRadioItem = React.forwardRef(({ className, value, children, onClick, ...props }, ref) => {
  const { setOpen } = React.useContext(DropdownContext);
  const group = React.useContext(RadioGroupContext);
  const checked = group.value === value;
  return (
    <button
      ref={ref}
      type="button"
      role="menuitemradio"
      aria-checked={checked}
      onClick={(e) => {
        onClick?.(e);
        group.onValueChange?.(value);
        setOpen(false);
      }}
      className={cn(
        'relative flex w-full cursor-pointer select-none items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-muted focus:bg-muted [&_svg]:size-4 [&_svg]:shrink-0',
        className
      )}
      {...props}
    >
      <span className="flex w-4 shrink-0 justify-center">{checked && <Check />}</span>
      {children}
    </button>
  );
});
DropdownMenuRadioItem.displayName = 'DropdownMenuRadioItem';

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem
};
