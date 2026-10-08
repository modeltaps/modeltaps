import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { cn } from '@/lib/utils';
import { AVAILABILITY } from '../shared/modelIndex';

// ==============================|| PLAYGROUND — STATUS DOT ||============================== //
// 模型可用性状态点：绿 = 可用、琥珀 = 降级、灰 = 不可用（未知一律按不可用画）。

const DOT_CLASS = {
  [AVAILABILITY.available]: 'bg-emerald-500',
  [AVAILABILITY.degraded]: 'bg-amber-500',
  [AVAILABILITY.unavailable]: 'bg-muted-foreground/40'
};

const DOT_LABEL_KEY = {
  [AVAILABILITY.available]: 'playground.picker.available',
  [AVAILABILITY.degraded]: 'playground.picker.degraded',
  [AVAILABILITY.unavailable]: 'playground.picker.unavailable'
};

export default function StatusDot({ availability, className }) {
  const { t } = useTranslation();
  const key = DOT_LABEL_KEY[availability] || DOT_LABEL_KEY[AVAILABILITY.unavailable];
  return (
    <span
      className={cn('inline-block size-2 shrink-0 rounded-full', DOT_CLASS[availability] || DOT_CLASS[AVAILABILITY.unavailable], className)}
      title={t(key)}
      aria-label={t(key)}
      role="img"
    />
  );
}

StatusDot.propTypes = {
  availability: PropTypes.string,
  className: PropTypes.string
};
