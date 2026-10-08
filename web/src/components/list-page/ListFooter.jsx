import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { PAGE_SIZE_OPTIONS } from 'constants';

// ==============================|| LIST PAGE — FOOTER PAGINATION ||============================== //
// 全后台统一分页页脚(以 panel/components/Pagination 为基础):「已选 N / 共 N 条 · 每页 N 条 · 首/上/第 x / y 页/下/末」。
// page 从 0 开始,与 v1 API 约定一致;窄屏隐藏每页条数与首末页按钮。

export default function ListFooter({
  total,
  page,
  pageSize,
  pageSizeOptions = PAGE_SIZE_OPTIONS,
  onPageChange,
  onPageSizeChange,
  selectedCount = 0
}) {
  const { t } = useTranslation();
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const lastPage = pages - 1;
  const current = Math.min(Math.max(0, page), lastPage);
  const atFirst = current <= 0;
  const atLast = current >= lastPage;

  const navBtn = (label, target, disabled, Icon, extra = '') => (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={`size-8 ${extra}`}
      aria-label={label}
      disabled={disabled}
      onClick={() => onPageChange(target)}
    >
      <Icon />
    </Button>
  );

  return (
    <div className="flex h-12 items-center gap-4 px-3 text-xs text-muted-foreground">
      <span className="tabular-nums">
        {selectedCount > 0 && <>{t('listPage.selectedOf', { count: selectedCount })} / </>}
        {t('pagination.total', { count: total })}
      </span>
      <span className="flex-1" />
      {onPageSizeChange && (
        <label className="hidden items-center gap-2 sm:flex">
          <span>{t('pagination.rows')}</span>
          <select
            value={pageSize}
            onChange={(e) => onPageSizeChange(parseInt(e.target.value, 10))}
            className="h-7 rounded-sm border border-input bg-background px-1.5 text-xs text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {pageSizeOptions.map((opt) => (
              <option key={opt} value={opt}>
                {opt}
              </option>
            ))}
          </select>
        </label>
      )}
      <div className="flex items-center gap-1">
        {navBtn(t('listPage.firstPage'), 0, atFirst, ChevronsLeft, 'hidden sm:inline-flex')}
        {navBtn(t('listPage.prevPage'), current - 1, atFirst, ChevronLeft)}
        <span className="px-1.5 tabular-nums text-foreground">{t('listPage.pageOf', { page: current + 1, pages })}</span>
        {navBtn(t('listPage.nextPage'), current + 1, atLast, ChevronRight)}
        {navBtn(t('listPage.lastPage'), lastPage, atLast, ChevronsRight, 'hidden sm:inline-flex')}
      </div>
    </div>
  );
}

ListFooter.propTypes = {
  total: PropTypes.number.isRequired,
  page: PropTypes.number.isRequired,
  pageSize: PropTypes.number.isRequired,
  pageSizeOptions: PropTypes.arrayOf(PropTypes.number),
  onPageChange: PropTypes.func.isRequired,
  onPageSizeChange: PropTypes.func,
  selectedCount: PropTypes.number
};
