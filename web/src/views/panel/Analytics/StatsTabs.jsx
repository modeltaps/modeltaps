import { Link, useLocation } from 'react-router';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import PageTitleCenter from '@/components/chrome/PageTitleCenter';
import { admin } from 'routes/paths';

// ==============================|| ANALYTICS — STATS TABS ||============================== //
// The sidebar's 统计 item covers two pages: platform analytics and per-user statistics. This
// switch, centered on the page title row, moves between them; both keep their own route.

export const STATS_TABS = [
  { id: 'platform', url: admin.stats(), labelKey: 'nav.stats.platform' },
  { id: 'byUser', url: admin.statsByUser(), labelKey: 'nav.stats.byUser' }
];

export default function StatsTabs() {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  // The longest matching URL wins: /stats/users is under /stats, so a plain prefix test would
  // light up both tabs on the per-user page.
  const activeId = STATS_TABS.filter((tab) => pathname === tab.url || pathname.startsWith(`${tab.url}/`)).sort(
    (a, b) => b.url.length - a.url.length
  )[0]?.id;
  return (
    <PageTitleCenter>
      <nav aria-label={t('nav.stats.tabs')} className="flex items-center gap-0.5 rounded-lg bg-control p-0.5">
        {STATS_TABS.map((tab) => {
          const active = tab.id === activeId;
          return (
            <Link
              key={tab.id}
              to={tab.url}
              aria-current={active ? 'page' : undefined}
              className={cn(
                'flex h-7 items-center rounded-md px-3 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                active ? 'bg-control-thumb text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {t(tab.labelKey)}
            </Link>
          );
        })}
      </nav>
    </PageTitleCenter>
  );
}
