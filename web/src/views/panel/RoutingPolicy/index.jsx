import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { PanelRight, Plus, RotateCcw, Trash2 } from 'lucide-react';

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
import { ROW_OPENS, rowOpenProps, RowControl, useObjectRoute } from '@/components/objects';
import { showError, showSuccess } from 'utils/common';
import RoutingPolicyPanel from './RoutingPolicyPanel';
import NewRoutingPolicySheet from './NewRoutingPolicySheet';
import { POLICY_TAB_KEYS } from './policyTabs';
import {
  deleteRoutingPolicy,
  getRoutingPolicyStats,
  listCatalogModelNames,
  listRoutingPolicies,
  setRoutingPolicyEnabled
} from './routingPolicyApi';

// 自动路由策略列表：策略不多，一次取全量，不分页。点一行在右侧打开策略面板（?id=，Tab 各自保存），
// 新建用右侧面板（?new=1），只填必需项。
export default function RoutingPolicy() {
  const { t } = useTranslation();
  const route = useObjectRoute(POLICY_TAB_KEYS);
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modelOptions, setModelOptions] = useState([]);
  const [pendingDelete, setPendingDelete] = useState(null);
  // 近 7 天概览（请求数 / 兜底率），按策略逐个取；策略很少，并行几次请求可以接受。
  const [weekStats, setWeekStats] = useState({});

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const { success, message, data } = await listRoutingPolicies();
      if (success) {
        const list = data || [];
        setPolicies(list);
        const entries = await Promise.all(
          list.map(async (p) => {
            try {
              const res = await getRoutingPolicyStats(p.id, 7);
              return [p.id, res.success ? res.data : null];
            } catch (e) {
              return [p.id, null];
            }
          })
        );
        setWeekStats(Object.fromEntries(entries));
      } else showError(message);
    } catch (e) {
      // surfaced globally
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // 目录模型名只取一次，供决策 / 目标 / 兜底三处下拉建议。
  useEffect(() => {
    listCatalogModelNames()
      .then(setModelOptions)
      .catch(() => {});
  }, []);

  const toggleEnabled = async (policy, enabled) => {
    try {
      const { success, message } = await setRoutingPolicyEnabled(policy.id, enabled);
      if (success) {
        setPolicies((prev) => prev.map((p) => (p.id === policy.id ? { ...p, enabled } : p)));
        return true;
      }
      showError(message);
    } catch (e) {
      // surfaced globally
    }
    return false;
  };

  const confirmDelete = async () => {
    const policy = pendingDelete;
    setPendingDelete(null);
    if (!policy) return;
    try {
      const { success, message } = await deleteRoutingPolicy(policy.id);
      if (success) {
        showSuccess(t('userPage.operationSuccess'));
        if (policy.id === route.openId) route.close();
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
            <div className="flex max-w-xs flex-wrap gap-1">
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
        id: 'week',
        header: t('routingPolicy.columns.week'),
        cell: (c) => {
          const s = weekStats[c.row.original.id];
          if (!s) return <span className="text-muted-foreground">—</span>;
          if (!s.requests) return <span className="text-muted-foreground">0</span>;
          const rate = Math.round((s.fallback_requests / s.requests) * 100);
          return (
            <span className="tabular-nums">
              {s.requests} · {t('routingPolicy.columns.fallbackRate', { rate })}
            </span>
          );
        }
      },
      {
        accessorKey: 'enabled',
        header: t('routingPolicy.columns.enabled'),
        cell: (c) => {
          const p = c.row.original;
          return (
            <RowControl>
              <Switch
                checked={Boolean(p.enabled)}
                aria-label={t('routingPolicy.columns.enabled')}
                onCheckedChange={(v) => toggleEnabled(p, v)}
              />
            </RowControl>
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
            <RowControl>
              <RowActions
                actions={[
                  { label: t('objects.open'), icon: PanelRight, onClick: () => route.open(p.id) },
                  { label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => setPendingDelete(p) }
                ]}
              />
            </RowControl>
          );
        }
      }
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, weekStats, route.openId]
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
        <ResponsiveToolbarButton primary icon={Plus} label={t('routingPolicy.create')} onClick={route.openNew} />
      </PageActions>

      <Card>
        <div className="hidden overflow-x-auto md:block">
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
                  <TableRow
                    key={row.id}
                    className={ROW_OPENS}
                    data-state={row.original.id === route.openId ? 'selected' : undefined}
                    {...rowOpenProps(() => route.open(row.original.id))}
                  >
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
          onRowClick={(p) => route.open(p.id)}
        />
      </Card>

      {route.openId > 0 && (
        <RoutingPolicyPanel
          key={route.openId}
          policyId={route.openId}
          tab={route.tab}
          onTabChange={route.setTab}
          onClose={route.close}
          onChanged={refresh}
          onToggleEnabled={toggleEnabled}
          onDelete={setPendingDelete}
          modelOptions={modelOptions}
        />
      )}

      {route.isNew && (
        <NewRoutingPolicySheet
          modelOptions={modelOptions}
          onClose={route.close}
          onCreated={(policy) => {
            showSuccess(t('routingPolicy.new.created'));
            refresh();
            if (policy?.id) route.open(policy.id);
            else route.close();
          }}
        />
      )}

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
