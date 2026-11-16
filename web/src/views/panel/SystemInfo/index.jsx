import { useTranslation } from 'react-i18next';
import { Navigate } from 'react-router';
import { Download } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import useRouteTab from 'hooks/useRouteTab';
import { useIsRoot } from 'utils/common';
import { workbench } from 'routes/paths';
import SystemLogs from './SystemLogs';
import SystemProblems from './SystemProblems';

// ==============================|| PANEL — SYSTEM LOGS (root) ||============================== //
// Root-only, like the `/api/system_info` routes behind it (RootAuth) and its menu entry.
// Problems (`/panel/admin/system-logs`) is the default tab: warnings and errors folded by
// template. Recent (`/system-logs/recent`) is the raw in-memory history. The full log file is a
// download; searching all of it is a log collector's job.

const TABS = ['problems', 'recent'];
const TAB_TRIGGER_CLASS =
  '-mb-px rounded-none border-b-2 px-1 pb-2 pt-0 shadow-none bg-transparent data-[state=active]:bg-transparent data-[state=active]:shadow-none';

export default function SystemInfo() {
  const { t } = useTranslation();
  const isRoot = useIsRoot();
  const { tab, setTab, redirecting } = useRouteTab({ tabs: TABS });

  if (!isRoot) return <Navigate to={workbench.dashboard()} replace />;
  if (redirecting) return null;

  return (
    <div className="space-y-4">
      <div className="flex items-end justify-between gap-4 border-b border-border">
        <Tabs value={tab} onValueChange={setTab}>
          <TabsList className="flex h-auto justify-start gap-5 rounded-none bg-transparent p-0">
            {TABS.map((value) => (
              <TabsTrigger
                key={value}
                value={value}
                className={cn(TAB_TRIGGER_CLASS, tab === value ? 'border-foreground text-foreground' : 'border-transparent')}
              >
                {t(`systemLogs.tabs.${value}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <Button variant="ghost" size="sm" asChild className="mb-1.5 gap-1.5 text-muted-foreground">
          <a href="/api/system_info/log/download" download title={t('systemLogs.downloadHint')}>
            <Download className="size-4" strokeWidth={1.5} />
            {t('systemLogs.download')}
          </a>
        </Button>
      </div>
      {tab === 'problems' ? <SystemProblems /> : <SystemLogs />}
    </div>
  );
}
