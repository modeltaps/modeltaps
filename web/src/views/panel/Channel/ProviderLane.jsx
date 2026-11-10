import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Plus } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { openIssues } from './channelGroups';
import { Dot, ProviderTile, useProviderName } from './channelParts';

// Left lane: one row per provider type that has channels, with a warning dot when one of its
// channels needs attention (or a faded dot when all are disabled) and the channel count.
export default function ProviderLane({ groups, selected, onSelect, dismissed, onNewOther }) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  return (
    <aside
      aria-label={t('channels.lane.label')}
      className="flex flex-col rounded-2xl bg-[color-mix(in_srgb,var(--muted)_70%,var(--background))] p-3 lg:sticky lg:top-4 lg:max-h-[calc(100vh-140px)]"
    >
      <div className="px-1.5 pb-2 pt-0.5 text-xs font-medium text-muted-foreground">{t('channels.lane.label')}</div>
      <div className="flex max-h-64 flex-col gap-1 overflow-y-auto lg:max-h-none">
        {groups.map((g) => {
          const on = g.type === selected;
          const warn = openIssues(g.channels, dismissed).length > 0;
          const allOff = g.channels.every((c) => c.status !== 1);
          return (
            <button
              key={g.type}
              type="button"
              aria-current={on ? 'true' : undefined}
              onClick={() => onSelect(g.type)}
              className={cn(
                'flex h-10 w-full shrink-0 items-center gap-2.5 rounded-lg border border-transparent px-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                on ? 'border-border bg-card' : 'hover:bg-foreground/[0.05]'
              )}
            >
              <ProviderTile type={g.type} />
              <span className={cn('min-w-0 flex-1 truncate text-[13px]', on && 'font-medium')}>{providerName(g.type)}</span>
              {warn ? (
                <Dot tone="warn" title={t('channels.lane.attention')} />
              ) : allOff ? (
                <Dot tone="off" title={t('channels.lane.allOff')} />
              ) : null}
              <span className="font-mono text-xs text-muted-foreground">{g.channels.length}</span>
            </button>
          );
        })}
      </div>
      <div className="mt-2 border-t border-border pt-2">
        <Button variant="ghost" size="sm" className="w-full justify-start rounded-lg" onClick={onNewOther}>
          <Plus />
          {t('channels.lane.newOther')}
        </Button>
      </div>
    </aside>
  );
}

ProviderLane.propTypes = {
  groups: PropTypes.array.isRequired,
  selected: PropTypes.number,
  onSelect: PropTypes.func.isRequired,
  dismissed: PropTypes.instanceOf(Set).isRequired,
  onNewOther: PropTypes.func.isRequired
};
