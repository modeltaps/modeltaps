import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { MoreHorizontal, PanelRight, Trash2, Users, Wallet } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { useNaturalWidth } from '@/components/list-page';
import { cn } from '@/lib/utils';
import { renderQuota, SpendAmount, timestamp2string } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';

// ==============================|| ORG ADMIN — ORGANIZATIONS TABLE ||============================== //
// Site-admin list of organizations (/api/organization/, RootAuth backend).
// Status 1=enabled 2=disabled mirrors model.OrganizationStatus*.

const ENABLED = 1;

// Typical table width before it has been measured.
export const ORG_TABLE_ESTIMATE = 820;
// MemberCount 成员数 / 生效上限:max_members 为 null 时跟随站点默认(标注「默认」),0 为不限制。
function MemberCount({ org }) {
  const { t } = useTranslation();
  const limit = org.effective_max_members > 0 ? org.effective_max_members : '∞';
  return (
    <span className="font-mono tabular-nums">
      {org.member_count} / {limit}
      {org.max_members == null && (
        <span className="ml-1.5 font-sans text-xs text-muted-foreground">{t('orgAdmin.maxMembersDefaultTag')}</span>
      )}
    </span>
  );
}

MemberCount.propTypes = { org: PropTypes.object.isRequired };

// The owner by username; the id when the name is missing.
export function ownerLabel(org) {
  if (org.owner_name) return org.owner_name;
  return org.owner_id ? `#${org.owner_id}` : '—';
}

