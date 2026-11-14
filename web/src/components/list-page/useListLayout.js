import useMediaQuery from 'hooks/useMediaQuery';

// ==============================|| LIST PAGE — TABLE OR PHONE LIST ||============================== //
// A list is a table on every screen from the md breakpoint (768px) up and the phone list below
// it, decided by the screen width alone, never by what the table holds. A table wider than its
// column scrolls sideways inside its card (the shared Table wraps it in an overflow-x container);
// the sidebar does not collapse to make room.

export const PHONE_QUERY = '(max-width: 767px)';

export function useListLayout() {
  const narrow = useMediaQuery(PHONE_QUERY);
  return { layout: narrow ? 'list' : 'table', narrow };
}
