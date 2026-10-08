import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Activity, CreditCard, Download, Network, Users, Wallet } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { API } from 'utils/api';
import { formatSpendAmount, renderQuota, showError, showSuccess } from 'utils/common';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';

// ==============================|| ANALYTICS — STATISTICS CARDS ||============================== //
// shadcn/Tailwind port of views/Analytics/component/Statistics. Same
// `/api/analytics/statistics` + `/api/analytics/recharge` endpoints; only the UI is new.

function csvCell(value) {
  const s = value === null || value === undefined ? '' : String(value);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

function StatTile({ icon: Icon, title, content, subContent, accent = false, loading = false, filter = null }) {
  return (
    <Card className={accent ? 'border-foreground/30' : undefined}>
      <CardContent className="flex flex-col gap-2 p-5">
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{title}</span>
          {filter}
          {Icon && <Icon className={accent ? 'size-4 text-foreground' : 'size-4 text-muted-foreground'} />}
        </div>
        {loading ? (
          <div className="h-8 w-24 animate-pulse rounded-md bg-muted" />
        ) : (
          <span className="font-mono text-2xl font-semibold tabular-nums text-foreground">
            {content}
          </span>
        )}
        {subContent && <span className="text-xs leading-relaxed text-muted-foreground">{subContent}</span>}
      </CardContent>
    </Card>
  );
}

export default function Statistics() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const [basicLoading, setBasicLoading] = useState(true);
  const [rechargeLoading, setRechargeLoading] = useState(true);
  const [user, setUser] = useState({});
  const [channel, setChannel] = useState({ active: 0, disabled: 0, test_disabled: 0, total: 0 });
  const [recharge, setRecharge] = useState({ total: '0', Redemption: '0', Order: '0', OrderContent: '' });
  const [rpmTpm, setRpmTpm] = useState({ rpm: 0, tpm: 0, cpm: 0, ppm: 0 });
  const [timeFilter, setTimeFilter] = useState('month');

  const timeFilterOptions = ['day', 'week', 'month', 'year', 'all'];

  const fetchBasic = async () => {
    try {
      const res = await API.get('/api/analytics/statistics');
      const { success, message, data } = res.data;
      if (success) {
        if (data.user_statistics) {
          const u = data.user_statistics;
          setUser({
            ...u,
            total_quota: renderQuota(u.total_quota),
            total_used_quota: renderQuota(u.total_used_quota),
            total_direct_user: u.total_user - u.total_inviter_user
          });
        }
        if (data.channel_statistics) {
          const c = { active: 0, disabled: 0, test_disabled: 0, total: 0 };
          data.channel_statistics.forEach((i) => {
            c.total += i.total_channels;
            if (i.status === 1) c.active = i.total_channels;
            else if (i.status === 2) c.disabled = i.total_channels;
            else if (i.status === 3) c.test_disabled = i.total_channels;
          });
          setChannel(c);
        }
        if (data.rpm_tpm_statistics) setRpmTpm(data.rpm_tpm_statistics);
      } else {
        showError(message);
      }
    } catch (error) {
      // network errors surfaced globally by the API interceptor
    } finally {
      setBasicLoading(false);
    }
  };

  // reqId 守卫(UX-15):Select 快切时间档位时丢弃过期响应;fetchBasic 仅挂载时调用一次,无并发不需守卫。
  const rechargeGuardRef = useRef(null);
  if (!rechargeGuardRef.current) rechargeGuardRef.current = createRequestGuard();

  const fetchRecharge = (range) =>
    runGuardedFetch(rechargeGuardRef.current, () => API.get('/api/analytics/recharge', { params: { time_range: range } }), {
      onStart: () => setRechargeLoading(true),
      onResult: (res) => {
        const { success, message, data } = res.data;
        if (success && data) {
          setRecharge({
            total: renderQuota(data.total || 0),
            Redemption: renderQuota(data.redemption_amount || 0),
            Order: renderQuota(data.order_amount || 0),
            OrderContent: data.order_currency_info || ''
          });
        } else if (!success) {
          showError(message);
        }
      },
      // network errors surfaced globally by the API interceptor
      onFinally: () => setRechargeLoading(false)
    });

  const handleExportRecharge = () => {
    // No backend recharge export endpoint; generate CSV client-side from the
    // currently displayed summary so the file matches the on-screen values.
    const rangeLabel = t(`analytics_index.timeFilter.${timeFilter}`);
    const header = [
      t('analytics_index.rechargeExportTimeRange'),
      t('analytics_index.rechargeExportTotal'),
      t('analytics_index.redemptionCode'),
      t('analytics_index.order'),
      t('analytics_index.rechargeExportOrderCurrency')
    ];
    const row = [rangeLabel, recharge.total, recharge.Redemption, recharge.Order, recharge.OrderContent];
    const lines = [header.map(csvCell).join(','), row.map(csvCell).join(',')];
    // \ufeff BOM so Excel opens the UTF-8 file with CJK headers intact
    const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' });
    const link = document.createElement('a');
    link.href = window.URL.createObjectURL(blob);
    link.download = `recharge_statistics_${timeFilter}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(link.href);
    showSuccess(t('analytics_index.rechargeExportSuccess'));
  };

  useEffect(() => {
    fetchBasic();
    fetchRecharge(timeFilter);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
      <StatTile
        icon={Wallet}
        loading={basicLoading}
        title={t('analytics_index.totalUserSpending')}
        content={user.total_used_quota || '0'}
        subContent={`${t('analytics_index.totalUserBalance')}：${user.total_quota || '0'}`}
      />
      <StatTile
        icon={Users}
        loading={basicLoading}
        title={t('analytics_index.totalUsers')}
        content={user.total_user || '0'}
        subContent={
          <>
            {t('analytics_index.directRegistration')}：{user.total_direct_user || '0'}
            <br />
            {t('analytics_index.invitationRegistration')}：{user.total_inviter_user || '0'}
          </>
        }
      />
      <StatTile
        icon={Network}
        loading={basicLoading}
        title={t('analytics_index.channelCount')}
        content={channel.total}
        subContent={
          <>
            {t('analytics_index.active')}：{channel.active} / {t('analytics_index.disabled')}：{channel.disabled}
            <br />
            {t('analytics_index.testDisabled')}：{channel.test_disabled}
          </>
        }
      />
      <StatTile
        icon={CreditCard}
        accent
        loading={rechargeLoading}
        title={t('analytics_index.rechargeStatistics')}
        content={recharge.total}
        subContent={
          <>
            {t('analytics_index.redemptionCode')}：{recharge.Redemption}
            <br />
            {t('analytics_index.order')}：{recharge.Order} {recharge.OrderContent ? `/ ${recharge.OrderContent}` : ''}
          </>
        }
        filter={
          <div className="flex items-center gap-1">
            <Select
              value={timeFilter}
              onValueChange={(v) => {
                setTimeFilter(v);
                fetchRecharge(v);
              }}
            >
              <SelectTrigger className="h-7 w-24 text-xs">
                <SelectValue>{t(`analytics_index.timeFilter.${timeFilter}`)}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {timeFilterOptions.map((o) => (
                  <SelectItem key={o} value={o}>
                    {t(`analytics_index.timeFilter.${o}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="size-7"
              onClick={handleExportRecharge}
              disabled={rechargeLoading}
              title={t('analytics_index.rechargeExportButton')}
              aria-label={t('analytics_index.rechargeExportButton')}
            >
              <Download className="size-4" />
            </Button>
          </div>
        }
      />
      <StatTile
        icon={Activity}
        loading={basicLoading}
        title={t('analytics_index.realTimeTraffic')}
        content={`${rpmTpm.rpm} RPM`}
        subContent={
          <>
            {t('analytics_index.tpmDescription')}：{rpmTpm.tpm.toLocaleString()}
            <br />
            {t('analytics_index.cpmDescription')}：{formatSpendAmount(rpmTpm.cpm)}
            {siteInfo.PaymentUSDRate ? ` / ${formatSpendAmount(rpmTpm.cpm * siteInfo.PaymentUSDRate, '¥')}` : ''}
            <br />
            {t('analytics_index.ppmDescription')}：{formatSpendAmount(rpmTpm.ppm ?? 0)}
            {siteInfo.PaymentUSDRate ? ` / ${formatSpendAmount((rpmTpm.ppm ?? 0) * siteInfo.PaymentUSDRate, '¥')}` : ''}
            {siteInfo.PaymentUSDRate ? (
              <>
                <br />
                <span className="text-muted-foreground">{t('analytics_index.cnyEstimateNote', { rate: siteInfo.PaymentUSDRate })}</span>
              </>
            ) : null}
          </>
        }
      />
    </div>
  );
}
