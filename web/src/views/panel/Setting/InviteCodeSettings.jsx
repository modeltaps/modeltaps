import { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Info, Plus, RefreshCw, Search, Ticket, Trash2 } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError, copy, trims } from 'utils/common';
import { PAGE_SIZE_OPTIONS, getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import InviteCodeTable from './InviteCodeTable';
import InviteCodeSheet from './InviteCodeSheet';

const tk = (k) => `setting_index.inviteCodeSettings.${k}`;

export default function InviteCodeSettings() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('inviteCode'));
  const [listCount, setListCount] = useState(0);
  const [searching, setSearching] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [keywordInput, setKeywordInput] = useState('');
  const [statusFilter, setStatusFilter] = useState('0');
  const [inviteCodes, setInviteCodes] = useState([]);
  const [refreshFlag, setRefreshFlag] = useState(false);

  const [selected, setSelected] = useState([]);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [batchConfirm, setBatchConfirm] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);

  // reqId 守卫(UX-13):快速切页/搜索/切状态时丢弃过期响应。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  const fetchData = useCallback(() => {
    const keyword = trims(searchKeyword);
    const params = { page: page + 1, size: rowsPerPage, order: '-id', keyword };
    if (statusFilter !== '0') params.status = parseInt(statusFilter, 10);
    return runGuardedFetch(guardRef.current, () => API.get('/api/invite-code/', { params }), {
      onStart: () => setSearching(true),
      onResult: (res) => {
        const { success, message, data } = res.data;
        if (success) {
          setListCount(data.total_count);
          setInviteCodes(data.data || []);
          setSelected([]);
        } else {
          showError(message);
        }
      },
      onError: (error) => {
        console.error(error);
        setInviteCodes([]);
      },
      onFinally: () => setSearching(false)
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, rowsPerPage, searchKeyword, statusFilter, refreshFlag]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleRefresh = () => setRefreshFlag((f) => !f);

  const onSelectAll = () => {
    setSelected((prev) => (prev.length === inviteCodes.length ? [] : inviteCodes.map((c) => c.id)));
  };
  const onSelectOne = (id) => {
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  const onCopy = (item) => copy(item.code, t(tk('headLabels.code')));

  const onToggleStatus = async (item) => {
    const next = item.status === 1 ? 2 : 1;
    try {
      const res = await API.put(`/api/invite-code/${item.id}`, { status: next });
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

  const confirmBatchDelete = async () => {
    if (selected.length === 0) return;
    setBatchDeleting(true);
    try {
      const res = await API.post('/api/invite-code/batch-delete', { ids: selected });
      if (res.data?.success) {
        toast.success(t(tk('batchDeleteOk'), { count: selected.length }));
        setSelected([]);
        handleRefresh();
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setBatchDeleting(false);
      setBatchConfirm(false);
    }
  };

  const totalPages = Math.max(1, Math.ceil(listCount / rowsPerPage));

  return (
    <>
      <div className="space-y-6">
        <div className="flex items-center justify-end">
          <Button onClick={openCreate}>
            <Plus className="size-4" /> {t(tk('createButton'))}
          </Button>
        </div>

        <Alert variant="info">
          <Info />
          <AlertDescription>
            <p>{t(tk('usageNote.line1'))}</p>
            <p>{t(tk('usageNote.line2'))}</p>
            <p>{t(tk('usageNote.line3'))}</p>
            <p>{t(tk('usageNote.line4'))}</p>
          </AlertDescription>
        </Alert>

        <Card>
          <div className="flex flex-col gap-2 border-b border-border p-4 sm:flex-row sm:items-center sm:justify-between">
            <form
              className="relative w-full flex-1"
              onSubmit={(e) => {
                e.preventDefault();
                setPage(0);
                setSearchKeyword(keywordInput);
              }}
            >
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input className="pl-9" placeholder={t(tk('searchPlaceholder'))} value={keywordInput} onChange={(e) => setKeywordInput(e.target.value)} />
            </form>
            <div className="flex items-center gap-2">
              <Select
                value={statusFilter}
                onValueChange={(v) => {
                  setStatusFilter(v);
                  setPage(0);
                }}
              >
                <SelectTrigger className="w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="0">{t(tk('statusAll'))}</SelectItem>
                  <SelectItem value="1">{t(tk('statusEnabled'))}</SelectItem>
                  <SelectItem value="2">{t(tk('statusDisabled'))}</SelectItem>
                </SelectContent>
              </Select>
              <Button variant="destructive" disabled={selected.length === 0} onClick={() => setBatchConfirm(true)}>
                <Trash2 className="size-4" /> {t(tk('batchDelete'), { count: selected.length })}
              </Button>
              <Button variant="outline" onClick={handleRefresh}>
                <RefreshCw className="size-4" /> {t(tk('refreshButton'))}
              </Button>
            </div>
          </div>

          {inviteCodes.length === 0 && !searching ? (
            <div className="flex flex-col items-center justify-center gap-3 p-12 text-center">
              <Ticket className="size-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">{t(tk('emptyHint'))}</p>
              <Button onClick={openCreate}>
                <Plus className="size-4" /> {t(tk('createButton'))}
              </Button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <InviteCodeTable
                items={inviteCodes}
                selected={selected}
                onSelectAll={onSelectAll}
                onSelectOne={onSelectOne}
                onCopy={onCopy}
                onEdit={openEdit}
                onDelete={setDeleteTarget}
                onToggleStatus={onToggleStatus}
              />
            </div>
          )}

          <div className="flex items-center justify-between gap-4 border-t border-border p-4 text-sm">
            <span className="text-muted-foreground">{listCount}</span>
            <div className="flex items-center gap-2">
              <select
                className="h-9 rounded-lg border border-input bg-background px-2 text-sm"
                value={rowsPerPage}
                onChange={(e) => {
                  const n = parseInt(e.target.value, 10);
                  setRowsPerPage(n);
                  setPage(0);
                  savePageSize('inviteCode', n);
                }}
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

        <Dialog open={batchConfirm} onOpenChange={(o) => !o && setBatchConfirm(false)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('common.delete')}</DialogTitle>
              <DialogDescription>{t('common.deleteConfirm', { title: t(tk('batchDeleteTitle'), { count: selected.length }) })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setBatchConfirm(false)}>
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
