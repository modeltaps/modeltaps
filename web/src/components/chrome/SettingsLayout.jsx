import PropTypes from 'prop-types';
import { Link } from 'react-router';

import { cn } from '@/lib/utils';

// ==============================|| CHROME — SETTINGS LAYOUT (in-page subnav) ||============================== //
// Settings pages (personal, organization, platform) render inside the sidebar shell: a left
// subnav of the page's sections and the active section on the right. Below md the subnav
// becomes a horizontally scrolling row above the content. Each section is its own route, so
// deep links keep working.

export default function SettingsLayout({ label, items, activeId, children, className }) {
  return (
    <div className={cn('md:grid md:grid-cols-[13rem_minmax(0,1fr)] md:items-start md:gap-8', className)}>
      <nav
        aria-label={label}
        className="-mx-1 mb-5 flex gap-1 overflow-x-auto px-1 pb-1 [scrollbar-width:none] md:sticky md:top-0 md:mx-0 md:mb-0 md:flex-col md:gap-0.5 md:overflow-visible md:p-0"
      >
        {items.map((item) => {
          const active = item.id === activeId;
          const Icon = item.icon;
          return (
            <Link
              key={item.id}
              to={item.to}
              aria-current={active ? 'page' : undefined}
              className={cn(
                'flex h-[34px] shrink-0 items-center gap-2.5 whitespace-nowrap rounded-[10px] px-3 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                active
                  ? 'bg-card font-medium text-foreground shadow-[inset_0_0_0_1px_var(--border)]'
                  : 'text-muted-foreground hover:bg-muted hover:text-foreground'
              )}
            >
              {Icon && <Icon className="size-4 shrink-0" />}
              <span className="truncate">{item.label}</span>
            </Link>
          );
        })}
      </nav>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

SettingsLayout.propTypes = {
  label: PropTypes.string.isRequired,
  items: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      to: PropTypes.string.isRequired,
      label: PropTypes.node,
      icon: PropTypes.elementType
    })
  ).isRequired,
  activeId: PropTypes.string,
  children: PropTypes.node,
  className: PropTypes.string
};
