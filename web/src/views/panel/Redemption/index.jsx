import { useState, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Plus, RotateCcw, Ticket, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import { FilterBar, filterStateToParams, paramsToFilterState, isEntryActive } from '@/components/filter-bar';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { API } from 'utils/api';
import { showError, downloadTextAsFile } from 'utils/common';
import { PAGE_SIZE_OPTIONS } from 'constants';
import usePaginatedList from 'hooks/usePaginatedList';
import RedemptionTable from './RedemptionTable';
import RedemptionSheet from './RedemptionSheet';

// FilterBar 字段:单个名称文本 chip(后端 keyword 单值,按兑换码名称模糊匹配)。
const REDEMPTION_FILTER_FIELDS = [
  { key: 'keyword', labelKey: 'redemptionPage.headLabels.name', type: 'text', placeholderKey: 'redemptionPage.searchPlaceholder' }
];
const REDEMPTION_FILTER_PARAM_KEYS = ['keyword'];

export default function Redemption() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(REDEMPTION_FILTER_FIELDS, searchParams));

  const [sheetOpen, setSheetOpen] = useState(false);
  const [editId, setEditId] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [selectedIds, setSelectedIds] = useState([]);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);

  // 分页列表状态 + 守卫化 fetch 生命周期(UX-13);fetcher 保留原有参数构造逻辑。
  const {
    page,
    setPage,
    rowsPerPage,
    order,
    orderBy,
    listCount,
    searching,
    rows: redemptions,
    setRows: setRedemptions,
    handleRefresh,
    handleSort,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'redemption',
    fetcher: async ({ page, rowsPerPage, order, orderBy }) => {
      const ob = order === 'desc' ? '-' + orderBy : orderBy;
      const params = { page: page + 1, size: rowsPerPage, order: ob, ...filterStateToParams(REDEMPTION_FILTER_FIELDS, filterState) };
      const res = await API.get('/api/redemption/', { params });
      return res.data;
    },
    deps: [filterState]
  });

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        REDEMPTION_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(REDEMPTION_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        return params;
      },
      { replace: true }
    );
  }, [filterState, setSearchParams]);

  // 级联筛选受控回调:任何改动即时应用并回到第一页;清空同理。
  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };

  // 已生效的筛选字段(仅用于判断是否渲染标题行下方的 chips 容器,对齐 Log 页)。
  const activeChipFields = useMemo(() => REDEMPTION_FILTER_FIELDS.filter((f) => isEntryActive(f, filterState[f.key])), [filterState]);

  useEffect(() => {
    setSelectedIds([]);
  }, [redemptions]);

  const manageRedemption = async (id, action, value) => {
    try {
      let res;
      if (action === 'delete') res = await API.delete('/api/redemption/' + id);
      else res = await API.put('/api/redemption/?status_only=true', { id, status: value });
      const { success } = res.data;
      if (success) {
        toast.success(t('redemptionPage.successMessage'));
        if (action === 'delete') handleRefresh();
      }
      return res.data;
    } catch (error) {
      showError(error);
    }
  };

  const onToggleStatus = async (item) => {
    const next = item.status === 1 ? 2 : 1;
    const r = await manageRedemption(item.id, 'status', next);
    if (r?.success) setRedemptions((prev) => prev.map((x) => (x.id === item.id ? { ...x, status: next } : x)));
  };

  const onCopyKey = (item) => {
    try {
      navigator.clipboard.writeText(item.key);
      toast.success(`${t('token_index.copy')} ✓`);
    } catch (e) {
      toast.error(item.key);
    }
  };

  const openCreate = () => {
    setEditId(0);
    setSheetOpen(true);
  };
  const openEdit = (id) => {
    setEditId(id);
    setSheetOpen(true);
  };
  const onSaved = () => {
    setSheetOpen(false);
    setEditId(0);
    handleRefresh();
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await manageRedemption(deleteTarget.id, 'delete', '');
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const allSelected = redemptions.length > 0 && selectedIds.length === redemptions.length;
  const someSelected = selectedIds.length > 0 && !allSelected;

  const toggleSelect = (id) => {
    setSelectedIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  const toggleSelectAll = () => {
    setSelectedIds((prev) => (prev.length === redemptions.length ? [] : redemptions.map((r) => r.id)));
  };

  const exportBatch = async (item) => {
    const name = item.name;
    if (!name) return;
    try {
      const keys = [];
      const size = 100;
      let p = 1;
      while (p <= 100) {
        const res = await API.get('/api/redemption/', { params: { page: p, size, keyword: name, order: 'id' } });
        const { success, message, data } = res.data;
        if (!success) {
          showError(message);
          return;
        }
        const rows = data.data || [];
        for (const r of rows) if (r.name === name && r.key) keys.push(r.key);
        if (rows.length < size) break;
        p += 1;
      }
      if (!keys.length) {
        toast.error(t('redemptionPage.exportEmpty'));
        return;
      }
      downloadTextAsFile(keys.join('\n') + '\n', `${name}.txt`);
      toast.success(t('redemptionPage.exportSuccess', { count: keys.length }));
    } catch (error) {
      showError(error);
    }
  };

  const confirmBatchDelete = async () => {
    if (!selectedIds.length) return;
    setBatchDeleting(true);
    let ok = 0;
    let fail = 0;
    for (const id of selectedIds) {
      try {
        const res = await API.delete('/api/redemption/' + id);
        if (res.data?.success) ok += 1;
        else fail += 1;
      } catch (error) {
        fail += 1;
      }
    }
    setBatchDeleting(false);
    setBatchDeleteOpen(false);
    if (fail === 0) toast.success(t('redemptionPage.batchDeleteSuccess', { count: ok }));
    else toast.error(t('redemptionPage.batchDeletePartial', { success: ok, failed: fail }));
    setSelectedIds([]);
    handleRefresh();
  };

  const totalPages = Math.max(1, Math.ceil(listCount / rowsPerPage));

  return (
    <>
      <div className="space-y-6">
        {/* 标题行右侧操作:刷新 → 筛选 → 批量删除 → 新建(primary),对齐 Log 页。 */}
        <PageActions>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="sm" onClick={handleRefresh} aria-label={t('redemptionPage.refreshButton')}>
                  <RotateCcw className="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('redemptionPage.refreshButton')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <FilterBar
            fields={REDEMPTION_FILTER_FIELDS}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            hideChips
            t={t}
          />
          {selectedIds.length > 0 && (
            <Button variant="destructive" size="sm" onClick={() => setBatchDeleteOpen(true)}>
              <Trash2 className="size-4" /> {t('redemptionPage.batchDelete')} ({selectedIds.length})
            </Button>
          )}
          <ResponsiveToolbarButton primary icon={Plus} label={t('redemptionPage.createRedemptionCode')} onClick={openCreate} />
        </PageActions>

        {/* 标题行下方:OpenRouter 式全宽 chips 容器;仅在有激活筛选时渲染,无筛选不占空间。 */}
        {activeChipFields.length > 0 && (
          <FilterBar
            fields={REDEMPTION_FILTER_FIELDS}
            state={filterState}
            onChange={handleFilterChange}
            onClearAll={handleClearFilters}
            triggerVariant="plus"
            clearAlignEnd
            t={t}
            className="w-full rounded-lg border border-input bg-background px-3 py-2"
          />
        )}

        <Card>
          {redemptions.length === 0 && !searching ? (
            <div className="flex flex-col items-center justify-center gap-3 p-12 text-center">
              <Ticket className="size-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t('redemptionPage.pageTitle')}</p>
              <Button onClick={openCreate}>
                <Plus className="size-4" /> {t('redemptionPage.createRedemptionCode')}
              </Button>
            </div>
          ) : (
            <RedemptionTable
              redemptions={redemptions}
              onCopyKey={onCopyKey}
              onEdit={openEdit}
              onDelete={setDeleteTarget}
              onToggleStatus={onToggleStatus}
              onExportBatch={exportBatch}
              order={order}
              orderBy={orderBy}
              onSort={handleSort}
              selectedIds={selectedIds}
              onToggleSelect={toggleSelect}
              onToggleSelectAll={toggleSelectAll}
              allSelected={allSelected}
              someSelected={someSelected}
            />
          )}

          <div className="flex items-center justify-between gap-4 border-t border-border px-4 py-3 text-sm">
            <span className="text-muted-foreground">{listCount}</span>
            <div className="flex items-center gap-2">
              <select
                className="h-9 rounded-lg border border-input bg-background px-2 text-sm"
                value={rowsPerPage}
                onChange={(e) => onRowsPerPageChange(parseInt(e.target.value, 10))}
              >
                {PAGE_SIZE_OPTIONS.map((n) => (
                  <option key={n} value={n}>
                    {n} / page
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

        <RedemptionSheet open={sheetOpen} onOpenChange={setSheetOpen} redemptionId={editId} onSaved={onSaved} />

        <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('common.delete')}</DialogTitle>
              <DialogDescription>{t('common.deleteConfirm', { title: `"${deleteTarget?.name}"` })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteTarget(null)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmDelete} disabled={deleting}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={batchDeleteOpen} onOpenChange={(o) => !o && setBatchDeleteOpen(false)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('redemptionPage.batchDelete')}</DialogTitle>
              <DialogDescription>{t('redemptionPage.batchDeleteConfirm', { count: selectedIds.length })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setBatchDeleteOpen(false)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmBatchDelete} disabled={batchDeleting}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </>
  );
}
