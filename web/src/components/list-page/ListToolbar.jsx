import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { FilterBar } from '@/components/filter-bar';

// ==============================|| LIST PAGE — TOOLBAR ||============================== //
// 搜索框 + FilterBar 分面(已选 pill 逐个可清除)+ 右侧视图控件槽。受控、不取数。
// search: { value, onChange(value), placeholder? };filters: FilterBar 的 fields/state/onChange/onClearAll 等。

export default function ListToolbar({ search, filters, viewControls, className }) {
  const { t } = useTranslation();
  return (
    <div className={cn('flex flex-wrap items-center gap-2 py-3', className)}>
      {search && (
        <div className="relative w-full sm:w-64">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="search"
            value={search.value}
            onChange={(e) => search.onChange(e.target.value)}
            placeholder={search.placeholder ?? t('listPage.searchPlaceholder')}
            aria-label={search.placeholder ?? t('listPage.searchPlaceholder')}
            className="flex h-9 w-full rounded-lg border border-input bg-background pl-8 pr-8 text-sm placeholder:text-muted-foreground focus-visible:border-primary focus-visible:outline-none [&::-webkit-search-cancel-button]:hidden"
          />
          {search.value && (
            <button
              type="button"
              onClick={() => search.onChange('')}
              aria-label={t('listPage.clearSearch')}
              className="absolute right-2 top-1/2 flex size-5 -translate-y-1/2 items-center justify-center rounded-sm text-muted-foreground hover:text-foreground"
            >
              <X className="size-3.5" />
            </button>
          )}
        </div>
      )}
      {filters && <FilterBar t={t} {...filters} className={cn('min-w-0', filters.className)} />}
      {viewControls && <div className="ml-auto flex items-center gap-2">{viewControls}</div>}
    </div>
  );
}

ListToolbar.propTypes = {
  search: PropTypes.shape({
    value: PropTypes.string.isRequired,
    onChange: PropTypes.func.isRequired,
    placeholder: PropTypes.string
  }),
  filters: PropTypes.shape({
    fields: PropTypes.array.isRequired,
    state: PropTypes.object.isRequired,
    onChange: PropTypes.func.isRequired,
    onClearAll: PropTypes.func,
    specials: PropTypes.array,
    onSpecial: PropTypes.func,
    className: PropTypes.string
  }),
  viewControls: PropTypes.node,
  className: PropTypes.string
};
