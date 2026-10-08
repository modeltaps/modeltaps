import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { PAGE_SIZE_OPTIONS } from 'constants';

// ==============================|| PANEL — SHARED PAGINATION BAR ||============================== //
// Server-side pagination control. `page` is zero-based to match the v1 API contract.

export default function Pagination({ page, rowsPerPage, count, options = PAGE_SIZE_OPTIONS, onPageChange, onRowsPerPageChange }) {
  const { t } = useTranslation();
  const lastPage = Math.max(0, Math.ceil(count / rowsPerPage) - 1);
  const from = count === 0 ? 0 : page * rowsPerPage + 1;
  const to = Math.min(count, (page + 1) * rowsPerPage);

  return (
    <div className="flex flex-col items-center gap-3 border-t border-border px-3 py-2 text-sm sm:flex-row sm:justify-between">
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{t('pagination.rows', { defaultValue: 'Rows' })}</span>
        <select
          value={rowsPerPage}
          onChange={(e) => onRowsPerPageChange(parseInt(e.target.value, 10))}
          className="h-9 rounded-lg border border-input bg-background px-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {options.map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
      </div>

      <div className="flex items-center gap-4">
        <span className="tabular-nums text-muted-foreground">
          {from}–{to} / {count}
        </span>
        <div className="flex items-center gap-1">
          <Button variant="outline" size="icon" className="size-9" aria-label="First" disabled={page <= 0} onClick={() => onPageChange(0)}>
            <ChevronsLeft className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            className="size-9"
            aria-label="Prev"
            disabled={page <= 0}
            onClick={() => onPageChange(page - 1)}
          >
            <ChevronLeft className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            className="size-9"
            aria-label="Next"
            disabled={page >= lastPage}
            onClick={() => onPageChange(page + 1)}
          >
            <ChevronRight className="size-4" />
          </Button>
          <Button
            variant="outline"
            size="icon"
            className="size-9"
            aria-label="Last"
            disabled={page >= lastPage}
            onClick={() => onPageChange(lastPage)}
          >
            <ChevronsRight className="size-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}

Pagination.propTypes = {
  page: PropTypes.number.isRequired,
  rowsPerPage: PropTypes.number.isRequired,
  count: PropTypes.number.isRequired,
  options: PropTypes.array,
  onPageChange: PropTypes.func.isRequired,
  onRowsPerPageChange: PropTypes.func.isRequired
};
