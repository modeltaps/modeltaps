import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

// ==============================|| LIST PAGE — BATCH BAR ||============================== //
// 勾选后替换工具栏:「已选 N 项」+ 动作按钮 + 取消选择。count 为 0 时不渲染。
// `floating` pins it to the bottom of the screen (phones), where the thumb reaches it.

export default function BatchBar({ count, actions, onClear, floating = false, className }) {
  const { t } = useTranslation();
  if (!count || count <= 0) return null;
  const clear = (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      className={cn('text-muted-foreground', floating ? 'size-8 shrink-0 p-0' : 'ml-auto')}
      aria-label={t('listPage.clearSelection')}
      onClick={onClear}
    >
      <X />
      {!floating && <span className="hidden sm:inline">{t('listPage.clearSelection')}</span>}
    </Button>
  );
  // Floating (phones): one row, the clear button first, the count, then the actions at the end.
  if (floating) {
    return (
      <div
        role="toolbar"
        aria-label={t('listPage.selected', { count })}
        className={cn(
          'fixed inset-x-0 bottom-0 z-40 flex items-center gap-1.5 border-t border-border bg-popover px-2 pb-[max(0.625rem,env(safe-area-inset-bottom))] pt-2.5 shadow-[0_-4px_16px_rgba(0,0,0,0.06)]',
          className
        )}
      >
        {clear}
        <span className="mr-auto whitespace-nowrap text-sm font-medium tabular-nums">{t('listPage.selected', { count })}</span>
        <div className="flex shrink-0 items-center gap-1.5">{actions}</div>
      </div>
    );
  }
  return (
    <div
      role="toolbar"
      aria-label={t('listPage.selected', { count })}
      className={cn('flex min-h-[3.75rem] flex-wrap items-center gap-2 py-3', className)}
    >
      <span className="mr-1 text-sm font-medium tabular-nums">{t('listPage.selected', { count })}</span>
      {actions}
      {clear}
    </div>
  );
}

BatchBar.propTypes = {
  count: PropTypes.number.isRequired,
  actions: PropTypes.node,
  onClear: PropTypes.func.isRequired,
  floating: PropTypes.bool,
  className: PropTypes.string
};
