import { useState, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Ban, Check, Plus, RotateCcw, Search, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import {
  BatchBar,
  ColumnPicker,
  ListEmptyState,
  ListFooter,
  ListSkeleton,
  PhoneListToolbar,
  StatusSegments,
  useColumnVisibility,
  useListLayout,
  useRowSelection
} from '@/components/list-page';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { API } from 'utils/api';
import { showError, downloadTextAsFile } from 'utils/common';
import usePaginatedList from 'hooks/usePaginatedList';
import { useObjectRoute } from '@/components/objects';
import RedemptionTable, { REDEMPTION_COLUMNS, RedemptionCards } from './RedemptionTable';
import RedemptionPanel from './RedemptionPanel';
import NewRedemptionSheet from './NewRedemptionSheet';

// Segments (?status=), counted by the list response (status_counts). They do not overlap.
const SEGMENTS = ['all', 'usable', 'expired', 'disabled', 'used'];
const KEYWORD_DEBOUNCE_MS = 300;

export default function Redemption() {
  const { t, i18n } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [keywordInput, setKeywordInput] = useState(() => searchParams.get('keyword') || '');
  const [keyword, setKeyword] = useState(() => (searchParams.get('keyword') || '').trim());
  const [status, setStatus] = useState(() => (SEGMENTS.includes(searchParams.get('status')) ? searchParams.get('status') : 'all'));
  const [statusCounts, setStatusCounts] = useState(null);

  const route = useObjectRoute(['details']);
  // Bumped after a list action so an open code panel reloads.
  const [reloadKey, setReloadKey] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [batchBusy, setBatchBusy] = useState(false);

  const {
    page,
    setPage,
    rowsPerPage,
    order,
    orderBy,
    listCount,
    searching,
    loaded,
    rows: redemptions,
    setRows: setRedemptions,
    handleRefresh,
    handleSort,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'redemption',
    fetcher: async ({ page, rowsPerPage, order, orderBy }) => {
      const ob = orderBy ? (order === 'desc' ? '-' + orderBy : orderBy) : '-id';
      const params = {
        page: page + 1,
        size: rowsPerPage,
        order: ob,
        ...(keyword ? { keyword } : {}),
        ...(status !== 'all' ? { status } : {})
      };
      const res = await API.get('/api/redemption/', { params });
      return res.data;
    },
    onSuccess: (payload) => setStatusCounts(payload.status_counts ?? null),
    deps: [keyword, status]
  });

  // The search box applies after a short pause in typing.
  useEffect(() => {
    const handle = setTimeout(() => {
      const next = keywordInput.trim();
      if (next === keyword) return;
      setPage(0);
      setKeyword(next);
    }, KEYWORD_DEBOUNCE_MS);
    return () => clearTimeout(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keywordInput]);

  // Search and segment live in the URL (replace), so a reload restores them.
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        params.delete('keyword');
        params.delete('status');
        if (keyword) params.set('keyword', keyword);
        if (status !== 'all') params.set('status', status);
        return params;
      },
      { replace: true }
    );
  }, [keyword, status, setSearchParams]);

  const changeStatus = (next) => {
    setPage(0);
    setStatus(next);
  };

  const columns = useColumnVisibility({ storageKey: 'redemption-column-visibility-v1', columns: REDEMPTION_COLUMNS });
  const selection = useRowSelection(redemptions, [page, rowsPerPage, keyword, status]);
  // Full table while it fits (the sidebar collapses first), phone cards otherwise.
  const { narrow } = useListLayout();
  const rtf = useMemo(
    () => new Intl.RelativeTimeFormat((i18n.language || 'zh_CN').replace('_', '-'), { numeric: 'auto' }),
    [i18n.language]
  );

  const manageRedemption = async (id, action, value) => {
    try {
      let res;
      if (action === 'delete') res = await API.delete('/api/redemption/' + id);
      else res = await API.put('/api/redemption/?status_only=true', { id, status: value });
      const { success } = res.data;
      if (success) {
        toast.success(t('redemptionPage.successMessage'));
        if (action === 'delete') {
          if (id === route.openId) route.close();
          handleRefresh();
        } else setReloadKey((k) => k + 1);
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

  // Bulk enable / disable / delete: there is no batch endpoint, so each code goes on its own and
  // failures are counted. Redeemed codes are skipped by enable / disable.
  const runBatch = async (action) => {
    const ids = [...selection.selectedIds];
    if (ids.length === 0) return;
    setBatchBusy(true);
    let ok = 0;
    let fail = 0;
    for (const id of ids) {
      const item = redemptions.find((r) => r.id === id);
      if (action !== 'delete' && item?.status === 3) continue;
      try {
        const res =
          action === 'delete'
            ? await API.delete('/api/redemption/' + id)
            : await API.put('/api/redemption/?status_only=true', { id, status: action === 'enable' ? 1 : 2 });
        if (res.data?.success) ok += 1;
        else fail += 1;
      } catch (error) {
        fail += 1;
      }
    }
    setBatchBusy(false);
    setBatchDeleteOpen(false);
    if (fail === 0)
      toast.success(action === 'delete' ? t('redemptionPage.batchDeleteSuccess', { count: ok }) : t('redemptionPage.successMessage'));
    else toast.error(t('redemptionPage.batchDeletePartial', { success: ok, failed: fail }));
    selection.exitSelectMode();
    handleRefresh();
  };

  const actions = useMemo(
    () => ({ onOpen: route.open, onCopyKey, onExportBatch: exportBatch, onToggleStatus, onDelete: setDeleteTarget }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [route.open, redemptions]
  );

  const batchActions = (
    <>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('enable')}>
        <Check className="size-3.5" />
        {t('redemptionList.batchEnable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('disable')}>
        <Ban className="size-3.5" />
        {t('redemptionList.batchDisable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7 text-destructive" disabled={batchBusy} onClick={() => setBatchDeleteOpen(true)}>
        <Trash2 className="size-3.5" />
        {t('common.delete')}
      </Button>
    </>
  );

  const segmentOptions = SEGMENTS.map((value) => ({ value, label: t(`redemptionList.segments.${value}`) }));
  const hasFilters = !!keyword || status !== 'all';

  let body;
  if (!loaded) {
    body = narrow ? <ListSkeleton layout="list" rows={4} /> : <ListSkeleton layout="table" cells={[96, 64, 80, 80, 32]} />;
  } else if (!searching && redemptions.length === 0) {
    body = hasFilters ? (
      <ListEmptyState
        variant="noMatch"
        onClearFilters={() => {
          setKeywordInput('');
          setKeyword('');
          changeStatus('all');
        }}
      />
    ) : (
      <ListEmptyState
        title={t('redemptionPage.pageTitle')}
        action={
          <Button size="sm" onClick={route.openNew}>
            <Plus className="size-4" /> {t('redemptionPage.createRedemptionCode')}
          </Button>
        }
      />
    );
  } else if (narrow) {
    body = (
      <RedemptionCards
        redemptions={redemptions}
        rtf={rtf}
        selectMode={selection.selectMode}
        selectedIds={selection.selectedIds}
        onToggleSelect={selection.toggle}
        openId={route.openId}
        actions={actions}
      />
    );
  } else {
    body = (
      <RedemptionTable
        redemptions={redemptions}
        columnVisibility={columns.visibility}
        rtf={rtf}
        openId={route.openId}
        order={order}
        orderBy={orderBy}
        onSort={handleSort}
        selectedIds={selection.selectedIds}
        onToggleSelect={selection.toggle}
        onToggleSelectAll={selection.toggleAll}
        actions={actions}
      />
    );
  }

  const footer = (
    <ListFooter
      total={listCount}
      page={page}
      pageSize={rowsPerPage}
      selectedCount={selection.count}
      onPageChange={setPage}
      onPageSizeChange={onRowsPerPageChange}
    />
  );

  return (
    <>
      <div className={cn('space-y-3', narrow && selection.selectMode && 'pb-20')}>
        {/* Title row: refresh (icon), then create. */}
        <PageActions>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="sm" onClick={handleRefresh} aria-label={t('redemptionPage.refreshButton')}>
                  <RotateCcw className={searching ? 'size-4 animate-spin' : 'size-4'} />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('redemptionPage.refreshButton')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <ResponsiveToolbarButton primary icon={Plus} label={t('redemptionPage.createRedemptionCode')} onClick={route.openNew} />
        </PageActions>

        {narrow ? (
          <div className="space-y-2">
            <StatusSegments chips value={status} onChange={changeStatus} options={segmentOptions} counts={statusCounts} />
            <PhoneListToolbar
              search={{ value: keywordInput, onChange: setKeywordInput, placeholder: t('redemptionList.searchShort') }}
              select={{
                active: selection.selectMode,
                onToggle: () => (selection.selectMode ? selection.exitSelectMode() : selection.enterSelectMode())
              }}
            />
          </div>
        ) : (
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <StatusSegments value={status} onChange={changeStatus} options={segmentOptions} counts={statusCounts} />
              <div className="relative min-w-[12rem] flex-1">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={keywordInput}
                  onChange={(e) => setKeywordInput(e.target.value)}
                  placeholder={t('redemptionList.searchShort')}
                  aria-label={t('redemptionList.searchShort')}
                  className="h-8 pl-8"
                />
              </div>
              <ColumnPicker columns={columns} />
            </div>
            {selection.count > 0 && (
              <BatchBar
                count={selection.count}
                onClear={selection.clear}
                className="min-h-10 rounded-lg border border-primary/30 bg-primary/[0.04] px-3 py-1.5"
                actions={batchActions}
              />
            )}
          </div>
        )}

        {narrow ? (
          <>
            {body}
            <Card>{footer}</Card>
          </>
        ) : (
          <Card>
            {body}
            <div className="border-t border-border">{footer}</div>
          </Card>
        )}

        {route.openId > 0 && (
          <RedemptionPanel
            key={route.openId}
            codeId={route.openId}
            onClose={route.close}
            onChanged={handleRefresh}
            onToggleStatus={onToggleStatus}
            onCopyKey={onCopyKey}
            onExportBatch={exportBatch}
            onDelete={setDeleteTarget}
            reloadKey={reloadKey}
          />
        )}

        {route.isNew && (
          <NewRedemptionSheet
            onClose={route.close}
            onCreated={() => {
              toast.success(t('redemption_edit.addOk'));
              route.close();
              handleRefresh();
            }}
          />
        )}

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
              <DialogDescription>{t('redemptionPage.batchDeleteConfirm', { count: selection.count })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setBatchDeleteOpen(false)}>
                {t('common.cancel')}
              </Button>
              <Button variant="destructive" onClick={() => runBatch('delete')} disabled={batchBusy}>
                {t('common.delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {narrow && selection.selectMode && (
        <BatchBar count={selection.count} onClear={selection.exitSelectMode} floating actions={batchActions} />
      )}
    </>
  );
}
