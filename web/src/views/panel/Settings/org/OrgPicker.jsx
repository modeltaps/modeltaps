import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Building2, Check, ChevronsUpDown } from 'lucide-react';

import { cn } from '@/lib/utils';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel } from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { sidebarRow } from '@/components/chrome/SidebarControls';

// ==============================|| SETTINGS — ORGANIZATION PICKER ||============================== //
// The organization picker of the settings sidebar, right under its back row: the organizations the
// user manages, with their role. It reads like a row of the tree below it: the same plain icon (the
// organization's logo when it has one), the same size and the same columns for icon and text, so
// the icons line up; in the collapsed rail only the icon, opening to the right. Picking one only
// changes which organization the settings show (the URL); the active context of the workbench
// stays as it is.

function OrgIcon({ org, className }) {
  if (org?.avatar_url) {
    return <img src={org.avatar_url} alt="" aria-hidden="true" className={cn('shrink-0 rounded-[4px] object-cover', className)} />;
  }
  return <Building2 aria-hidden="true" className={cn('shrink-0 text-muted-foreground', className)} />;
}

OrgIcon.propTypes = { org: PropTypes.object, className: PropTypes.string };

export default function OrgPicker({ orgs, value, onChange, collapsed = false }) {
  const { t } = useTranslation();
  const current = orgs.find((org) => org.id === value) || null;
  const label = t('settingsPage.orgSelector.label', { defaultValue: 'Select organization' });

  const trigger = (
    <DropdownMenuTrigger asChild>
      <button
        type="button"
        aria-label={collapsed && current ? `${label}: ${current.name}` : label}
        className={cn(sidebarRow.base, sidebarRow.idle, collapsed ? 'size-9 w-9 justify-center' : 'h-9 gap-2.5 px-3 text-left')}
      >
        <OrgIcon org={current} className={collapsed ? 'size-5' : 'size-4'} />
        {!collapsed && (
          <>
            <span className="min-w-0 flex-1 truncate text-sm font-medium text-foreground">{current ? current.name : label}</span>
            <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
          </>
        )}
      </button>
    </DropdownMenuTrigger>
  );

  return (
    <DropdownMenu className={cn(!collapsed && 'w-full')}>
      {collapsed ? (
        <Tooltip>
          <TooltipTrigger asChild>{trigger}</TooltipTrigger>
          <TooltipContent side="right">{current?.name ?? label}</TooltipContent>
        </Tooltip>
      ) : (
        trigger
      )}
      <DropdownMenuContent align="start" side={collapsed ? 'right' : 'bottom'} matchTriggerWidth={!collapsed} className="min-w-60">
        <DropdownMenuLabel>{label}</DropdownMenuLabel>
        {orgs.map((org) => (
          <DropdownMenuItem key={org.id} onClick={() => org.id !== value && onChange(org.id)}>
            <span className="min-w-0 flex-1 truncate">{org.name}</span>
            <span className="shrink-0 text-[11px] text-muted-foreground">{t(`org.roles.${org.role}`, { defaultValue: org.role })}</span>
            {org.id === value && <Check className="size-4 shrink-0" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

OrgPicker.propTypes = {
  orgs: PropTypes.arrayOf(PropTypes.shape({ id: PropTypes.number.isRequired, name: PropTypes.string, role: PropTypes.string })).isRequired,
  value: PropTypes.number,
  onChange: PropTypes.func.isRequired,
  // The collapsed rail: only the icon, the list opens to the right.
  collapsed: PropTypes.bool
};
