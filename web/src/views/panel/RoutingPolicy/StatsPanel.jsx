import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { renderQuota as renderQuotaBase } from 'utils/common';

// 单条策略的金额通常很小，保留 4 位小数，否则全显示成 $0.00。
const renderQuota = (quota) => renderQuotaBase(quota, 4);
import { getRoutingPolicyStats } from './routingPolicyApi';

const REASONS = ['classified', 'pinned', 'below_threshold', 'decision_error', 'empty_input', 'auxiliary'];

function Stat({ label, value, hint }) {
  return (
    <div className="rounded-md border border-border p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
      {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
    </div>
  );
}

const pct = (part, total) => (total > 0 ? `${Math.round((part / total) * 100)}%` : '—');

// 策略效果：近 7 / 30 天请求数、兜底占比（按原因拆）、各标签命中与费用、平均决策耗时、决策成本、
// 按「全部走兜底」估算的节省。只读，供调阈值与标签描述时参考。
export default function StatsPanel({ policyId, fallbackModel }) {
  const { t } = useTranslation();
  const [days, setDays] = useState(7);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!policyId) return undefined;
    let cancelled = false;
    setLoading(true);
    getRoutingPolicyStats(policyId, days)
      .then(({ success, data }) => {
        if (!cancelled && success) setStats(data);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [policyId, days]);

  if (!policyId) {
    return <p className="text-sm text-muted-foreground">{t('routingPolicy.stats.saveFirst')}</p>;
  }

  const total = stats?.requests || 0;
  const saved = stats?.saved_quota || 0;

  return (
    <div className="space-y-5" data-testid="routing-stats">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{t('routingPolicy.stats.intro')}</p>
        <div className="flex gap-1">
          {[7, 30].map((d) => (
            <Button key={d} type="button" size="sm" variant={days === d ? 'default' : 'outline'} onClick={() => setDays(d)}>
              {t('routingPolicy.stats.days', { days: d })}
            </Button>
          ))}
        </div>
      </div>

      {loading && !stats ? (
        <p className="text-sm text-muted-foreground">{t('common.loading', { defaultValue: 'Loading…' })}</p>
      ) : total === 0 ? (
        <p className="text-sm text-muted-foreground">{t('routingPolicy.stats.empty', { days })}</p>
      ) : (
        <>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Stat label={t('routingPolicy.stats.requests')} value={total} />
            <Stat
              label={t('routingPolicy.stats.fallbackRate')}
              value={pct(stats.fallback_requests, total)}
              hint={t('routingPolicy.stats.fallbackCount', { count: stats.fallback_requests })}
            />
            <Stat
              label={t('routingPolicy.stats.avgDecision')}
              value={stats.avg_decision_ms > 0 ? `${stats.avg_decision_ms} ms` : '—'}
              hint={t('routingPolicy.stats.decisionCalls', { count: stats.decision_calls, quota: renderQuota(stats.decision_quota) })}
            />
            {stats.fallback_price_known ? (
              <Stat
                label={t('routingPolicy.stats.saved')}
                value={`${saved < 0 ? '−' : ''}${renderQuota(Math.abs(saved))}`}
                hint={t('routingPolicy.stats.savedHint', {
                  actual: renderQuota(stats.actual_quota),
                  fallback: renderQuota(stats.fallback_quota),
                  model: fallbackModel
                })}
              />
            ) : (
              <Stat
                label={t('routingPolicy.stats.saved')}
                value="—"
                hint={t('routingPolicy.stats.noFallbackPrice', { model: fallbackModel })}
              />
            )}
          </div>

          <div>
            <p className="mb-2 text-sm font-medium">{t('routingPolicy.stats.byReason')}</p>
            <div className="flex flex-wrap gap-1.5">
              {REASONS.filter((r) => stats.by_reason?.[r]).map((r) => (
                <Badge key={r} variant="outline" className="font-normal">
                  {t(`routingPolicy.reasons.${r}`)} · {stats.by_reason[r]} ({pct(stats.by_reason[r], total)})
                </Badge>
              ))}
            </div>
          </div>

          <div className="overflow-hidden rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t('routingPolicy.stats.label')}</TableHead>
                  <TableHead>{t('routingPolicy.stats.target')}</TableHead>
                  <TableHead className="text-right">{t('routingPolicy.stats.requests')}</TableHead>
                  <TableHead className="text-right">{t('routingPolicy.stats.share')}</TableHead>
                  <TableHead className="text-right">{t('routingPolicy.stats.quota')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {stats.labels.map((row) => (
                  <TableRow key={`${row.label}|${row.target_model}`}>
                    <TableCell>{row.label || <span className="text-muted-foreground">{t('routingPolicy.stats.noLabel')}</span>}</TableCell>
                    <TableCell className="font-mono text-sm">{row.target_model}</TableCell>
                    <TableCell className="text-right tabular-nums">{row.requests}</TableCell>
                    <TableCell className="text-right tabular-nums">{pct(row.requests, total)}</TableCell>
                    <TableCell className="text-right tabular-nums">{renderQuota(row.quota)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </>
      )}
    </div>
  );
}
