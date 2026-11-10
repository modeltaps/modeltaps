// Object pages: a list where a row opens the object's right-side panel (tabs that each save on
// their own) and a right-side sheet that creates a new object with only the required fields.
export { default as ObjectPanel, ObjectTabs, SaveBar } from './ObjectPanel';
export { ObjectSheet, SheetBar, ObjectTile } from './ObjectSheet';
export { default as NewObjectSheet } from './NewObjectSheet';
export { default as useTabDrafts } from './useTabDrafts';
export { default as useObjectRoute } from './useObjectRoute';
export { Dot, Field, InfoRow, InfoRows, OptionList, OptionRow, Section, Segmented, StatRow, ToggleField } from './fields';

// A table row that opens the object's panel; controls inside it stop the click with `stopRowClick`.
export const ROW_OPENS = 'cursor-pointer';
export const stopRowClick = (e) => e.stopPropagation();

// Keyboard access for a clickable row: Enter or Space opens it.
export const rowOpenProps = (onOpen) => ({
  tabIndex: 0,
  onClick: onOpen,
  onKeyDown: (e) => {
    if (e.target !== e.currentTarget) return;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onOpen();
    }
  }
});
