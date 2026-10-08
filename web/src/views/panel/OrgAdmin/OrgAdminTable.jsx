import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Trash2, Wallet } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { RowActions, RowActionsSurfaceContext } from '@/components/ui/row-actions';
import { renderQuota, SpendAmount, timestamp2string } from 'utils/common';

// ==============================|| ORG ADMIN — ORGANIZATIONS TABLE ||============================== //
// Site-admin list of organizations (/api/organization/, RootAuth backend).
// Status 1=enabled 2=disabled mirrors model.OrganizationStatus*.

export default function OrgAdminTable({ rows, searching, onToggleStatus, onAdjustQuota, onDissolve }) {
  const { t } = useTranslation();

  const rowActions = (org) => [
    { label: t('orgAdmin.adjustQuota'), icon: Wallet, onClick: () => onAdjustQuota(org) },
    { label: t('orgAdmin.dissolve'), icon: Trash2, destructive: true, onClick: () => onDissolve(org) }
  ];

  return (
    <>
      <div className="hidden md:block">
        <Table className="min-w-[800px]">
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead>ID</TableHead>
              <TableHead>{t('orgAdmin.columns.name')}</TableHead>
              <TableHead>{t('orgAdmin.columns.owner')}</TableHead>
              <TableHead>{t('orgAdmin.columns.members')}</TableHead>
              <TableHead>{t('orgAdmin.columns.quota')}</TableHead>
              <TableHead>{t('orgAdmin.columns.usedQuota')}</TableHead>
              <TableHead>{t('orgAdmin.columns.status')}</TableHead>
              <TableHead>{t('orgAdmin.columns.createdTime')}</TableHead>
              <TableHead className="sticky right-0 z-10 w-[1%] bg-card text-right">{t('orgAdmin.columns.actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="h-24 text-center text-sm text-muted-foreground">
                  {searching ? t('logPage.searching') : t('dashboard_index.no_data', { defaultValue: 'No data' })}
                </TableCell>
              </TableRow>
            ) : (
              rows.map((org) => (
                <TableRow key={org.id}>
                  <TableCell className="font-mono tabular-nums">{org.id}</TableCell>
                  <TableCell>
                    <div className="flex flex-col">
                      <span className="font-medium">{org.name}</span>
                      <span className="text-xs text-muted-foreground">{org.slug}</span>
                    </div>
                  </TableCell>
                  <TableCell>{org.owner_id ? <Badge variant="outline">#{org.owner_id}</Badge> : '-'}</TableCell>
                  <TableCell className="font-mono tabular-nums">{org.member_count}</TableCell>
                  <TableCell className="font-mono tabular-nums">{renderQuota(org.quota, 2)}</TableCell>
                  <TableCell className="font-mono tabular-nums">
                    <SpendAmount quota={org.used_quota} />
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Switch checked={org.status === 1} onCheckedChange={() => onToggleStatus(org)} />
                      <Badge variant={org.status === 1 ? 'default' : 'secondary'}>
                        {org.status === 1 ? t('common.enable') : t('common.disable')}
                      </Badge>
                    </div>
                  </TableCell>
                  <TableCell className="whitespace-nowrap">{timestamp2string(org.created_time)}</TableCell>
                  <TableCell className="sticky right-0 z-10 bg-card text-right">
                    <RowActions actions={rowActions(org)} />
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {rows.length === 0 ? (
        <div className="md:hidden rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          {searching ? t('logPage.searching') : t('dashboard_index.no_data', { defaultValue: 'No data' })}
        </div>
      ) : (
        <div className="md:hidden flex flex-col gap-3">
          {rows.map((org) => (
            <div key={org.id} className="rounded-lg border border-border bg-card p-3 shadow-sm">
              <div className="mb-1.5">
                <div className="text-sm font-medium">{org.name}</div>
                <div className="text-xs text-muted-foreground">{org.slug}</div>
              </div>
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
                  <dd className="min-w-0 flex-1 truncate text-sm font-mono tabular-nums">{org.member_count}</dd>
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
      )}
    </>
  );
}

OrgAdminTable.propTypes = {
  rows: PropTypes.array,
  searching: PropTypes.bool,
  onToggleStatus: PropTypes.func.isRequired,
  onAdjustQuota: PropTypes.func.isRequired,
  onDissolve: PropTypes.func.isRequired
};
