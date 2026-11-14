import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { useOrg } from 'contexts/OrgContext';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { MoreHorizontal, PanelRight, Plus, RotateCcw, Route, Trash2, TriangleAlert } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { RowActions } from '@/components/ui/row-actions';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { ROW_OPENS, rowOpenProps, RowControl, Segmented, useObjectRoute } from '@/components/objects';
import { PageEmptyState } from '@/components/list-page';
import { showError, showSuccess } from 'utils/common';
import RoutingPolicyPanel from './RoutingPolicyPanel';
import NewRoutingPolicySheet from './NewRoutingPolicySheet';
import { POLICY_TAB_KEYS } from './policyTabs';
import PolicyCallName from './PolicyCallName';
import { organizationReviewApi, personalReviewApi, platformRoutingPolicyApi, policyCallName } from './routingPolicyApi';

const SCOPE_APIS = { platform: platformRoutingPolicyApi, organization: organizationReviewApi, personal: personalReviewApi };
const SCOPE_INTROS = { organization: 'routingPolicy.organizationsIntro', personal: 'routingPolicy.personalIntro' };

// Platform console: the platform's own policies, or (范围 = 组织 / 个人) every organization's or
// user's policies for review. The scope is kept in the URL (?scope=organization).
export default function RoutingPolicy() {
  const { t } = useTranslation();
  const { orgEnabled: organizationsEnabled } = useOrg();
  const [searchParams, setSearchParams] = useSearchParams();
  const scopes = organizationsEnabled ? ['platform', 'organization', 'personal'] : ['platform', 'personal'];
  const requested = searchParams.get('scope');
  const scope = scopes.includes(requested) ? requested : 'platform';
  // Switching scope closes the open panel: ids are not shared between the lists.
  const setScope = (next) => setSearchParams(next === 'platform' ? {} : { scope: next });
  const filter = (
    <Segmented
      label={t('routingPolicy.scope.label')}
      value={scope}
      onChange={setScope}
      options={scopes.map((value) => ({ value, label: t(`routingPolicy.scope.${value}`) }))}
    />
  );
  return (
    <RoutingPolicyList key={scope} api={SCOPE_APIS[scope]} filter={filter} intro={SCOPE_INTROS[scope] ? t(SCOPE_INTROS[scope]) : null} />
  );
}

