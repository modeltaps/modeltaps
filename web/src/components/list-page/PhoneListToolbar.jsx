import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ArrowDownUp, Check, ListChecks, ListFilter, Search, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

// ==============================|| LIST PAGE — PHONE TOOLBAR ||============================== //
// The phone list's toolbar: one row with the search box and icon buttons for filter (with a count
// of active conditions), sort (a list has no headers to click) and select mode, then the active
// conditions as removable chips. Every part is optional.
//   search:  { value, onChange, placeholder }
//   filter:  { count, onOpen }
//   sort:    { options: [{ key, label, active, onSelect }] }
//   select:  { active, onToggle }
//   chips:   [{ key, label, onRemove }]

export default function PhoneListToolbar({ search, filter, sort, select, chips = [] }) {
  const { t } = useTranslation();
  return (
    <>
      <div className="flex items-center gap-2">
        {search && (
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search.value}
              onChange={(e) => search.onChange(e.target.value)}
              placeholder={search.placeholder ?? t('listPage.searchPlaceholder')}
              aria-label={search.placeholder ?? t('listPage.searchPlaceholder')}
              className="h-8 pl-8"
            />
          </div>
        )}
        {filter && (
          <Button
            variant="outline"
            size="icon"
            className="relative size-8 shrink-0"
            aria-label={t('listPage.filter')}
            onClick={filter.onOpen}
          >
            <ListFilter className="size-4" />
            {filter.count > 0 && (
              <span className="absolute -right-1 -top-1 flex min-w-[16px] items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-medium leading-4 text-background">
                {filter.count}
              </span>
            )}
          </Button>
        )}
        {sort && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="icon" className="size-8 shrink-0" aria-label={t('listPage.sort')}>
                <ArrowDownUp className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-44">
              <DropdownMenuLabel>{t('listPage.sort')}</DropdownMenuLabel>
              {sort.options.map((o) => (
                <DropdownMenuItem key={o.key} onClick={o.onSelect} className="justify-between">
                  {o.label}
                  {o.active && <Check className="size-4" />}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        {select && (
          <Button
            variant={select.active ? 'secondary' : 'outline'}
            size="icon"
            className="size-8 shrink-0"
            aria-label={t('listPage.selectMode')}
            aria-pressed={select.active}
            onClick={select.onToggle}
          >
            <ListChecks className="size-4" />
          </Button>
        )}
      </div>
      {chips.length > 0 && (
        <div className="-mx-1 flex gap-1.5 overflow-x-auto px-1 no-scrollbar">
          {chips.map((c) => (
            <button
              key={c.key}
              type="button"
              onClick={c.onRemove}
              aria-label={`${t('filterBar.remove')} ${c.label}`}
              className="inline-flex h-7 shrink-0 items-center gap-1 rounded-full border border-input bg-field pl-2.5 pr-1.5 text-xs"
            >
              {c.label}
              <X className="size-3.5 text-muted-foreground" />
            </button>
          ))}
        </div>
      )}
    </>
  );
}

PhoneListToolbar.propTypes = {
  search: PropTypes.shape({ value: PropTypes.string, onChange: PropTypes.func, placeholder: PropTypes.string }),
  filter: PropTypes.shape({ count: PropTypes.number, onOpen: PropTypes.func }),
  sort: PropTypes.shape({ options: PropTypes.array }),
  select: PropTypes.shape({ active: PropTypes.bool, onToggle: PropTypes.func }),
  chips: PropTypes.array
};
