import { Link, useLocation } from 'react-router';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import PageTitleExtra from '@/components/chrome/PageTitleExtra';

// ==============================|| ANALYTICS — STATS TABS ||============================== //
// The sidebar's 统计 item covers two pages: platform analytics and per-user statistics. This
// switch next to the page title moves between them; both keep their own route.

export const STATS_TABS = [
  { id: 'platform', url: '/panel/analytics', labelKey: 'nav.stats.platform' },
  { id: 'byUser', url: '/panel/multi_user_stats', labelKey: 'nav.stats.byUser' }
];

export default function StatsTabs() {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  return (
    <PageTitleExtra>
      <nav aria-label={t('nav.stats.tabs')} className="flex items-center gap-0.5 rounded-lg bg-muted p-0.5">
        {STATS_TABS.map((tab) => {
          const active = pathname === tab.url || pathname.startsWith(`${tab.url}/`);
          return (
            <Link
              key={tab.id}
              to={tab.url}
              aria-current={active ? 'page' : undefined}
              className={cn(
                'flex h-7 items-center rounded-md px-3 text-[13px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                active ? 'bg-card text-foreground shadow-[0_0_0_1px_var(--border)]' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {t(tab.labelKey)}
            </Link>
          );
        })}
      </nav>
    </PageTitleExtra>
  );
}
