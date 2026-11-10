import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Link, UNSAFE_DataRouterContext, useBlocker } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Search, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { dirtyGroupIds, hasDirtyCards, reportDirty, searchCards, shouldShowSearch, staysOnPage } from './settingsSearch';

// ==============================|| SETTINGS — ONE SETTINGS PAGE ||============================== //
// Platform, organization and personal settings share this page: a sticky subnav of groups on the
// left (a scrolling row below md), the cards of the active group on the right. Every group stays
// mounted (inactive ones are hidden), so unsaved edits survive a group switch, the subnav can mark
// groups holding unsaved cards with a dot, and the search box can match the text of every card,
// collection rows included. Leaving the page (another route, closing the tab) with unsaved cards
// asks first. Each group is its own route, so deep links keep working.

const CardScopeContext = createContext(null);

// The card's place on the page ({ cardId, groupId, report }); null outside a SettingsPage.
export const useCardScope = () => useContext(CardScopeContext);

// Form cards report whether they hold unsaved edits; the page turns that into subnav dots and the
// leave guard. Outside a SettingsPage this is a no-op.
export function useReportDirty(dirty) {
  const scope = useCardScope();
  const report = scope?.report;
  useEffect(() => {
    if (!report) return undefined;
    report(Boolean(dirty));
    return () => report(false);
  }, [report, dirty]);
}

function CardSlot({ card, group, groupId, hidden, showGroup, setDirty, slotRef }) {
  const cardId = card.id;
  // Stable per card, so the cards' report effects do not re-run on every render.
  const report = useCallback((value) => setDirty((prev) => reportDirty(prev, cardId, groupId, value)), [setDirty, cardId, groupId]);
  const scope = useMemo(() => ({ cardId, groupId, report }), [cardId, groupId, report]);
  return (
    <div hidden={hidden} className="space-y-2 [&:has(>[data-card-slot]:empty)]:hidden">
      {showGroup && <p className="px-1 text-xs text-muted-foreground">{group}</p>}
      <div data-card-slot={card.id} ref={slotRef}>
        <CardScopeContext.Provider value={scope}>{card.render()}</CardScopeContext.Provider>
      </div>
    </div>
  );
}

CardSlot.propTypes = {
  card: PropTypes.object.isRequired,
  group: PropTypes.node,
  groupId: PropTypes.string.isRequired,
  hidden: PropTypes.bool,
  showGroup: PropTypes.bool,
  setDirty: PropTypes.func.isRequired,
  slotRef: PropTypes.func
};

