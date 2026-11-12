import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { Download, Loader2, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import TimeRangeSelect, { computeRange } from '@/components/TimeRangeSelect';
import { API } from 'utils/api';
import { calculateQuota, showError, showSuccess, thousandsSeparator } from 'utils/common';
import StatsTabs from '../Analytics/StatsTabs';

// ==============================|| PANEL — MULTI-USER TOKEN STATISTICS ||============================== //
// Admin tool on `/api/analytics/multi_user_stats` (+ `/export`). One toolbar row: the usernames
// (comma separated), the time range, query and CSV export. Results are a table per user (56px
// rows on phones), a spend comparison when there is more than one user, and per-model requests.

const DEFAULT_PRESET = '1mo';
const day = (ts) => dayjs.unix(Math.min(ts, dayjs().unix())).format('YYYY-MM-DD');
// Two decimals in the list; the full figure on hover.
const money = (quota) => `$${calculateQuota(quota, 2)}`;
const moneyFull = (quota) => `$${calculateQuota(quota, 6)}`;

function StatTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-md">
      <p className="mb-1 text-muted-foreground">{label}</p>
      <p className="font-medium tabular-nums text-foreground">{'$' + payload[0].value}</p>
    </div>
  );
}

function ResultCard({ title, children }) {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="p-4 pb-2 sm:p-5 sm:pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
      </CardHeader>
      <CardContent className="p-0">{children}</CardContent>
    </Card>
  );
}

export default function MultiUserStats() {
  const { t, i18n } = useTranslation();
  const [usernames, setUsernames] = useState('');
  const [preset, setPreset] = useState(DEFAULT_PRESET);
  const [range, setRange] = useState(() => computeRange(DEFAULT_PRESET));
  const [searching, setSearching] = useState(false);
  const [statistics, setStatistics] = useState([]);
  const [modelUsage, setModelUsage] = useState([]);

  const startDate = day(range.start_timestamp);
  const endDate = day(range.end_timestamp);
  const params = () => ({ usernames: usernames.trim(), start_time: startDate, end_time: endDate });

  const handleSearch = async (e) => {
    e?.preventDefault();
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
    <div className="space-y-3 sm:space-y-4">
      <StatsTabs />
      <form className="flex flex-wrap items-center gap-2" onSubmit={handleSearch}>
        <div className="relative min-w-0 flex-1 basis-full sm:basis-64">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-9 pl-8"
            value={usernames}
            onChange={(e) => setUsernames(e.target.value)}
            placeholder={t('multi_user_stats_index.usernamesPlaceholder')}
            aria-label={t('multi_user_stats_index.usernamesLabel')}
            title={t('multi_user_stats_index.usernamesHelper')}
          />
        </div>
        <TimeRangeSelect
          value={range}
          preset={preset}
          onApply={(next, id) => {
            setRange(next);
            setPreset(id);
          }}
          t={t}
          locale={i18n.language}
        />
        <Button type="submit" className="h-9" disabled={searching}>
          {searching && <Loader2 className="size-4 animate-spin" />}
          {t('multi_user_stats_index.searchButton')}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="icon"
          className="size-9 shrink-0"
          onClick={handleExportCSV}
          disabled={searching || statistics.length === 0}
          aria-label={t('multi_user_stats_index.exportButton')}
          title={t('multi_user_stats_index.exportButton')}
        >
          <Download className="size-4" />
        </Button>
      </form>

      {statistics.length > 0 ? (
        <>
          <ResultCard title={t('multi_user_stats_index.userStatsTitle')}>
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
                      <TableCell className="text-right tabular-nums">{thousandsSeparator(s.request_count)}</TableCell>
                      <TableCell className="text-right font-medium tabular-nums" title={moneyFull(s.quota)}>
                        {money(s.quota)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">{thousandsSeparator(s.prompt_tokens)}</TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">
                        {thousandsSeparator(s.completion_tokens)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums text-muted-foreground">{(s.request_time / 1000).toFixed(2)}s</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            {/* Phone rows: the user and spend on top, requests and tokens (in / out) below. */}
            <ul className="divide-y divide-border border-t border-border md:hidden">
              {statistics.map((s, i) => (
                <li key={i} className="flex h-14 flex-col justify-center px-4">
                  <div className="flex items-baseline gap-3 text-sm">
                    <span className="min-w-0 truncate font-medium">{s.username}</span>
                    <span className="ml-auto shrink-0 font-medium tabular-nums" title={moneyFull(s.quota)}>
                      {money(s.quota)}
                    </span>
                  </div>
                  <div className="mt-0.5 flex items-baseline gap-3 text-xs tabular-nums text-muted-foreground">
                    <span className="min-w-0 truncate">
                      {thousandsSeparator(s.request_count)} {t('multi_user_stats_index.requestCount')}
                    </span>
                    <span className="ml-auto shrink-0">
                      {thousandsSeparator(s.prompt_tokens)} / {thousandsSeparator(s.completion_tokens)}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          </ResultCard>

          {statistics.length > 1 && (
            <Card>
              <CardHeader className="p-4 pb-2 sm:p-5 sm:pb-2">
                <CardTitle className="text-sm font-medium">{t('multi_user_stats_index.quotaComparisonTitle')}</CardTitle>
              </CardHeader>
              <CardContent className="p-4 pt-0 sm:p-5 sm:pt-0">
                <ResponsiveContainer width="100%" height={240}>
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
                    <Bar dataKey="quota" fill="var(--primary)" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>
          )}

          {modelUsage.length > 0 && (
            <ResultCard title={t('multi_user_stats_index.modelUsageTitle')}>
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
                        <TableCell className="text-right tabular-nums">{thousandsSeparator(m.request_count)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <ul className="divide-y divide-border border-t border-border md:hidden">
                {modelUsage.map((m, i) => (
                  <li key={i} className="flex h-14 flex-col justify-center px-4">
                    <div className="flex items-baseline gap-3 text-sm">
                      <span className="min-w-0 truncate font-medium">{m.model_name}</span>
                      <span className="ml-auto shrink-0 tabular-nums">{thousandsSeparator(m.request_count)}</span>
                    </div>
                    <div className="mt-0.5 truncate text-xs text-muted-foreground">{m.username}</div>
                  </li>
                ))}
              </ul>
            </ResultCard>
          )}
        </>
      ) : (
        !searching && (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">{t('multi_user_stats_index.emptyTitle')}</CardContent>
          </Card>
        )
      )}
    </div>
  );
}
