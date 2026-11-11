import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Ban, Check, MoreHorizontal, PanelRight, Trash2, Users, Wallet } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
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
import { RowActions, RowActionsSurfaceContext } from '@/components/ui/row-actions';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';
import { useNaturalWidth } from '@/components/list-page';
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

// Row menu: open, adjust the pool balance, the member limit, enable / disable, then dissolve
// (destructive, last).
export function OrgActions({ org, onOpen, onToggleStatus, onDissolve }) {
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
        <DropdownMenuItem onClick={() => onToggleStatus(org)}>
          {org.status === ENABLED ? <Ban className="size-4" /> : <Check className="size-4" />}
          {org.status === ENABLED ? t('common.disable') : t('common.enable')}
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
  onToggleStatus: PropTypes.func.isRequired,
  onDissolve: PropTypes.func.isRequired
};

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
              <RowControl className="flex">
                <Switch
                  size="sm"
                  checked={org.status === ENABLED}
                  aria-label={t('orgAdmin.columns.status')}
                  onCheckedChange={() => onToggleStatus(org)}
                />
              </RowControl>
            </TableCell>
            <TableCell className="sticky right-0 z-10 bg-card text-right">
              <RowControl className="justify-end">
                <OrgActions org={org} onOpen={onOpen} onToggleStatus={onToggleStatus} onDissolve={onDissolve} />
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

// Phone cards: kept as they were until the shared phone-card layout lands.
export function OrgAdminCards({ rows, onOpen, onToggleStatus, onDissolve }) {
  const { t } = useTranslation();
  const rowActions = (org) => [
    { label: t('objects.open'), icon: PanelRight, onClick: () => onOpen(org.id) },
    { label: t('orgAdmin.adjustQuota'), icon: Wallet, onClick: () => onOpen(org.id, 'balance') },
    { label: t('orgAdmin.maxMembers'), icon: Users, onClick: () => onOpen(org.id, 'members') },
    { label: t('orgAdmin.dissolve'), icon: Trash2, destructive: true, onClick: () => onDissolve(org) }
  ];
  return (
    <div className="flex flex-col divide-y divide-border overflow-hidden rounded-lg border border-border bg-card">
      {rows.map((org) => (
        <div key={org.id} className="p-3">
          <button type="button" className="mb-1.5 block w-full text-left" onClick={() => onOpen(org.id)}>
            <div className="text-sm font-medium">{org.name}</div>
            <div className="text-xs text-muted-foreground">{org.slug}</div>
          </button>
          <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5">
            <div className="flex items-baseline gap-1.5">
              <dt className="shrink-0 text-xs font-medium text-muted-foreground">ID</dt>
              <dd className="min-w-0 flex-1 truncate text-sm font-mono tabular-nums">{org.id}</dd>
            </div>
            <div className="flex items-baseline gap-1.5">
              <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgAdmin.columns.owner')}</dt>
              <dd className="flex min-w-0 flex-1 flex-wrap gap-1 text-sm">
                {org.owner_id ? <Badge variant="outline">#{org.owner_id}</Badge> : '-'}
              </dd>
            </div>
            <div className="flex items-baseline gap-1.5">
              <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgAdmin.columns.members')}</dt>
              <dd className="min-w-0 flex-1 truncate text-sm">
                <MemberCount org={org} />
              </dd>
            </div>
            <div className="flex items-baseline gap-1.5">
              <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgAdmin.columns.quota')}</dt>
              <dd className="min-w-0 flex-1 truncate text-sm font-mono tabular-nums">{renderQuota(org.quota, 2)}</dd>
            </div>
            <div className="flex items-baseline gap-1.5">
              <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgAdmin.columns.usedQuota')}</dt>
              <dd className="min-w-0 flex-1 truncate text-sm font-mono tabular-nums">
                <SpendAmount quota={org.used_quota} />
              </dd>
            </div>
            <div className="flex items-center gap-1.5">
              <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgAdmin.columns.status')}</dt>
              <dd className="flex min-w-0 flex-1 items-center gap-2 text-sm">
                <Switch checked={org.status === 1} onCheckedChange={() => onToggleStatus(org)} />
                <Badge variant={org.status === 1 ? 'default' : 'secondary'}>
                  {org.status === 1 ? t('common.enable') : t('common.disable')}
                </Badge>
              </dd>
            </div>
            <div className="flex items-baseline gap-1.5">
              <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgAdmin.columns.createdTime')}</dt>
              <dd className="min-w-0 flex-1 truncate text-sm">{timestamp2string(org.created_time)}</dd>
            </div>
          </dl>
          <div className="mt-2 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-2">
            <RowActionsSurfaceContext.Provider value="card">
              <RowActions actions={rowActions(org)} />
            </RowActionsSurfaceContext.Provider>
          </div>
        </div>
      ))}
    </div>
  );
}

OrgAdminCards.propTypes = {
  rows: PropTypes.array.isRequired,
  onOpen: PropTypes.func.isRequired,
  onToggleStatus: PropTypes.func.isRequired,
  onDissolve: PropTypes.func.isRequired
};
