import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ArrowUpRight } from 'lucide-react';

import { listPauses, listSupplies } from './supplyApi';
import { modelSupplyPath, supplyState } from './supplyHelpers';
import { SupplyStateLabel, UpstreamName } from './supplyParts';

// Read-only list of the public models this channel supplies, shown in the channel panel's
// Models tab. Supplies are edited on the model page; each row links there.
export default function ChannelSupplies({ channelId }) {
  const { t } = useTranslation();
  const [supplies, setSupplies] = useState(null);
  const [pauses, setPauses] = useState([]);
  const [now, setNow] = useState(0);

  useEffect(() => {
    let alive = true;
    Promise.all([listSupplies({ channel_id: channelId }), listPauses()])
      .then(([s, p]) => {
        if (!alive) return;
        setSupplies(s.success ? s.data || [] : []);
        setPauses(p.success ? p.data || [] : []);
        setNow(Math.floor(Date.now() / 1000));
      })
      .catch(() => alive && setSupplies([]));
    return () => {
      alive = false;
    };
  }, [channelId]);

  if (supplies === null) return null;

  return (
    <section className="mt-6 space-y-2" aria-labelledby={`channel-supplies-${channelId}`}>
      <div className="flex items-baseline justify-between gap-3">
        <h3 id={`channel-supplies-${channelId}`} className="text-sm font-medium">
          {t('modelSupply.channelSupplies.title', { count: supplies.length })}
        </h3>
        <span className="text-xs text-muted-foreground">{t('modelSupply.channelSupplies.readonly')}</span>
      </div>
      {supplies.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">{t('modelSupply.channelSupplies.none')}</p>
      ) : (
        <div className="overflow-hidden rounded-[10px] border border-border">
          <table className="w-full table-fixed border-collapse text-[13px]">
            <colgroup>
              <col />
              <col className="w-[24%]" />
              <col className="w-[13%]" />
              <col className="w-[11%]" />
              <col className="w-[22%]" />
              <col className="w-9" />
            </colgroup>
            <thead>
              <tr className="text-left text-xs text-muted-foreground [&>th]:h-9 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
                <th>{t('modelSupply.model')}</th>
                <th>{t('modelSupply.upstream')}</th>
                <th className="text-right">{t('modelSupply.priority')}</th>
                <th className="text-right">{t('modelSupply.weight')}</th>
                <th>{t('modelSupply.stateLabel')}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {supplies.map((s) => (
                <tr
                  key={s.id}
                  data-testid="channel-supply"
                  className="[&>td]:h-11 [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0"
                >
                  <td className="font-mono" title={s.model}>
                    {s.model}
                  </td>
                  <td className="!whitespace-normal">
                    <UpstreamName upstream={s.upstream_model} model={s.model} />
                  </td>
                  <td className="text-right tabular-nums">{s.priority ?? 0}</td>
                  <td className="text-right tabular-nums">{s.weight || 1}</td>
                  <td>
                    <SupplyStateLabel state={supplyState(s, pauses, now)} />
                  </td>
                  <td className="!px-1 text-right">
                    <Link
                      to={modelSupplyPath(s.model)}
                      aria-label={t('modelSupply.channelSupplies.open', { model: s.model })}
                      className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                    >
                      <ArrowUpRight className="size-3.5" />
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

ChannelSupplies.propTypes = { channelId: PropTypes.number.isRequired };
