import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { sellSideOf } from './channelModelDetails';

// A channel's price next to the model's base sell price, as the percentage it is above or below
// it ("+14%", "−18%", "="): amber (the loss warning colour) when the cost is above what the price brings in, muted when dearer
// than the list price but still inside the margin, green when cheaper. `cmp` is sellDeltaOf's.
export default function SellDelta({ cmp, className }) {
  const { t } = useTranslation();
  const side = sellSideOf(cmp);
  if (!side) return null;
  const pct = Math.round(Math.abs(cmp.delta) * 100);
  const text = side === 'equal' ? '=' : `${side === 'above' ? '+' : '−'}${pct || '<1'}%`;
  const title =
    side === 'equal'
      ? t('sellDelta.equalTip')
      : t(cmp.above ? 'sellDelta.lossTip' : side === 'above' ? 'sellDelta.aboveTip' : 'sellDelta.belowTip', { pct: pct || '<1' });
  return (
    <span
      data-testid="sell-delta"
      data-side={side}
      title={title}
      className={cn(
        'inline-block min-w-[2.75rem] whitespace-nowrap text-right text-xs tabular-nums',
        cmp.above ? 'text-warning' : side === 'below' ? 'text-success' : 'text-muted-foreground',
        className
      )}
    >
      {text}
    </span>
  );
}

SellDelta.propTypes = { cmp: PropTypes.shape({ delta: PropTypes.number, above: PropTypes.bool }), className: PropTypes.string };
