import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, ChevronsUpDown } from 'lucide-react';

import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel } from '@/components/ui/dropdown-menu';
import { ContextMark } from '@/components/chrome/ContextSwitcher';

// ==============================|| SETTINGS — ORGANIZATION PICKER ||============================== //
// Heads 组织设置 in the settings sidebar: the organizations the user manages, with their role.
// Picking one only changes which organization the settings show (the URL); the active context
// in the top-left switcher stays as it is.

export default function OrgPicker({ orgs, value, onChange }) {
  const { t } = useTranslation();
  const current = orgs.find((org) => org.id === value) || null;
  const label = t('settingsPage.orgSelector.label', { defaultValue: 'Select organization' });

  return (
    <DropdownMenu className="w-full">
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={label}
          className="flex h-[34px] w-full items-center gap-2.5 rounded-lg px-3 text-left text-sm font-medium text-foreground transition-colors hover:bg-sidebar-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ContextMark kind="org" org={current} className="size-5" />
          <span className="min-w-0 flex-1 truncate">{current ? current.name : label}</span>
          <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" matchTriggerWidth className="w-0 min-w-0">
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
  onChange: PropTypes.func.isRequired
};
