import { useCallback, useContext, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Plus, RefreshCw, ShieldCheck } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { LoadStatusContext } from 'contexts/StatusContext';
import OidcProviderTable from './OidcProviderTable';
import OidcProviderSheet from './OidcProviderSheet';

const tk = (k) => `setting_index.oidcProviders.${k}`;

// 后台 OIDC 提供方管理：列表 + 新建/编辑抽屉。登录入口以本表为准（见 controller/misc.go）。
export default function OidcProviderSettings({ serverAddress }) {
  const { t } = useTranslation();
  const loadStatus = useContext(LoadStatusContext);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/oidc_provider/');
      const { success, message, data } = res.data;
      if (success) setItems(data || []);
      else showError(message);
    } catch (error) {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const onSaved = async () => {
    setSheetOpen(false);
    setEditing(null);
    await fetchData();
    if (loadStatus) await loadStatus();
  };

  const onToggleStatus = async (item) => {
    try {
      const res = await API.put(`/api/oidc_provider/${item.id}/status`, { enabled: !item.enabled });
      if (res.data?.success) {
        setItems((prev) => prev.map((x) => (x.id === item.id ? { ...x, enabled: !item.enabled } : x)));
        if (loadStatus) await loadStatus();
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      /* 错误已由 utils/api.js 响应拦截器弹过 toast */
    }
  };

  const onTest = async (item) => {
    try {
      const res = await API.post(`/api/oidc_provider/${item.id}/test`);
      if (res.data?.success) toast.success(res.data.message || t(tk('testOk')));
      else showError(res.data?.message);
    } catch (error) {
      /* discovery 失败的原文已由拦截器弹出 */
    }
  };

  // 删除返回 409（该提供方下仍有已绑定身份）时，后端错误信息由拦截器原样弹出。
  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const res = await API.delete(`/api/oidc_provider/${deleteTarget.id}`);
      if (res.data?.success) {
        toast.success(t('common.deleteSuccess'));
        setDeleteTarget(null);
        await fetchData();
        if (loadStatus) await loadStatus();
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      /* 409 等错误信息已由拦截器弹出，保留对话框让管理员看到上下文 */
    } finally {
      setDeleting(false);
    }
  };

  const openCreate = () => {
    setEditing(null);
    setSheetOpen(true);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-end gap-2">
        <Button variant="outline" onClick={fetchData} disabled={loading}>
          <RefreshCw className="size-4" /> {t(tk('refresh'))}
        </Button>
        <Button onClick={openCreate}>
          <Plus className="size-4" /> {t(tk('createButton'))}
        </Button>
      </div>

      <Card>
        {items.length === 0 && !loading ? (
          <div className="flex flex-col items-center justify-center gap-3 p-12 text-center">
            <ShieldCheck className="size-10 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">{t(tk('emptyHint'))}</p>
            <Button onClick={openCreate}>
              <Plus className="size-4" /> {t(tk('createButton'))}
            </Button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <OidcProviderTable
              items={items}
              onEdit={(item) => {
                setEditing(item);
                setSheetOpen(true);
              }}
              onTest={onTest}
              onDelete={setDeleteTarget}
              onToggleStatus={onToggleStatus}
            />
          </div>
        )}
      </Card>

      <OidcProviderSheet open={sheetOpen} onOpenChange={setSheetOpen} editing={editing} serverAddress={serverAddress} onSaved={onSaved} />

      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
            <DialogDescription>
              {t('common.deleteConfirm', { title: `"${deleteTarget?.display_name || deleteTarget?.slug}"` })}
            </DialogDescription>
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
  );
}
