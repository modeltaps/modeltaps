import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Navigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Plus, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import { ListEmptyState, ListFooter, ListSkeleton, StatusSegments, useListLayout } from '@/components/list-page';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError, useIsAdmin } from 'utils/common';
import { getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import { enabledChannels } from '../Pricing/modelCatalog';
import ModelOwnedbyTable, { VENDOR_TABLE_ESTIMATE, VendorList } from './ModelOwnedbyTable';
import ModelOwnedbySheet from './ModelOwnedbySheet';

// ==============================|| PANEL — MODEL OWNEDBY (admin) ||============================== //
// Vendors tab of the model catalog: admin CRUD over `/api/model_ownedby/` (flat list: id, name,
// slug, icon). Each row carries how many catalog models belong to the vendor and how many of
// those have no enabled channel (from /api/model_info/catalog).

// Segments: all, vendors with catalog models, and the rest (mostly built-in channel types).
const SEGMENTS = [
  { value: 'all', labelKey: 'modelsPage.stats.all' },
  { value: 'inuse', labelKey: 'modelsPage.vendorInUse' },
  { value: 'unused', labelKey: 'modelsPage.vendorUnused' }
];
const inSegment = (row, segment) => (segment === 'inuse' ? row.model_count > 0 : segment === 'unused' ? !row.model_count : true);

// Local sort; names ascending first, counts descending first.
const compareRows = ({ key, dir }) => {
  const sign = dir === 'asc' ? 1 : -1;
  return (a, b) => {
    if (key === 'name') return sign * String(a.name || '').localeCompare(String(b.name || ''));
    return sign * ((a[key] || 0) - (b[key] || 0));
  };
};

export default function ModelOwnedby() {
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();

  const [items, setItems] = useState([]);
  const [catalog, setCatalog] = useState([]);
  const [loaded, setLoaded] = useState(false);
  const [segment, setSegment] = useState('all');
  const [sort, setSort] = useState({ key: null, dir: 'desc' });
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
        onResult: (res) => {
          const { success, message, data } = res.data;
          if (success) {
            setItems(Array.isArray(data) ? data : []);
          } else {
            showError(message);
          }
        },
        onError: (error) => console.error(error),
        onFinally: () => setLoaded(true)
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

  const counts = useMemo(
    () => Object.fromEntries(SEGMENTS.map((s) => [s.value, rows.filter((row) => inSegment(row, s.value)).length])),
    [rows]
  );

  const filtered = useMemo(() => {
    const k = keyword.trim().toLowerCase();
    const list = rows.filter(
      (row) => inSegment(row, segment) && (!k || row.name?.toLowerCase().includes(k) || row.slug?.toLowerCase().includes(k))
    );
    return sort.key ? [...list].sort(compareRows(sort)) : list;
  }, [rows, keyword, segment, sort]);

  // Click a header: first click picks its natural direction, the next flips it.
  const handleSort = (key) =>
    setSort((prev) =>
      prev.key === key ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: key === 'name' ? 'asc' : 'desc' }
    );

  // Full table while it fits (the sidebar collapses first), phone list otherwise.
  const { pageRef, layout, narrow, onNaturalWidth } = useListLayout({ estimate: VENDOR_TABLE_ESTIMATE, resetKey: 'vendors' });

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

  const changeKeyword = (value) => {
    setPage(0);
    setKeyword(value);
  };
  const changeSegment = (value) => {
    setPage(0);
    setSegment(value);
  };
  const segmentOptions = SEGMENTS.map((s) => ({ value: s.value, label: t(s.labelKey) }));

  let body;
  if (!loaded) {
    body = <ListSkeleton layout={layout} rows={6} checkbox={false} cells={[64, 64, 32]} />;
  } else if (items.length === 0) {
    body = <ListEmptyState title={t('modelOwnedby.title')} />;
  } else if (filtered.length === 0) {
    body = (
      <ListEmptyState
        variant="noMatch"
        onClearFilters={() => {
          changeKeyword('');
          changeSegment('all');
        }}
      />
    );
  } else if (narrow) {
    body = <VendorList items={pageRows} onEdit={openEdit} onDelete={setDeleteTarget} />;
  } else {
    body = (
      <ModelOwnedbyTable
        items={pageRows}
        sort={sort}
        onSort={handleSort}
        onEdit={openEdit}
        onDelete={setDeleteTarget}
        onNaturalWidth={onNaturalWidth}
      />
    );
  }

  const search = (
    <div className="relative min-w-0 flex-1 sm:max-w-80">
      <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        value={keyword}
        onChange={(e) => changeKeyword(e.target.value)}
        placeholder={t('modelsPage.searchVendors')}
        aria-label={t('modelsPage.searchVendors')}
        className="h-8 pl-8"
      />
    </div>
  );

  return (
    <>
      <div ref={pageRef} className="space-y-3">
        {/* On the vendors tab the title row keeps only "Add vendor". */}
        <PageActions>
          <ResponsiveToolbarButton primary icon={Plus} label={t('modelsPage.addVendor')} onClick={openCreate} />
        </PageActions>

        {narrow ? (
          <div className="space-y-2">
            <StatusSegments chips value={segment} onChange={changeSegment} options={segmentOptions} counts={loaded ? counts : null} />
            <div className="flex">{search}</div>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            <StatusSegments value={segment} onChange={changeSegment} options={segmentOptions} counts={loaded ? counts : null} />
            {search}
          </div>
        )}

        <Card>
          {body}
          <div className="border-t border-border">
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
          </div>
        </Card>

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
