import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Search } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import TimeRangeSelect, { computeRange } from '@/components/TimeRangeSelect';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import { API } from 'utils/api';
import { renderNumber, showError } from 'utils/common';
import { buildChannelSeries, buildOrdersData, buildRedemptionData, buildRegistrationData, getDates } from './chartUtils';
import { RedemptionCard, SingleBarCard, StackedBarCard, StackedRegistrationCard } from './AnalyticsCharts';

// ==============================|| ANALYTICS — PERIOD OVERVIEW ||============================== //
// Same `/api/analytics/period` endpoint. One toolbar row (time range, group by, user ID) that
// applies as it changes, then the charts: spend full width, tokens and requests side by side,
// then redemptions, registrations and recharge.

const GROUP_TYPES = [
  { value: 'model_type', i18n: 'groupModelType' },
  { value: 'model', i18n: 'groupModel' },
  { value: 'channel', i18n: 'groupChannel' }
];

const DEFAULT_PRESET = '1w';

const spend = (v) => `$${Number(v || 0).toFixed(2)}`;

export default function Overview() {
  const { t, i18n } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [preset, setPreset] = useState(DEFAULT_PRESET);
  const [range, setRange] = useState(() => computeRange(DEFAULT_PRESET));
  const [groupType, setGroupType] = useState('model_type');
  const [userInput, setUserInput] = useState('');
  const [userId, setUserId] = useState(0);
  const [channel, setChannel] = useState({ rows: { costs: [], tokens: [], requests: [] }, totals: {}, series: [] });
  const [redemption, setRedemption] = useState([]);
  const [registration, setRegistration] = useState({ rows: [], total: 0 });
  const [orders, setOrders] = useState({ rows: [], total: 0 });

  // reqId guard: a quick change of range or grouping drops the older response.
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  useEffect(() => {
    // The charts are per day: whole days from the first to the last day of the range.
    const start = dayjs.unix(range.start_timestamp).startOf('day');
    const end = dayjs.unix(Math.min(range.end_timestamp, dayjs().unix())).endOf('day');
    const dates = getDates(start, end);
    runGuardedFetch(
      guardRef.current,
      () =>
        API.get('/api/analytics/period', {
          params: { start_timestamp: start.unix(), end_timestamp: end.unix(), group_type: groupType, user_id: userId }
        }),
      {
        onStart: () => setLoading(true),
        onResult: (res) => {
          const { success, message, data } = res.data;
          if (success && data) {
            setChannel(buildChannelSeries(data.channel_statistics, dates));
            setRedemption(buildRedemptionData(data.redemption_statistics, dates));
            setRegistration(buildRegistrationData(data.user_statistics, dates));
            setOrders(buildOrdersData(data.order_statistics, dates));
          } else if (!success) {
            showError(message);
          }
        },
        // network errors surfaced globally by the API interceptor
        onFinally: () => setLoading(false)
      }
    );
  }, [range, groupType, userId]);

  const applyUser = (e) => {
    e?.preventDefault();
    setUserId(Number(userInput) || 0);
  };

  return (
    <div className="space-y-3 sm:space-y-4">
      <form className="flex flex-wrap items-center gap-2" onSubmit={applyUser}>
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
        <Select value={groupType} onValueChange={setGroupType}>
          <SelectTrigger className="h-9 w-auto min-w-32 gap-2" aria-label={t('analytics_index.groupType')}>
            <span className="text-muted-foreground">{t('analytics_index.groupType')}</span>
            <SelectValue>{t(`analytics_index.${GROUP_TYPES.find((g) => g.value === groupType)?.i18n}`)}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {GROUP_TYPES.map((g) => (
              <SelectItem key={g.value} value={g.value}>
                {t(`analytics_index.${g.i18n}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="relative w-full sm:w-32">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-9 pl-8"
            inputMode="numeric"
            placeholder={t('analytics_index.userId')}
            aria-label={t('analytics_index.userId')}
            value={userInput}
            onChange={(e) => setUserInput(e.target.value.replace(/\D/g, ''))}
            onBlur={() => (Number(userInput) || 0) !== userId && applyUser()}
          />
        </div>
      </form>

      <StackedBarCard
        title={t('analytics_index.consumptionStatistics')}
        total={spend(channel.totals.costs)}
        loading={loading}
        data={channel.rows.costs}
        series={channel.series}
        formatValue={(v) => '$' + v}
      />
      <div className="grid grid-cols-1 gap-3 sm:gap-4 lg:grid-cols-2">
        <StackedBarCard
          title={t('analytics_index.tokensStatistics')}
          total={renderNumber(channel.totals.tokens || 0)}
          loading={loading}
          data={channel.rows.tokens}
          series={channel.series}
          formatValue={(v) => renderNumber(v)}
          height={200}
        />
        <StackedBarCard
          title={t('analytics_index.requestsCount')}
          total={renderNumber(channel.totals.requests || 0)}
          loading={loading}
          data={channel.rows.requests}
          series={channel.series}
          formatValue={(v) => renderNumber(v)}
          height={200}
        />
      </div>

      <div className="grid grid-cols-1 gap-3 sm:gap-4 lg:grid-cols-2 xl:grid-cols-3">
        <RedemptionCard
          title={t('analytics_index.redemptionStatistics')}
          loading={loading}
          data={redemption}
          amountLabel={t('analytics_index.redemptionAmount')}
          usersLabel={t('analytics_index.uniqueUsers')}
        />
        <StackedRegistrationCard
          title={t('analytics_index.registrationStatistics')}
          total={renderNumber(registration.total)}
          loading={loading}
          data={registration.rows}
          directLabel={t('analytics_index.directRegistration')}
          inviteLabel={t('analytics_index.invitationRegistration')}
        />
        <SingleBarCard
          title={t('analytics_index.recharge')}
          total={orders.total.toFixed(2)}
          loading={loading}
          data={orders.rows}
          label={t('analytics_index.recharge')}
        />
      </div>
    </div>
  );
}
