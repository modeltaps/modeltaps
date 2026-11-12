import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ArrowUpRight, Loader2, RefreshCw, Search } from 'lucide-react';

import { timestamp2string } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/ui/sonner';
import { StatusSegments } from '@/components/list-page';
import { checkChannelDrift, fetchChannelUpstreamModels } from './channelApi';
import { UPSTREAM_FILTERS, filterUpstreamModels, upstreamCounts } from './upstreamModelList';
import { modelSupplyPath } from '../ModelInfo/supply/supplyHelpers';
import { StateDot } from '../ModelInfo/supply/supplyParts';

const TONES = { in_use: 'ok', off: 'off', available: 'off', not_in_catalog: 'off', unknown: 'warn' };

// Search shows once the list is long enough to need it.
const SEARCH_FROM = 9;

// Everything the channel's upstream listed at the last model check, with the public model each
// name belongs to and whether this channel supplies it. Read-only: which models are public and
// who supplies them is decided on the model page.
export default function UpstreamModels({ channelId, canCheck = true }) {
  const { t } = useTranslation();
  const [data, setData] = useState(null);
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    () =>
      fetchChannelUpstreamModels(channelId)
        .then(setData)
        .catch(() => setData({ checked_at: 0, models: [] })),
    [channelId]
  );

  // The panel remounts this per channel (key), so the effect only has to load.
  useEffect(() => {
    load();
  }, [load]);

  const checkNow = async () => {
    setBusy(true);
    try {
      const res = await checkChannelDrift(channelId);
      if (!res.success) toast.error(res.message || t('channels.upstream.checkFailed'));
      else await load();
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusy(false);
    }
  };

  const models = useMemo(() => data?.models || [], [data]);
  const counts = useMemo(() => upstreamCounts(models), [models]);
  const rows = useMemo(() => filterUpstreamModels(models, filter, query), [models, filter, query]);
  const options = UPSTREAM_FILTERS.map((value) => ({ value, label: t(`channels.upstream.filter.${value}`) }));

  if (data === null) return null;

  return (
    <section className="mt-6 space-y-2" aria-labelledby={`channel-upstream-${channelId}`}>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <h3 id={`channel-upstream-${channelId}`} className="text-sm font-medium">
          {t('channels.upstream.title')}
          <span className="ml-1.5 font-normal tabular-nums text-muted-foreground">{models.length}</span>
        </h3>
        <span className="text-xs text-muted-foreground">
          {data.checked_at ? t('channelDetail.driftCheckedAt', { time: timestamp2string(data.checked_at) }) : t('channelDetail.driftNever')}
        </span>
        <span className="flex-1" />
        {canCheck && (
          <Button variant="outline" size="sm" className="h-[30px] rounded-lg bg-card" disabled={busy} onClick={checkNow}>
            {busy ? <Loader2 className="animate-spin" /> : <RefreshCw />}
            {t('channels.upstream.check')}
          </Button>
        )}
      </div>
      <p className="text-[13px] text-muted-foreground">{t('channels.upstream.hint')}</p>

      {models.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground">
          {canCheck ? t('channels.upstream.empty') : t('channels.upstream.unsupported')}
        </p>
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <StatusSegments label={t('channels.upstream.title')} value={filter} onChange={setFilter} options={options} counts={counts} />
            <span className="flex-1" />
            {models.length >= SEARCH_FROM && (
              <div className="relative w-56 max-w-full">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={t('channels.upstream.search')}
                  aria-label={t('channels.upstream.search')}
                  className="h-[30px] rounded-lg pl-8 text-[13px]"
                />
              </div>
            )}
          </div>
          <div className="max-h-[420px] overflow-auto rounded-[10px] border border-border">
            <table className="w-full table-fixed border-collapse text-[13px]">
              <colgroup>
                <col />
                <col className="w-[34%]" />
                <col className="w-[22%]" />
                <col className="w-9" />
              </colgroup>
              <thead className="sticky top-0 z-[1] bg-card">
                <tr className="text-left text-xs text-muted-foreground [&>th]:h-9 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
                  <th>{t('channels.upstream.name')}</th>
                  <th>{t('channels.upstream.publicModel')}</th>
                  <th>{t('channels.upstream.state')}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {rows.length === 0 && (
                  <tr>
                    <td colSpan={4} className="h-16 px-3 text-center text-muted-foreground">
                      {t('channels.upstream.noMatch')}
                    </td>
                  </tr>
                )}
                {rows.map((m) => {
                  const inCatalog = m.state === 'in_use' || m.state === 'off' || m.state === 'available';
                  return (
                    <tr
                      key={m.name}
                      data-testid="upstream-model"
                      className="[&>td]:h-11 [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0"
                    >
                      <td className="font-mono" title={m.name}>
                        {m.name}
                      </td>
                      <td className={inCatalog ? 'font-mono' : 'font-mono text-muted-foreground'} title={m.public_model || ''}>
                        {m.public_model || '—'}
                      </td>
                      <td>
                        <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
                          <StateDot tone={TONES[m.state] || 'warn'} />
                          {t(`channels.upstream.states.${m.state}`, { defaultValue: m.state })}
                        </span>
                      </td>
                      <td className="!px-1 text-right">
                        {inCatalog && (
                          <Link
                            to={modelSupplyPath(m.public_model)}
                            aria-label={t('modelSupply.channelSupplies.open', { model: m.public_model })}
                            className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                          >
                            <ArrowUpRight className="size-3.5" />
                          </Link>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </section>
  );
}

UpstreamModels.propTypes = { channelId: PropTypes.number.isRequired, canCheck: PropTypes.bool };
