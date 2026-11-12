import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';
import { useSelector } from 'react-redux';
import dayjs from 'dayjs';
import { Download } from 'lucide-react';
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import TimeRangeSelect, { computeRange } from '@/components/TimeRangeSelect';
import PageActions from '@/components/chrome/PageActions';
import { FilterBar, isEntryActive, OP_IN } from '@/components/filter-bar';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { useOrg } from 'contexts/OrgContext';
import { calculateQuota, formatSpendAmount, showInfo } from 'utils/common';
import { exportUsageCsv } from './usageExport';

// ==============================|| PANEL — USAGE ANALYTICS (T22c-2) ||============================== //
// Personal usage analytics page (OpenRouter-Activity-like): three metric cards +
// daily trend chart, with multi-select token/model filters and client-side CSV export.
// Chart patterns follow Organization/UsageTab.jsx. Data source switches on
// org context: personal scope uses /api/user/self/analytics (T22c-1); when an
// organization is active it explicitly calls /api/org/:id/analytics with
// member_id = self (orgScope.js has no rewrite rule for /api/user/self/analytics),
// so the page always shows "my usage" — never other members'.

// URL query helpers: default range + list (de)serialization. Preset ids come from
// the shared TimeRangeSelect (relative 9 / calendar 8 / custom), resolved via computeRange.
const DEFAULT_RANGE = '1w';
const parseListParam = (raw) =>
  raw
    ? raw
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
    : [];

const SORTABLE_COLUMNS = [
  { key: 'request_count', i18n: 'colRequests' },
  { key: 'quota', i18n: 'colQuota' },
  { key: 'prompt_tokens', i18n: 'colInputTokens' },
  { key: 'completion_tokens', i18n: 'colOutputTokens' }
];

// 起止时间(unix 秒)展开为逐日日期串列表,缺数据日期由调用方补零(同 UsageTab)
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

// Abbreviate a number to K/M/B with ~3 significant figures (trailing zeros trimmed):
// 2.29e9 → "2.29B", 898e6 → "898M", 16234 → "16.2K"; values < 1000 use locale grouping.
function abbreviateNumber(num) {
  const n = Number(num) || 0;
  const abs = Math.abs(n);
  const sign = n < 0 ? '-' : '';
  const fmt = (v, suffix) => {
    const decimals = v >= 100 ? 0 : v >= 10 ? 1 : 2;
    // Trim trailing zeros of the decimals only ("1.50" → "1.5", "2.00" → "2"); "160" stays "160".
    return (
      sign +
      v
        .toFixed(decimals)
        .replace(/(\.\d*?)0+$/, '$1')
        .replace(/\.$/, '') +
      suffix
    );
  };
  if (abs >= 1e9) return fmt(abs / 1e9, 'B');
  if (abs >= 1e6) return fmt(abs / 1e6, 'M');
  if (abs >= 1e3) return fmt(abs / 1e3, 'K');
  return sign + Math.round(abs).toLocaleString();
}

// Spend in one format across the page: ≥ $1000 abbreviated ("$3.2K"), two decimals otherwise,
// "<$0.01" for amounts that would round to zero. Points mode abbreviates the raw quota.
function formatSpendCompact(quota) {
  const displayInCurrency = localStorage.getItem('display_in_currency') === 'true';
  if (!displayInCurrency) return abbreviateNumber(quota);
  const amount = Number(calculateQuota(quota, 6));
  const abs = Math.abs(amount);
  const sign = amount < 0 ? '-' : '';
  if (abs >= 1000) return sign + '$' + abbreviateNumber(abs);
  if (abs > 0 && abs < 0.01) return sign + '<$0.01';
  return sign + '$' + abs.toFixed(2);
}

// Axis/tooltip spend formatter for the daily-spend bar chart: mirrors formatSpendCompact's
// currency branch (abbreviate ≥ $1000, truncation rule below).
function formatSpendAxis(v) {
  const num = Number(v) || 0;
  const abs = Math.abs(num);
  if (abs >= 1000) return (num < 0 ? '-$' : '$') + abbreviateNumber(abs);
  return formatSpendAmount(num);
}