// Row menu: open, adjust the pool balance, the member limit, then dissolve (destructive, last).
// Enable / disable is the switch next to it.
export function OrgActions({ org, onOpen, onDissolve }) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t('orgAdmin.columns.actions')}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuItem onClick={() => onOpen(org.id)}>
          <PanelRight className="size-4" />
          {t('objects.open')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onOpen(org.id, 'balance')}>
          <Wallet className="size-4" />
          {t('orgAdmin.adjustQuota')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onOpen(org.id, 'members')}>
          <Users className="size-4" />
          {t('orgAdmin.maxMembers')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDissolve(org)}>
          <Trash2 className="size-4" />
          {t('orgAdmin.dissolve')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

OrgActions.propTypes = {
  org: PropTypes.object.isRequired,
  onOpen: PropTypes.func.isRequired,
  onDissolve: PropTypes.func.isRequired
};

function OrgSwitch({ org, onToggleStatus }) {
  const { t } = useTranslation();
  return (
    <RowControl className="flex">
      <Switch
        size="sm"
        checked={org.status === ENABLED}
        aria-label={t('orgAdmin.columns.status')}
        onCheckedChange={() => onToggleStatus(org)}
      />
    </RowControl>
  );
}

OrgSwitch.propTypes = { org: PropTypes.object.isRequired, onToggleStatus: PropTypes.func.isRequired };

// Organizations table: one line per organization (name, slug in mono), owner, members, pool
// balance and spend right-aligned, age in words, then the status switch and the row menu last.
export default function OrgAdminTable({ rows, rtf, openId, onOpen, onToggleStatus, onDissolve, onNaturalWidth }) {
  const { t } = useTranslation();
  const tableRef = useNaturalWidth(onNaturalWidth);
  return (
    <Table ref={tableRef}>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="whitespace-nowrap">{t('orgAdmin.columns.name')}</TableHead>
          <TableHead className="whitespace-nowrap">{t('orgAdmin.columns.owner')}</TableHead>
          <TableHead className="whitespace-nowrap text-right">{t('orgAdmin.columns.members')}</TableHead>
          <TableHead className="whitespace-nowrap text-right">{t('orgAdmin.columns.quota')}</TableHead>
          <TableHead className="whitespace-nowrap text-right">{t('orgAdmin.columns.usedQuota')}</TableHead>
          <TableHead className="whitespace-nowrap">{t('orgAdmin.columns.createdTime')}</TableHead>
          <TableHead className="whitespace-nowrap">{t('orgAdmin.columns.status')}</TableHead>
          <TableHead className="sticky right-0 z-10 w-[1%] bg-card">
            <span className="sr-only">{t('orgAdmin.columns.actions')}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((org) => (
          <TableRow
            key={org.id}
            className={ROW_OPENS}
            data-state={org.id === openId ? 'selected' : undefined}
            {...rowOpenProps(() => onOpen(org.id))}
          >
            <TableCell className="max-w-80 whitespace-nowrap">
              <span className="flex min-w-0 items-center gap-2">
                <span className={org.status === ENABLED ? 'truncate font-medium' : 'truncate font-medium text-muted-foreground'}>
                  {org.name}
                </span>
                {org.slug && <code className="shrink-0 font-mono text-xs text-muted-foreground">{org.slug}</code>}
              </span>
            </TableCell>
            <TableCell className="whitespace-nowrap">{ownerLabel(org)}</TableCell>
            <TableCell className="whitespace-nowrap text-right">
              <MemberCount org={org} />
            </TableCell>
            <TableCell className="whitespace-nowrap text-right font-mono tabular-nums">{renderQuota(org.quota, 2)}</TableCell>
            <TableCell className="whitespace-nowrap text-right font-mono tabular-nums">
              <SpendAmount quota={org.used_quota} />
            </TableCell>
            <TableCell className="whitespace-nowrap">
              <span title={timestamp2string(org.created_time)}>{relativeTime(rtf, org.created_time)}</span>
            </TableCell>
            <TableCell>
              <OrgSwitch org={org} onToggleStatus={onToggleStatus} />
            </TableCell>
            <TableCell className="sticky right-0 z-10 bg-card text-right">
              <RowControl className="justify-end">
                <OrgActions org={org} onOpen={onOpen} onDissolve={onDissolve} />
              </RowControl>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

OrgAdminTable.propTypes = {
  rows: PropTypes.array.isRequired,
  rtf: PropTypes.object.isRequired,
  openId: PropTypes.number,
  onOpen: PropTypes.func.isRequired,
  onToggleStatus: PropTypes.func.isRequired,
  onDissolve: PropTypes.func.isRequired,
  onNaturalWidth: PropTypes.func
};

// Phone cards, one per organization (same layout as the other phone cards). Top: name and slug,
// then the switch and the row menu on the right. Middle: the pool balance, large. Bottom:
// members, owner and age.
export function OrgAdminCards({ rows, rtf, openId, onOpen, onToggleStatus, onDissolve }) {
  return (
    <ul className="space-y-2">
      {rows.map((org) => {
        const enabled = org.status === ENABLED;
        return (
          <li
            key={org.id}
            className={cn(
              'rounded-xl border border-border bg-card px-4 py-3 transition-colors',
              ROW_OPENS,
              org.id === openId && 'border-primary/40 bg-muted/40'
            )}
            {...rowOpenProps(() => onOpen(org.id))}
          >
            <div className="flex items-center gap-3">
              <div className="flex min-w-0 flex-1 items-baseline gap-2">
                <span className={cn('truncate text-sm font-medium', !enabled && 'text-muted-foreground')}>{org.name}</span>
                {org.slug && <code className="shrink-0 font-mono text-xs text-muted-foreground">{org.slug}</code>}
              </div>
              <div className="-my-1.5 -mr-2 flex shrink-0 items-center gap-1">
                <OrgSwitch org={org} onToggleStatus={onToggleStatus} />
                <RowControl>
                  <OrgActions org={org} onOpen={onOpen} onDissolve={onDissolve} />
                </RowControl>
              </div>
            </div>
            <div className={cn('mt-1 font-mono text-xl font-medium tabular-nums', !enabled && 'text-muted-foreground')}>
              {renderQuota(org.quota, 2)}
            </div>
            <div className="mt-1 truncate text-xs text-muted-foreground">
              <MemberCount org={org} /> · {ownerLabel(org)} · {relativeTime(rtf, org.created_time)}
            </div>
          </li>
        );
      })}
    </ul>
  );
}

OrgAdminCards.propTypes = {
  rows: PropTypes.array.isRequired,
  rtf: PropTypes.object.isRequired,
  openId: PropTypes.number,
  onOpen: PropTypes.func.isRequired,
  onToggleStatus: PropTypes.func.isRequired,
  onDissolve: PropTypes.func.isRequired
};
