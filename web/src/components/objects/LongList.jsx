import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ChevronLeft, ChevronRight, Search } from 'lucide-react';

import { Input } from '@/components/ui/input';

// Lists in an object panel can hold hundreds of rows (a gateway channel lists every model it
// resells). Past `limit` rows the list gets a search box; past `pageSize` matches it shows one
// page at a time with a pager below.
export const LONG_LIST_LIMIT = 20;
export const LONG_LIST_PAGE_SIZE = 50;

// The page to show of `total` rows: `page` (0-based) kept inside the pages there are, with the
// [start, end) slice and the page count.
export function pageOf(total, page, size = LONG_LIST_PAGE_SIZE) {
  const pages = Math.max(1, Math.ceil(total / size));
  const current = Math.min(Math.max(0, page), pages - 1);
  const start = current * size;
  return { page: current, pages, start, end: Math.min(total, start + size) };
}

// `resetKey` names what the rows are (a filter, a sort): when it changes the list goes back to
// the first page. Edits to the rows themselves keep the page.
export function useLongList(items, { text, limit = LONG_LIST_LIMIT, pageSize = LONG_LIST_PAGE_SIZE, resetKey = '' }) {
  const [query, setQueryState] = useState('');
  const [page, setPage] = useState(0);
  const [pageKey, setPageKey] = useState(resetKey);
  if (pageKey !== resetKey) {
    setPageKey(resetKey);
    setPage(0);
  }
  const q = query.trim().toLowerCase();
  const matches = useMemo(() => (q ? items.filter((item) => text(item).toLowerCase().includes(q)) : items), [items, q, text]);
  const long = items.length > limit;
  // A page past the end (the list shrank) shows the last one.
  const slice = pageOf(matches.length, page, pageSize);
  return {
    long,
    query,
    // A new search starts on the first page.
    setQuery: (value) => {
      setQueryState(value);
      setPage(0);
    },
    visible: matches.slice(slice.start, slice.end),
    total: matches.length,
    ...slice,
    setPage
  };
}

// The search box above a long list; renders nothing while the list is short.
export function LongListSearch({ list, placeholder, className }) {
  const { t } = useTranslation();
  if (!list.long) return null;
  return (
    <div className={className}>
      <div className="relative">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={list.query}
          onChange={(e) => list.setQuery(e.target.value)}
          placeholder={placeholder || t('longList.search')}
          aria-label={placeholder || t('longList.search')}
          className="h-8 pl-8 text-[13px]"
        />
      </div>
    </div>
  );
}

LongListSearch.propTypes = { list: PropTypes.object.isRequired, placeholder: PropTypes.string, className: PropTypes.string };

const pagerButton =
  'inline-flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-40';

// Below the list: the pager ("51–100 / 511" with previous / next) when there is more than one
// page, or a note when the search matches nothing.
export function LongListMore({ list }) {
  const { t } = useTranslation();
  if (list.total === 0 && list.query) {
    return <p className="py-6 text-center text-[13px] text-muted-foreground">{t('longList.noMatch')}</p>;
  }
  if (list.pages <= 1) return null;
  return (
    <nav aria-label={t('longList.pager')} className="flex h-10 items-center justify-end gap-1 border-t border-border px-2 text-[13px]">
      <span className="mr-2 tabular-nums text-muted-foreground">
        {t('longList.range', { from: list.start + 1, to: list.end, total: list.total })}
      </span>
      <button
        type="button"
        className={pagerButton}
        aria-label={t('longList.previous')}
        disabled={list.page === 0}
        onClick={() => list.setPage(list.page - 1)}
      >
        <ChevronLeft className="size-4" />
      </button>
      <span className="min-w-[3.5rem] text-center tabular-nums text-muted-foreground">
        {list.page + 1} / {list.pages}
      </span>
      <button
        type="button"
        className={pagerButton}
        aria-label={t('longList.next')}
        disabled={list.page >= list.pages - 1}
        onClick={() => list.setPage(list.page + 1)}
      >
        <ChevronRight className="size-4" />
      </button>
    </nav>
  );
}

LongListMore.propTypes = { list: PropTypes.object.isRequired };
