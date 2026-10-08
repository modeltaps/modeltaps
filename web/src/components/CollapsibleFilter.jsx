import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ChevronDown } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

// ==============================|| SHARED — COLLAPSIBLE FILTER PANEL ||============================== //
// Common collapsible wrapper for every panel page's filter area. Width-based
// default: collapsed below sm (640px, where the filter grids drop to a single
// column), expanded at sm and above. The breakpoint is watched live: crossing
// it resets the open state to match the new size, while manual toggles within
// the same breakpoint are preserved (effect only fires on isWide changes).
// Supports both uncontrolled (internal state) and controlled (open/onToggle)
// usage; auto-reset applies to the uncontrolled state only.

const FILTER_BREAKPOINT_QUERY = '(min-width: 640px)';

export const getDefaultFiltersOpen = () =>
  typeof window === 'undefined' || typeof window.matchMedia !== 'function' ? true : window.matchMedia(FILTER_BREAKPOINT_QUERY).matches;

export default function CollapsibleFilter({ title, open, onToggle, activeCount = 0, className, contentClassName, children }) {
  const { t } = useTranslation();
  const [isWide, setIsWide] = useState(getDefaultFiltersOpen);
  const [innerOpen, setInnerOpen] = useState(isWide);
  const isControlled = typeof open === 'boolean';
  const isOpen = isControlled ? open : innerOpen;

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return undefined;
    const mql = window.matchMedia(FILTER_BREAKPOINT_QUERY);
    const handleChange = (event) => setIsWide(event.matches);
    setIsWide(mql.matches);
    mql.addEventListener('change', handleChange);
    return () => mql.removeEventListener('change', handleChange);
  }, []);

  useEffect(() => {
    setInnerOpen(isWide);
  }, [isWide]);

  const handleToggle = () => {
    if (!isControlled) setInnerOpen((v) => !v);
    onToggle?.();
  };

  return (
    <div className={cn('rounded-lg border border-border bg-card', className)}>
      <button
        type="button"
        onClick={handleToggle}
        className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium"
        aria-expanded={isOpen}
      >
        <span className="flex items-center gap-2">
          {title || t('common.filters', { defaultValue: 'Filters' })}
          {activeCount > 0 && <Badge variant="secondary">{t('common.filtersActive', { count: activeCount })}</Badge>}
        </span>
        <ChevronDown className={cn('size-4 transition-transform', isOpen && 'rotate-180')} />
      </button>
      {isOpen && <div className={cn('border-t border-border p-4', contentClassName)}>{children}</div>}
    </div>
  );
}

CollapsibleFilter.propTypes = {
  title: PropTypes.node,
  open: PropTypes.bool,
  onToggle: PropTypes.func,
  activeCount: PropTypes.number,
  className: PropTypes.string,
  contentClassName: PropTypes.string,
  children: PropTypes.node
};
