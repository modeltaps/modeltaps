import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Link, UNSAFE_DataRouterContext, useBlocker } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Search } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useSearch } from '@/components/chrome/SearchContext';
import { cardIndex, dirtyGroupIds, hasDirtyCards, reportDirty, shouldShowSearch, staysOnPage } from './settingsSearch';
import { useSettingsPlane } from './SettingsPlane';

// ==============================|| SETTINGS — ONE SETTINGS PAGE ||============================== //
// Platform, organization and personal settings share this page: a sticky subnav of groups on the
// left (a scrolling row below md), the cards of the active group on the right. On 设置 (personal
// and organization settings) the subnav and its unsaved dots live in the settings sidebar instead
// (SettingsPlane), and the page shows only the cards. Every group stays mounted (inactive ones are
// hidden), so unsaved edits survive a group switch and the subnav can mark groups holding unsaved
// cards with a dot. Search is the panel's search palette (SearchContext): the page lists its cards
// there with everything written on them, collection rows included, and the subnav's search button
// opens it. Leaving the page (another route, closing the tab) with unsaved cards asks first. Each
// group is its own route, so deep links keep working.

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

function CardSlot({ card, groupId, setDirty, slotRef }) {
  const cardId = card.id;
  // Stable per card, so the cards' report effects do not re-run on every render.
  const report = useCallback((value) => setDirty((prev) => reportDirty(prev, cardId, groupId, value)), [setDirty, cardId, groupId]);
  const scope = useMemo(() => ({ cardId, groupId, report }), [cardId, groupId, report]);
  return (
    <div className="space-y-2 [&:has(>[data-card-slot]:empty)]:hidden">
      <div data-card-slot={card.id} ref={slotRef}>
        <CardScopeContext.Provider value={scope}>{card.render()}</CardScopeContext.Provider>
      </div>
    </div>
  );
}

CardSlot.propTypes = {
  card: PropTypes.object.isRequired,
  groupId: PropTypes.string.isRequired,
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
  const plane = useSettingsPlane();
  const search = useSearch();
  const [dirty, setDirty] = useState({});
  const slots = useRef({});

  const active = groups.find((g) => g.id === activeGroupId) || groups[0];
  const dirtyGroups = dirtyGroupIds(dirty);
  const paths = groups.map((g) => g.to);

  // The search palette lists this page's cards, read when it opens (titles, labels, hints, rows).
  const groupsRef = useRef(groups);
  useLayoutEffect(() => {
    groupsRef.current = groups;
  });
  const registerPageIndex = search?.registerPageIndex;
  useEffect(() => registerPageIndex?.(() => cardIndex(groupsRef.current, slots.current)), [registerPageIndex]);

  // On 设置, report the groups holding unsaved cards (their dots are in the sidebar tree).
  const publish = plane?.publish;
  const dirtyKey = [...dirtyGroups].sort().join(',');
  useEffect(() => {
    publish?.({ dirtyGroups: dirtyKey ? dirtyKey.split(',') : [] });
  }, [publish, dirtyKey]);
  useEffect(() => () => publish?.(null), [publish]);

  return (
    <div className={cn(!plane && 'md:grid md:grid-cols-[13rem_minmax(0,1fr)] md:items-start md:gap-8', className)}>
      <nav hidden={Boolean(plane)} aria-label={label} className="mb-5 md:sticky md:top-0 md:mb-0 md:space-y-3">
        {search && shouldShowSearch(groups) && (
          <button
            type="button"
            onClick={search.openSearch}
            className="mb-2 flex h-9 w-full items-center gap-2.5 rounded-lg border border-border bg-field px-3 text-sm text-muted-foreground transition-colors hover:border-foreground/20 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:mb-0"
          >
            <Search className="size-4 shrink-0" />
            <span className="flex-1 text-left">{t('settingsUi.search.placeholder')}</span>
            <kbd className="rounded border border-border px-1 font-mono text-[0.6875rem]">⌘K</kbd>
          </button>
        )}
        <div className="-mx-1 flex gap-1 overflow-x-auto px-1 pb-1 [scrollbar-width:none] md:mx-0 md:flex-col md:gap-0.5 md:overflow-visible md:p-0">
          {groups.map((group) => {
            const current = group.id === active.id;
            const Icon = group.icon;
            return (
              <Link
                key={group.id}
                to={group.to}
                aria-current={current ? 'page' : undefined}
                className={cn(
                  'flex h-[34px] shrink-0 items-center gap-2.5 whitespace-nowrap rounded-lg px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                  current
                    ? 'bg-card font-medium text-foreground shadow-[inset_0_0_0_1px_var(--border)]'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                )}
              >
                {Icon && <Icon className="size-4 shrink-0" />}
                <span className="truncate">{group.label}</span>
                {group.badge && <span className="text-[11px] text-muted-foreground">{group.badge}</span>}
                {dirtyGroups.has(group.id) && (
                  <span className="ml-auto size-1.5 shrink-0 rounded-full bg-orange-500" title={t('settingsUi.unsaved')}>
                    <span className="sr-only">{t('settingsUi.unsaved')}</span>
                  </span>
                )}
              </Link>
            );
          })}
        </div>
      </nav>

      <div className="min-w-0 max-w-4xl">
        {/* On 设置 the page title already names the group. */}
        {!plane && (
          <div className="mb-5 space-y-1">
            <h2 className="text-base font-semibold">{active.label}</h2>
            {active.description && <p className="text-sm text-muted-foreground">{active.description}</p>}
          </div>
        )}
        <div className="space-y-6">
          {groups.map((group) => (
            <section key={group.id} hidden={group.id !== active.id} aria-label={group.label} className="space-y-6">
              {group.cards.map((card) => (
                <CardSlot
                  key={card.id}
                  card={card}
                  groupId={group.id}
                  setDirty={setDirty}
                  slotRef={(el) => {
                    slots.current[card.id] = el;
                  }}
                />
              ))}
            </section>
          ))}
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
      // Named with the group in search results, when groups of two parts share a page.
      sectionLabel: PropTypes.string,
      description: PropTypes.node,
      icon: PropTypes.elementType,
      badge: PropTypes.node,
      cards: PropTypes.arrayOf(PropTypes.shape({ id: PropTypes.string.isRequired, render: PropTypes.func.isRequired })).isRequired
    })
  ).isRequired,
  activeGroupId: PropTypes.string,
  className: PropTypes.string
};