// 自动路由策略列表：策略不多，一次取全量，不分页。点一行在右侧打开策略面板（?id=，Tab 各自保存），
// 新建用右侧面板（?new=1），只填必需项。Shared by the platform console and the workbench's 路由策略
// page (personal and organization policies): `api` is the scoped client (routingPolicyApi.js). With a
// review client the list shows other scopes' policies, read-only except the enable switch; with a
// readOnly client (organization members) nothing can be changed.
export function RoutingPolicyList({ api, filter = null, intro = null, introDetail = null }) {
  const { t } = useTranslation();
  const review = Boolean(api.review);
  const readOnly = Boolean(api.readOnly);
  const canEdit = !review && !readOnly;
  const route = useObjectRoute(POLICY_TAB_KEYS);
  const [policies, setPolicies] = useState([]);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [modelOptions, setModelOptions] = useState([]);
  const [pendingDelete, setPendingDelete] = useState(null);
  // 近 7 天概览（请求数 / 兜底率），按策略逐个取；策略很少，并行几次请求可以接受。
  const [weekStats, setWeekStats] = useState({});

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const { success, message, data } = await api.list();
      if (success) {
        const list = data || [];
        setPolicies(list);
        // Members cannot read the statistics.
        if (readOnly) return;
        const entries = await Promise.all(
          list.map(async (p) => {
            try {
              const res = await api.getStats(p.id, 7);
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
      setLoaded(true);
    }
  }, [api, readOnly]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // 目录模型名只取一次，供决策 / 目标 / 兜底三处下拉建议。
  useEffect(() => {
    if (!canEdit) return;
    api
      .listModels()
      .then(setModelOptions)
      .catch(() => {});
  }, [api, canEdit]);

  const toggleEnabled = async (policy, enabled) => {
    try {
      const { success, message } = await api.setEnabled(policy.id, enabled);
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
      const { success, message } = await api.remove(policy.id);
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
      ...(review
        ? [
            {
              id: 'owner',
              header: t('routingPolicy.columns.owner'),
              cell: (c) => {
                const p = c.row.original;
                return p.user_id ? p.username || `#${p.user_id}` : p.organization_name || `#${p.organization_id}`;
              }
            }
          ]
        : []),
      {
        accessorKey: 'name',
        header: t('routingPolicy.columns.name'),
        cell: (c) => {
          const p = c.row.original;
          return (
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                {api.callPrefix ? (
                  <PolicyCallName name={policyCallName(api, p)} hoverCopy />
                ) : (
                  <span className="font-mono text-sm">{p.name}</span>
                )}
                {p.overrides_platform && (
                  <Badge
                    variant="outline"
                    className="gap-1 border-amber-500/40 font-normal"
                    title={t('routingPolicy.overridesPlatformHint')}
                  >
                    <TriangleAlert className="size-3 text-amber-500" />
                    {t('routingPolicy.overridesPlatform')}
                  </Badge>
                )}
              </div>
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
      ...(readOnly
        ? []
        : [
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
            }
          ]),
      {
        accessorKey: 'enabled',
        header: t('routingPolicy.columns.enabled'),
        // On phone cards this switch sits next to the row menu (DataCards switchColumnId).
        cell: (c) => {
          const p = c.row.original;
          return (
            <RowControl className="flex">
              <Switch
                size="sm"
                checked={Boolean(p.enabled)}
                disabled={readOnly}
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
                  ...(canEdit ? [{ label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => setPendingDelete(p) }] : [])
                ]}
              />
            </RowControl>
          );
        }
      }
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, weekStats, route.openId, review, readOnly, canEdit, api]
  );

  const openPolicy = policies.find((p) => p.id === route.openId);

  const table = useTable({ features: appTableFeatures, data: policies, columns });
  const emptyText = review ? t('routingPolicy.reviewEmpty') : t('routingPolicy.empty');
  // No policy in this scope yet: the list becomes the first-run empty state (the scope switch stays).
  const blank = loaded && !loading && policies.length === 0;
  let blankDescription = t('pageEmpty.routing.description');
  if (review) blankDescription = t('routingPolicy.reviewEmpty');
  else if (readOnly) blankDescription = t('pageEmpty.routing.memberDescription');

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
        {canEdit && <ResponsiveToolbarButton primary icon={Plus} label={t('routingPolicy.create')} onClick={route.openNew} />}
      </PageActions>

      {(filter || intro) && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          {filter}
          {/* One line; the full explanation (overrides, who pays for 试一试) is the tooltip. */}
          {intro && (
            <p className="text-sm text-muted-foreground" title={introDetail || undefined}>
              {intro}
            </p>
          )}
        </div>
      )}

      {blank ? (
        <PageEmptyState
          icon={Route}
          title={t('pageEmpty.routing.title')}
          description={blankDescription}
          docsPath="/guide/routing"
          action={canEdit && <Button onClick={route.openNew}>{t('routingPolicy.create')}</Button>}
        />
      ) : (
        <>
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
                        {loading ? t('common.loading', { defaultValue: 'Loading…' }) : emptyText}
                      </TableCell>
                    </TableRow>
                  ) : (
                    table.getRowModel().rows.map((row) => (
                      <TableRow
                        key={row.id}
                        className={`group ${ROW_OPENS}`}
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
            {/* Phone rows (56px): the call name with the switch and ⋯ on the right; below, the display
              name (or owner when reviewing), the routes as label → target and the fallback. */}
            <div className="md:hidden">
              {table.getRowModel().rows.length === 0 ? (
                <p className="px-4 py-8 text-center text-sm text-muted-foreground">
                  {loading ? t('common.loading', { defaultValue: 'Loading…' }) : emptyText}
                </p>
              ) : (
                <ul className="divide-y divide-border">
                  {table.getRowModel().rows.map((row) => {
                    const p = row.original;
                    const cell = (id) => {
                      const c = row.getVisibleCells().find((x) => x.column.id === id);
                      return c ? flexRender(c.column.columnDef.cell, c.getContext()) : null;
                    };
                    const owner = review
                      ? p.user_id
                        ? p.username || `#${p.user_id}`
                        : p.organization_name || `#${p.organization_id}`
                      : null;
                    const secondary = [
                      owner || (p.display_name && p.display_name !== p.name ? p.display_name : null),
                      (p.routes || []).map((r) => `${r.label} → ${r.target_model}`).join(' · '),
                      `${t('routingPolicy.columns.fallback')} ${p.fallback_model}`
                    ]
                      .filter(Boolean)
                      .join(' · ');
                    return (
                      <li
                        key={row.id}
                        className={`flex h-14 items-center gap-3 px-4 ${ROW_OPENS} ${p.id === route.openId ? 'bg-muted/60' : ''}`}
                        {...rowOpenProps(() => route.open(p.id))}
                      >
                        <div className="min-w-0 flex-1">
                          <div className="flex min-w-0 items-center gap-1.5">
                            <span className="truncate font-mono text-sm font-medium">
                              {api.callPrefix ? policyCallName(api, p) : p.name}
                            </span>
                            {p.overrides_platform && (
                              <TriangleAlert
                                className="size-3.5 shrink-0 text-amber-500"
                                aria-label={t('routingPolicy.overridesPlatform')}
                              />
                            )}
                          </div>
                          <div className="mt-0.5 truncate text-xs text-muted-foreground">{secondary}</div>
                        </div>
                        <div className="-mr-2 flex shrink-0 items-center gap-1">
                          {cell('enabled')}
                          <RowControl>
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="icon" className="size-8" aria-label={t('userPage.action')}>
                                  <MoreHorizontal className="size-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end" className="w-40">
                                <DropdownMenuItem onClick={() => route.open(p.id)}>
                                  <PanelRight className="size-4" />
                                  {t('objects.open')}
                                </DropdownMenuItem>
                                {canEdit && (
                                  <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setPendingDelete(p)}>
                                    <Trash2 className="size-4" />
                                    {t('common.delete')}
                                  </DropdownMenuItem>
                                )}
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </RowControl>
                        </div>
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          </Card>
        </>
      )}
      {route.openId > 0 && (
        <RoutingPolicyPanel
          key={route.openId}
          api={api}
          listed={openPolicy}
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

      {route.isNew && canEdit && (
        <NewRoutingPolicySheet
          api={api}
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

RoutingPolicyList.propTypes = {
  api: PropTypes.object.isRequired,
  filter: PropTypes.node,
  intro: PropTypes.node,
  introDetail: PropTypes.string
};
