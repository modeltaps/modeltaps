import PropTypes from 'prop-types';

import { Skeleton } from '@/components/ui/skeleton';

// ==============================|| LIST PAGE — LOADING SKELETON ||============================== //
// Placeholder shaped like the list until its first response: table rows (a checkbox, a name and
// the cells given by `cells` as widths in px, the last one pushed right) or phone rows with a
// status dot and two lines. Sized like real rows so nothing jumps when the data arrives.

export default function ListSkeleton({ layout = 'table', rows = 5, cells = [64, 192, 32], checkbox = true }) {
  const items = Array.from({ length: rows }, (_, i) => i);
  if (layout === 'list') {
    return (
      <ul className="divide-y divide-border" aria-busy="true">
        {items.map((i) => (
          <li key={i} className="flex h-14 items-center gap-3 px-4">
            <Skeleton className="size-2 rounded-full" />
            <div className="flex-1 space-y-2">
              <div className="flex justify-between gap-6">
                <Skeleton className="h-3.5 w-28" />
                <Skeleton className="h-3.5 w-12" />
              </div>
              <div className="flex justify-between gap-6">
                <Skeleton className="h-3 w-36" />
                <Skeleton className="h-3 w-8" />
              </div>
            </div>
          </li>
        ))}
      </ul>
    );
  }
  return (
    <div aria-busy="true">
      <div className="flex h-10 items-center gap-6 border-b border-border px-4">
        {checkbox && <Skeleton className="size-4 rounded-sm" />}
        <Skeleton className="h-3 w-16" />
        <Skeleton className="ml-auto h-3 w-12" />
      </div>
      {items.map((i) => (
        <div key={i} className="flex h-[53px] items-center gap-6 border-b border-border px-4 last:border-0">
          {checkbox && <Skeleton className="size-4 rounded-sm" />}
          <Skeleton className="h-3.5 w-40" />
          {cells.map((w, j) => (
            <Skeleton key={j} className={j === cells.length - 1 ? 'ml-auto h-3.5' : 'h-3.5'} style={{ width: w }} />
          ))}
        </div>
      ))}
    </div>
  );
}

ListSkeleton.propTypes = {
  layout: PropTypes.oneOf(['table', 'list']),
  rows: PropTypes.number,
  cells: PropTypes.arrayOf(PropTypes.number),
  checkbox: PropTypes.bool
};
