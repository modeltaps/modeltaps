import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import { hasOffPeak, peakHoursText } from './offPeak';

// The off-peak discount of a price, which comes with the vendor's official price (DeepSeek). It
// is shown, not edited here; removing it charges the full price at all hours.
export default function OffPeakNote({ value, onChange }) {
  const { t, i18n } = useTranslation();
  if (!hasOffPeak(value)) return null;
  const locale = (i18n.language || 'en').replace('_', '-');
  return (
    <div className="space-y-2 rounded-md border border-border p-3">
      <div className="flex items-center gap-2">
        <p className="flex-1 text-sm font-medium">{t('pricing_edit.offPeak.title')}</p>
        <Button type="button" size="sm" variant="outline" onClick={() => onChange(null)}>
          {t('pricing_edit.offPeak.remove')}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        {t('pricing_edit.offPeak.summary', { percent: Math.round(value.ratio * 100), hours: peakHoursText(value, t, locale) })}
      </p>
    </div>
  );
}

OffPeakNote.propTypes = {
  value: PropTypes.shape({ ratio: PropTypes.number, peak: PropTypes.array }),
  onChange: PropTypes.func.isRequired
};
