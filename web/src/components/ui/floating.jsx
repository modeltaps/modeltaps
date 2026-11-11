import * as React from 'react';
import { createPortal } from 'react-dom';

import { cn } from '@/lib/utils';

// ==============================|| FLOATING — ANCHORED PANELS THAT ARE NEVER CLIPPED ||============================== //
// Dropdown panels used to sit absolutely inside their trigger's container, so a dialog, sheet,
// card or table with overflow hidden cut them off, and in a short window they ran past the
// bottom edge. A floating panel renders into document.body with fixed positioning anchored to
// its trigger:
// - `side="bottom"` opens below and moves above when there is more room there; `side="top"` the
//   reverse; `side="right"` opens beside the trigger and flips left at the right edge.
// - Its height is capped to the room on the chosen side, and it scrolls inside, so it always fits
//   the window. `clamp={false}` skips the cap for panels that manage their own scrolling and
//   overflow (cascading menus with flyouts).
// - `matchWidth` makes it at least as wide as the trigger (selects, comboboxes); `maxHeight` caps
//   the height below the room available (a long list stays a short scrolling box).
// Callers that close on an outside click must treat clicks inside the panel as inside: check
// the panel's ref as well as their own root.

const MARGIN = 8;
// The smallest height worth showing before a panel moves to the other side or scrolls.
const MIN_HEIGHT = 120;

export function computeFloatingStyle({ anchor, width, height, side, align, offset, matchWidth, clamp, maxHeight: cap, viewport }) {
  const { innerWidth: vw, innerHeight: vh } = viewport;
  const w = matchWidth ? Math.max(width, anchor.width) : width;
  if (cap) height = Math.min(height, cap);
  let top;
  let left;
  let maxHeight;
  if (side === 'right') {
    left = anchor.right + offset;
    if (left + w > vw - MARGIN && anchor.left - offset - w >= MARGIN) left = anchor.left - offset - w;
    maxHeight = vh - 2 * MARGIN;
    const h = clamp ? Math.min(height, maxHeight) : height;
    top = align === 'end' ? anchor.bottom - h : anchor.top;
    top = Math.min(Math.max(top, MARGIN), Math.max(vh - h - MARGIN, MARGIN));
  } else {
    const below = vh - anchor.bottom - offset - MARGIN;
    const above = anchor.top - offset - MARGIN;
    const placeAbove =
      side === 'top' ? !(height > above && below > above) : height > below && above > below && below < Math.max(height, MIN_HEIGHT);
    maxHeight = Math.max(MIN_HEIGHT, placeAbove ? above : below);
    const h = clamp ? Math.min(height, maxHeight) : height;
    top = placeAbove ? anchor.top - offset - h : anchor.bottom + offset;
    left = align === 'end' ? anchor.right - w : anchor.left;
  }
  left = Math.min(Math.max(left, MARGIN), Math.max(vw - w - MARGIN, MARGIN));
  if (cap) maxHeight = Math.min(maxHeight, cap);
  return {
    position: 'fixed',
    top,
    left,
    ...(clamp ? { maxHeight } : null),
    ...(matchWidth ? { minWidth: anchor.width } : null)
  };
}

// Positions `contentRef` against `anchorRef` while `open`; returns the style to apply (null until
// measured, so the first frame can render hidden and be measured).
export function useFloatingPosition({
  open,
  anchorRef,
  contentRef,
  side = 'bottom',
  align = 'start',
  offset = 4,
  matchWidth = false,
  clamp = true,
  maxHeight
}) {
  const [style, setStyle] = React.useState(null);
  React.useLayoutEffect(() => {
    if (!open) {
      setStyle(null);
      return undefined;
    }
    const update = () => {
      const anchor = anchorRef.current;
      const content = contentRef.current;
      if (!anchor || !content) return;
      setStyle(
        computeFloatingStyle({
          anchor: anchor.getBoundingClientRect(),
          width: content.offsetWidth,
          // scrollHeight is the natural height even while a previous cap is applied.
          height: content.scrollHeight,
          side,
          align,
          offset,
          matchWidth,
          clamp,
          maxHeight,
          viewport: window
        })
      );
    };
    update();
    window.addEventListener('scroll', update, true);
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update, true);
      window.removeEventListener('resize', update);
    };
  }, [open, anchorRef, contentRef, side, align, offset, matchWidth, clamp, maxHeight]);
  return style;
}

const HIDDEN = { position: 'fixed', top: 0, left: 0, visibility: 'hidden' };

// A panel anchored to `anchorRef`, rendered into document.body. `open=false` renders nothing
// unless `keepMounted` (then it stays in the DOM, hidden, e.g. so select items register labels).
export const FloatingPanel = React.forwardRef(
  (
    {
      open,
      anchorRef,
      side,
      align,
      offset,
      matchWidth,
      clamp = true,
      maxHeight,
      keepMounted = false,
      className,
      style,
      children,
      ...props
    },
    ref
  ) => {
    const contentRef = React.useRef(null);
    const position = useFloatingPosition({ open, anchorRef, contentRef, side, align, offset, matchWidth, clamp, maxHeight });
    const setRefs = React.useCallback(
      (node) => {
        contentRef.current = node;
        if (typeof ref === 'function') ref(node);
        else if (ref) ref.current = node;
      },
      [ref]
    );
    if (!open && !keepMounted) return null;
    if (typeof document === 'undefined') return null;
    return createPortal(
      <div
        ref={setRefs}
        style={{ ...(open ? (position ?? HIDDEN) : { display: 'none' }), ...style }}
        className={cn('z-[1300]', clamp && 'overflow-y-auto', className)}
        {...props}
      >
        {children}
      </div>,
      document.body
    );
  }
);
FloatingPanel.displayName = 'FloatingPanel';
