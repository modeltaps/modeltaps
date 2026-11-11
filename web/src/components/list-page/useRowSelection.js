import { useEffect, useState } from 'react';

// ==============================|| LIST PAGE — ROW SELECTION ||============================== //
// Selected row ids on the current page, for bulk actions. `resetDeps` are the inputs that change
// which rows show (page, page size, search, segment, conditions, context); the selection clears
// whenever one changes, so a bulk action never touches rows the user cannot see. In the phone
// list, checkboxes only show in select mode.

export default function useRowSelection(rows, resetDeps = []) {
  const [selectedIds, setSelectedIds] = useState(() => new Set());
  const [selectMode, setSelectMode] = useState(false);
  useEffect(() => {
    setSelectedIds(new Set());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, resetDeps);
  const clear = () => setSelectedIds(new Set());
  return {
    selectedIds,
    count: selectedIds.size,
    isSelected: (id) => selectedIds.has(id),
    toggle: (id) =>
      setSelectedIds((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      }),
    toggleAll: (all) => setSelectedIds(all ? new Set(rows.map((r) => r.id)) : new Set()),
    clear,
    selectMode,
    enterSelectMode: () => setSelectMode(true),
    exitSelectMode: () => {
      setSelectMode(false);
      clear();
    }
  };
}
