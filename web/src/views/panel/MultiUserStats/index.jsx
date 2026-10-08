import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { ChartNoAxesColumn, Download, Loader2, Search, Users } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import CollapsibleFilter from '@/components/CollapsibleFilter';
import { API } from 'utils/api';
import { calculateQuota, showError, showSuccess, thousandsSeparator } from 'utils/common';

// ==============================|| PANEL — MULTI-USER TOKEN STATISTICS ||============================== //
// shadcn/Tailwind port of views/MultiUserStats. Admin tool that queries the same
// `/api/analytics/multi_user_stats` (+ `/export`) endpoints; only the UI layer is new.

function StatTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-md">
      <p className="mb-1 font-medium text-muted-foreground">{label}</p>
      <p className="font-mono font-semibold tabular-nums text-foreground">{'$' + calculateQuota(payload[0].value, 6)}</p>
    </div>
  );
}

export default function MultiUserStats() {
  const { t } = useTranslation();
  const [usernames, setUsernames] = useState('');
  const [startDate, setStartDate] = useState(dayjs().subtract(30, 'day').format('YYYY-MM-DD'));
  const [endDate, setEndDate] = useState(dayjs().format('YYYY-MM-DD'));
  const [searching, setSearching] = useState(false);
  const [statistics, setStatistics] = useState([]);
  const [modelUsage, setModelUsage] = useState([]);

  const params = () => ({ usernames: usernames.trim(), start_time: startDate, end_time: endDate });

  const handleSearch = async () => {
    if (!usernames.trim()) return showError(t('multi_user_stats_index.enterUsername'));
    setSearching(true);
    try {
      const res = await API.get('/api/analytics/multi_user_stats', { params: params() });
      const { success, message, data, model_usage } = res.data;
      if (success) {
        const stats = data || [];
        setStatistics(stats);
        setModelUsage(model_usage || []);
        showSuccess(
          stats.length === 0 ? t('multi_user_stats_index.searchNoData') : t('multi_user_stats_index.searchSuccess', { count: stats.length })
        );
      } else {
        showError(message);
      }
    } catch (error) {
      showError(`${t('multi_user_stats_index.queryFailed')}: ${error.response?.data?.message || error.message}`);
    } finally {
      setSearching(false);
    }
  };

  const handleExportCSV = async () => {
    if (!usernames.trim()) return showError(t('multi_user_stats_index.enterUsername'));
    setSearching(true);
    try {
      const res = await API.get('/api/analytics/multi_user_stats/export', { params: params(), responseType: 'blob' });
      const url = window.URL.createObjectURL(new Blob([res.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', `multi_user_stats_${startDate}_${endDate}.csv`);
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
      showSuccess(t('multi_user_stats_index.exportSuccess'));
    } catch (error) {
      showError(`${t('multi_user_stats_index.exportError')}: ${error.response?.data?.message || error.message}`);
    } finally {
      setSearching(false);
    }
  };

  const chartData = statistics.map((s) => ({ label: s.username, quota: Number(calculateQuota(s.quota, 6)) }));

  return (
    <div className="space-y-6">
      <CollapsibleFilter activeCount={usernames.trim() ? 1 : 0} contentClassName="space-y-4">
        <div className="space-y-4">
          <Label htmlFor="usernames">{t('multi_user_stats_index.usernamesLabel')}</Label>
          <Input
            id="usernames"
            value={usernames}
            onChange={(e) => setUsernames(e.target.value)}
            placeholder={t('multi_user_stats_index.usernamesPlaceholder')}
          />
          <p className="text-xs text-muted-foreground">{t('multi_user_stats_index.usernamesHelper')}</p>
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-4">
            <Label htmlFor="start_date">{t('multi_user_stats_index.startDate')}</Label>
            <Input id="start_date" type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
          </div>
          <div className="space-y-4">
            <Label htmlFor="end_date">{t('multi_user_stats_index.endDate')}</Label>
            <Input id="end_date" type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" onClick={handleSearch} disabled={searching}>
            {searching ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />}
            {searching ? t('multi_user_stats_index.searching') : t('multi_user_stats_index.searchButton')}
          </Button>
          <Button type="button" variant="outline" onClick={handleExportCSV} disabled={searching || statistics.length === 0}>
            <Download className="size-4" />
            {t('multi_user_stats_index.exportButton')}
          </Button>
        </div>
      </CollapsibleFilter>

      {statistics.length > 0 ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Users className="size-4 text-muted-foreground" />
                {t('multi_user_stats_index.userStatsTitle')}
              </CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <div className="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('multi_user_stats_index.username')}</TableHead>
                      <TableHead className="text-right">{t('multi_user_stats_index.requestCount')}</TableHead>
                      <TableHead className="text-right">{t('multi_user_stats_index.quota')}</TableHead>
                      <TableHead className="text-right">{t('multi_user_stats_index.inputTokens')}</TableHead>
                      <TableHead className="text-right">{t('multi_user_stats_index.outputTokens')}</TableHead>
                      <TableHead className="text-right">{t('multi_user_stats_index.requestTime')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {statistics.map((s, i) => (
                      <TableRow key={i}>
                        <TableCell className="font-medium">{s.username}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{thousandsSeparator(s.request_count)}</TableCell>
                        <TableCell className="text-right font-mono font-semibold tabular-nums text-foreground">${calculateQuota(s.quota, 6)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{thousandsSeparator(s.prompt_tokens)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{thousandsSeparator(s.completion_tokens)}</TableCell>
                        <TableCell className="text-right font-mono tabular-nums">{(s.request_time / 1000).toFixed(2)}s</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div className="md:hidden flex flex-col gap-3 p-3">
                {statistics.map((s, i) => (
                  <div key={i} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                    <div className="mb-2 text-sm font-medium">{s.username}</div>
                    <dl className="flex flex-col divide-y divide-border/60">
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('multi_user_stats_index.requestCount')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                          {thousandsSeparator(s.request_count)}
                        </dd>
                      </div>
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('multi_user_stats_index.quota')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono font-semibold tabular-nums text-foreground">
                          ${calculateQuota(s.quota, 6)}
                        </dd>
                      </div>
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('multi_user_stats_index.inputTokens')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                          {thousandsSeparator(s.prompt_tokens)}
                        </dd>
                      </div>
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('multi_user_stats_index.outputTokens')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                          {thousandsSeparator(s.completion_tokens)}
                        </dd>
                      </div>
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('multi_user_stats_index.requestTime')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                          {(s.request_time / 1000).toFixed(2)}s
                        </dd>
                      </div>
                    </dl>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ChartNoAxesColumn className="size-4 text-muted-foreground" />
                {t('multi_user_stats_index.quotaComparisonTitle')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <ResponsiveContainer width="100%" height={320}>
                <BarChart data={chartData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
                  <XAxis
                    dataKey="label"
                    tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }}
                    axisLine={{ stroke: 'var(--border)' }}
                    tickLine={false}
                  />
                  <YAxis
                    width={56}
                    tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }}
                    axisLine={false}
                    tickLine={false}
                    tickFormatter={(v) => '$' + v}
                  />
                  <Tooltip content={<StatTooltip />} cursor={{ fill: 'var(--muted)', opacity: 0.3 }} />
                  <Bar dataKey="quota" fill="var(--primary)" radius={[6, 6, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>

          {modelUsage.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">{t('multi_user_stats_index.modelUsageTitle')}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <div className="hidden md:block">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('multi_user_stats_index.username')}</TableHead>
                        <TableHead>{t('multi_user_stats_index.modelName')}</TableHead>
                        <TableHead className="text-right">{t('multi_user_stats_index.requestCount')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {modelUsage.map((m, i) => (
                        <TableRow key={i}>
                          <TableCell className="font-medium">{m.username}</TableCell>
                          <TableCell>{m.model_name}</TableCell>
                          <TableCell className="text-right font-mono tabular-nums">{thousandsSeparator(m.request_count)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>

                <div className="md:hidden flex flex-col gap-3 p-3">
                  {modelUsage.map((m, i) => (
                    <div key={i} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                      <div className="mb-2 text-sm font-medium">{m.username}</div>
                      <dl className="flex flex-col divide-y divide-border/60">
                        <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                          <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            {t('multi_user_stats_index.modelName')}
                          </dt>
                          <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{m.model_name}</dd>
                        </div>
                        <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                          <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            {t('multi_user_stats_index.requestCount')}
                          </dt>
                          <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                            {thousandsSeparator(m.request_count)}
                          </dd>
                        </div>
                      </dl>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}
        </>
      ) : (
        !searching && (
          <Card>
            <CardContent className="flex flex-col items-center gap-3 py-16 text-center">
              <ChartNoAxesColumn className="size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">{t('multi_user_stats_index.emptyTitle')}</p>
            </CardContent>
          </Card>
        )
      )}
    </div>
  );
}
