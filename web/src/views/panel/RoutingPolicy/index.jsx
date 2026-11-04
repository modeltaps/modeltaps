import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { Pencil, Plus, RotateCcw, Trash2 } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { RowActions } from '@/components/ui/row-actions';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { showError, showSuccess } from 'utils/common';
import RoutingPolicySheet from './RoutingPolicySheet';
import { deleteRoutingPolicy, listRoutingPolicies, setRoutingPolicyEnabled } from './routingPolicyApi';

// 自动路由策略列表：策略不多，一次取全量，不分页。
export default function RoutingPolicy() {
  const { t } = useTranslation();
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editId, setEditId] = useState(0);
  const [pendingDelete, setPendingDelete] = useState(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const { success, message, data } = await listRoutingPolicies();
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

  const openEdit = (id) => {
    setEditId(id);
    setSheetOpen(true);
  };

  const toggleEnabled = async (policy, enabled) => {
    try {
      const { success, message } = await setRoutingPolicyEnabled(policy.id, enabled);
      if (success) {
        setPolicies((prev) => prev.map((p) => (p.id === policy.id ? { ...p, enabled } : p)));
      } else {
        showError(message);
      }
    } catch (e) {
      // surfaced globally
    }
  };

  const confirmDelete = async () => {
    const policy = pendingDelete;
    setPendingDelete(null);
    if (!policy) return;
    try {
      const { success, message } = await deleteRoutingPolicy(policy.id);
      if (success) {
        showSuccess(t('userPage.operationSuccess'));
        refresh();
      } else {
        showError(message);
      }
    } catch (e) {
      // surfaced globally
    }
  };

  const columns = useMemo(
    () => [
      {
        accessorKey: 'name',
        header: t('routingPolicy.columns.name'),
        cell: (c) => {
          const p = c.row.original;
          return (
            <div className="min-w-0">
              <div className="font-mono text-sm">{p.name}</div>
              {p.display_name && p.display_name !== p.name && (
                <div className="truncate text-xs text-muted-foreground">{p.display_name}</div>
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
        cell: (c) => {
          const routes = c.row.original.routes || [];
          return (
            <div className="flex max-w-md flex-wrap gap-1">
              {routes.map((r) => (
                <Badge key={r.label} variant="outline" className="font-normal">
                  {r.label} → {r.target_model}
                </Badge>
              ))}
            </div>
          );
        }
      },
      {
        accessorKey: 'fallback_model',
        header: t('routingPolicy.columns.fallback'),
        cell: (c) => <span className="font-mono text-sm">{c.getValue()}</span>
      },
      {
        accessorKey: 'threshold',
        header: t('routingPolicy.columns.threshold'),
        cell: (c) => `${Math.round(Number(c.getValue() || 0) * 100)}%`
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
      },
      {
        id: 'action',
        header: t('userPage.action'),
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => {
          const p = c.row.original;
          return (
            <RowActions
              actions={[
                { label: t('common.edit'), icon: Pencil, onClick: () => openEdit(p.id) },
                { label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => setPendingDelete(p) }
              ]}
            />
          );
        }
      }
    ],

    [t]
  );

  const table = useTable({ features: appTableFeatures, data: policies, columns });

  return (
    <div className="space-y-6">
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
        <ResponsiveToolbarButton primary icon={Plus} label={t('routingPolicy.create')} onClick={() => openEdit(0)} />
      </PageActions>

      <Card>
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id} className="hover:bg-transparent">
                  {hg.headers.map((h) => (
                    <TableHead key={h.id} className={`whitespace-nowrap ${h.column.columnDef.meta?.className ?? ''}`}>
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
                    {loading ? t('common.loading', { defaultValue: 'Loading…' }) : t('routingPolicy.empty')}
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
                        {flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext())}
                      </TableCell>
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
          actionColumnIds={['action']}
          empty={t('routingPolicy.empty')}
          searching={loading}
        />
      </Card>

      <RoutingPolicySheet
        open={sheetOpen}
        policyId={editId}
        onClose={() => setSheetOpen(false)}
        onSaved={() => {
          setSheetOpen(false);
          refresh();
        }}
      />

      <Dialog open={Boolean(pendingDelete)} onOpenChange={(o) => !o && setPendingDelete(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <p className="text-sm text-muted-foreground">{t('common.deleteConfirm', { title: pendingDelete?.name })}</p>
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)}>
              {t('userPage.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmDelete}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
