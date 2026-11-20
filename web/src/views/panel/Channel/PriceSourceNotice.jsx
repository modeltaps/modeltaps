import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { AlertTriangle } from 'lucide-react';

import { Alert as Notice, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { timestamp2string } from 'utils/common';

// The last reads of the channel's upstream price list failed (model.PriceSourceState): the prices
// shown are the ones read before. A list held back because it looked broken (reason shrank or
// zeroed) can be taken anyway, for a platform that really dropped that many models.
export default function PriceSourceNotice({ state, busy, onForce }) {
  const { t } = useTranslation();
  if (!state?.failing_since) return null;
  const held = state.last_reason === 'shrank' || state.last_reason === 'zeroed';
  const since = timestamp2string(state.failing_since);
  const kept = state.last_success_at ? timestamp2string(state.last_success_at) : '';
  return (
    <Notice variant="warning" className="mb-3">
      <AlertTriangle className="size-4" />
      <AlertDescription className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="min-w-0 flex-1">
          {t(`channels.models.priceSource.${held ? 'held' : 'failed'}`, { since, count: state.failures })}{' '}
          {kept ? t('channels.models.priceSource.kept', { time: kept }) : t('channels.models.priceSource.none')}
          {!held && state.last_error && <span className="mt-0.5 block break-all text-xs text-muted-foreground">{state.last_error}</span>}
        </span>
        {held && (
          <Button size="sm" variant="outline" disabled={busy} onClick={onForce}>
            {t('channels.models.priceSource.useNew')}
          </Button>
        )}
      </AlertDescription>
    </Notice>
  );
}

PriceSourceNotice.propTypes = {
  state: PropTypes.shape({
    failing_since: PropTypes.number,
    failures: PropTypes.number,
    last_success_at: PropTypes.number,
    last_reason: PropTypes.string,
    last_error: PropTypes.string
  }),
  busy: PropTypes.bool,
  onForce: PropTypes.func.isRequired
};
