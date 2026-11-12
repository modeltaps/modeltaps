import { useEffect, useMemo, useState } from 'react';
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
  ListEmptyState,
  ListFooter,
  ListSkeleton,
  PhoneListToolbar,
  StatusSegments,
  useListLayout,
  useRowSelection
} from '@/components/list-page';
import { API } from 'utils/api';
import { showError, copy } from 'utils/common';
import usePaginatedList from 'hooks/usePaginatedList';
import PageActions from '@/components/chrome/PageActions';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import InviteCodeTable, { InviteCodeCards } from './InviteCodeTable';
import InviteCodeSheet from './InviteCodeSheet';

const tk = (k) => `setting_index.inviteCodeSettings.${k}`;

// Segments (?state=), counted by the list response (state_counts). They do not overlap.
const SEGMENTS = ['all', 'usable', 'pending', 'expired', 'exhausted', 'disabled'];
const KEYWORD_DEBOUNCE_MS = 300;

// ==============================|| PANEL — INVITE CODES (admin) ||============================== //
// Registration invite codes: segments, search, bulk actions and the create / edit sheet. Whether
// registration needs a code is a switch in platform settings (sign-in and registration).

export default function InviteCode() {
  const { t, i18n } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [keywordInput, setKeywordInput] = useState(() => searchParams.get('keyword') || '');
  const [keyword, setKeyword] = useState(() => (searchParams.get('keyword') || '').trim());
  const [state, setState] = useState(() => (SEGMENTS.includes(searchParams.get('state')) ? searchParams.get('state') : 'all'));
  const [stateCounts, setStateCounts] = useState(null);

  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [batchBusy, setBatchBusy] = useState(false);

  const {
    page,
    setPage,
    rowsPerPage,
    listCount,
    searching,
    loaded,
    rows: inviteCodes,
    setRows: setInviteCodes,
    handleRefresh,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'inviteCode',
    fetcher: async ({ page, rowsPerPage }) => {
      const params = {
        page: page + 1,
        size: rowsPerPage,
        order: '-id',
        ...(keyword ? { keyword } : {}),
        ...(state !== 'all' ? { state } : {})
      };
      const res = await API.get('/api/invite-code/', { params });
      return res.data;
    },
    onSuccess: (payload) => setStateCounts(payload.state_counts ?? null),
    deps: [keyword, state]
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
        params.delete('state');
        if (keyword) params.set('keyword', keyword);
        if (state !== 'all') params.set('state', state);
        return params;
      },
      { replace: true }
    );
  }, [keyword, state, setSearchParams]);

  const changeState = (next) => {
    setPage(0);
    setState(next);
  };

  const selection = useRowSelection(inviteCodes, [page, rowsPerPage, keyword, state]);
  // Full table while it fits (the sidebar collapses first), phone cards otherwise.
  const { narrow } = useListLayout();
  const rtf = useMemo(
    () => new Intl.RelativeTimeFormat((i18n.language || 'zh_CN').replace('_', '-'), { numeric: 'auto' }),
    [i18n.language]
  );

  const onCopy = (item) => copy(item.code, t(tk('headLabels.code')));

  const setStatus = (id, status) => API.put(`/api/invite-code/${id}`, { status });

  const onToggleStatus = async (item) => {
    const next = item.status === 1 ? 2 : 1;
    try {
      const res = await setStatus(item.id, next);
      if (res.data?.success) {
        setInviteCodes((prev) => prev.map((x) => (x.id === item.id ? { ...x, status: next } : x)));
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const openCreate = () => {
    setEditing(null);
    setSheetOpen(true);
  };
  const openEdit = (item) => {
    setEditing(item);
    setSheetOpen(true);
  };
  const onSaved = () => {
    setSheetOpen(false);
    setEditing(null);
    handleRefresh();
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const res = await API.delete(`/api/invite-code/${deleteTarget.id}`);
      if (res.data?.success) {
        toast.success(t('common.deleteSuccess'));
        handleRefresh();
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  // Bulk enable / disable go code by code (failures are counted); delete has a batch endpoint.
  const runBatch = async (action) => {
    const ids = [...selection.selectedIds];
    if (ids.length === 0) return;
    setBatchBusy(true);
    try {
      if (action === 'delete') {
        const res = await API.post('/api/invite-code/batch-delete', { ids });
        if (res.data?.success) toast.success(t(tk('batchDeleteOk'), { count: ids.length }));
        else showError(res.data?.message);
      } else {
        const results = await Promise.allSettled(ids.map((id) => setStatus(id, action === 'enable' ? 1 : 2)));
        const failed = results.filter((r) => r.status !== 'fulfilled' || !r.value.data?.success).length;
        if (failed > 0) showError(t('userPage.batchFailed', { count: failed }));
        else toast.success(t('userPage.operationSuccess'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setBatchBusy(false);
      setBatchDeleteOpen(false);
      selection.exitSelectMode();
      handleRefresh();
    }
  };

  const actions = { onEdit: openEdit, onCopy, onToggleStatus, onDelete: setDeleteTarget };

  const batchActions = (
    <>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('enable')}>
        <Check className="size-3.5" />
        {t('inviteList.enable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7" disabled={batchBusy} onClick={() => runBatch('disable')}>
        <Ban className="size-3.5" />
        {t('inviteList.disable')}
      </Button>
      <Button size="sm" variant="outline" className="h-7 text-destructive" disabled={batchBusy} onClick={() => setBatchDeleteOpen(true)}>
        <Trash2 className="size-3.5" />
        {t('common.delete')}
      </Button>
    </>
  );

  const segmentOptions = SEGMENTS.map((value) => ({ value, label: t(`inviteList.segments.${value}`) }));
  const hasFilters = !!keyword || state !== 'all';

  let body;
  if (!loaded) {
    body = narrow ? <ListSkeleton layout="list" rows={4} /> : <ListSkeleton layout="table" cells={[64, 96, 80, 32]} />;
  } else if (!searching && inviteCodes.length === 0) {
    body = hasFilters ? (
      <ListEmptyState
        variant="noMatch"
        onClearFilters={() => {
          setKeywordInput('');
          setKeyword('');
          changeState('all');
        }}
      />
    ) : (
      <ListEmptyState
        title={t(tk('emptyHint'))}
        action={
          <Button size="sm" onClick={openCreate}>
            <Plus className="size-4" /> {t(tk('createButton'))}
          </Button>
        }
      />
    );
  } else if (narrow) {
    body = (
      <InviteCodeCards
        items={inviteCodes}
        rtf={rtf}
        selectMode={selection.selectMode}
        selectedIds={selection.selectedIds}
        onToggleSelect={selection.toggle}
        actions={actions}
      />
    );
  } else {
    body = (
      <InviteCodeTable
        items={inviteCodes}
        rtf={rtf}
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
                <Button variant="outline" size="sm" onClick={handleRefresh} aria-label={t(tk('refreshButton'))}>
                  <RotateCcw className={searching ? 'size-4 animate-spin' : 'size-4'} />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t(tk('refreshButton'))}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <ResponsiveToolbarButton primary icon={Plus} label={t(tk('createButton'))} onClick={openCreate} />
        </PageActions>

        <p className="text-xs text-muted-foreground">{t(tk('usageNote.line1'))}</p>

        {narrow ? (
          <div className="space-y-2">
            <StatusSegments chips value={state} onChange={changeState} options={segmentOptions} counts={stateCounts} />
            <PhoneListToolbar
              search={{ value: keywordInput, onChange: setKeywordInput, placeholder: t('inviteList.searchShort') }}
              select={{
                active: selection.selectMode,
                onToggle: () => (selection.selectMode ? selection.exitSelectMode() : selection.enterSelectMode())
              }}
            />
          </div>
        ) : (
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <StatusSegments value={state} onChange={changeState} options={segmentOptions} counts={stateCounts} />
              <div className="relative min-w-[12rem] flex-1">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={keywordInput}
                  onChange={(e) => setKeywordInput(e.target.value)}
                  placeholder={t('inviteList.searchShort')}
                  aria-label={t('inviteList.searchShort')}
                  className="h-8 pl-8"
                />
              </div>
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

        <InviteCodeSheet open={sheetOpen} onOpenChange={setSheetOpen} editing={editing} onSaved={onSaved} />

        <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('common.delete')}</DialogTitle>
              <DialogDescription>{t('common.deleteConfirm', { title: `"${deleteTarget?.name || deleteTarget?.code}"` })}</DialogDescription>
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
              <DialogTitle>{t('common.delete')}</DialogTitle>
              <DialogDescription>
                {t('common.deleteConfirm', { title: t(tk('batchDeleteTitle'), { count: selection.count }) })}
              </DialogDescription>
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