function LeaveDialog({ open, onStay, onLeave }) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onStay()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('settingsUi.leave.title')}</DialogTitle>
          <DialogDescription>{t('settingsUi.leave.description')}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onStay}>
            {t('settingsUi.leave.stay')}
          </Button>
          <Button variant="destructive" onClick={onLeave}>
            {t('settingsUi.leave.discard')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

LeaveDialog.propTypes = { open: PropTypes.bool, onStay: PropTypes.func.isRequired, onLeave: PropTypes.func.isRequired };

// Route blocker; only mounted under a data router (tests render with a plain MemoryRouter).
function RouteLeaveGuard({ when, paths }) {
  const pathsKey = paths.join('|');
  const shouldBlock = useCallback(
    ({ currentLocation, nextLocation }) =>
      when && currentLocation.pathname !== nextLocation.pathname && !staysOnPage(nextLocation.pathname, pathsKey.split('|')),
    [when, pathsKey]
  );
  const blocker = useBlocker(shouldBlock);
  return <LeaveDialog open={blocker.state === 'blocked'} onStay={() => blocker.reset?.()} onLeave={() => blocker.proceed?.()} />;
}

RouteLeaveGuard.propTypes = { when: PropTypes.bool, paths: PropTypes.arrayOf(PropTypes.string).isRequired };

function LeaveGuard({ when, paths }) {
  const dataRouter = useContext(UNSAFE_DataRouterContext);
  useEffect(() => {
    if (!when) return undefined;
    const onBeforeUnload = (e) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [when]);
  return dataRouter ? <RouteLeaveGuard when={when} paths={paths} /> : null;
}

LeaveGuard.propTypes = { when: PropTypes.bool, paths: PropTypes.arrayOf(PropTypes.string).isRequired };

export default function SettingsPage({ label, groups, activeGroupId, className }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const [dirty, setDirty] = useState({});
  const [result, setResult] = useState({ matches: new Set(), counts: {} });
  const slots = useRef({});

  const searching = query.trim() !== '';
  const active = groups.find((g) => g.id === activeGroupId) || groups[0];
  const dirtyGroups = dirtyGroupIds(dirty);
  const paths = groups.map((g) => g.to);

  // Search reads what the cards actually render (titles, labels, hints, collection rows).
  useLayoutEffect(() => {
    if (!searching) return;
    const entries = groups.flatMap((g) =>
      g.cards.map((card) => ({ cardId: card.id, groupId: g.id, text: slots.current[card.id]?.textContent || '' }))
    );
    setResult(searchCards(entries, query));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, searching]);

  // A group switch (link click or deep link) ends the search.
  useEffect(() => setQuery(''), [activeGroupId]);

  const matchCount = result.matches.size;

  return (
    <div className={cn('md:grid md:grid-cols-[13rem_minmax(0,1fr)] md:items-start md:gap-8', className)}>
      <nav aria-label={label} className="mb-5 md:sticky md:top-0 md:mb-0 md:space-y-3">
        {shouldShowSearch(groups) && (
          <div className="relative mb-2 md:mb-0">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Escape' && setQuery('')}
              placeholder={t('settingsUi.search.placeholder')}
              aria-label={t('settingsUi.search.placeholder')}
              className="pl-9 [&::-webkit-search-cancel-button]:hidden"
            />
            {searching && (
              <button
                type="button"
                onClick={() => setQuery('')}
                aria-label={t('settingsUi.search.clear')}
                className="absolute right-2 top-1/2 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:text-foreground"
              >
                <X className="size-3.5" />
              </button>
            )}
          </div>
        )}
        <div className="-mx-1 flex gap-1 overflow-x-auto px-1 pb-1 [scrollbar-width:none] md:mx-0 md:flex-col md:gap-0.5 md:overflow-visible md:p-0">
          {groups.map((group) => {
            const current = !searching && group.id === active.id;
            const count = result.counts[group.id] || 0;
            const Icon = group.icon;
            return (
              <Link
                key={group.id}
                to={group.to}
                onClick={() => setQuery('')}
                aria-current={current ? 'page' : undefined}
                className={cn(
                  'flex h-[34px] shrink-0 items-center gap-2.5 whitespace-nowrap rounded-[10px] px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                  current
                    ? 'bg-card font-medium text-foreground shadow-[inset_0_0_0_1px_var(--border)]'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                  searching && count === 0 && 'opacity-40'
                )}
              >
                {Icon && <Icon className="size-4 shrink-0" />}
                <span className="truncate">{group.label}</span>
                {group.badge && <span className="text-[11px] text-muted-foreground">{group.badge}</span>}
                {dirtyGroups.has(group.id) ? (
                  <span className="ml-auto size-1.5 shrink-0 rounded-full bg-orange-500" title={t('settingsUi.unsaved')}>
                    <span className="sr-only">{t('settingsUi.unsaved')}</span>
                  </span>
                ) : (
                  searching && <span className="ml-auto text-xs tabular-nums text-muted-foreground">{count}</span>
                )}
              </Link>
            );
          })}
        </div>
      </nav>

      <div className="min-w-0 max-w-4xl">
        <div className="mb-5 space-y-1">
          {searching ? (
            <>
              <h2 className="text-base font-semibold">{t('settingsUi.search.title', { query: query.trim() })}</h2>
              <p className="text-sm text-muted-foreground">{t('settingsUi.search.count', { count: matchCount })}</p>
            </>
          ) : (
            <>
              <h2 className="text-base font-semibold">{active.label}</h2>
              {active.description && <p className="text-sm text-muted-foreground">{active.description}</p>}
            </>
          )}
        </div>
        <div className="space-y-6">
          {groups.map((group) => (
            <section
              key={group.id}
              hidden={searching ? !(result.counts[group.id] > 0) : group.id !== active.id}
              aria-label={group.label}
              className="space-y-6"
            >
              {group.cards.map((card) => (
                <CardSlot
                  key={card.id}
                  card={card}
                  group={group.label}
                  groupId={group.id}
                  hidden={searching && !result.matches.has(card.id)}
                  showGroup={searching}
                  setDirty={setDirty}
                  slotRef={(el) => {
                    slots.current[card.id] = el;
                  }}
                />
              ))}
            </section>
          ))}
          {searching && matchCount === 0 && (
            <p className="rounded-xl border border-dashed border-border px-6 py-10 text-center text-sm text-muted-foreground">
              {t('settingsUi.search.empty')}
            </p>
          )}
        </div>
      </div>

      <LeaveGuard when={hasDirtyCards(dirty)} paths={paths} />
    </div>
  );
}

SettingsPage.propTypes = {
  label: PropTypes.string.isRequired,
  groups: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      to: PropTypes.string.isRequired,
      label: PropTypes.node.isRequired,
      description: PropTypes.node,
      icon: PropTypes.elementType,
      badge: PropTypes.node,
      cards: PropTypes.arrayOf(PropTypes.shape({ id: PropTypes.string.isRequired, render: PropTypes.func.isRequired })).isRequired
    })
  ).isRequired,
  activeGroupId: PropTypes.string,
  className: PropTypes.string
};
