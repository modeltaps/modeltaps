import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| ERROR STATE ||============================== //
// Full-page error layout shared by the 404 view and the render-crash screen, composed like the home
// hero: the drip scene as the poster (ErrorDrip), then the status code in mono, a title, one sentence
// on what happened and the way out.

export default function ErrorState({ art, code, title, description, actions, children, className }) {
  return (
    <div className={cn('flex min-h-[70vh] w-full flex-col items-center justify-center px-6 py-16 text-center', className)}>
      {art && <div className="w-full max-w-lg">{art}</div>}
      <div className={cn('flex flex-col items-center gap-2', art && 'mt-8')}>
        {code && <p className="font-mono text-xs tabular-nums text-muted-foreground">{code}</p>}
        {title && <h1 className="text-balance text-2xl font-semibold tracking-[-0.01em] text-foreground">{title}</h1>}
        {description && <p className="max-w-md text-balance text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="mt-8 flex flex-wrap items-center justify-center gap-2">{actions}</div>}
      {children}
    </div>
  );
}

ErrorState.propTypes = {
  art: PropTypes.node,
  code: PropTypes.node,
  title: PropTypes.node,
  description: PropTypes.node,
  actions: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};
