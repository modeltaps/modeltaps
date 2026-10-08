import {
  columnVisibilityFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  rowSortingFeature,
  tableFeatures
} from '@tanstack/react-table';

// TanStack Table v9 only exposes the APIs of registered features. The admin tables sort and
// paginate on the server and use column visibility and row selection, so they share this set.
export const appTableFeatures = tableFeatures({
  columnVisibilityFeature,
  rowSelectionFeature,
  rowSortingFeature,
  rowPaginationFeature
});
