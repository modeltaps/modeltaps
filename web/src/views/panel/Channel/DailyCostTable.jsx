import PropTypes from 'prop-types';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, RefreshCw } from 'lucide-react';
import { toast } from '@/components/ui/sonner';
import { timestamp2string } from 'utils/common';
import { fetchChannelDailyCosts, refreshChannelDailyCosts } from './channelApi';
import { costDrift, formatCost, formatDrift, isFlagged } from './upstreamCosts';

const ROW =
  '[&>td]:h-11 [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0';

// The account's own daily bill next to what this site expected, per UTC day of the last 30.
// The upstream bills the account, so the expected side sums every channel that shares the
// management key. Renders nothing for a channel whose upstream has no report or that has no
// management key.
export default function DailyCostTable({ channel }) {
  const { t } = useTranslation();
  const [data, setData] = useState(null);
  const [refreshing, setRefreshing] = useState(false);

  useEffect(() => {
    let alive = true;
    fetchChannelDailyCosts(channel.id)
      .then((d) => alive && setData(d))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [channel.id, channel.key_status?.manage_masked]);

  if (!data?.supported || !data.has_manage_key) return null;

  const refresh = async () => {
    setRefreshing(true);
    try {
      setData(await refreshChannelDailyCosts(channel.id));
    } catch (e) {
      toast.error(e.message);
    } finally {
      setRefreshing(false);
    }
  };

  const others = data.channels.filter((c) => c.id !== channel.id).map((c) => c.name);
  const flag = t('channels.upstreamCosts.flag');

  return (
    <div>
      <div className="flex items-center gap-2">
        <p className="text-[13px] font-medium">{t('channels.dailyCosts.title')}</p>
        <span className="flex-1" />
        {data.updated_at > 0 && (
          <span className="text-xs text-muted-foreground">
            {t('channelDetail.balanceUpdated', { time: timestamp2string(data.updated_at) })}
          </span>
        )}
        <button
          type="button"
          className="inline-flex items-center gap-1 text-xs text-info hover:underline disabled:opacity-50"
          disabled={refreshing}
          onClick={refresh}
        >
          {refreshing ? <Loader2 className="size-3 animate-spin" /> : <RefreshCw className="size-3" />}
          {t('channels.dailyCosts.refresh')}
        </button>
      </div>
      <p className="pb-2 pt-0.5 text-xs text-muted-foreground">
        {t('channels.dailyCosts.hint')}
        {others.length > 0 && ` ${t('channels.dailyCosts.shared', { names: others.join(', ') })}`}
      </p>
      {data.days.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border py-6 text-center text-sm text-muted-foreground">
          {t('channels.dailyCosts.none')}
        </p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border">
          <table className="w-full table-fixed border-collapse text-[13px]">
            <colgroup>
              <col />
              <col className="w-[4.5rem]" />
              <col className="w-[5.5rem]" />
              <col className="w-[5.5rem]" />
              <col className="w-[5rem]" />
            </colgroup>
            <thead>
              <tr className="text-left text-xs text-muted-foreground [&>th]:h-9 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
                <th>{t('channels.dailyCosts.day')}</th>
                <th className="text-right">{t('channels.performance.requests')}</th>
                <th className="text-right">{t('channels.upstreamCosts.expected')}</th>
                <th className="text-right">{t('channels.dailyCosts.billed')}</th>
                <th className="text-right">{t('channels.upstreamCosts.diff')}</th>
              </tr>
            </thead>
            <tbody>
              {data.days.map((d) => {
                const billed = d.upstream_cost !== null && d.upstream_cost !== undefined;
                const drift = billed ? costDrift(d.upstream_cost, d.expected_cost) : null;
                const flagged = isFlagged(drift);
                return (
                  <tr key={d.date} className={ROW}>
                    <td className="tabular-nums">{d.date}</td>
                    <td className="text-right tabular-nums">{d.request_count.toLocaleString()}</td>
                    <td className="text-right tabular-nums">{formatCost(d.expected_cost)}</td>
                    <td className="text-right tabular-nums">{billed ? formatCost(d.upstream_cost) : '—'}</td>
                    <td
                      className={`text-right tabular-nums ${flagged ? 'font-medium text-warning' : ''}`}
                      title={flagged ? flag : undefined}
                    >
                      {formatDrift(drift)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

DailyCostTable.propTypes = { channel: PropTypes.object.isRequired };
