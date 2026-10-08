import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| MAIN LAYOUT — PUSH DRAWER FRAME ||============================== //
// Below the md breakpoint the sidebar is a push drawer: the panel slides in from
// the left and the whole content column (header + page) shifts right by the
// drawer width, with no dimming / blur. A transparent close area covers the
// pushed content while the drawer is open, so a tap anywhere on it closes the
// drawer (the pushed content is not interactive until then).
//
// The shift is done with `left` on relatively positioned boxes rather than
// `transform`: a transform (or `translate`) would turn the column into the
// containing block of every `position: fixed` descendant (inline dialogs and
// sheets), pinning them inside the shifted box. `left` has no such effect.
//
// Layering: the drawer panel sits at z-[1200] so it stays above the close area
// (z-[1100]); floating content portaled to body (dropdown menus) must be
// z-[1300] or higher to appear above the panel — see overlay-layering.test.js.

export default function MobilePushFrame({ open, onClose, sidebar, drawer, children }) {
  return (
    <div className="relative flex min-h-0 flex-1 overflow-hidden [--drawer-w:min(18rem,85vw)]">
      {sidebar}

      <div
        data-testid="push-drawer-panel"
        data-state={open ? 'open' : 'closed'}
        aria-hidden={!open}
        inert={!open}
        className={cn(
          'absolute inset-y-0 z-[1200] flex w-[var(--drawer-w)] flex-col border-r border-border bg-card shadow-xl transition-[left] duration-200 ease-out md:hidden',
          open ? 'left-0' : '-left-[var(--drawer-w)]'
        )}
      >
        <div className="min-h-0 flex-1">{drawer}</div>
      </div>

      <div
        data-testid="push-content-column"
        className={cn(
          'relative flex min-w-0 flex-1 flex-col transition-[left] duration-200 ease-out md:left-0',
          open ? 'left-[var(--drawer-w)]' : 'left-0'
        )}
      >
        {children}
        {open && <div data-testid="push-close-area" className="absolute inset-0 z-[1100] md:hidden" onClick={onClose} aria-hidden="true" />}
      </div>
    </div>
  );
}

MobilePushFrame.propTypes = {
  open: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  sidebar: PropTypes.node,
  drawer: PropTypes.node,
  children: PropTypes.node
};