// Chart colours: the accent leads, the second series of a stacked chart is a quiet neutral.
const ACCENT = 'var(--primary)';
const QUIET = 'var(--muted-foreground)';

// vs previous-period delta indicator; hidden when delta is null/non-finite.
// `unit` (default '%') and `decimals` let cards render percentage-point deltas (pp).
function DeltaBadge({ delta, unit = '%', decimals = 0 }) {
  if (delta === null || delta === undefined || !Number.isFinite(delta)) return null;
  const up = delta >= 0;
  return (
    <span className={cn('inline-flex items-center text-xs font-medium', up ? 'text-emerald-600' : 'text-red-600')}>
      {up ? '↑' : '↓'}
      {Math.abs(delta).toFixed(decimals)}
      {unit}
    </span>
  );
}

DeltaBadge.propTypes = {
  delta: PropTypes.number,
  unit: PropTypes.string,
  decimals: PropTypes.number
};

// KPI card: the label, the figure, and the change against the previous period of the same
// length ("↑12% 较上一周期"); without a previous period to compare, that line is left out.
function StatCard({ label, value, delta, deltaLabel, deltaUnit, deltaDecimals }) {
  const hasDelta = delta !== null && delta !== undefined && Number.isFinite(delta);
  return (
    <Card className="p-4 sm:p-5">
      <p className="text-[13px] text-muted-foreground">{label}</p>
      <p className="mt-1 truncate text-2xl font-medium tabular-nums">{value}</p>
      <p className="mt-1 flex min-h-4 items-center gap-1.5 text-xs text-muted-foreground">
        {hasDelta && (
          <>
            <DeltaBadge delta={delta} unit={deltaUnit} decimals={deltaDecimals} />
            {deltaLabel}
          </>
        )}
      </p>
    </Card>
  );
}

StatCard.propTypes = {
  label: PropTypes.string.isRequired,
  value: PropTypes.node,
  delta: PropTypes.number,
  deltaLabel: PropTypes.string,
  deltaUnit: PropTypes.string,
  deltaDecimals: PropTypes.number
};

