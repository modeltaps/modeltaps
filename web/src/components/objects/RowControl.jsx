import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

const stop = (e) => e.stopPropagation();

export default function RowControl({ children, className }) {
  return (
    <div role="presentation" className={cn('inline-flex items-center', className)} onClick={stop} onKeyDown={stop}>
      {children}
    </div>
  );
}

RowControl.propTypes = { children: PropTypes.node, className: PropTypes.string };
