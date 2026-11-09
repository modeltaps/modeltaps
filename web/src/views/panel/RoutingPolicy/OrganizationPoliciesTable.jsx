import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { RotateCcw } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import PageActions from '@/components/chrome/PageActions';
import { showError } from 'utils/common';
import { platformRoutingPolicyApi } from './routingPolicyApi';

// Read-only review of every organization's routing policies for system admins. Organizations edit
// their own policies in their settings; here an admin can only switch one off (for abuse).
export default function OrganizationPoliciesTable() {
  const { t } = useTranslation();
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const { success, message, data } = await platformRoutingPolicyApi.list({ organization_id: -1 });
      if (success) setPolicies(data || []);
      else showError(message);
    } catch (e) {
      // surfaced globally
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const toggleEnabled = async (policy, enabled) => {
    try {
      const { success, message } = await platformRoutingPolicyApi.setEnabled(policy.id, enabled);
      if (success) setPolicies((prev) => prev.map((p) => (p.id === policy.id ? { ...p, enabled } : p)));
      else showError(message);
    } catch (e) {
      // surfaced globally
    }
  };

  const columns = useMemo(
    () => [
      {
        accessorKey: 'organization_name',
        header: t('routingPolicy.columns.organization'),
        cell: (c) => c.getValue() || `#${c.row.original.organization_id}`
      },
      {
        accessorKey: 'name',
        header: t('routingPolicy.columns.name'),
        cell: (c) => {
          const p = c.row.original;
          return (
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="font-mono text-sm">{p.name}</span>
              {p.overrides_platform && (
                <Badge variant="secondary" className="font-normal">
                  {t('routingPolicy.overridesPlatform')}
                </Badge>
              )}
            </div>
          );
        }
      },
      {
        accessorKey: 'decision_model',
        header: t('routingPolicy.columns.decisionModel'),
        cell: (c) => <span className="font-mono text-sm">{c.getValue()}</span>
      },
      {
        id: 'routes',
        header: t('routingPolicy.columns.routes'),
        cell: (c) => (
          <div className="flex max-w-xs flex-wrap gap-1">
            {(c.row.original.routes || []).map((r) => (
              <Badge key={r.label} variant="outline" className="font-normal">
                {r.label} → {r.target_model}
              </Badge>
            ))}
          </div>
        )
      },
      {
        accessorKey: 'fallback_model',
        header: t('routingPolicy.columns.fallback'),
        cell: (c) => <span className="font-mono text-sm">{c.getValue()}</span>
      },
      {
        accessorKey: 'enabled',
        header: t('routingPolicy.columns.enabled'),
        cell: (c) => {
          const p = c.row.original;
          return (
            <Switch
              checked={Boolean(p.enabled)}
              aria-label={t('routingPolicy.columns.enabled')}
              onCheckedChange={(v) => toggleEnabled(p, v)}
            />
          );
        }
      }
    ],
    [t]
  );

  const table = useTable({ features: appTableFeatures, data: policies, columns });

  return (
    <div className="space-y-4">
      <PageActions>
        <TooltipProvider delayDuration={150}>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" size="sm" aria-label={t('userPage.refresh')} onClick={refresh}>
                <RotateCcw className={loading ? 'size-4 animate-spin' : 'size-4'} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t('userPage.refresh')}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </PageActions>
      <p className="text-sm text-muted-foreground">{t('routingPolicy.organizationsIntro')}</p>
      <Card>
        <div className="hidden overflow-x-auto md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id} className="hover:bg-transparent">
                  {hg.headers.map((h) => (
                    <TableHead key={h.id} className="whitespace-nowrap">
                      {flexRender(h.column.columnDef.header, h.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={columns.length} className="py-10 text-center text-sm text-muted-foreground">
                    {loading ? t('common.loading', { defaultValue: 'Loading…' }) : t('routingPolicy.organizationsEmpty')}
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext())}</TableCell>
                    ))}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <DataCards
          table={table}
          className="md:hidden"
          primaryColumnId="name"
          empty={t('routingPolicy.organizationsEmpty')}
          searching={loading}
        />
      </Card>
    </div>
  );
}
