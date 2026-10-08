import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import axios from 'axios';
import { RefreshCw } from 'lucide-react';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

// ==============================|| DASHBOARD — STATUS PANEL ||============================== //
// shadcn equivalent of the v1 StatusPanel. Same Uptime-Kuma endpoints; renders
// per-monitor uptime % plus the recent heartbeat history bars.

const STATUS_PAGE_URL = '/api/user/dashboard/uptimekuma/status-page';
const HEARTBEAT_URL = '/api/user/dashboard/uptimekuma/status-page/heartbeat';

export default function StatusPanel() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [statusData, setStatusData] = useState(null);
  const [heartbeatData, setHeartbeatData] = useState(null);
  const [refreshing, setRefreshing] = useState(false);

  const fetchData = async () => {
    try {
      setError(null);
      const [statusRes, heartbeatRes] = await Promise.all([axios.get(STATUS_PAGE_URL), axios.get(HEARTBEAT_URL)]);
      setStatusData(statusRes.data);
      setHeartbeatData(heartbeatRes.data);
    } catch (err) {
      setError(err.message || 'Failed to fetch status data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleRefresh = async () => {
    setRefreshing(true);
    await fetchData();
    setRefreshing(false);
  };

  const getLatestStatus = (id) => {
    const list = heartbeatData?.heartbeatList?.[id];
    return list?.length ? list[list.length - 1] : null;
  };

  const getUptime = (id) => {
    const v = heartbeatData?.uptimeList?.[`${id}_24`];
    return v !== undefined ? (v * 100).toFixed(2) : null;
  };

  const uptimeColor = (pct) => {
    if (pct === null) return 'text-muted-foreground';
    if (pct >= 90) return 'text-green-500';
    if (pct > 70) return 'text-amber-500';
    return 'text-destructive';
  };

  const renderHistory = (id) => {
    const list = heartbeatData?.heartbeatList?.[id];
    if (!list) return null;
    const last = list.slice(-50);
    return (
      <div className="mt-2 flex gap-px">
        {last.map((hb, i) => {
          const color = hb.status === 1 ? 'bg-green-500' : hb.status === 2 ? 'bg-amber-500' : 'bg-destructive';
          const status = hb.status === 1 ? 'Up' : hb.status === 2 ? 'Warn' : 'Down';
          const bj = new Date(new Date(hb.time).getTime() + 8 * 60 * 60 * 1000).toLocaleString();
          return <span key={i} title={`${status} - ${bj}(UTC+8)`} className={cn('h-3 max-w-[10px] flex-1 rounded-sm opacity-80', color)} />;
        })}
      </div>
    );
  };

  const renderMonitor = (monitor) => {
    const latest = getLatestStatus(monitor.id);
    const uptime = getUptime(monitor.id);
    const isNormal = latest && latest.status === 1;
    return (
      <Card key={monitor.id} className={cn('p-3', !isNormal && 'border-destructive/40')}>
        <div className="mb-1 flex items-center gap-2">
          <span className={cn('size-2.5 rounded-full', isNormal ? 'bg-green-500' : 'bg-destructive')} />
          <span className="truncate text-sm font-medium">{monitor.name}</span>
        </div>
        <div className="text-xs">
          <span className={cn('font-mono font-semibold', uptimeColor(uptime))}>{uptime}%</span>
          <span className="text-muted-foreground"> {t('dashboard_index.availability')}(24H)</span>
        </div>
        {renderHistory(monitor.id)}
      </Card>
    );
  };

  return (
    <Card>
      <CardHeader className="flex-row items-center gap-2 space-y-0">
        <CardTitle className="text-base">{t('dashboard_index.tab_status')}</CardTitle>
        <Button variant="ghost" size="icon" className="size-7" onClick={handleRefresh} disabled={loading || refreshing}>
          <RefreshCw className={cn('size-4', refreshing && 'animate-spin')} />
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">{t('dashboard_index.loading')}</div>
        ) : error ? (
          <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div>
        ) : !statusData?.publicGroupList || !heartbeatData ? (
          <div className="py-8 text-center text-sm text-muted-foreground">{t('dashboard_index.no_data_available')}</div>
        ) : (
          <div className="space-y-6">
            {statusData.publicGroupList.map((group) => (
              <div key={group.id}>
                <h3 className="mb-2 border-b border-border pb-2 text-sm font-semibold">{group.name}</h3>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  {group.monitorList.map((monitor) => renderMonitor(monitor))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
