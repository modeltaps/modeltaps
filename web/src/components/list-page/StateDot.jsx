import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

const TONES = { live: 'bg-success', warn: 'bg-warning', bad: 'bg-destructive', off: 'bg-muted-foreground/60' };

// Small status dot that leads the meta line of a phone card. Phone cards carry one control (the
// row menu, top right), so a row's on / off state reads from this dot and the words after it.
export default function StateDot({ tone = 'off', className }) {
  return <span className={cn('mr-1.5 inline-block size-1.5 shrink-0 rounded-full align-middle', TONES[tone], className)} aria-hidden />;
}

StateDot.propTypes = { tone: PropTypes.oneOf(Object.keys(TONES)), className: PropTypes.string };
