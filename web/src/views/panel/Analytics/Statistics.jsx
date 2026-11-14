import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Check, ChevronDown, Download } from 'lucide-react';

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import StatCard, { MetricStrip } from '../Dashboard/StatCard';
import { API } from 'utils/api';
import { formatSpendAmount, renderQuota, showError, showSuccess } from 'utils/common';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';

// ==============================|| ANALYTICS — STATISTICS CARDS ||============================== //
// Same `/api/analytics/statistics` + `/api/analytics/recharge` endpoints. Four stat cards (a label,
// the figure and one line of context, no icons) and the real-time traffic on a quiet strip below.
// The recharge card's label is its period menu, which also holds the CSV export.

function csvCell(value) {
  const s = value === null || value === undefined ? '' : String(value);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
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

  const cny = (usd) => (siteInfo.PaymentUSDRate ? formatSpendAmount(usd * siteInfo.PaymentUSDRate, '¥') : undefined);
  const cnyHint = siteInfo.PaymentUSDRate ? t('analytics_index.cnyEstimateNote', { rate: siteInfo.PaymentUSDRate }) : undefined;

  const rechargeLabel = (
    <DropdownMenu>
      <DropdownMenuTrigger className="-mx-1 inline-flex items-center gap-1 rounded px-1 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
        {t('analytics_index.recharge')} · {t(`analytics_index.timeFilter.${timeFilter}`)}
        <ChevronDown className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-40">
        {timeFilterOptions.map((o) => (
          <DropdownMenuItem
            key={o}
            className="justify-between"
            onClick={() => {
              setTimeFilter(o);
              fetchRecharge(o);
            }}
          >
            {t(`analytics_index.timeFilter.${o}`)}
            {o === timeFilter && <Check className="size-4" />}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={handleExportRecharge} disabled={rechargeLoading}>
          <Download className="size-4" />
          {t('analytics_index.rechargeExportButton')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <div className="space-y-3 sm:space-y-4">
      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <StatCard
          loading={basicLoading}
          label={t('analytics_index.totalUserSpending')}
          value={user.total_used_quota || '0'}
          hint={`${t('analytics_index.totalUserBalance')} ${user.total_quota || '0'}`}
        />
        <StatCard
          loading={basicLoading}
          label={t('analytics_index.totalUsers')}
          value={user.total_user || '0'}
          hint={`${t('analytics_index.invitationRegistration')} ${user.total_inviter_user || 0}`}
        />
        <StatCard
          loading={basicLoading}
          label={t('analytics_index.channelCount')}
          value={channel.total}
          hint={[
            `${t('analytics_index.active')} ${channel.active}`,
            channel.disabled ? `${t('analytics_index.disabled')} ${channel.disabled}` : null,
            channel.test_disabled ? `${t('analytics_index.testDisabled')} ${channel.test_disabled}` : null
          ]
            .filter(Boolean)
            .join(' · ')}
        />
        <StatCard
          loading={rechargeLoading}
          label={rechargeLabel}
          value={recharge.total}
          hint={`${t('analytics_index.redemptionCode')} ${recharge.Redemption} · ${t('analytics_index.order')} ${recharge.Order}`}
        />
      </div>
      <MetricStrip
        loading={basicLoading}
        items={[
          {
            label: `RPM · ${t('analytics_index.realTimeTraffic')}`,
            value: rpmTpm.rpm.toLocaleString(),
            hint: t('analytics_index.last60SecondsStats')
          },
          { label: t('analytics_index.tpmDescription'), value: rpmTpm.tpm.toLocaleString() },
          { label: t('analytics_index.cpmDescription'), value: formatSpendAmount(rpmTpm.cpm), valueTitle: cny(rpmTpm.cpm), hint: cnyHint },
          {
            label: t('analytics_index.ppmDescription'),
            value: formatSpendAmount(rpmTpm.ppm ?? 0),
            valueTitle: cny(rpmTpm.ppm ?? 0),
            hint: cnyHint
          }
        ]}
      />
    </div>
  );
}
