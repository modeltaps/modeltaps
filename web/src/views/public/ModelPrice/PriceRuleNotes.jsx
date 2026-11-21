import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { hasOffPeak, peakHoursText } from '../../panel/Pricing/offPeak';

// What changes a model's price besides the group ratio, shown under its price table: the
// long-context tier and the off-peak discount (model.LongContextTier, model.OffPeakPricing).
// A request charged at the long-context tier otherwise looks overcharged against the listed price.
// Prices per call or per minute have neither.
export default function PriceRuleNotes({ price }) {
  const { t, i18n } = useTranslation();
  if (!price || price.type === 'times' || price.type === 'minutes') return null;

  const tier = price.long_context;
  // A ratio that is not positive leaves that side at the listed price, as on the server.
  const input = tier?.input_ratio > 0 ? tier.input_ratio : 1;
  const output = tier?.output_ratio > 0 ? tier.output_ratio : 1;
  const longContext = tier?.threshold > 0 && (input !== 1 || output !== 1);
  const offPeak = hasOffPeak(price.off_peak) ? price.off_peak : null;
  if (!longContext && !offPeak) return null;

  const locale = (i18n.language || 'en').replace('_', '-');
  return (
    <div className="mt-3 space-y-1.5 text-sm leading-relaxed text-muted-foreground">
      {longContext && <p>{t('modelpricePage.longContextRule', { threshold: tier.threshold.toLocaleString(locale), input, output })}</p>}
      {offPeak && (
        <p>{t('pricing_edit.offPeak.summary', { percent: Math.round(offPeak.ratio * 100), hours: peakHoursText(offPeak, t, locale) })}</p>
      )}
    </div>
  );
}

PriceRuleNotes.propTypes = {
  price: PropTypes.shape({
    type: PropTypes.string,
    long_context: PropTypes.shape({ threshold: PropTypes.number, input_ratio: PropTypes.number, output_ratio: PropTypes.number }),
    off_peak: PropTypes.shape({ ratio: PropTypes.number, peak: PropTypes.array })
  })
};
