import { useState } from 'react';
import PropTypes from 'prop-types';
import { useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Check, ChevronsUpDown, Plus } from 'lucide-react';

import { cn } from '@/lib/utils';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel } from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useOrg } from 'contexts/OrgContext';
import { useIsRoot } from 'utils/common';
import CreateOrgDialog from '../org/CreateOrgDialog';
import { contextSwitchTarget } from './nav-config';

// ==============================|| CHROME — CONTEXT SWITCHER ||============================== //
// One line under the brand row that only says which space the user is in: 个人, the active
// organization's name, or 平台管理. No user identity here; that lives in the account row at the
// bottom of the sidebar. The popover lists personal, the user's organizations (role muted) and
// "create organization"; platform admin is entered from the account menu, and while it is open
// this row names it and the popover leads back to personal or an organization. Markers are neutral outlined squares
// (个 / the org's first letter / 管), deliberately unlike the round avatar.

export const firstLetter = (name) => (Array.from((name || '').trim())[0] || '?').toUpperCase();

export function ContextMark({ children, className }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex shrink-0 items-center justify-center rounded-md border border-border bg-card font-medium leading-none text-foreground',
        className
      )}
    >
      {children}
    </span>
  );
}

ContextMark.propTypes = { children: PropTypes.node, className: PropTypes.string };

export default function ContextSwitcher({ context, collapsed = false, onNavigate }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const isRoot = useIsRoot();
  const { orgEnabled, orgRoutingPolicyEnabled, organizations, currentOrg, switchOrg } = useOrg();
  const [createOpen, setCreateOpen] = useState(false);

  const roleLabel = (role) => t(`org.roles.${role}`, { defaultValue: role });
  const current =
    context === 'admin'
      ? { mark: t('nav.context.adminMark'), name: t('nav.context.admin') }
      : context === 'org' && currentOrg
        ? { mark: firstLetter(currentOrg.name), name: currentOrg.name }
        : { mark: t('nav.context.personalMark'), name: t('nav.context.personal') };

  const select = (to) => {
    const target = contextSwitchTarget({ pathname, to, isRoot, features: { orgRoutingPolicy: orgRoutingPolicyEnabled } });
    if (to.kind === 'personal') switchOrg(null);
    if (to.kind === 'org') switchOrg(to.orgId);
    if (target) navigate(target);
    onNavigate?.();
  };

  const hasMenu = orgEnabled || context === 'admin';
  const markSize = collapsed ? 'size-7 text-[13px]' : 'size-6 text-xs';
  const triggerClass = collapsed
    ? 'flex size-9 items-center justify-center rounded-lg transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'
    : 'flex h-9 w-full items-center gap-2.5 rounded-lg px-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

  const body = (
    <>
      <ContextMark className={markSize}>{current.mark}</ContextMark>
      {!collapsed && <span className="min-w-0 flex-1 truncate text-sm font-medium text-foreground">{current.name}</span>}
      {!collapsed && hasMenu && <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />}
    </>
  );

  if (!hasMenu) {
    return <div className={cn(triggerClass, 'hover:bg-transparent')}>{body}</div>;
  }

  const check = <Check className="size-4 shrink-0" />;
  const rowMark = 'size-[22px] text-[11px]';

  const trigger = (
    <DropdownMenuTrigger asChild>
      <button type="button" aria-label={`${t('nav.context.switch')}: ${current.name}`} className={triggerClass}>
        {body}
      </button>
    </DropdownMenuTrigger>
  );

  return (
    // [&>div]:w-full: the DropdownMenu wrapper is inline-block; stretch it so the row fills the rail.
    <div className={cn(!collapsed && '[&>div]:w-full')}>
      <DropdownMenu>
        {collapsed ? (
          <Tooltip>
            <TooltipTrigger asChild>{trigger}</TooltipTrigger>
            <TooltipContent side="right">{current.name}</TooltipContent>
          </Tooltip>
        ) : (
          trigger
        )}
        <DropdownMenuContent align="start" side={collapsed ? 'right' : 'bottom'} matchTriggerWidth={!collapsed} className="min-w-60">
          <DropdownMenuItem onClick={() => select({ kind: 'personal' })}>
            <ContextMark className={rowMark}>{t('nav.context.personalMark')}</ContextMark>
            <span className="flex-1 truncate">{t('nav.context.personal')}</span>
            {context === 'personal' && check}
          </DropdownMenuItem>
          {orgEnabled && (
            <>
              <DropdownMenuLabel className="font-medium">{t('nav.context.organizations')}</DropdownMenuLabel>
              {organizations.map((org) => {
                const active = context === 'org' && currentOrg?.id === org.id;
                return (
                  <DropdownMenuItem key={org.id} onClick={() => select({ kind: 'org', orgId: org.id, role: org.role })}>
                    <ContextMark className={rowMark}>{firstLetter(org.name)}</ContextMark>
                    <span className="min-w-0 flex-1 truncate">{org.name}</span>
                    {active ? check : <span className="shrink-0 text-xs text-muted-foreground">{roleLabel(org.role)}</span>}
                  </DropdownMenuItem>
                );
              })}
              <DropdownMenuItem onClick={() => setCreateOpen(true)} className="text-muted-foreground">
                <span className="flex size-[22px] shrink-0 items-center justify-center">
                  <Plus className="size-4" />
                </span>
                <span className="flex-1 truncate">{t('org.createOrg')}</span>
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      {orgEnabled && <CreateOrgDialog open={createOpen} onClose={() => setCreateOpen(false)} />}
    </div>
  );
}

ContextSwitcher.propTypes = {
  context: PropTypes.oneOf(['personal', 'org', 'admin']).isRequired,
  collapsed: PropTypes.bool,
  onNavigate: PropTypes.func
};
