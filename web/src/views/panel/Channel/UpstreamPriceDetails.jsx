import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ArrowUpRight } from 'lucide-react';

import { timestamp2string } from 'utils/common';
import { cn } from '@/lib/utils';
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { usd } from './channelPrices';

// The rows the details list for an upstream price entry: every price it has, in the order the
// upstream's own pages use. Token prices are per million tokens.
export function priceDetailRows(price) {
  if (!price) return [];
  const rows = [];
  const add = (key, value, unit) => value && rows.push({ key, value: usd(value), unit });
  add('input', price.input, 'M');
  add('output', price.output, 'M');
  add('cacheRead', price.cache_read, 'M');
  add('cacheWrite', price.cache_write, 'M');
  add('image', price.image, 'image');
  add('request', price.request, 'request');
  add('videoSecond', price.video_second, 'second');
  return rows;
}

// A price cell that opens the upstream price's details on click: every price the upstream lists
// for the model (cache prices included), where and when it was read, the model's own sell price
// when given, and a link to the upstream's page for the model so the price can be checked there.
// `children` is the cell's short form, the trigger.
export default function UpstreamPriceDetails({ price, upstream, sell, losses = [], align = 'end', className, children }) {
  const { t } = useTranslation();
  const rows = priceDetailRows(price);
  const free = rows.length === 0 && !price.other;
  const source = price.source || '';
  const sellText = sell && (sell.input != null || sell.output != null) ? `${usd(sell.input ?? 0)} / ${usd(sell.output ?? 0)}` : '';
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('upstreamPrice.open', { model: upstream })}
          className={cn(
            'rounded-sm underline decoration-dotted decoration-muted-foreground/60 underline-offset-4 hover:decoration-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            className
          )}
        >
          {children}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent role="dialog" align={align} className="w-72 space-y-3 p-3 text-left text-xs">
        <div className="space-y-0.5">
          <p className="text-sm font-medium text-foreground">
            {source ? t('upstreamPrice.title', { source }) : t('upstreamPrice.titlePlain')}
          </p>
          <p className="break-all font-mono text-muted-foreground">{upstream}</p>
        </div>

        {free ? (
          <p className="text-success">{t('upstreamPrice.free')}</p>
        ) : price.other && rows.length === 0 ? (
          <p className="text-muted-foreground">{t('channels.models.priceOtherHint')}</p>
        ) : (
          <dl className="grid grid-cols-[1fr_auto] gap-x-3 gap-y-1">
            {rows.map((row) => (
              <div key={row.key} className="contents">
                <dt className="text-muted-foreground">{t(`upstreamPrice.row_${row.key}`)}</dt>
                <dd className="text-right tabular-nums text-foreground">
                  {row.value}
                  <span className="text-muted-foreground"> / {t(`upstreamPrice.unit_${row.unit}`)}</span>
                </dd>
              </div>
            ))}
          </dl>
        )}

        {(sellText || losses.length > 0) && (
          <div className="space-y-1 border-t border-border pt-2">
            {sellText && (
              <p className="flex justify-between gap-3">
                <span className="text-muted-foreground">{t('upstreamPrice.sell')}</span>
                <span className="tabular-nums text-foreground">{sellText}</span>
              </p>
            )}
            {losses.map((line) => (
              <p key={line} className="text-warning">
                {line}
              </p>
            ))}
          </div>
        )}

        <div className="space-y-1.5 border-t border-border pt-2 text-muted-foreground">
          {price.via && <p>{t('upstreamPrice.via', { via: price.via })}</p>}
          {price.synced_at > 0 && <p>{t('upstreamPrice.syncedAt', { time: timestamp2string(price.synced_at) })}</p>}
          {price.page_url ? (
            <a
              href={price.page_url}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 font-medium text-foreground hover:underline"
            >
              {price.page_exact ? t('upstreamPrice.openPage', { source }) : t('upstreamPrice.openPricing', { source })}
              <ArrowUpRight className="size-3.5" />
            </a>
          ) : (
            <p>{t('upstreamPrice.noPage')}</p>
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

UpstreamPriceDetails.propTypes = {
  price: PropTypes.object.isRequired,
  upstream: PropTypes.string.isRequired,
  sell: PropTypes.shape({ input: PropTypes.number, output: PropTypes.number }),
  losses: PropTypes.arrayOf(PropTypes.string),
  align: PropTypes.oneOf(['start', 'end']),
  className: PropTypes.string,
  children: PropTypes.node
};
