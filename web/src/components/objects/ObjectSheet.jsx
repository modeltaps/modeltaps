import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Sheet } from '@/components/ui/sheet';

// Widths of the right-side sheets: an object's panel (lg), a new-object sheet with many fields
// (md) and a short one (sm).
const WIDTH = { lg: 'max-w-[720px]', md: 'max-w-[640px]', sm: 'max-w-[480px]' };

// Right-side sheet floating over the page: a header (icon, title, one-line subtitle, actions,
// close), a scrolling body and an optional footer. Escape and a click outside call `onClose`,
// which decides whether unsaved work may be dropped.
export function ObjectSheet({
  titleId,
  size = 'lg',
  icon,
  title,
  subtitle,
  subtitleMono,
  actions,
  onClose,
  bodyClassName,
  footer,
  children
}) {
  const { t } = useTranslation();
  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className={cn(
          'absolute inset-y-2 right-2 flex w-[calc(100%-1rem)] flex-col rounded-2xl border border-border bg-popover shadow-2xl',
          WIDTH[size]
        )}
      >
        <div className="flex items-center justify-between gap-4 border-b border-border px-6 py-[18px]">
          <div className="flex min-w-0 items-center gap-3">
            {icon}
            <div className="min-w-0">
              <h2 id={titleId} className="truncate text-base font-medium">
                {title}
              </h2>
              {subtitle && <p className={cn('truncate text-xs text-muted-foreground', subtitleMono && 'font-mono')}>{subtitle}</p>}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {actions}
            <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('common.close')} onClick={onClose}>
              <X className="size-4" />
            </Button>
          </div>
        </div>

        <div className={cn('min-h-0 flex-1 overflow-y-auto px-6 py-5', bodyClassName)}>{children}</div>

        {footer}
      </div>
    </Sheet>
  );
}

ObjectSheet.propTypes = {
  titleId: PropTypes.string.isRequired,
  size: PropTypes.oneOf(['lg', 'md', 'sm']),
  icon: PropTypes.node,
  title: PropTypes.node,
  subtitle: PropTypes.node,
  subtitleMono: PropTypes.bool,
  actions: PropTypes.node,
  onClose: PropTypes.func.isRequired,
  bodyClassName: PropTypes.string,
  footer: PropTypes.node,
  children: PropTypes.node
};

// Footer of a sheet: a status line on the left (what is missing, unsaved changes, a test
// result) and the buttons on the right.
export function SheetBar({ status, statusClassName, children }) {
  return (
    <div className="flex items-center justify-between gap-3 border-t border-border py-3 pl-6 pr-4">
      <span className={cn('min-w-0 text-[13px] text-muted-foreground', statusClassName)} aria-live="polite">
        {status}
      </span>
      <div className="flex shrink-0 gap-2">{children}</div>
    </div>
  );
}

SheetBar.propTypes = { status: PropTypes.node, statusClassName: PropTypes.string, children: PropTypes.node };

// Square tile for an object's icon in a sheet header or a list row.
export function ObjectTile({ children, size = 'lg', className }) {
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center border border-border bg-muted text-muted-foreground',
        size === 'lg' ? 'size-9 rounded-lg [&_svg]:size-[18px]' : 'size-[22px] rounded-md [&_svg]:size-3.5',
        className
      )}
    >
      {children}
    </span>
  );
}

ObjectTile.propTypes = { children: PropTypes.node, size: PropTypes.oneOf(['sm', 'lg']), className: PropTypes.string };
