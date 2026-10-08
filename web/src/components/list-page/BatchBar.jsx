import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

// ==============================|| LIST PAGE — BATCH BAR ||============================== //
// 勾选后替换工具栏:「已选 N 项」+ 动作按钮 + 取消选择。count 为 0 时不渲染。

export default function BatchBar({ count, actions, onClear, className }) {
  const { t } = useTranslation();
  if (!count || count <= 0) return null;
  return (
    <div
      role="toolbar"
      aria-label={t('listPage.selected', { count })}
      className={cn('flex min-h-[3.75rem] flex-wrap items-center gap-2 py-3', className)}
    >
      <span className="mr-1 text-sm font-semibold tabular-nums">{t('listPage.selected', { count })}</span>
      {actions}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="ml-auto text-muted-foreground"
        aria-label={t('listPage.clearSelection')}
        onClick={onClear}
      >
        <X />
        <span className="hidden sm:inline">{t('listPage.clearSelection')}</span>
      </Button>
    </div>
  );
}

BatchBar.propTypes = {
  count: PropTypes.number.isRequired,
  actions: PropTypes.node,
  onClear: PropTypes.func.isRequired,
  className: PropTypes.string
};