// Top-5 breakdown: rank, name and spend (the list is ranked by spend), the tokens muted after
// it, and a thin bar for each row's share of the leader.
function TopBreakdownCard({ t, title, rows, nameOf }) {
  if (rows.length === 0) return null;
  const visibleRows = rows.slice(0, 5);
  const max = Math.max(...visibleRows.map((r) => Number(r.quota || 0)), 1);
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {visibleRows.map((row, idx) => {
          const tokens = Number(row.prompt_tokens || 0) + Number(row.completion_tokens || 0);
          const share = Number(row.quota || 0) / max;
          return (
            <div key={idx} className="text-sm">
              <div className="flex items-baseline gap-3">
                <span className="w-4 shrink-0 text-right text-xs tabular-nums text-muted-foreground">{idx + 1}</span>
                <span className="min-w-0 flex-1 truncate font-medium">{nameOf(row) || '-'}</span>
                <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
                  {abbreviateNumber(tokens)} {t('usagePage.tokSuffix')}
                </span>
                <span className="w-16 shrink-0 text-right tabular-nums">{formatSpendCompact(row.quota)}</span>
              </div>
              <div className="ml-7 mt-1.5 h-1 rounded-full bg-muted">
                <div className="h-1 rounded-full bg-primary/70" style={{ width: `${Math.max(share * 100, 2)}%` }} />
              </div>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}

TopBreakdownCard.propTypes = {
  t: PropTypes.func.isRequired,
  title: PropTypes.string.isRequired,
  rows: PropTypes.array.isRequired,
  nameOf: PropTypes.func.isRequired
};

const barTooltipStyle = {
  contentStyle: { background: 'var(--card)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 },
  labelStyle: { color: 'var(--muted-foreground)' }
};

// Daily bar chart card (OpenRouter-Activity style): title top-left, circular legend dots +
// names, and a single value formatter shared by the Y axis and tooltip so figures match the
// card's unit ($ or K/M/B abbreviations). Stacks multiple series; renders the shared empty
// state when there is no data.
function BarChartCard({ t, title, data, hasData, series, formatValue, height = 256, className }) {
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {!hasData ? (
          <div className="flex items-center justify-center text-sm text-muted-foreground" style={{ height }}>
            {t('usagePage.empty')}
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={height}>
            <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
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
                tickFormatter={formatValue}
              />
              <Tooltip
                cursor={{ fill: 'var(--muted)', opacity: 0.3 }}
                {...barTooltipStyle}
                formatter={(value, name) => [formatValue(value), name]}
                labelFormatter={(l) => `${t('usagePage.colDate')}: ${l}`}
              />
              <Legend wrapperStyle={{ fontSize: 12 }} iconType="circle" />
              {series.map((s, i) => (
                <Bar
                  key={s.key}
                  name={s.name}
                  dataKey={s.key}
                  stackId="a"
                  fill={s.color}
                  fillOpacity={s.opacity ?? 1}
                  radius={i === series.length - 1 ? [4, 4, 0, 0] : 0}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}

BarChartCard.propTypes = {
  t: PropTypes.func.isRequired,
  title: PropTypes.string.isRequired,
  data: PropTypes.array.isRequired,
  hasData: PropTypes.bool,
  series: PropTypes.arrayOf(
    PropTypes.shape({ key: PropTypes.string, name: PropTypes.string, color: PropTypes.string, opacity: PropTypes.number })
  ).isRequired,
  formatValue: PropTypes.func.isRequired,
  height: PropTypes.number,
  className: PropTypes.string
};

export default function Usage() {
  const { t, i18n } = useTranslation();
  const { currentOrgId } = useOrg();
  const account = useSelector((state) => state.account);
  const userId = account?.user?.id;
  const orgMode = Boolean(currentOrgId);
  const [searchParams, setSearchParams] = useSearchParams();

  // Time range state (shared TimeRangeSelect): preset id + concrete {start,end} range
  // (unix 秒), initialized from URL (default '1w' / Past 1 Week). Custom range persists
  // as unix start/end so refresh/deep-link restores the exact window.
  const [timePreset, setTimePreset] = useState(() => {
    const r = searchParams.get('range');
    if (r === 'custom') {
      const s = Number(searchParams.get('start'));
      const e = Number(searchParams.get('end'));
      return s > 0 && e > s ? 'custom' : DEFAULT_RANGE;
    }
    return r && computeRange(r) ? r : DEFAULT_RANGE;
  });
  const [timeRange, setTimeRange] = useState(() => {
    if (timePreset === 'custom') {
      return { start_timestamp: Number(searchParams.get('start')), end_timestamp: Number(searchParams.get('end')) };
    }
    return computeRange(timePreset);
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [rowsByDim, setRowsByDim] = useState({ date: [], model: [], token: [], app: [] });
  const [reloadFlag, setReloadFlag] = useState(0);
  const [prevTotals, setPrevTotals] = useState(null);

  // Filter state: shared FilterBar controlled state ({ model|token: { op, values } }),
  // initialized from URL (models/tokens comma-joined params, kept for bookmark parity).
  // selectedModels/selectedTokens are derived so request/export/URL logic stays unchanged.
  const [filterState, setFilterState] = useState(() => {
    const s = {};
    const modelList = parseListParam(searchParams.get('models'));
    const tokenList = parseListParam(searchParams.get('tokens'));
    const appList = parseListParam(searchParams.get('apps'));
    if (modelList.length) s.model = { op: OP_IN, values: modelList };
    if (tokenList.length) s.token = { op: OP_IN, values: tokenList };
    if (appList.length) s.app = { op: OP_IN, values: appList };
    return s;
  });
  const [tokens, setTokens] = useState([]);
  const [unfilteredModelOptions, setUnfilteredModelOptions] = useState([]); // T23b cache: preserve full model list
  const [unfilteredAppOptions, setUnfilteredAppOptions] = useState([]); // W9-M10 cache: preserve full app list

  // FilterBar 字段定义:两个多选(模型 / API Key),选项来自现有数据源;仅 include(不支持 exclude)。
  const filterFields = useMemo(
    () => [
      {
        key: 'model',
        labelKey: 'usagePage.filterByModel',
        type: 'enum',
        supportsExclude: false,
        options: unfilteredModelOptions.map((m) => ({ value: String(m), label: String(m) }))
      },
      {
        key: 'token',
        labelKey: 'usagePage.filterByToken',
        type: 'enum',
        supportsExclude: false,
        options: tokens.map((n) => ({ value: String(n), label: String(n) }))
      },
      {
        key: 'app',
        labelKey: 'usagePage.filterByApp',
        type: 'enum',
        supportsExclude: false,
        options: unfilteredAppOptions.map((a) => ({ value: String(a), label: String(a) }))
      }
    ],
    [unfilteredModelOptions, tokens, unfilteredAppOptions]
  );

  // 选中值映射:派生 selectedModels/selectedTokens/selectedApps,请求参数 / URL 持久化 / 导出的口径保持不变。
  const selectedModels = useMemo(() => filterState.model?.values ?? [], [filterState.model]);
  const selectedTokens = useMemo(() => filterState.token?.values ?? [], [filterState.token]);
  const selectedApps = useMemo(() => filterState.app?.values ?? [], [filterState.app]);

  // Fetch token list on mount
  useEffect(() => {
    (async () => {
      try {
        const res = await API.get('/api/token/', { params: { size: 100, page: 1 } });
        const list = res.data?.data?.data;
        if (res.data?.success && Array.isArray(list)) {
          setTokens(list.filter((t) => t.name).map((t) => t.name));
        }
      } catch (err) {
        console.error('Failed to fetch tokens:', err);
      }
    })();
  }, []);

  // Write current filter state back to the URL query (replace, so filter toggles
  // don't pollute the history stack). Defaults are omitted to keep the URL clean.
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        if (timePreset && timePreset !== DEFAULT_RANGE) params.set('range', timePreset);
        else params.delete('range');
        if (timePreset === 'custom') {
          params.set('start', String(timeRange.start_timestamp));
          params.set('end', String(timeRange.end_timestamp));
        } else {
          params.delete('start');
          params.delete('end');
        }
        if (selectedTokens.length > 0) params.set('tokens', selectedTokens.join(','));
        else params.delete('tokens');
        if (selectedModels.length > 0) params.set('models', selectedModels.join(','));
        else params.delete('models');
        if (selectedApps.length > 0) params.set('apps', selectedApps.join(','));
        else params.delete('apps');
        return params;
      },
      { replace: true }
    );
  }, [timePreset, timeRange, selectedTokens, selectedModels, selectedApps, setSearchParams]);

  // 起止时间(unix 秒)直接取自共享选择器的 range;rangeLabel 沿用当前档位 id(导出用)。
  const startTs = timeRange?.start_timestamp || 0;
  const endTs = timeRange?.end_timestamp || 0;
  const rangeLabel = timePreset;

  useEffect(() => {
    // 组织态需等 userId 就绪才能按 member_id=自己 过滤,避免误查全组织
    if (!startTs || (orgMode && !userId)) return;
    let cancelled = false;
    // Debounce: rapid filter toggles (live multi-select) coalesce into a single
    // request batch instead of firing one round-trip per checkbox click. Loading/
    // error state is set inside the timer so rapid toggles don't flash a spinner.
    const timer = setTimeout(() => {
      setLoading(true);
      setError('');
      (async () => {
        const fetchGroup = async (groupBy) => {
          const params = { start_timestamp: startTs, end_timestamp: endTs, group_by: groupBy };
          if (orgMode) params.member_id = userId;
          if (selectedTokens.length > 0) params.token_name = selectedTokens[0];
          if (selectedApps.length > 0) params.app_name = selectedApps[0];

          // Backend only supports single model_name (model/log.go:169 is string type).
          // Pass first selected model to backend; we'll filter client-side for additional models.
          // This is a progressive enhancement - UI ready for backend multi-select when available.
          if (selectedModels.length > 0) {
            params.model_name = selectedModels[0];
          }

          const res = orgMode
            ? await API.get(`/api/org/${currentOrgId}/analytics`, { params })
            : await API.get('/api/user/self/analytics', { params });
          if (!res.data?.success) throw new Error(res.data?.message || '');
          return res.data.data || [];
        };
        try {
          const [byDate, byModel, byToken, byApp] = await Promise.all(['date', 'model', 'token', 'app'].map(fetchGroup));
          if (cancelled) return;

          // Client-side filter for additional selected models/tokens/apps (first is applied
          // server-side). Rows without the field (e.g. date dim) pass through untouched.
          const filterByModels = (rows) => {
            if (selectedModels.length === 0) return rows;
            return rows.filter((r) => r.model_name == null || r.model_name === '' || selectedModels.includes(r.model_name));
          };
          const filterByTokens = (rows) => {
            if (selectedTokens.length === 0) return rows;
            return rows.filter((r) => r.token_name == null || r.token_name === '' || selectedTokens.includes(r.token_name));
          };
          const filterByApps = (rows) => {
            if (selectedApps.length === 0) return rows;
            return rows.filter((r) => r.app_name == null || r.app_name === '' || selectedApps.includes(r.app_name));
          };
          const applyFilters = (rows) => filterByApps(filterByTokens(filterByModels(rows)));

          setRowsByDim({
            date: applyFilters(byDate),
            model: applyFilters(byModel),
            token: applyFilters(byToken),
            app: applyFilters(byApp)
          });

          // T23b cache: Update unfiltered model options only when token filter is empty
          // This ensures the model list doesn't collapse when switching token filters
          if (selectedTokens.length === 0 && byModel.length > 0) {
            setUnfilteredModelOptions(byModel.map((r) => r.model_name).filter(Boolean));
          }
          // W9-M10 cache: refresh app options only when no app filter is active, so the
          // app list doesn't collapse to the single selected app.
          if (selectedApps.length === 0 && byApp.length > 0) {
            setUnfilteredAppOptions(byApp.map((r) => r.app_name).filter(Boolean));
          }
        } catch (err) {
          if (cancelled) return;
          setRowsByDim({ date: [], model: [], token: [], app: [] });
          setError(err?.message || t('usagePage.loadFailed'));
        } finally {
          if (!cancelled) setLoading(false);
        }
      })();
    }, 300);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [orgMode, currentOrgId, userId, startTs, endTs, selectedTokens, selectedModels, selectedApps, reloadFlag, t]);

  // 额外拉取"上一期"(与当前窗口等长、紧邻其前)的按日汇总,仅用于 Hero 卡 vs 上一期 delta 对比
  useEffect(() => {
    if (!startTs || (orgMode && !userId)) return;
    const duration = endTs - startTs;
    const prevEnd = startTs - 1;
    const prevStart = prevEnd - duration;
    if (prevStart <= 0) {
      setPrevTotals(null);
      return;
    }
    let cancelled = false;
    const timer = setTimeout(() => {
      (async () => {
        const params = { start_timestamp: prevStart, end_timestamp: prevEnd, group_by: 'date' };
        if (orgMode) params.member_id = userId;
        if (selectedTokens.length > 0) params.token_name = selectedTokens[0];
        if (selectedModels.length > 0) params.model_name = selectedModels[0];
        if (selectedApps.length > 0) params.app_name = selectedApps[0];
        try {
          const res = orgMode
            ? await API.get(`/api/org/${currentOrgId}/analytics`, { params })
            : await API.get('/api/user/self/analytics', { params });
          if (cancelled) return;
          if (!res.data?.success) {
            setPrevTotals(null);
            return;
          }
          // group_by=date rows carry no model_name/token_name; server already applied
          // the first selected value via params, so no client-side filtering is needed.
          const rows = res.data.data || [];
          setPrevTotals(
            rows.reduce(
              (acc, r) => ({
                quota: acc.quota + Number(r.quota || 0),
                requests: acc.requests + Number(r.request_count || 0),
                tokens: acc.tokens + Number(r.prompt_tokens || 0) + Number(r.completion_tokens || 0),
                promptTokens: acc.promptTokens + Number(r.prompt_tokens || 0),
                completionTokens: acc.completionTokens + Number(r.completion_tokens || 0),
                cachedReadTokens: acc.cachedReadTokens + Number(r.cached_read_tokens || 0)
              }),
              { quota: 0, requests: 0, tokens: 0, promptTokens: 0, completionTokens: 0, cachedReadTokens: 0 }
            )
          );
        } catch {
          if (!cancelled) setPrevTotals(null);
        }
      })();
    }, 300);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [orgMode, currentOrgId, userId, startTs, endTs, selectedTokens, selectedModels, selectedApps, reloadFlag]);

  // 与 UsageTab 一致:按起止日期对齐逐日序列,缺数据日期补零
  const chartData = useMemo(() => {
    if (!startTs) return [];
    const byDateMap = Object.fromEntries(rowsByDim.date.map((r) => [r.date, r]));
    return getDateList(startTs, endTs).map((date) => {
      const r = byDateMap[date] || {};
      const prompt = Number(r.prompt_tokens || 0);
      const cached = Number(r.cached_read_tokens || 0);
      return {
        label: date.slice(5),
        requests: Number(r.request_count || 0),
        quota: Number(calculateQuota(r.quota || 0, 6)),
        tokens: prompt + Number(r.completion_tokens || 0),
        promptTokens: prompt,
        completionTokens: Number(r.completion_tokens || 0),
        cachedTokens: cached,
        uncachedTokens: Math.max(prompt - cached, 0),
        cacheHit: prompt > 0 ? (cached / prompt) * 100 : 0
      };
    });
  }, [rowsByDim.date, startTs, endTs]);

  // 三指标卡来自按日序列汇总,不发起额外请求
  const totals = useMemo(
    () =>
      rowsByDim.date.reduce(
        (acc, r) => ({
          quota: acc.quota + Number(r.quota || 0),
          requests: acc.requests + Number(r.request_count || 0),
          tokens: acc.tokens + Number(r.prompt_tokens || 0) + Number(r.completion_tokens || 0),
          promptTokens: acc.promptTokens + Number(r.prompt_tokens || 0),
          completionTokens: acc.completionTokens + Number(r.completion_tokens || 0),
          cachedReadTokens: acc.cachedReadTokens + Number(r.cached_read_tokens || 0)
        }),
        { quota: 0, requests: 0, tokens: 0, promptTokens: 0, completionTokens: 0, cachedReadTokens: 0 }
      ),
    [rowsByDim.date]
  );

  // Models by quota, fully sorted (hide when a model filter is active or there is no data)
  const topModels = useMemo(() => {
    if (selectedModels.length > 0 || rowsByDim.model.length === 0) return [];
    return [...rowsByDim.model].sort((a, b) => Number(b.quota || 0) - Number(a.quota || 0));
  }, [selectedModels, rowsByDim.model]);

  // API keys by quota, fully sorted (hide when a token filter is active or there is no data)
  const topTokens = useMemo(() => {
    if (selectedTokens.length > 0 || rowsByDim.token.length === 0) return [];
    return [...rowsByDim.token].sort((a, b) => Number(b.quota || 0) - Number(a.quota || 0));
  }, [selectedTokens, rowsByDim.token]);

  // Apps by quota, fully sorted (hide when an app filter is active or no data); rows without
  // app attribution (empty app_name, older/unattributed logs) are excluded from the ranking.
  const topApps = useMemo(() => {
    if (selectedApps.length > 0 || rowsByDim.app.length === 0) return [];
    return [...rowsByDim.app].filter((r) => r.app_name).sort((a, b) => Number(b.quota || 0) - Number(a.quota || 0));
  }, [selectedApps, rowsByDim.app]);

  // vs previous-period delta percentages (null when no prior data or prior total is 0)
  const deltas = useMemo(() => {
    const calc = (cur, prev) => (prev > 0 ? ((cur - prev) / prev) * 100 : null);
    if (!prevTotals) return { quota: null, tokens: null, promptTokens: null, completionTokens: null, requests: null };
    return {
      quota: calc(totals.quota, prevTotals.quota),
      tokens: calc(totals.tokens, prevTotals.tokens),
      promptTokens: calc(totals.promptTokens, prevTotals.promptTokens),
      completionTokens: calc(totals.completionTokens, prevTotals.completionTokens),
      requests: calc(totals.requests, prevTotals.requests)
    };
  }, [totals, prevTotals]);

  // Cache hit rate (cached_read_tokens ÷ prompt_tokens) as a percentage; 0 when there
  // are no prompt tokens. Its delta vs the previous period is a percentage-point
  // difference (not a relative % change like the other cards).
  const cacheHit = useMemo(() => {
    const rate = totals.promptTokens > 0 ? (totals.cachedReadTokens / totals.promptTokens) * 100 : 0;
    const prevRate = prevTotals && prevTotals.promptTokens > 0 ? (prevTotals.cachedReadTokens / prevTotals.promptTokens) * 100 : null;
    return { rate, delta: prevRate === null ? null : rate - prevRate };
  }, [totals, prevTotals]);

  const handleExport = () => {
    const rows = rowsByDim.date || [];
    if (rows.length === 0) {
      showInfo(t('usagePage.exportEmpty'));
      return;
    }
    exportUsageCsv({
      dimension: 'date',
      rows,
      nameHeader: t('usagePage.colDate'),
      headers: SORTABLE_COLUMNS.map((c) => t(`usagePage.${c.i18n}`)),
      nameOf: (r) => r.date || '-',
      startDate: dayjs.unix(startTs).format('YYYY-MM-DD'),
      endDate: dayjs.unix(endTs).format('YYYY-MM-DD'),
      rangeLabel,
      selectedTokens,
      selectedModels
    });
  };

  const handleApplyRange = (r, presetId) => {
    setTimeRange(r);
    setTimePreset(presetId);
  };

  // 级联筛选受控回调(FilterBar 传入函数式 updater);清空同 Logs。
  const handleFilterChange = (next) => setFilterState(next);
  const handleClearFilters = () => setFilterState({});

  // 已生效字段(仅决定是否渲染标题下方 chips 行);chips/「+」/Clear 由容器内 FilterBar 承担。
  const activeChipFields = useMemo(() => filterFields.filter((f) => isEntryActive(f, filterState[f.key])), [filterFields, filterState]);

  const hasChartData = rowsByDim.date.length > 0;

  return (
    <div className="space-y-6">
      {/* Filters/export live on the title row via the chrome PageHeader actions slot */}
      <PageActions>
        <FilterBar
          fields={filterFields}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          hideChips
          t={t}
        />

        <TimeRangeSelect value={timeRange} preset={timePreset} onApply={handleApplyRange} t={t} locale={i18n.language} />

        <Button variant="outline" size="sm" className="h-9" onClick={handleExport} disabled={loading}>
          <Download className="size-4" />
          {t('usagePage.export')}
        </Button>
      </PageActions>

      {/* 标题行下方:Logs 同款全宽 chips 行(chips/「+」/Clear);仅在有激活筛选时渲染。 */}
      {activeChipFields.length > 0 && (
        <FilterBar
          fields={filterFields}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          triggerVariant="plus"
          clearAlignEnd
          t={t}
          className="w-full rounded-lg border border-input bg-field px-3 py-2"
        />
      )}

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
        <StatCard
          label={t('usagePage.metricSpend')}
          value={loading ? '-' : formatSpendCompact(totals.quota)}
          delta={loading ? null : deltas.quota}
          deltaLabel={t('usagePage.vsPrevPeriod')}
        />
        <StatCard
          label={t('usagePage.metricRequests')}
          value={loading ? '-' : abbreviateNumber(totals.requests)}
          delta={loading ? null : deltas.requests}
          deltaLabel={t('usagePage.vsPrevPeriod')}
        />
        <StatCard
          label={t('usagePage.metricTokenVolume')}
          value={loading ? '-' : abbreviateNumber(totals.tokens)}
          delta={loading ? null : deltas.tokens}
          deltaLabel={t('usagePage.vsPrevPeriod')}
        />
        <StatCard
          label={t('usagePage.metricCacheHitRate')}
          value={loading ? '-' : cacheHit.rate.toFixed(1) + '%'}
          delta={loading ? null : cacheHit.delta}
          deltaUnit="pp"
          deltaDecimals={1}
          deltaLabel={t('usagePage.vsPrevPeriod')}
        />
      </div>

      {(topModels.length > 0 || topTokens.length > 0 || topApps.length > 0) && (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {topModels.length > 0 && (
            <TopBreakdownCard t={t} title={t('usagePage.topModels')} rows={topModels} nameOf={(r) => r.model_name} />
          )}
          {topTokens.length > 0 && (
            <TopBreakdownCard t={t} title={t('usagePage.topApiKeys')} rows={topTokens} nameOf={(r) => r.token_name} />
          )}
          {topApps.length > 0 && <TopBreakdownCard t={t} title={t('usagePage.topApps')} rows={topApps} nameOf={(r) => r.app_name} />}
        </div>
      )}

      {loading ? (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {[0, 1, 2, 3].map((i) => (
            <Card key={i}>
              <CardHeader>
                <div className="h-5 w-32 animate-pulse rounded bg-muted" />
              </CardHeader>
              <CardContent>
                <div className="h-64 w-full animate-pulse rounded-md bg-muted" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : error ? (
        <Card>
          <CardContent>
            <div className="flex h-72 flex-col items-center justify-center gap-3 text-sm text-muted-foreground">
              <p>{error || t('usagePage.loadFailed')}</p>
              <Button variant="outline" size="sm" onClick={() => setReloadFlag((v) => v + 1)}>
                {t('usagePage.retry')}
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {/* Spend leads at full width; requests, tokens and cache follow smaller, one accent each. */}
          <BarChartCard
            t={t}
            title={t('usagePage.chartSpendTitle')}
            data={chartData}
            hasData={hasChartData}
            formatValue={formatSpendAxis}
            height={280}
            series={[{ key: 'quota', name: t('usagePage.metricSpend'), color: ACCENT }]}
          />
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
            <BarChartCard
              t={t}
              title={t('usagePage.chartRequestsTitle')}
              data={chartData}
              hasData={hasChartData}
              formatValue={abbreviateNumber}
              height={180}
              series={[{ key: 'requests', name: t('usagePage.metricRequests'), color: ACCENT }]}
            />
            <BarChartCard
              t={t}
              title={t('usagePage.chartTokensTitle')}
              data={chartData}
              hasData={hasChartData}
              formatValue={abbreviateNumber}
              height={180}
              series={[
                { key: 'promptTokens', name: t('usagePage.chartInputTokens'), color: ACCENT },
                { key: 'completionTokens', name: t('usagePage.chartOutputTokens'), color: QUIET, opacity: 0.4 }
              ]}
            />
            <BarChartCard
              t={t}
              title={t('usagePage.chartCacheTitle')}
              data={chartData}
              hasData={hasChartData}
              formatValue={abbreviateNumber}
              height={180}
              series={[
                { key: 'cachedTokens', name: t('usagePage.chartCached'), color: ACCENT },
                { key: 'uncachedTokens', name: t('usagePage.chartUncached'), color: QUIET, opacity: 0.4 }
              ]}
            />
          </div>
        </div>
      )}
    </div>
  );
}
