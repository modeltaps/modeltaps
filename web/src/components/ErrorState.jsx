import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| ERROR STATE ||============================== //
// Full-page error layout shared by the 404 view and the render-crash screen: a large status code
// or line icon, a title, one sentence on what happened, the address involved and the way out.
// Same tone as PageEmptyState (muted, no illustration) so errors read as part of the product.

export default function ErrorState({ code, icon: Icon, title, description, path, actions, children, className }) {
  return (
    <div className={cn('flex min-h-[70vh] w-full flex-col items-center justify-center px-6 py-16 text-center', className)}>
      {code ? (
        <p
          aria-hidden="true"
          className="select-none text-[6rem] font-semibold leading-none tracking-tighter text-muted-foreground/25 tabular-nums sm:text-[8rem]"
        >
          {code}
        </p>
      ) : (
        Icon && <Icon className="size-[5.5rem] text-muted-foreground" strokeWidth={1} aria-hidden="true" />
      )}
      {title && <h1 className="mt-6 text-xl font-semibold text-foreground">{title}</h1>}
      {description && <p className={cn('max-w-md text-balance text-sm text-muted-foreground', title ? 'mt-2' : 'mt-6')}>{description}</p>}
      {path && (
        <code className="mt-4 max-w-full truncate rounded-md bg-muted px-2 py-1 font-mono text-xs text-muted-foreground" title={path}>
          {path}
        </code>
      )}
      {actions && <div className="mt-8 flex flex-wrap items-center justify-center gap-2">{actions}</div>}
      {children}
    </div>
  );
}

ErrorState.propTypes = {
  code: PropTypes.node,
  icon: PropTypes.elementType,
  title: PropTypes.node,
  description: PropTypes.node,
  path: PropTypes.string,
  actions: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};
