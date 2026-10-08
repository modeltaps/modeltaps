import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Activity, ChevronDown, ChevronUp, Coins, Wallet } from 'lucide-react';
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { calculateQuota, formatSpendAmount, renderNumber, renderSpend, showError, SpendAmount } from 'utils/common';

// ==============================|| ORGANIZATION — USAGE TAB (T14) ||============================== //
// Usage reports from /api/org/:id/analytics + /logs/stat (spec §3.6): time range +
// total consumption card, then member / token / model sortable tables and a daily
// trend chart. Member visibility policy is enforced server-side; the member filter
// is hidden when a plain member cannot see others' usage.

const RANGES = [
  { key: 'today', i18n: 'rangeToday' },
  { key: '7d', i18n: 'range7d' },
  { key: '30d', i18n: 'range30d' },
  { key: 'custom', i18n: 'rangeCustom' }
];

const DIMENSIONS = [
  { key: 'member', i18n: 'byMember', nameOf: (r) => r.username || (r.member_id ? `#${r.member_id}` : '-') },
  { key: 'token', i18n: 'byToken', nameOf: (r) => r.token_name || '-' },
  { key: 'model', i18n: 'byModel', nameOf: (r) => r.model_name || '-' }
];

const SORTABLE_COLUMNS = [
  { key: 'request_count', i18n: 'colRequests' },
  { key: 'quota', i18n: 'colQuota' },
  { key: 'prompt_tokens', i18n: 'colInputTokens' },
  { key: 'completion_tokens', i18n: 'colOutputTokens' }
];

const METRICS = [
  { key: 'requests', i18n: 'metricRequests', format: (v) => Number(v).toLocaleString() },
  { key: 'quota', i18n: 'metricQuota', format: (v) => formatSpendAmount(v) },
  { key: 'tokens', i18n: 'metricTokens', format: (v) => Number(v).toLocaleString() }
];

// 起止时间(unix 秒)展开为逐日日期串列表,缺数据日期由调用方补零
function getDateList(startTs, endTs) {
  const dates = [];
  let day = dayjs.unix(startTs).startOf('day');
  const last = dayjs.unix(endTs).startOf('day');
  while (!day.isAfter(last) && dates.length < 366) {
    dates.push(day.format('YYYY-MM-DD'));
    day = day.add(1, 'day');
  }
  return dates;
}

// 与 OverviewTab 的 StatCard 同构,保证两个 Tab 的卡片栅格视觉一致
function StatCard({ icon: Icon, label, value }) {
  return (
    <Card className="flex items-center justify-between gap-3 p-5">
      <div className="min-w-0 space-y-1">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className="truncate text-2xl font-semibold tabular-nums">{value}</p>
      </div>
      <span className="flex size-10 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
        <Icon className="size-5" />
      </span>
    </Card>
  );
}

StatCard.propTypes = {
  icon: PropTypes.elementType.isRequired,
  label: PropTypes.string.isRequired,
  value: PropTypes.node
};

