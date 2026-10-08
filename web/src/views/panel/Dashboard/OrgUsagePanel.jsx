import PropTypes from 'prop-types';
import { useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Boxes, DollarSign, Download, Gauge, Loader2, PiggyBank, Send, UserRound, Users, Wallet } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { API } from 'utils/api';
import { calculateQuota, renderNumber, renderSpend, showError, showSuccess, SpendAmount } from 'utils/common';
import { compactCurrency } from 'utils/money';
import { getLastSevenDays, getLastNDays } from 'utils/chart';
import { useOrg } from 'contexts/OrgContext';
import StatCard from './StatCard';
import UsageChart from './UsageChart';
import { exportOrgUsageCsv } from './orgUsageExport';

// ==============================|| DASHBOARD — ORG USAGE PANEL ||============================== //
// Organization-context replacement for the personal dashboard (spec §3.6):
// 7-day org usage from /api/org/:id/analytics, aggregated by date for the
// stat cards + trend chart, and by member / token / model for the tables.
// Member visibility policy is enforced server-side.

// range → 天数映射;上限由后端 clamp
const RANGE_DAYS = { '7d': 7, '30d': 30, '90d': 90 };

const DIMENSIONS = [
  { key: 'member', labelKey: 'org.usage.byMember', nameOf: (r) => r.username || (r.member_id ? `#${r.member_id}` : '-') },
  { key: 'token', labelKey: 'org.usage.byToken', nameOf: (r) => r.token_name || '-' },
  { key: 'model', labelKey: 'org.usage.byModel', nameOf: (r) => r.model_name || '-' }
];

function DimensionTable({ t, rows, nameOf }) {
  return (
    <>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead>{t('org.usage.dimension')}</TableHead>
              <TableHead className="text-right">{t('org.usage.requests')}</TableHead>
              <TableHead className="text-right">{t('org.usage.tokens')}</TableHead>
              <TableHead className="text-right">{t('org.usage.quota')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} className="h-24 text-center text-sm text-muted-foreground">
                  {t('dashboard_index.no_data', { defaultValue: 'No data' })}
                </TableCell>
              </TableRow>
            ) : (
              rows.map((r, idx) => (
                <TableRow key={idx}>
                  <TableCell className="font-medium">{nameOf(r)}</TableCell>
                  <TableCell className="text-right font-mono tabular-nums">{renderNumber(r.request_count)}</TableCell>
                  <TableCell className="text-right font-mono tabular-nums">
                    {renderNumber((r.prompt_tokens || 0) + (r.completion_tokens || 0))}
                  </TableCell>
                  <TableCell className="text-right font-mono tabular-nums">
                    <SpendAmount value={calculateQuota(r.quota || 0, 6)} />
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
      {rows.length === 0 ? (
        <div className="md:hidden rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          {t('dashboard_index.no_data', { defaultValue: 'No data' })}
        </div>
      ) : (
        <div className="md:hidden flex flex-col gap-3">
          {rows.map((r, idx) => (
            <div key={idx} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
              <div className="mb-2 text-sm font-medium">{nameOf(r)}</div>
              <dl className="flex flex-col divide-y divide-border/60">
                <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                  <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t('org.usage.requests')}
                  </dt>
                  <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                    {renderNumber(r.request_count)}
                  </dd>
                </div>
                <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                  <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t('org.usage.tokens')}
                  </dt>
                  <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                    {renderNumber((r.prompt_tokens || 0) + (r.completion_tokens || 0))}
                  </dd>
                </div>
                <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                  <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t('org.usage.quota')}
                  </dt>
                  <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                    <SpendAmount value={calculateQuota(r.quota || 0, 6)} />
                  </dd>
                </div>
              </dl>
            </div>
          ))}
        </div>
      )}
    </>
  );
}

DimensionTable.propTypes = {
  t: PropTypes.func.isRequired,
  rows: PropTypes.array,
  nameOf: PropTypes.func
};

