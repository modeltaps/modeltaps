import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useSelector } from 'react-redux';

import { brandWordmark, isDefaultBrand } from 'utils/brand';
import { DASHBOARD_URL } from './nav-config';
import { BrandLockup, LogoMark } from './Logo';
import { CollapseButton, SidebarSearch } from './SidebarControls';

// ==============================|| CHROME — SIDEBAR BRAND ROW ||============================== //
// The first row of every sidebar (workbench, platform admin, 设置): the brand (links to the
// dashboard), then the search button (the panel's search palette) and the collapse button when
// one is given (the drawer uses it to close). The collapsed rail shows the mark only; its search
// icon sits under the switcher row.

export default function SidebarBrandRow({ collapsed = false, onItemClick, onOpenCommand, onToggleCollapse }) {
  const siteInfo = useSelector((state) => state.siteInfo);
  const brand = brandWordmark(siteInfo?.system_name);

  if (collapsed) {
    return (
      <div className="flex h-16 shrink-0 items-center justify-center">
        <Link to={DASHBOARD_URL} onClick={onItemClick} aria-label={brand} className="text-foreground hover:opacity-90">
          <LogoMark size={20} />
        </Link>
      </div>
    );
  }
  return (
    <div className="flex h-16 shrink-0 items-center gap-1 pl-4 pr-2">
      <Link
        to={DASHBOARD_URL}
        onClick={onItemClick}
        className="flex min-w-0 flex-1 items-center gap-2.5 text-foreground transition-opacity hover:opacity-90"
      >
        <BrandLockup text={brand} wordmark={isDefaultBrand(siteInfo?.system_name)} truncate />
      </Link>
      {onOpenCommand && <SidebarSearch onClick={onOpenCommand} />}
      {onToggleCollapse && <CollapseButton onClick={onToggleCollapse} />}
    </div>
  );
}

SidebarBrandRow.propTypes = {
  collapsed: PropTypes.bool,
  onItemClick: PropTypes.func,
  onOpenCommand: PropTypes.func,
  onToggleCollapse: PropTypes.func
};
