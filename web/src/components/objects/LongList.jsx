import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';

import { Input } from '@/components/ui/input';

// Lists in an object panel can hold hundreds of rows (a gateway channel lists every model it
// resells). Up to `limit` rows show as they are; past that the list gets a search box and shows
// the first `limit` matches with a button for the rest.
export const LONG_LIST_LIMIT = 20;

export function useLongList(items, { text, limit = LONG_LIST_LIMIT }) {
  const [query, setQuery] = useState('');
  const [expanded, setExpanded] = useState(false);
  const q = query.trim().toLowerCase();
  const matches = useMemo(() => (q ? items.filter((item) => text(item).toLowerCase().includes(q)) : items), [items, q, text]);
  const long = items.length > limit;
  const visible = expanded || !long ? matches : matches.slice(0, limit);
  return {
    long,
    query,
    setQuery,
    visible,
    total: matches.length,
    hidden: matches.length - visible.length,
    expand: () => setExpanded(true)
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

// Below the list: "show the other N", or a note when the search matches nothing.
export function LongListMore({ list }) {
  const { t } = useTranslation();
  if (list.total === 0 && list.query) {
    return <p className="py-6 text-center text-[13px] text-muted-foreground">{t('longList.noMatch')}</p>;
  }
  if (list.hidden <= 0) return null;
  return (
    <button
      type="button"
      onClick={list.expand}
      className="flex h-9 w-full items-center justify-center border-t border-border text-[13px] text-muted-foreground hover:bg-muted/50 hover:text-foreground"
    >
      {t('longList.showMore', { count: list.hidden })}
    </button>
  );
}

LongListMore.propTypes = { list: PropTypes.object.isRequired };
