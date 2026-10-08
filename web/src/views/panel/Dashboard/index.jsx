import { useEffect, useRef, useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { BarChart3, Boxes, DollarSign, Gauge, Network, PiggyBank, Send, Wallet } from 'lucide-react';

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { API } from 'utils/api';
import { calculateQuota, renderNumber, showError } from 'utils/common';
import { compactCurrency } from 'utils/money';
import { getLastSevenDays, getLastNDays } from 'utils/chart';
import { useOrg } from 'contexts/OrgContext';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import OrgUsagePanel from './OrgUsagePanel';
import StatCard from './StatCard';
import UsageChart from './UsageChart';
import ActivityTable from './ActivityTable';
import SupportModels from './SupportModels';
import ModelUsagePieChart from './ModelUsagePieChart';
import ModelBarChart from './ModelBarChart';
import QuickStartCard from './QuickStartCard';
import InviteCard from '@/components/InviteCard';
import StatusPanel from './StatusPanel';

// ==============================|| DASHBOARD (shadcn) ||============================== //
// Functional parity with the v1 MUI dashboard: same `/api/user/dashboard` and
// `/api/user/dashboard/rate` calls, same 7-day grouping. Only the UI layer is new.

// range → 天数映射;上限由后端 clamp 到 MaxLogQuerySpanDays
const RANGE_DAYS = { '7d': 7, '30d': 30, '90d': 90 };

function groupByDay(data, dayCount = 7) {
  const days = getLastNDays(dayCount);
  return days.map((date) => {
    const dayData = data.filter((item) => item.Date === date);
    const requests = dayData.reduce((s, i) => s + i.RequestCount, 0);
    const quotaRaw = dayData.reduce((s, i) => s + i.Quota, 0);
    const prompt = dayData.reduce((s, i) => s + i.PromptTokens, 0);
    const completion = dayData.reduce((s, i) => s + i.CompletionTokens, 0);
    const duration = dayData.reduce((s, i) => s + (i.RequestTime || 0), 0);
    return {
      date,
      requests,
      quota: Number(calculateQuota(quotaRaw, 6)),
      tokens: prompt + completion,
      tokensLabel: `${prompt}/${completion}`,
      duration: (duration / 1000).toFixed(3)
    };
  });
}

// Today-vs-yesterday change for a stat card. Returns null when there is no
// prior day to compare against; guards against NaN/Infinity when yesterday is 0.
function computeTrend(todayVal, yesterdayVal) {
  if (yesterdayVal === undefined || yesterdayVal === null) return null;
  const todayNum = Number(todayVal) || 0;
  const yesterdayNum = Number(yesterdayVal) || 0;
  if (todayNum === 0 && yesterdayNum === 0) return { direction: 'flat', percent: 0 };
  if (yesterdayNum === 0) return { direction: 'up', percent: 100 };
  const pct = Math.round(((todayNum - yesterdayNum) / yesterdayNum) * 100);
  if (pct > 0) return { direction: 'up', percent: pct };
  if (pct < 0) return { direction: 'down', percent: Math.abs(pct) };
  return { direction: 'flat', percent: 0 };
}

// Aggregate request counts per model for the usage donut.
function buildModelUsage(data) {
  const usage = {};
  for (const item of data) {
    usage[item.ModelName] = (usage[item.ModelName] || 0) + item.RequestCount;
  }
  return Object.entries(usage).map(([name, value]) => ({ name, value }));
}

// Per-model 7-day cost rows for the stacked bar chart (mirrors v1 getBarDataGroup).
function buildModelBar(data) {
  const days = getLastSevenDays();
  const idx = new Map(days.map((d, i) => [d, i]));
  const rows = days.map((d) => ({ label: d.slice(5) }));
  const series = [];
  const seen = new Set();
  let total = 0;
  for (const item of data) {
    if (!seen.has(item.ModelName)) {
      seen.add(item.ModelName);
      series.push(item.ModelName);
    }
    const i = idx.get(item.Date);
    if (i === undefined) continue;
    const cost = Number(calculateQuota(item.Quota, 6));
    rows[i][item.ModelName] = (rows[i][item.ModelName] || 0) + cost;
    total += cost;
  }
  return { rows, series, total: Number(total.toFixed(6)) };
}

export default function Dashboard() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  // 组织上下文:/api/user/dashboard 是个人维度且不参与组织改写,
  // 整页换成组织用量视图,避免组织上下文展示个人数据(规格 §5.5 数据串台风险)
  const { currentOrgId } = useOrg();
  const [loading, setLoading] = useState(true);
  const [grouped, setGrouped] = useState([]);
  const [modelUsage, setModelUsage] = useState([]);
  const [modelBar, setModelBar] = useState({ rows: [], series: [], total: 0 });
  const [rate, setRate] = useState({ rpm: 0, tpm: 0 });
  // 提示缓存命中率(近 7 天):cached_read_tokens / prompt_tokens;无分母显示 —
  const [cacheHitRate, setCacheHitRate] = useState('—');
  // 折扣节省额(近 7 天,USD):口径同日志单行划线原价 sum(original-实收)
  const [weekSaved, setWeekSaved] = useState(0);
  // 趋势图时间范围(仅驱动趋势图,其余卡片/图表沿用近 7 天)
  const [range, setRange] = useState('7d');
  const [trendGrouped, setTrendGrouped] = useState([]);
  const [trendLoading, setTrendLoading] = useState(false);

  const fetchDashboard = async () => {
    try {
      const res = await API.get('/api/user/dashboard');
      const { success, message, data } = res.data;
      if (success) {
        if (data) {
          setGrouped(groupByDay(data));
          setModelUsage(buildModelUsage(data));
          setModelBar(buildModelBar(data));
          // 默认 7d 复用主拉取数据,避免额外请求
          setTrendGrouped(groupByDay(data));
        }
      } else {
        showError(message);
      }
    } catch (error) {
      // network errors are surfaced globally by the API interceptor
    } finally {
      setLoading(false);
    }
  };

  // 切换到非默认范围时单独拉取趋势数据
  // reqId 守卫(UX-19):快切时间档位时丢弃过期响应,结果与 finally 双分支均受守卫。
  const trendGuardRef = useRef(null);
  if (!trendGuardRef.current) trendGuardRef.current = createRequestGuard();

  const fetchTrend = (r) =>
    runGuardedFetch(trendGuardRef.current, () => API.get('/api/user/dashboard', { params: r === '7d' ? {} : { range: r } }), {
      onStart: () => setTrendLoading(true),
      onResult: (res) => {
        const { success, data } = res.data;
        if (success && data) setTrendGrouped(groupByDay(data, RANGE_DAYS[r] || 7));
      },
      // ignore — trend fetch is non-critical; network errors surfaced by the API interceptor
      onFinally: () => setTrendLoading(false)
    });

  const handleRangeChange = (r) => {
    if (r === range) return;
    setRange(r);
    fetchTrend(r);
  };

  // /api/user/dashboard 不含缓存明细/折扣口径,命中率与节省额单走 self/analytics 聚合(近 7 天,单查询)
  const fetchSelfAnalytics = async () => {
    try {
      const startTimestamp = Math.floor(Date.now() / 1000) - 7 * 86400;
      const res = await API.get('/api/user/self/analytics', { params: { group_by: 'date', start_timestamp: startTimestamp } });
      const { success, data } = res.data;
      if (success && Array.isArray(data)) {
        const cachedRead = data.reduce((s, r) => s + Number(r.cached_read_tokens || 0), 0);
        const prompt = data.reduce((s, r) => s + Number(r.prompt_tokens || 0), 0);
        setCacheHitRate(prompt > 0 ? `${((cachedRead / prompt) * 100).toFixed(1)}%` : '—');
        const savedRaw = data.reduce((s, r) => s + Number(r.saved_quota || 0), 0);
        setWeekSaved(Number(calculateQuota(savedRaw, 6)));
      }
    } catch (error) {
      // ignore — cache hit rate / savings are non-critical
    }
  };

  const fetchRate = async () => {
    try {
      const res = await API.get('/api/user/dashboard/rate');
      const { success, data } = res.data;
      if (success && data) setRate({ rpm: data.rpm || 0, tpm: data.tpm || 0 });
    } catch (error) {
      // ignore — rate is non-critical
    }
  };

  useEffect(() => {
    // 组织上下文不拉个人数据;切回个人上下文时重新拉取
    if (currentOrgId) return;
    fetchDashboard();
    fetchRate();
    fetchSelfAnalytics();
  }, [currentOrgId]);

  const today = grouped[grouped.length - 1] || { requests: 0, quota: 0, tokens: 0 };
  const yesterday = grouped[grouped.length - 2];
  const weekQuota = grouped.reduce((s, d) => s + d.quota, 0);
  const todayCost = compactCurrency(today.quota);
  const weekCost = compactCurrency(weekQuota);
  const weekSavedCost = compactCurrency(weekSaved);

  const seriesRequests = grouped.map((d) => d.requests);
  const seriesQuota = grouped.map((d) => d.quota);
  const seriesTokens = grouped.map((d) => d.tokens);

  const chartData = trendGrouped.map((d) => ({
    label: d.date.slice(5),
    requests: d.requests,
    quota: d.quota,
    tokens: d.tokens
  }));

  const tableRows = grouped
    .map((d) => ({
      date: d.date,
      requests: d.requests,
      amount: d.quota,
      tokens: d.tokensLabel,
      duration: d.duration
    }))
    .reverse();

  if (currentOrgId) {
    return <OrgUsagePanel />;
  }

  const dashboardContent = (
    <div className="space-y-6">
      <SupportModels />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <StatCard
          label={t('dashboard_index.today_requests')}
          value={renderNumber(today.requests)}
          icon={Send}
          loading={loading}
          series={seriesRequests}
          seriesColor="#60A5FA"
          trend={yesterday ? computeTrend(today.requests, yesterday.requests) : null}
        />
        <StatCard
          label={t('dashboard_index.today_consumption')}
          value={todayCost.text}
          valueTitle={todayCost.title}
          icon={DollarSign}
          accent
          loading={loading}
          series={seriesQuota}
          seriesColor="#FBBF24"
          trend={yesterday ? computeTrend(today.quota, yesterday.quota) : null}
        />
        <StatCard
          label={t('dashboard_index.today_tokens')}
          value={renderNumber(today.tokens)}
          icon={Boxes}
          loading={loading}
          series={seriesTokens}
          seriesColor="#F87171"
          trend={yesterday ? computeTrend(today.tokens, yesterday.tokens) : null}
        />
        <StatCard label={t('dashboard_index.RPM')} value={renderNumber(rate.rpm)} icon={BarChart3} loading={loading} />
        <StatCard label={t('dashboard_index.TPM')} value={renderNumber(rate.tpm)} icon={Network} loading={loading} />
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

      <UsageChart data={chartData} loading={loading || trendLoading} range={range} onRangeChange={handleRangeChange} />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <ModelBarChart data={modelBar.rows} series={modelBar.series} total={modelBar.total} loading={loading} />
          <ActivityTable rows={tableRows} loading={loading} />
        </div>
        <div className="space-y-6">
          <ModelUsagePieChart data={modelUsage} loading={loading} />
          {siteInfo.builtin_chat_enabled !== false && <QuickStartCard />}
          {siteInfo.invite_reward_enabled !== false && <InviteCard />}
        </div>
      </div>
    </div>
  );

  return (
    <div className="space-y-6">
      {siteInfo.UptimeEnabled ? (
        <Tabs defaultValue="dashboard">
          <TabsList className="h-auto flex-wrap">
            <TabsTrigger value="dashboard">{t('dashboard_index.tab_dashboard')}</TabsTrigger>
            <TabsTrigger value="status">{t('dashboard_index.tab_status')}</TabsTrigger>
          </TabsList>
          <TabsContent value="dashboard">{dashboardContent}</TabsContent>
          <TabsContent value="status">
            <StatusPanel />
          </TabsContent>
        </Tabs>
      ) : (
        dashboardContent
      )}
    </div>
  );
}
