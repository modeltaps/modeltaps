import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import ProviderIcon from '@/components/brand/ProviderIcon';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { previewSupplies } from './supplyApi';
import { leadOrder, orderText, previewEntryState } from './supplyHelpers';
import { SupplyStateLabel } from './supplyParts';

// Why a supply sits where it does, in words (reason and policy_tier from the preview API).
export function supplyReasonText(t, entry) {
  const reason = entry.reason || 'channel';
  return t(`modelSupply.order.reason.${reason}`, { tier: entry.policy_tier || 0 });
}

function Glyph({ entry }) {
  return (
    <span className={cn('relative inline-flex shrink-0', !entry.usable && 'opacity-35 grayscale')}>
      <ProviderIcon type={entry.type} name={entry.name} className="size-4" />
      {entry.count > 1 && (
        <span className="absolute -bottom-1 -right-1.5 rounded-full bg-background px-0.5 text-[9px] font-medium leading-3 tabular-nums">
          {entry.count}
        </span>
      )}
    </span>
  );
}

Glyph.propTypes = { entry: PropTypes.object.isRequired };

// The line that takes requests now as provider icons, then how many other lines back it up.
// When the preferred lines are all down the icons are those of the backup now serving and the
// count turns to the warning colour; when nothing can serve, the first tier shows greyed out.
export function SupplyOrderGlyphs({ tiers }) {
  const { t } = useTranslation();
  const { lead, rest, fallback, down } = leadOrder(tiers);
  return (
    <span className="inline-flex items-center gap-1">
      {lead.map((entry) => (
        <Glyph key={entry.type} entry={entry} />
      ))}
      {rest > 0 && (
        <span className={cn('text-xs tabular-nums text-muted-foreground', (fallback || down) && 'text-warning')}>
          {t('modelSupply.order.more', { count: rest })}
        </span>
      )}
    </span>
  );
}

SupplyOrderGlyphs.propTypes = { tiers: PropTypes.array.isRequired };

// The order a request for the model takes, loaded when the panel opens: each tier, each supply's
// share and state, and why it sits there.
function OrderDetail({ model }) {
  const { t } = useTranslation();
  const [preview, setPreview] = useState(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let live = true;
    previewSupplies({ model })
      .then((res) => {
        if (!live) return;
        if (res.success) setPreview(res.data);
        else setFailed(true);
      })
      .catch(() => live && setFailed(true));
    return () => {
      live = false;
    };
  }, [model]);

  if (failed) return <p className="p-3 text-[13px] text-muted-foreground">{t('modelSupply.order.failed')}</p>;
  if (!preview) {
    return (
      <div className="flex justify-center p-4 text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
      </div>
    );
  }
  const tiers = preview.tiers || [];
  return (
    <div className="w-[26rem] max-w-[calc(100vw-2rem)] space-y-2 p-3 text-[13px]">
      <p className="text-xs text-muted-foreground">
        {preview.allocation === 'policy'
          ? t('modelSupply.order.byPolicy')
          : preview.allocation === 'manual'
            ? t('modelSupply.order.byModel')
            : t('modelSupply.order.byChannel')}
      </p>
      {tiers.length === 0 && <p className="text-muted-foreground">{t('modelSupply.order.none')}</p>}
      <ol className="space-y-2">
        {tiers.map((tier, i) => (
          <li key={tier.priority} className="space-y-1">
            <p className="text-xs font-medium text-muted-foreground">
              {i === 0 ? t('modelSupply.order.first') : t('modelSupply.order.then', { n: i })}
            </p>
            <ul className="divide-y divide-border overflow-hidden rounded-[10px] border border-border">
              {(tier.supplies || []).map((s) => (
                <li key={s.id} data-testid="order-supply" className="grid grid-cols-[1fr_auto] items-start gap-x-3 gap-y-0.5 px-2.5 py-1.5">
                  <span className="flex min-w-0 items-center gap-1.5">
                    <ProviderIcon type={s.channel_type} name={s.channel_name} className="size-4 shrink-0" />
                    <span className="truncate">{s.channel_name || t('modelSupply.channelDeleted', { id: s.channel_id })}</span>
                    {s.share > 0 && (
                      <span className="shrink-0 text-xs tabular-nums text-muted-foreground">{Math.round(s.share * 100)}%</span>
                    )}
                  </span>
                  <SupplyStateLabel state={previewEntryState(s)} />
                  <span className="col-span-2 text-xs text-muted-foreground">{supplyReasonText(t, s)}</span>
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ol>
    </div>
  );
}

OrderDetail.propTypes = { model: PropTypes.string.isRequired };

// List cell: the supply order as icons; a click shows the order in full and why.
export default function SupplyOrder({ model, tiers, label, onManage, children }) {
  const { t } = useTranslation();
  const manageLabel = t('modelSupply.order.manage');
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        type="button"
        title={[orderText(tiers), label].filter(Boolean).join(' · ')}
        aria-label={[orderText(tiers), label].filter(Boolean).join(' · ')}
        className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-md text-left hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {children}
        <SupplyOrderGlyphs tiers={tiers} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="p-0">
        <OrderDetail model={model} />
        <DropdownMenuSeparator className="my-0" />
        <DropdownMenuItem className="m-1 justify-center" onClick={onManage}>
          {manageLabel}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

SupplyOrder.propTypes = {
  model: PropTypes.string.isRequired,
  tiers: PropTypes.array.isRequired,
  label: PropTypes.string,
  onManage: PropTypes.func,
  children: PropTypes.node
};
