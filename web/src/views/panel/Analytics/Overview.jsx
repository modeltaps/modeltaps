import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { Loader2, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import CollapsibleFilter from '@/components/CollapsibleFilter';
import { API } from 'utils/api';
import { renderNumber, showError } from 'utils/common';
import {
  buildChannelSeries,
  buildOrdersData,
  buildRedemptionData,
  buildRegistrationData,
  defaultRange,
  getDates
} from './chartUtils';
import { RedemptionCard, SingleBarCard, StackedBarCard, StackedRegistrationCard } from './AnalyticsCharts';

// ==============================|| ANALYTICS — PERIOD OVERVIEW ||============================== //
// shadcn/recharts port of views/Analytics/component/Overview. Same `/api/analytics/period`
// endpoint, date range + group type + user filter, and per-day charts.

const GROUP_TYPES = [
  { value: 'model_type', i18n: 'groupModelType' },
  { value: 'model', i18n: 'groupModel' },
  { value: 'channel', i18n: 'groupChannel' }
];

export default function Overview() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [start, setStart] = useState(defaultRange().start.format('YYYY-MM-DD'));
  const [end, setEnd] = useState(defaultRange().end.format('YYYY-MM-DD'));
  const [groupType, setGroupType] = useState('model_type');
  const [userId, setUserId] = useState(0);
  const [channel, setChannel] = useState({ rows: { costs: [], tokens: [], requests: [] }, totals: {}, series: [] });
  const [redemption, setRedemption] = useState([]);
  const [registration, setRegistration] = useState({ rows: [], total: 0 });
  const [orders, setOrders] = useState({ rows: [], total: 0 });

  const fetchData = async () => {
    setLoading(true);
    const range = { start: dayjs(start).startOf('day'), end: dayjs(end).endOf('day') };
    const dates = getDates(range.start, range.end);
    try {
      const res = await API.get('/api/analytics/period', {
        params: {
          start_timestamp: range.start.unix(),
          end_timestamp: range.end.unix(),
          group_type: groupType,
          user_id: userId
        }
      });
      const { success, message, data } = res.data;
      if (success && data) {
        setChannel(buildChannelSeries(data.channel_statistics, dates));
        setRedemption(buildRedemptionData(data.redemption_statistics, dates));
        setRegistration(buildRegistrationData(data.user_statistics, dates));
        setOrders(buildOrdersData(data.order_statistics, dates));
      } else if (!success) {
        showError(message);
      }
    } catch (error) {
      // network errors surfaced globally by the API interceptor
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Applied filter count for the collapsed-filter badge. Date fields are
  // excluded: they always carry a default range and never read as "inactive".
  const activeFilterCount = (groupType !== 'model_type' ? 1 : 0) + (userId !== 0 ? 1 : 0);

  return (
    <div className="space-y-6">
      <CollapsibleFilter activeCount={activeFilterCount} contentClassName="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-4">
          <Label htmlFor="an_start">{t('analytics_index.startTime')}</Label>
          <Input id="an_start" type="date" value={start} onChange={(e) => setStart(e.target.value)} />
        </div>
        <div className="space-y-4">
          <Label htmlFor="an_end">{t('analytics_index.endTime')}</Label>
          <Input id="an_end" type="date" value={end} onChange={(e) => setEnd(e.target.value)} />
        </div>
        <div className="space-y-4">
          <Label>{t('analytics_index.groupType')}</Label>
          <Select value={groupType} onValueChange={setGroupType}>
            <SelectTrigger>
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
        </div>
        <div className="space-y-4">
          <Label htmlFor="an_user">{t('analytics_index.userId')}</Label>
          <Input id="an_user" type="number" value={userId} onChange={(e) => setUserId(Number(e.target.value))} />
        </div>
        <div className="flex items-end sm:col-span-2 lg:col-span-4">
          <Button type="button" onClick={fetchData} disabled={loading}>
            {loading ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />}
            {t('analytics_index.search')}
          </Button>
        </div>
      </CollapsibleFilter>

      <div>
        <h2 className="text-lg font-semibold tracking-tight">
          {start} — {end}
        </h2>
        <div className="mt-3 border-t" />
      </div>

      <StackedBarCard
        title={t('analytics_index.consumptionStatistics')}
        total={`${t('analytics_index.totalConsumption')}：$${(channel.totals.costs || 0).toFixed(3)}`}
        loading={loading}
        data={channel.rows.costs}
        series={channel.series}
        formatValue={(v) => '$' + v}
      />
      <StackedBarCard
        title={t('analytics_index.tokensStatistics')}
        total={`${t('analytics_index.totalTokens')}：${renderNumber(channel.totals.tokens || 0)}`}
        loading={loading}
        data={channel.rows.tokens}
        series={channel.series}
        formatValue={(v) => renderNumber(v)}
      />
      <StackedBarCard
        title={t('analytics_index.requestsCount')}
        total={`${t('analytics_index.totalRequests')}：${renderNumber(channel.totals.requests || 0)}`}
        loading={loading}
        data={channel.rows.requests}
        series={channel.series}
        formatValue={(v) => renderNumber(v)}
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <RedemptionCard
          title={t('analytics_index.redemptionStatistics')}
          loading={loading}
          data={redemption}
          amountLabel={t('analytics_index.redemptionAmount')}
          usersLabel={t('analytics_index.uniqueUsers')}
        />
        <StackedRegistrationCard
          title={t('analytics_index.registrationStatistics')}
          total={`${t('analytics_index.totalRegistrations')}：${renderNumber(registration.total)}`}
          loading={loading}
          data={registration.rows}
          directLabel={t('analytics_index.directRegistration')}
          inviteLabel={t('analytics_index.invitationRegistration')}
        />
        <SingleBarCard
          title={t('analytics_index.recharge')}
          total={`${t('analytics_index.totalRecharge')}：${orders.total.toFixed(2)}`}
          loading={loading}
          data={orders.rows}
          label={t('analytics_index.recharge')}
        />
      </div>
    </div>
  );
}
