import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { RotateCcw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import PageActions from '@/components/chrome/PageActions';
import { FilterBar, filterStateToParams, paramsToFilterState } from '@/components/filter-bar';
import { useObjectRoute } from '@/components/objects';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { PAGE_SIZE_OPTIONS } from 'constants';
import usePaginatedList from 'hooks/usePaginatedList';
import OrgAdminTable from './OrgAdminTable';
import OrgPanel from './OrgPanel';
import { ORG_TAB_KEYS } from './orgTabs';

// FilterBar 字段:单个关键字文本(后端 keyword 单值,按组织名称/slug 模糊匹配)。
const ORG_FILTER_FIELDS = [{ key: 'keyword', labelKey: 'filterBar.keyword', type: 'text', placeholderKey: 'orgAdmin.searchPlaceholder' }];
const ORG_FILTER_PARAM_KEYS = ['keyword'];

// ==============================|| PANEL — ORG ADMIN (site admin) ||============================== //
// Site-level organization list (spec §3.8 / a8): list + enable/disable + dissolve; a row opens
// the organization's panel (?id=) with pool balance adjustment and the member limit. Global org options moved to the Setting page
// (`/panel/setting/orgpolicy`, root-only) per IA-2/IA-4 — this page is a pure list now.
// Backend is /api/organization/* (RootAuth); route is wrapped in AdminGuard like
// other admin pages and non-admins get the in-place 404.

export default function OrgAdmin() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(ORG_FILTER_FIELDS, searchParams));

  const route = useObjectRoute(ORG_TAB_KEYS);
  const [dissolveTarget, setDissolveTarget] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  // Bumped after a list action so an open organization panel reloads.
  const [reloadKey, setReloadKey] = useState(0);

  // 分页列表状态 + 守卫化 fetch 生命周期(UX-13);本页无排序,fetcher 忽略 order/orderBy。
  const { page, setPage, rowsPerPage, listCount, searching, rows, setRows, refresh, onRowsPerPageChange } = usePaginatedList({
    pageSizeKey: 'org_admin',
    fetcher: async ({ page, rowsPerPage }) => {
      const keyword = filterStateToParams(ORG_FILTER_FIELDS, filterState).keyword || '';
      const res = await API.get('/api/organization/', {
        params: { page: page + 1, size: rowsPerPage, keyword }
      });
      return res.data;
    },
    deps: [filterState]
  });

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        ORG_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(ORG_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        return params;
      },
      { replace: true }
    );
  }, [filterState, setSearchParams]);

  // 后端变更后原地重载(保留当前筛选),供额度调整/解散对话框复用。
  const handleRefresh = () => refresh();

  // 级联筛选受控回调:任何改动即时应用并回到第一页;清空同理。
  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };

  // 刷新/清除:清空筛选并回到第一页后重新拉取(对齐 Log 刷新语义)。
  const handleReset = () => {
    setPage(0);
    setFilterState({});
    refresh();
  };

  const onToggleStatus = async (org) => {
    const next = org.status === 1 ? 2 : 1;
    try {
      const res = await API.put(`/api/organization/${org.id}/status`, { status: next });
      if (res.data.success) {
        toast.success(t('common.operationSuccess'));
        setRows((prev) => prev.map((x) => (x.id === org.id ? { ...x, status: next } : x)));
        setReloadKey((k) => k + 1);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const confirmDissolve = async () => {
    if (!dissolveTarget) return;
    setSubmitting(true);
    try {
      const res = await API.delete(`/api/organization/${dissolveTarget.id}`);
      if (res.data.success) {
        toast.success(t('common.operationSuccess'));
        if (dissolveTarget.id === route.openId) route.close();
        setDissolveTarget(null);
        handleRefresh();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const totalPages = Math.max(1, Math.ceil(listCount / rowsPerPage));

  return (
    <>
      <div className="space-y-6">
        {/* 标题行右侧操作:刷新 → 筛选,对齐 Log 页 PageActions 结构(本页无时间维度)。 */}
        <PageActions>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="sm" onClick={handleReset} aria-label={t('logPage.refreshButton')}>
                  <RotateCcw className="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('logPage.refreshButton')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <FilterBar
            fields={ORG_FILTER_FIELDS}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            hideChips
            t={t}
          />
        </PageActions>

        <Card>
          <OrgAdminTable
            rows={rows}
            searching={searching}
            openId={route.openId}
            onOpen={route.open}
            onToggleStatus={onToggleStatus}
            onDissolve={setDissolveTarget}
          />

          <div className="flex items-center justify-between gap-4 border-t border-border px-4 py-3 text-sm">
            <span className="text-muted-foreground">{t('pagination.total', { count: listCount })}</span>
            <div className="flex items-center gap-2">
              <select
                className="h-9 rounded-lg border border-input bg-background px-2 text-sm"
                value={rowsPerPage}
                onChange={(e) => onRowsPerPageChange(parseInt(e.target.value, 10))}
              >
                {PAGE_SIZE_OPTIONS.map((n) => (
                  <option key={n} value={n}>
                    {t('pagination.perPage', { count: n })}
                  </option>
                ))}
              </select>
              <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>
                ‹
              </Button>
              <span className="tabular-nums">
                {page + 1} / {totalPages}
              </span>
              <Button variant="outline" size="sm" disabled={page + 1 >= totalPages} onClick={() => setPage((p) => p + 1)}>
                ›
              </Button>
            </div>
          </div>
        </Card>

        {route.openId > 0 && (
          <OrgPanel
            key={route.openId}
            orgId={route.openId}
            tab={route.tab}
            onTabChange={route.setTab}
            onClose={route.close}
            onChanged={handleRefresh}
            onToggleStatus={onToggleStatus}
            onDissolve={setDissolveTarget}
            reloadKey={reloadKey}
          />
        )}

        <Dialog open={!!dissolveTarget} onOpenChange={(o) => !o && setDissolveTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('orgAdmin.dissolve')}</DialogTitle>
              <DialogDescription>{t('orgAdmin.dissolveConfirm', { name: dissolveTarget?.name })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDissolveTarget(null)}>
                {t('token_index.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmDissolve} disabled={submitting}>
                {t('orgAdmin.dissolve')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </>
  );
}
