import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Inbox, SearchX } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/ui/empty-state';

// ==============================|| LIST PAGE — EMPTY STATE ||============================== //
// 两种空态:variant="empty" 无数据(可带主操作 action);variant="noMatch" 筛选无匹配(带「清除筛选」)。

export default function ListEmptyState({ variant = 'empty', icon, title, description, action, onClearFilters, className }) {
  const { t } = useTranslation();
  if (variant === 'noMatch') {
    return (
      <EmptyState
        icon={icon || SearchX}
        title={title ?? t('listPage.noMatchTitle')}
        description={description ?? t('listPage.noMatchDescription')}
        action={
          onClearFilters && (
            <Button type="button" variant="outline" size="sm" className="mt-2" onClick={onClearFilters}>
              {t('listPage.clearFilters')}
            </Button>
          )
        }
        className={className}
      />
    );
  }
  return (
    <EmptyState
      icon={icon || Inbox}
      title={title ?? t('listPage.emptyTitle')}
      description={description}
      action={action && <div className="mt-2">{action}</div>}
      className={className}
    />
  );
}

ListEmptyState.propTypes = {
  variant: PropTypes.oneOf(['empty', 'noMatch']),
  icon: PropTypes.elementType,
  title: PropTypes.node,
  description: PropTypes.node,
  action: PropTypes.node,
  onClearFilters: PropTypes.func,
  className: PropTypes.string
};
