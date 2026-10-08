import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Navigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Plus } from 'lucide-react';

import { Button } from '@/components/ui/button';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { ListEmptyState, ListFooter, ListPage, ListToolbar } from '@/components/list-page';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError, useIsAdmin } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import { enabledChannels } from '../Pricing/modelCatalog';
import ModelOwnedbyTable from './ModelOwnedbyTable';
import ModelOwnedbySheet from './ModelOwnedbySheet';

// ==============================|| PANEL — MODEL OWNEDBY (admin) ||============================== //
// shadcn/Tailwind port of v1 `views/ModelOwnedby`. Admin-only CRUD over the
// unchanged `/api/model_ownedby/` API (flat list: id, name, icon).
// 每行附目录里归属该厂商的模型数与其中无启用渠道的数量(取自 /api/model_info/catalog)。

export default function ModelOwnedby() {
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();

  const [items, setItems] = useState([]);
  const [catalog, setCatalog] = useState([]);
  const [loading, setLoading] = useState(false);
  const [refreshFlag, setRefreshFlag] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(() => getPageSize('model_ownedby'));

  const [sheetOpen, setSheetOpen] = useState(false);
  const [editId, setEditId] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);

  // reqId 守卫(UX-13):快速连点刷新时丢弃过期响应;本页为一次性拉全量平铺列表,不迁 usePaginatedList。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  const fetchData = useCallback(
    () =>
      runGuardedFetch(guardRef.current, () => API.get('/api/model_ownedby/'), {
        onStart: () => setLoading(true),
        onResult: (res) => {
          const { success, message, data } = res.data;
          if (success) {
            setItems(Array.isArray(data) ? data : []);
          } else {
            showError(message);
          }
        },
        onError: (error) => console.error(error),
        onFinally: () => setLoading(false)
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [refreshFlag]
  );

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    API.get('/api/model_info/catalog')
      .then((res) => {
        if (res.data?.success) setCatalog(res.data.data || []);
      })
      .catch(() => {});
  }, [refreshFlag]);

  const rows = useMemo(() => {
    const counts = {};
    catalog.forEach((info) => {
      const entry = (counts[info.vendor_id || 0] ||= { model_count: 0, nochannel_count: 0 });
      entry.model_count += 1;
      if (enabledChannels(info).length === 0) entry.nochannel_count += 1;
    });
    return items.map((item) => ({ ...item, model_count: 0, nochannel_count: 0, ...counts[item.id] }));
  }, [items, catalog]);

  const filtered = useMemo(() => {
    const k = keyword.trim().toLowerCase();
    if (!k) return rows;
    return rows.filter((row) => row.name?.toLowerCase().includes(k) || row.slug?.toLowerCase().includes(k));
  }, [rows, keyword]);

  const pageRows = filtered.slice(page * pageSize, page * pageSize + pageSize);

  const handleRefresh = () => setRefreshFlag((f) => !f);

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
      const res = await API.delete('/api/model_ownedby/' + deleteTarget.id);
      const { success, message } = res.data;
      if (success) {
        toast.success(t('userPage.operationSuccess'));
        handleRefresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  if (!isAdmin) return <Navigate to="/panel/dashboard" replace />;

  return (
    <>
      <div>
        {/* 厂商页签时页头右侧只保留「添加厂商」。 */}
        <PageActions>
          <ResponsiveToolbarButton primary icon={Plus} label={t('modelsPage.addVendor')} onClick={openCreate} />
        </PageActions>

        <ListPage
          toolbar={
            <ListToolbar
              search={{
                value: keyword,
                onChange: (value) => {
                  setPage(0);
                  setKeyword(value);
                },
                placeholder: t('modelsPage.searchVendors')
              }}
            />
          }
          footer={
            <ListFooter
              total={filtered.length}
              page={page}
              pageSize={pageSize}
              onPageChange={setPage}
              onPageSizeChange={(n) => {
                setPageSize(n);
                setPage(0);
                savePageSize('model_ownedby', n);
              }}
            />
          }
        >
          {!loading && items.length === 0 ? (
            <ListEmptyState title={t('modelOwnedby.title')} />
          ) : !loading && filtered.length === 0 ? (
            <ListEmptyState variant="noMatch" onClearFilters={() => setKeyword('')} />
          ) : (
            <ModelOwnedbyTable items={pageRows} onEdit={openEdit} onDelete={setDeleteTarget} />
          )}
        </ListPage>

        <ModelOwnedbySheet open={sheetOpen} onOpenChange={setSheetOpen} ownedbyId={editId} onSaved={onSaved} />

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
      </div>
    </>
  );
}