function BudgetBar({ progress }) {
  const color = progress < 60 ? 'bg-emerald-500' : progress < 85 ? 'bg-amber-500' : 'bg-red-500';
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
      <div className={cn('h-full rounded-full transition-all', color)} style={{ width: `${progress}%` }} />
    </div>
  );
}

BudgetBar.propTypes = { progress: PropTypes.number };

// N3:侧栏上下文卡片收敛为单行切换器后,组织账户的余额 / 成员数 / 组织与本人周期预算 /
// 本人个人余额改由本卡片承载。可见性沿用接口本身的角色裁剪:池余额与组织预算仅
// Owner/Admin 返回,Member 看到降级提示与自己的预算。
function OrgAccountCard() {
  const { t } = useTranslation();
  const { orgDetail } = useOrg();
  const user = useSelector((state) => state.account.user);

  const quotaPerUnit = Number(localStorage.getItem('quota_per_unit')) || 500000;
  const myBalance = (user?.quota || 0) / quotaPerUnit;
  const orgQuota = orgDetail?.quota;
  const orgBalance = typeof orgQuota === 'number' ? orgQuota / quotaPerUnit : null;
  const memberCount = orgDetail?.member_count;

  const teamBudget = orgDetail?.budget;
  const teamBudgetUsed = orgDetail?.budget_used || 0;
  const hasTeamBudget = teamBudget?.period && teamBudget.limit > 0;
  const teamBudgetProgress = hasTeamBudget ? Math.min((teamBudgetUsed / teamBudget.limit) * 100, 100) : 0;

  const myBudget = orgDetail?.my_budget;
  const myBudgetUsed = orgDetail?.my_budget_used || 0;
  const myBudgetProgress = myBudget?.limit > 0 ? Math.min((myBudgetUsed / myBudget.limit) * 100, 100) : 0;

  const periodLabel = (period) => t(`orgPage.limits.period_${period}`, { defaultValue: period });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('org.accountTitle')}</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
        <div className="space-y-1.5">
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <Wallet className="size-3.5" />
              {orgBalance !== null ? t('org.poolBalance') : t('org.balanceHidden')}
            </span>
            {orgBalance !== null && <span className="font-mono tabular-nums">${orgBalance.toFixed(2)}</span>}
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <Users className="size-3.5" />
              {t('org.members')}
            </span>
            <span className="font-mono tabular-nums">{typeof memberCount === 'number' ? memberCount : '—'}</span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <UserRound className="size-3.5" />
              {t('org.myBalance')}
            </span>
            <span className="font-mono tabular-nums">${myBalance.toFixed(2)}</span>
          </div>
        </div>
        <div className="space-y-2.5">
          {hasTeamBudget && (
            <div className="space-y-1">
              <div className="flex items-center justify-between gap-2">
                <span className="flex items-center gap-1.5 text-muted-foreground">
                  <Gauge className="size-3.5" />
                  {t('org.teamBudget')}
                </span>
                <span className="truncate font-mono tabular-nums">
                  {`${renderSpend(teamBudgetUsed)} / ${renderSpend(teamBudget.limit)} · ${periodLabel(teamBudget.period)}`}
                </span>
              </div>
              <BudgetBar progress={teamBudgetProgress} />
            </div>
          )}
          {myBudget ? (
            <div className="space-y-1">
              <div className="flex items-center justify-between gap-2">
                <span className="flex items-center gap-1.5 text-muted-foreground">
                  <Gauge className="size-3.5" />
                  {t('org.myBudget')}
                </span>
                <span className="truncate font-mono tabular-nums">
                  {`${renderSpend(myBudgetUsed)} / ${renderSpend(myBudget.limit)} · ${periodLabel(myBudget.period)}`}
                </span>
              </div>
              <BudgetBar progress={myBudgetProgress} />
            </div>
          ) : (
            <div className="flex items-center justify-between gap-2">
              <span className="flex items-center gap-1.5 text-muted-foreground">
                <Gauge className="size-3.5" />
                {t('org.myBudget')}
              </span>
              <span>{t('org.budgetUnlimited')}</span>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

export default function OrgUsagePanel() {
  const { t } = useTranslation();
  const { currentOrgId } = useOrg();
  const [loading, setLoading] = useState(true);
  const [dateRows, setDateRows] = useState([]);
  const [dims, setDims] = useState({ member: [], token: [], model: [] });
  // 趋势图时间范围(仅驱动趋势图,卡片/维度表沿用近 7 天)
  const [trendRange, setTrendRange] = useState('7d');
  const [trendRows, setTrendRows] = useState([]);
  const [trendLoading, setTrendLoading] = useState(false);
  const [exporting, setExporting] = useState(false);

  // 导出维度表/卡片同口径的近 7 天聚合(与主拉取的时间窗口一致);服务端复用 /analytics 数据接口鉴权
  const handleExport = async () => {
    if (exporting || !currentOrgId) return;
    setExporting(true);
    try {
      const startTimestamp = dayjs().subtract(6, 'day').startOf('day').unix();
      await exportOrgUsageCsv({ orgId: currentOrgId, startTimestamp });
      showSuccess(t('org.usage.exportSuccess'));
    } catch (error) {
      showError(`${t('org.usage.exportError')}: ${error.message}`);
    } finally {
      setExporting(false);
    }
  };

  useEffect(() => {
    if (!currentOrgId) return;
    let cancelled = false;
    setLoading(true);
    (async () => {
      const startTimestamp = dayjs().subtract(6, 'day').startOf('day').unix();
      const fetchGroup = async (groupBy) => {
        const res = await API.get(`/api/org/${currentOrgId}/analytics`, {
          params: { group_by: groupBy, start_timestamp: startTimestamp }
        });
        return res.data?.success ? res.data.data || [] : [];
      };
      try {
        const [byDate, byMember, byToken, byModel] = await Promise.all(['date', 'member', 'token', 'model'].map(fetchGroup));
        if (cancelled) return;
        setDateRows(byDate);
        setDims({ member: byMember, token: byToken, model: byModel });
        // 默认 7d 复用主拉取的按日数据,避免额外请求
        setTrendRows(byDate);
      } catch (error) {
        // 错误已由全局拦截器提示
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [currentOrgId]);

  // 切换到非默认范围时单独拉取按日趋势数据
  const fetchTrend = async (r) => {
    if (!currentOrgId) return;
    setTrendLoading(true);
    try {
      const dayCount = RANGE_DAYS[r] || 7;
      const startTimestamp = dayjs()
        .subtract(dayCount - 1, 'day')
        .startOf('day')
        .unix();
      const res = await API.get(`/api/org/${currentOrgId}/analytics`, {
        params: { group_by: 'date', start_timestamp: startTimestamp }
      });
      if (res.data?.success) setTrendRows(res.data.data || []);
    } catch (error) {
      // ignore — trend fetch is non-critical
    } finally {
      setTrendLoading(false);
    }
  };

  const handleTrendRangeChange = (r) => {
    if (r === trendRange) return;
    setTrendRange(r);
    fetchTrend(r);
  };

  // 卡片/维度表沿用最近 7 天,缺数据的日期补零
  const days = getLastSevenDays();
  const byDateMap = Object.fromEntries(dateRows.map((r) => [r.date, r]));
  const grouped = days.map((date) => {
    const r = byDateMap[date] || {};
    return {
      date,
      requests: Number(r.request_count || 0),
      quota: Number(calculateQuota(r.quota || 0, 6)),
      tokens: Number(r.prompt_tokens || 0) + Number(r.completion_tokens || 0),
      cachedRead: Number(r.cached_read_tokens || 0),
      prompt: Number(r.prompt_tokens || 0),
      saved: Number(calculateQuota(r.saved_quota || 0, 6))
    };
  });
  const today = grouped[grouped.length - 1] || { requests: 0, quota: 0, tokens: 0 };
  const weekQuota = grouped.reduce((s, d) => s + d.quota, 0);
  const weekSaved = grouped.reduce((s, d) => s + d.saved, 0);
  const todayCost = compactCurrency(today.quota);
  const weekCost = compactCurrency(weekQuota);
  const weekSavedCost = compactCurrency(weekSaved);
  // 命中率口径:cached_read_tokens / prompt_tokens(近 7 天);无分母显示 —,不除零
  const weekCachedRead = grouped.reduce((s, d) => s + d.cachedRead, 0);
  const weekPrompt = grouped.reduce((s, d) => s + d.prompt, 0);
  const cacheHitRate = weekPrompt > 0 ? `${((weekCachedRead / weekPrompt) * 100).toFixed(1)}%` : '—';

  // 趋势图按所选范围对齐 N 天
  const trendDays = getLastNDays(RANGE_DAYS[trendRange] || 7);
  const trendByDateMap = Object.fromEntries(trendRows.map((r) => [r.date, r]));
  const chartData = trendDays.map((date) => {
    const r = trendByDateMap[date] || {};
    return {
      label: date.slice(5),
      requests: Number(r.request_count || 0),
      quota: Number(calculateQuota(r.quota || 0, 6)),
      tokens: Number(r.prompt_tokens || 0) + Number(r.completion_tokens || 0)
    };
  });

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-6">
        <StatCard
          label={t('dashboard_index.today_requests')}
          value={renderNumber(today.requests)}
          icon={Send}
          loading={loading}
          series={grouped.map((d) => d.requests)}
          seriesColor="#60A5FA"
        />
        <StatCard
          label={t('dashboard_index.today_consumption')}
          value={todayCost.text}
          valueTitle={todayCost.title}
          icon={DollarSign}
          accent
          loading={loading}
          series={grouped.map((d) => d.quota)}
          seriesColor="#FBBF24"
        />
        <StatCard
          label={t('dashboard_index.today_tokens')}
          value={renderNumber(today.tokens)}
          icon={Boxes}
          loading={loading}
          series={grouped.map((d) => d.tokens)}
          seriesColor="#F87171"
        />
        <StatCard
          label={t('dashboard_index.week_total_consumption')}
          value={weekCost.text}
          valueTitle={weekCost.title}
          icon={Wallet}
          loading={loading}
        />
        <StatCard
          label={t('dashboard_index.week_savings')}
          value={weekSavedCost.text}
          valueTitle={weekSavedCost.title}
          hint={t('dashboard_index.week_savings_hint')}
          icon={PiggyBank}
          loading={loading}
        />
        <StatCard
          label={t('dashboard_index.cache_hit_rate')}
          value={cacheHitRate}
          hint={t('dashboard_index.cache_hit_rate_hint')}
          icon={Gauge}
          loading={loading}
        />
      </div>

      <OrgAccountCard />

      <UsageChart data={chartData} loading={loading || trendLoading} range={trendRange} onRangeChange={handleTrendRangeChange} />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-2 space-y-0">
          <CardTitle className="text-base">{t('org.usage.title')}</CardTitle>
          <Button variant="outline" size="sm" onClick={handleExport} disabled={exporting || loading}>
            {exporting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
            <span className="ml-1.5">{exporting ? t('org.usage.exporting') : t('org.usage.export')}</span>
          </Button>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="member">
            <TabsList>
              {DIMENSIONS.map((d) => (
                <TabsTrigger key={d.key} value={d.key}>
                  {t(d.labelKey)}
                </TabsTrigger>
              ))}
            </TabsList>
            {DIMENSIONS.map((d) => (
              <TabsContent key={d.key} value={d.key}>
                {loading ? (
                  <div className="h-40 w-full animate-pulse rounded-md bg-muted" />
                ) : (
                  <DimensionTable t={t} rows={dims[d.key]} nameOf={d.nameOf} />
                )}
              </TabsContent>
            ))}
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