function DimensionTable({ t, rows, nameOf, onRowClick }) {
  const [sort, setSort] = useState({ key: 'quota', dir: 'desc' });
  const sorted = useMemo(() => {
    const list = [...rows];
    list.sort((a, b) => {
      const av = Number(a[sort.key] || 0);
      const bv = Number(b[sort.key] || 0);
      return sort.dir === 'asc' ? av - bv : bv - av;
    });
    return list;
  }, [rows, sort]);
  const toggle = (key) => setSort((s) => (s.key === key ? { key, dir: s.dir === 'desc' ? 'asc' : 'desc' } : { key, dir: 'desc' }));
  const arrow = (key) =>
    sort.key === key ? sort.dir === 'desc' ? <ChevronDown className="size-3.5" /> : <ChevronUp className="size-3.5" /> : null;

  return (
    <>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="h-[29px] py-0">{t('orgPage.usage.colName')}</TableHead>
              {SORTABLE_COLUMNS.map((c) => (
                <TableHead key={c.key} className="h-[29px] py-0 text-right">
                  <button type="button" className="inline-flex items-center gap-1 hover:text-foreground" onClick={() => toggle(c.key)}>
                    {t(`orgPage.usage.${c.i18n}`)} {arrow(c.key)}
                  </button>
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {sorted.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center text-sm text-muted-foreground">
                  {t('orgPage.usage.empty')}
                </TableCell>
              </TableRow>
            ) : (
              sorted.map((r, idx) => (
                <TableRow key={idx}>
                  <TableCell className="py-1.5 font-medium">
                    {onRowClick ? (
                      <button
                        type="button"
                        className="text-left underline-offset-2 hover:text-foreground hover:underline"
                        title={t('orgPage.usage.drillToTokens')}
                        onClick={() => onRowClick(r)}
                      >
                        {nameOf(r)}
                      </button>
                    ) : (
                      nameOf(r)
                    )}
                  </TableCell>
                  <TableCell className="py-1.5 text-right font-mono tabular-nums">{renderNumber(r.request_count || 0)}</TableCell>
                  <TableCell className="py-1.5 text-right font-mono tabular-nums">
                    <SpendAmount quota={r.quota || 0} />
                  </TableCell>
                  <TableCell className="py-1.5 text-right font-mono tabular-nums">{renderNumber(r.prompt_tokens || 0)}</TableCell>
                  <TableCell className="py-1.5 text-right font-mono tabular-nums">{renderNumber(r.completion_tokens || 0)}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
      {sorted.length === 0 ? (
        <div className="md:hidden rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          {t('orgPage.usage.empty')}
        </div>
      ) : (
        <div className="md:hidden flex flex-col gap-3">
          {sorted.map((r, idx) => (
            <div key={idx} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
              <div className="mb-2 text-sm font-medium">
                {onRowClick ? (
                  <button type="button" className="underline-offset-2 hover:underline" onClick={() => onRowClick(r)}>
                    {nameOf(r)}
                  </button>
                ) : (
                  nameOf(r)
                )}
              </div>
              <dl className="flex flex-col divide-y divide-border/60">
                <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                  <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t('orgPage.usage.colRequests')}
                  </dt>
                  <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                    {renderNumber(r.request_count || 0)}
                  </dd>
                </div>
                <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                  <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t('orgPage.usage.colQuota')}
                  </dt>
                  <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                    <SpendAmount quota={r.quota || 0} />
                  </dd>
                </div>
                <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                  <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t('orgPage.usage.colInputTokens')}
                  </dt>
                  <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                    {renderNumber(r.prompt_tokens || 0)}
                  </dd>
                </div>
                <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                  <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t('orgPage.usage.colOutputTokens')}
                  </dt>
                  <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm font-mono tabular-nums">
                    {renderNumber(r.completion_tokens || 0)}
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
  rows: PropTypes.array.isRequired,
  nameOf: PropTypes.func.isRequired,
  onRowClick: PropTypes.func
};

function ChartTooltip({ active, payload, label, format }) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-md">
      <p className="mb-1 font-medium text-muted-foreground">{label}</p>
      <p className="font-mono font-semibold tabular-nums text-foreground">{format(payload[0].value)}</p>
    </div>
  );
}

ChartTooltip.propTypes = {
  active: PropTypes.bool,
  payload: PropTypes.array,
  label: PropTypes.string,
  format: PropTypes.func.isRequired
};

function TrendChart({ t, data, hasData }) {
  const [metric, setMetric] = useState('requests');
  const active = METRICS.find((m) => m.key === metric) || METRICS[0];

  if (!hasData) {
    return <div className="flex h-72 items-center justify-center text-sm text-muted-foreground">{t('orgPage.usage.empty')}</div>;
  }
  return (
    <div className="space-y-3">
      <div className="flex justify-end gap-1">
        {METRICS.map((m) => (
          <Button
            key={m.key}
            size="sm"
            variant={m.key === metric ? 'secondary' : 'ghost'}
            className={cn('h-7 px-2.5 text-xs', m.key === metric && 'text-foreground')}
            onClick={() => setMetric(m.key)}
          >
            {t(`orgPage.usage.${m.i18n}`)}
          </Button>
        ))}
      </div>
      <ResponsiveContainer width="100%" height={288}>
        <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="rf-org-usage-fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--primary)" stopOpacity={0.35} />
              <stop offset="100%" stopColor="var(--primary)" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
          <XAxis
            dataKey="label"
            tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }}
            axisLine={{ stroke: 'var(--border)' }}
            tickLine={false}
          />
          <YAxis width={48} tick={{ fill: 'var(--muted-foreground)', fontSize: 12 }} axisLine={false} tickLine={false} />
          <Tooltip content={<ChartTooltip format={active.format} />} cursor={{ stroke: 'var(--border)' }} />
          <Area
            type="monotone"
            dataKey={active.key}
            stroke="var(--primary)"
            strokeWidth={2}
            fill="url(#rf-org-usage-fill)"
            dot={false}
            activeDot={{ r: 4, fill: 'var(--primary)' }}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

TrendChart.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array.isRequired,
  hasData: PropTypes.bool
};

export default function UsageTab({ orgId, isAdmin, detail }) {
  const { t } = useTranslation();
  const [range, setRange] = useState('7d');
  const [customStart, setCustomStart] = useState(dayjs().subtract(6, 'day').format('YYYY-MM-DD'));
  const [customEnd, setCustomEnd] = useState(dayjs().format('YYYY-MM-DD'));
  const [memberId, setMemberId] = useState('0');
  const [members, setMembers] = useState([]);
  const [dimension, setDimension] = useState('member');
  const [loading, setLoading] = useState(true);
  const [totalQuota, setTotalQuota] = useState(0);
  const [dateRows, setDateRows] = useState([]);
  const [dims, setDims] = useState({ member: [], token: [], model: [] });

  // Member 在可见性关闭时后端只返回自己的数据,此时隐藏成员筛选器(规格 §3.6 / a4)
  const showMemberFilter = isAdmin || (detail?.organization?.setting?.usage_visible_to_members ?? true);

  const { startTs, endTs } = useMemo(() => {
    if (range === 'custom') {
      const s = dayjs(customStart);
      const e = dayjs(customEnd);
      if (!s.isValid() || !e.isValid() || s.isAfter(e)) return { startTs: 0, endTs: 0 };
      return { startTs: s.startOf('day').unix(), endTs: e.endOf('day').unix() };
    }
    if (range === 'today') {
      return { startTs: dayjs().startOf('day').unix(), endTs: dayjs().endOf('day').unix() };
    }
    const days = range === '30d' ? 30 : 7;
    return {
      startTs: dayjs()
        .subtract(days - 1, 'day')
        .startOf('day')
        .unix(),
      endTs: dayjs().endOf('day').unix()
    };
  }, [range, customStart, customEnd]);

  useEffect(() => {
    if (!showMemberFilter) return;
    API.get(`/api/org/${orgId}/members`, { params: { page: 1, size: 100, order: 'id' } })
      .then((res) => {
        if (res.data?.success) setMembers(res.data.data?.data || []);
        else showError(res.data?.message || t('usagePage.loadFailed'));
      })
      .catch((error) => showError(error));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgId, showMemberFilter]);

  useEffect(() => {
    if (!startTs) return;
    let cancelled = false;
    setLoading(true);
    (async () => {
      const params = { start_timestamp: startTs, end_timestamp: endTs };
      const mid = parseInt(memberId, 10) || 0;
      if (mid > 0) params.member_id = mid;
      const fetchGroup = async (groupBy) => {
        const res = await API.get(`/api/org/${orgId}/analytics`, { params: { ...params, group_by: groupBy } });
        return res.data?.success ? res.data.data || [] : [];
      };
      try {
        const [stat, byDate, byMember, byToken, byModel] = await Promise.all([
          API.get(`/api/org/${orgId}/logs/stat`, { params }),
          ...['date', 'member', 'token', 'model'].map(fetchGroup)
        ]);
        if (cancelled) return;
        setTotalQuota(stat.data?.success ? stat.data.data?.quota || 0 : 0);
        setDateRows(byDate);
        setDims({ member: byMember, token: byToken, model: byModel });
      } catch (error) {
        // 错误已由全局拦截器提示
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [orgId, startTs, endTs, memberId]);

  // 与个人 Dashboard 一致:按起止日期对齐逐日序列,缺数据日期补零
  const chartData = useMemo(() => {
    if (!startTs) return [];
    const byDateMap = Object.fromEntries(dateRows.map((r) => [r.date, r]));
    return getDateList(startTs, endTs).map((date) => {
      const r = byDateMap[date] || {};
      return {
        label: date.slice(5),
        requests: Number(r.request_count || 0),
        quota: Number(calculateQuota(r.quota || 0, 6)),
        tokens: Number(r.prompt_tokens || 0) + Number(r.completion_tokens || 0)
      };
    });
  }, [dateRows, startTs, endTs]);

  // 汇总卡数据来自已获取的按日序列,不发起额外请求
  const totals = useMemo(
    () =>
      dateRows.reduce(
        (acc, r) => ({
          requests: acc.requests + Number(r.request_count || 0),
          tokens: acc.tokens + Number(r.prompt_tokens || 0) + Number(r.completion_tokens || 0)
        }),
        { requests: 0, tokens: 0 }
      ),
    [dateRows]
  );

  const memberLabel = (m) => m.display_name || m.username || `#${m.user_id}`;
  const activeDim = DIMENSIONS.find((d) => d.key === dimension);

  return (
    <div className="space-y-6">
      {/* 筛选工具栏:所有控件统一 h-9 高度、同一基线,窄屏自然换行 */}
      <div className="flex flex-wrap items-center gap-3">
        <Tabs value={range} onValueChange={setRange}>
          <TabsList>
            {RANGES.map((r) => (
              <TabsTrigger key={r.key} value={r.key}>
                {t(`orgPage.usage.${r.i18n}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        {range === 'custom' && (
          <div className="flex items-center gap-2">
            <Label htmlFor="org_usage_start" className="sr-only">
              {t('orgPage.usage.startDate')}
            </Label>
            <Input
              id="org_usage_start"
              type="date"
              className="h-9 w-40"
              value={customStart}
              onChange={(e) => setCustomStart(e.target.value)}
            />
            <span className="text-sm text-muted-foreground">-</span>
            <Label htmlFor="org_usage_end" className="sr-only">
              {t('orgPage.usage.endDate')}
            </Label>
            <Input id="org_usage_end" type="date" className="h-9 w-40" value={customEnd} onChange={(e) => setCustomEnd(e.target.value)} />
          </div>
        )}
        {showMemberFilter && (
          <Select value={memberId} onValueChange={setMemberId}>
            <SelectTrigger className="w-44" aria-label={t('orgPage.usage.memberFilter')}>
              <span className="truncate">
                {memberId !== '0'
                  ? (() => {
                      const m = members.find((x) => String(x.user_id) === memberId);
                      return m ? memberLabel(m) : `#${memberId}`;
                    })()
                  : t('orgPage.usage.allMembers')}
              </span>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="0">{t('orgPage.usage.allMembers')}</SelectItem>
              {members.map((m) => (
                <SelectItem key={m.user_id} value={String(m.user_id)}>
                  {memberLabel(m)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      {/* 与 OverviewTab 相同的 StatCard 栅格,避免单卡半宽悬空 */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard icon={Wallet} label={t('orgPage.usage.totalQuota')} value={loading ? '-' : renderSpend(totalQuota)} />
        <StatCard icon={Activity} label={t('orgPage.usage.metricRequests')} value={loading ? '-' : renderNumber(totals.requests)} />
        <StatCard icon={Coins} label={t('orgPage.usage.metricTokens')} value={loading ? '-' : renderNumber(totals.tokens)} />
      </div>

      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0">
          <CardTitle className="text-base">{t('orgPage.usage.title')}</CardTitle>
          <Tabs value={dimension} onValueChange={setDimension}>
            <TabsList>
              {DIMENSIONS.map((d) => (
                <TabsTrigger key={d.key} value={d.key}>
                  {t(`orgPage.usage.${d.i18n}`)}
                </TabsTrigger>
              ))}
              <TabsTrigger value="date">{t('orgPage.usage.byDate')}</TabsTrigger>
            </TabsList>
          </Tabs>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="h-72 w-full animate-pulse rounded-md bg-muted" />
          ) : dimension === 'date' ? (
            <TrendChart t={t} data={chartData} hasData={dateRows.length > 0} />
          ) : (
            <DimensionTable
              t={t}
              rows={dims[dimension]}
              nameOf={activeDim.nameOf}
              onRowClick={
                showMemberFilter && dimension === 'member'
                  ? (r) => {
                      setMemberId(String(r.member_id ?? '0'));
                      setDimension('token');
                    }
                  : undefined
              }
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

UsageTab.propTypes = {
  orgId: PropTypes.oneOfType([PropTypes.number, PropTypes.string]).isRequired,
  isAdmin: PropTypes.bool,
  detail: PropTypes.object
};
