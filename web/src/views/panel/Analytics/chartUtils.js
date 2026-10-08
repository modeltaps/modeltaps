import dayjs from 'dayjs';
import { calculateQuota } from 'utils/common';

// ==============================|| ANALYTICS — DATA TRANSFORMS ||============================== //
// Pure helpers that reshape the `/api/analytics/period` payload into recharts-friendly
// rows. Mirrors the aggregation logic of views/Analytics/component/Overview.

export const SERIES_COLORS = [
  'var(--primary)',
  '#22c55e',
  '#f59e0b',
  '#06b6d4',
  '#ec4899',
  '#64748b',
  '#ef4444',
  '#14b8a6',
  '#eab308',
  '#3b82f6'
];

export function getDates(start, end) {
  const dates = [];
  let current = start.startOf('day');
  const last = end.startOf('day');
  while (current.isBefore(last) || current.isSame(last)) {
    dates.push(current.format('YYYY-MM-DD'));
    current = current.add(1, 'day');
  }
  return dates;
}

// Build per-day rows + series names + totals for cost / tokens / requests.
export function buildChannelSeries(data, dates) {
  const index = new Map(dates.map((d, i) => [d, i]));
  const names = new Set();
  const empty = () => dates.map((d) => ({ label: d.slice(5) }));
  const rows = { costs: empty(), tokens: empty(), requests: empty() };
  const totals = { costs: 0, tokens: 0, requests: 0 };

  for (const item of data || []) {
    const i = index.get(item.Date);
    if (i === undefined) continue;
    const name = item.Channel || 'unknown';
    names.add(name);
    const cost = Number(calculateQuota(item.Quota, 3));
    const tokens = (item.PromptTokens || 0) + (item.CompletionTokens || 0);
    const requests = item.RequestCount || 0;
    rows.costs[i][name] = (rows.costs[i][name] || 0) + cost;
    rows.tokens[i][name] = (rows.tokens[i][name] || 0) + tokens;
    rows.requests[i][name] = (rows.requests[i][name] || 0) + requests;
    totals.costs += cost;
    totals.tokens += tokens;
    totals.requests += requests;
  }

  return { rows, totals, series: Array.from(names) };
}

export function buildRedemptionData(data, dates) {
  const index = new Map(dates.map((d, i) => [d, i]));
  const rows = dates.map((d) => ({ label: d.slice(5), amount: 0, users: 0 }));
  for (const item of data || []) {
    const i = index.get(item.date);
    if (i === undefined) continue;
    rows[i].amount = Number(calculateQuota(item.quota, 3));
    rows[i].users = item.user_count || 0;
  }
  return rows;
}

export function buildRegistrationData(data, dates) {
  const index = new Map(dates.map((d, i) => [d, i]));
  const rows = dates.map((d) => ({ label: d.slice(5), direct: 0, invite: 0 }));
  let total = 0;
  for (const item of data || []) {
    const i = index.get(item.date);
    if (i === undefined) continue;
    rows[i].direct = (item.user_count || 0) - (item.inviter_user_count || 0);
    rows[i].invite = item.inviter_user_count || 0;
    total += item.user_count || 0;
  }
  return { rows, total };
}

export function buildOrdersData(data, dates) {
  const index = new Map(dates.map((d, i) => [d, i]));
  const rows = dates.map((d) => ({ label: d.slice(5), money: 0 }));
  let total = 0;
  for (const item of data || []) {
    const i = index.get(item.date);
    if (i === undefined) continue;
    rows[i].money += item.money || 0;
    total += item.money || 0;
  }
  return { rows, total };
}

export const defaultRange = () => ({
  start: dayjs().subtract(6, 'day').startOf('day'),
  end: dayjs().endOf('day')
});
