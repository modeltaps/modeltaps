import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, ChevronDown, Plus, Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Dot } from '@/components/objects';
import { openIssues } from './channelGroups';
import { ProviderTile, useProviderName } from './channelParts';

// The provider lane folded into one control, for widths where a 300px column would squeeze the
// channel table: the selected provider with its channel count, opening a menu with the search
// over every provider's channels and the provider list (attention dots as in the lane).
export default function ProviderSwitcher({ groups, selected, onSelect, dismissed, onNewOther, query, onQueryChange }) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const [text, setText] = useState(query);
  // Write the search to the URL shortly after typing stops, like the lane does.
  useEffect(() => {
    if (text === query) return undefined;
    const timer = setTimeout(() => onQueryChange(text), 200);
    return () => clearTimeout(timer);
  }, [text, query, onQueryChange]);
  const current = groups.find((g) => g.type === selected);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex h-11 w-full min-w-0 items-center gap-2.5 rounded-xl border border-input bg-field px-3 text-left transition-colors hover:bg-muted"
        >
          {current ? <ProviderTile type={current.type} /> : <Search className="size-4 text-muted-foreground" />}
          <span className="min-w-0 flex-1 truncate text-sm font-medium">
            {current ? providerName(current.type) : t('channels.lane.label')}
          </span>
          {current && (
            <span className="shrink-0 text-xs text-muted-foreground">
              {t('channels.switcher.count', { count: current.channels.length })}
            </span>
          )}
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" matchTriggerWidth className="p-1.5">
        <div className="relative mb-1.5">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => e.stopPropagation()}
            placeholder={t('channels.search.placeholder')}
            aria-label={t('channels.search.label')}
            className="h-8 pl-8 text-[13px]"
          />
        </div>
        {query && groups.length === 0 && <p className="px-2 py-3 text-[13px] text-muted-foreground">{t('channels.search.none')}</p>}
        {groups.map((g) => {
          const on = g.type === selected;
          const warn = openIssues(g.channels, dismissed).length > 0;
          const allOff = g.channels.every((c) => c.status !== 1);
          return (
            <DropdownMenuItem key={g.type} onClick={() => onSelect(g.type)} className={cn('h-10 gap-2.5', on && 'font-medium')}>
              <ProviderTile type={g.type} />
              <span className="min-w-0 flex-1 truncate text-[13px]">{providerName(g.type)}</span>
              {warn ? (
                <Dot tone="warn" title={t('channels.lane.attention')} />
              ) : allOff ? (
                <Dot tone="off" title={t('channels.lane.allOff')} />
              ) : null}
              <span className="font-mono text-xs text-muted-foreground">{g.channels.length}</span>
              {on ? <Check className="size-4" /> : <span className="size-4" />}
            </DropdownMenuItem>
          );
        })}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onNewOther}>
          <Plus className="size-4" />
          {t('channels.lane.newOther')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

ProviderSwitcher.propTypes = {
  groups: PropTypes.array.isRequired,
  selected: PropTypes.number,
  onSelect: PropTypes.func.isRequired,
  dismissed: PropTypes.instanceOf(Set).isRequired,
  onNewOther: PropTypes.func.isRequired,
  query: PropTypes.string,
  onQueryChange: PropTypes.func.isRequired
};
