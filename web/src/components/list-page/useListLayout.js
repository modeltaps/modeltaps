import { useEffect, useLayoutEffect, useRef, useState } from 'react';

import useElementWidth from 'hooks/useElementWidth';
import { useContentMinWidth } from '@/components/chrome/ContentWidth';

// ==============================|| LIST PAGE — TABLE OR PHONE LIST ||============================== //
// A list never drops columns as the window narrows. The table reports its natural (max-content)
// width; the page asks the shell for that much room, which collapses the sidebar first; only when
// even that is not enough does the page switch to its phone list.

// 'list' when the area is measured and narrower than the table (plus the card's 2px border).
export const listLayout = (areaWidth, tableWidth) => (areaWidth > 0 && areaWidth < tableWidth + 2 ? 'list' : 'table');

// `estimate` is the table width to assume before it has been measured (and after `resetKey`
// changes, e.g. when columns are shown or hidden). Returns the page ref, the layout and the
// callback the table reports its natural width through.
export function useListLayout({ estimate, resetKey }) {
  const [pageRef, pageWidth] = useElementWidth();
  const [measured, setMeasured] = useState(null);
  useEffect(() => {
    setMeasured(null);
  }, [resetKey]);
  const tableWidth = measured ?? estimate;
  useContentMinWidth(tableWidth + 2);
  const layout = listLayout(pageWidth, tableWidth);
  return { pageRef, layout, narrow: layout === 'list', tableWidth, onNaturalWidth: setMeasured };
}

// For the table: attach the returned ref to the <table>; after every render it measures the
// table's max-content width and reports it (React skips the update when it did not change).
export function useNaturalWidth(onNaturalWidth) {
  const tableRef = useRef(null);
  useLayoutEffect(() => {
    const el = tableRef.current;
    if (!el || !onNaturalWidth) return;
    const prev = el.style.width;
    el.style.width = 'max-content';
    const natural = Math.ceil(el.getBoundingClientRect().width);
    el.style.width = prev;
    onNaturalWidth(natural);
  });
  return tableRef;
}

// Sum of typical column widths for the visible columns, as the estimate above.
export const estimateWidth = (columnIds, visibility, widths, fallback = 120) =>
  columnIds.filter((id) => visibility[id] !== false).reduce((sum, id) => sum + (widths[id] || fallback), 0);
