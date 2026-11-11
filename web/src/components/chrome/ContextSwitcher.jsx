import { useState } from 'react';
import PropTypes from 'prop-types';
import { useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Building2, Check, ChevronsUpDown, Plus, ShieldUser, User } from 'lucide-react';

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
// this row names it and the popover leads back to personal or an organization. Markers are rounded
// squares, deliberately unlike the round avatar: fixed icons for personal and platform admin (not
// customizable, so they never repeat the user's avatar), and the organization's uploaded logo or a
// default building icon.

const CONTEXT_ICONS = { personal: User, admin: ShieldUser, org: Building2 };

export function ContextMark({ kind, org = null, className }) {
  if (kind === 'org' && org?.avatar_url) {
    return (
      <img
        src={org.avatar_url}
        alt=""
        aria-hidden="true"
        className={cn('shrink-0 rounded-md border border-border object-cover', className)}
      />
    );
  }
  const Icon = CONTEXT_ICONS[kind] || User;
  return (
    <span
      aria-hidden="true"
      className={cn('flex shrink-0 items-center justify-center rounded-md border border-border bg-card text-muted-foreground', className)}
    >
      <Icon className="size-[62%]" strokeWidth={1.75} />
    </span>
  );
}

ContextMark.propTypes = {
  kind: PropTypes.oneOf(['personal', 'org', 'admin']).isRequired,
  org: PropTypes.object,
  className: PropTypes.string
};

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
      ? { kind: 'admin', name: t('nav.context.admin') }
      : context === 'org' && currentOrg
        ? { kind: 'org', org: currentOrg, name: currentOrg.name }
        : { kind: 'personal', name: t('nav.context.personal') };

  const select = (to) => {
    const target = contextSwitchTarget({ pathname, to, isRoot, features: { orgRoutingPolicy: orgRoutingPolicyEnabled } });
    if (to.kind === 'personal') switchOrg(null);
    if (to.kind === 'org') switchOrg(to.orgId);
    if (target) navigate(target);
    onNavigate?.();
  };

  const hasMenu = orgEnabled || context === 'admin';
  const markSize = collapsed ? 'size-7' : 'size-6';
  const triggerClass = collapsed
    ? 'flex size-9 items-center justify-center rounded-lg transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'
    : 'flex h-9 w-full items-center gap-2.5 rounded-lg px-2 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

  const body = (
    <>
      <ContextMark kind={current.kind} org={current.org} className={markSize} />
      {!collapsed && <span className="min-w-0 flex-1 truncate text-sm font-medium text-foreground">{current.name}</span>}
      {!collapsed && hasMenu && <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />}
    </>
  );

  if (!hasMenu) {
    return <div className={cn(triggerClass, 'hover:bg-transparent')}>{body}</div>;
  }

  const check = <Check className="size-4 shrink-0" />;
  const rowMark = 'size-[22px]';

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
            <ContextMark kind="personal" className={rowMark} />
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
                    <ContextMark kind="org" org={org} className={rowMark} />
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
