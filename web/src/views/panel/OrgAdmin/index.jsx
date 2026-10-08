import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { RotateCcw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import PageActions from '@/components/chrome/PageActions';
import { FilterBar, filterStateToParams, paramsToFilterState } from '@/components/filter-bar';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { renderQuota, showError, trims } from 'utils/common';
import { PAGE_SIZE_OPTIONS } from 'constants';
import usePaginatedList from 'hooks/usePaginatedList';
import OrgAdminTable from './OrgAdminTable';

// FilterBar 字段:单个关键字文本(后端 keyword 单值,按组织名称/slug 模糊匹配)。
const ORG_FILTER_FIELDS = [{ key: 'keyword', labelKey: 'filterBar.keyword', type: 'text', placeholderKey: 'orgAdmin.searchPlaceholder' }];
const ORG_FILTER_PARAM_KEYS = ['keyword'];

// ==============================|| PANEL — ORG ADMIN (site admin) ||============================== //
// Site-level organization list (spec §3.8 / a8): list + enable/disable +
// pool quota adjustment + dissolve. Global org options moved to the Setting page
// (`/panel/setting/org`, root-only) per IA-2/IA-4 — this page is a pure list now.
// Backend is /api/organization/* (RootAuth); route is wrapped in AdminGuard like
// other admin pages and non-admins get the in-place 404.

export default function OrgAdmin() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(ORG_FILTER_FIELDS, searchParams));

  const [quotaTarget, setQuotaTarget] = useState(null);
  const [quotaDelta, setQuotaDelta] = useState('');
  const [quotaRemark, setQuotaRemark] = useState('');
  const [dissolveTarget, setDissolveTarget] = useState(null);
  const [submitting, setSubmitting] = useState(false);

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
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const openQuotaDialog = (org) => {
    setQuotaTarget(org);
    setQuotaDelta('');
    setQuotaRemark('');
  };

  const submitQuota = async () => {
    const delta = Number(quotaDelta);
    if (!Number.isInteger(delta) || delta === 0) {
      showError(t('orgAdmin.quotaNonZero'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.put(`/api/organization/${quotaTarget.id}/quota`, { quota: delta, remark: trims(quotaRemark) });
      if (res.data.success) {
        toast.success(t('common.operationSuccess'));
        setQuotaTarget(null);
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

  const confirmDissolve = async () => {
    if (!dissolveTarget) return;
    setSubmitting(true);
    try {
      const res = await API.delete(`/api/organization/${dissolveTarget.id}`);
      if (res.data.success) {
        toast.success(t('common.operationSuccess'));
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
            onToggleStatus={onToggleStatus}
            onAdjustQuota={openQuotaDialog}
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

        <Dialog open={!!quotaTarget} onOpenChange={(o) => !o && setQuotaTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('orgAdmin.quotaTitle', { name: quotaTarget?.name })}</DialogTitle>
              <DialogDescription>{t('orgAdmin.quotaDesc')}</DialogDescription>
            </DialogHeader>
            <div className="space-y-4">
              <QuotaInput
                id="org-quota-delta"
                label={t('orgAdmin.quotaLabel')}
                value={quotaDelta}
                onChange={setQuotaDelta}
                maxDeduct={quotaTarget?.quota}
                helperText={`${t('orgAdmin.columns.quota')}: ${renderQuota(quotaTarget?.quota || 0, 2)}`}
              />
              <div className="space-y-4">
                <Label htmlFor="org-quota-remark">{t('orgAdmin.remarkLabel')}</Label>
                <Textarea
                  id="org-quota-remark"
                  rows={2}
                  value={quotaRemark}
                  onChange={(e) => setQuotaRemark(e.target.value)}
                  placeholder={t('orgAdmin.remarkPlaceholder')}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setQuotaTarget(null)}>
                {t('token_index.cancel')}
              </Button>
              <Button onClick={submitQuota} disabled={submitting}>
                {t('common.submit')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

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
