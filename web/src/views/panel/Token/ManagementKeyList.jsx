import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Trash2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';
import { managementKeyTypeLabel } from './tokenData';

// Management keys (AGENT-4), listed under the API keys: each row has its type tag (read-only,
// read-write, or a deprecated access token), the masked key, its expiry and last use, and delete.
// They are created from the same "New API key" dialog. Nothing is shown until the user has one.
export default function ManagementKeyList({ reloadKey = 0 }) {
  const { t, i18n } = useTranslation();
  const [keys, setKeys] = useState([]);
  // When the list was loaded (Unix seconds), to tell expired keys apart.
  const [loadedAt, setLoadedAt] = useState(0);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const rtf = useMemo(
    () => new Intl.RelativeTimeFormat((i18n.language || 'en_US').replace('_', '-'), { numeric: 'auto' }),
    [i18n.language]
  );

  useEffect(() => {
    let cancelled = false;
    API.get('/api/management_key/')
      .then((res) => {
        if (cancelled || !res.data?.success) return;
        setKeys(res.data.data || []);
        setLoadedAt(Math.floor(Date.now() / 1000));
      })
      .catch(() => {
        // surfaced globally
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  const confirmDelete = async () => {
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      const res = await API.delete(`/api/management_key/${deleteTarget.id}`);
      if (res.data?.success) {
        toast.success(t('common.operationSuccess'));
        setKeys((prev) => prev.filter((k) => k.id !== deleteTarget.id));
        setDeleteTarget(null);
      } else showError(res.data?.message);
    } catch {
      // surfaced globally
    } finally {
      setDeleting(false);
    }
  };

  if (keys.length === 0) return null;
  const nowSec = loadedAt;

  return (
    <Card data-testid="management-keys">
      <div className="border-b border-border px-4 py-3">
        <h2 className="text-sm font-semibold">{t('apiKey.management.title')}</h2>
        <p className="mt-0.5 text-xs text-muted-foreground">{t('apiKey.management.hint')}</p>
      </div>
      <ul className="divide-y divide-border">
        {keys.map((key) => {
          const expired = key.expired_time !== -1 && key.expired_time <= nowSec;
          const expiry =
            key.expired_time === -1
              ? t('apiKey.management.never')
              : expired
                ? t('apiKey.management.expired')
                : t('apiKey.management.expires', { when: relativeTime(rtf, key.expired_time) });
          const lastUse =
            key.accessed_time > 0
              ? t('apiKey.management.lastUsed', { when: relativeTime(rtf, key.accessed_time) })
              : t('apiKey.management.neverUsed');
          return (
            <li key={key.id} className="flex items-center gap-3 px-4 py-2.5" data-testid="management-key-row">
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-2">
                  <span className="min-w-0 truncate text-sm font-medium">{key.name}</span>
                  <Badge
                    variant={key.scope === 'read' || key.scope === 'write' ? 'secondary' : 'destructive'}
                    className="shrink-0 px-1.5 py-0 text-[11px] font-normal"
                    data-testid="management-key-type"
                  >
                    {t(managementKeyTypeLabel(key.scope))}
                  </Badge>
                </div>
                <div className="mt-0.5 flex min-w-0 flex-wrap items-baseline gap-x-2 text-xs text-muted-foreground">
                  <span className="font-mono">{key.key_hint}</span>
                  <span aria-hidden="true">·</span>
                  <span className={cn(expired && 'text-destructive')}>{expiry}</span>
                  <span aria-hidden="true">·</span>
                  <span>{lastUse}</span>
                </div>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="size-8 shrink-0 text-muted-foreground hover:text-destructive"
                aria-label={`${t('common.delete')} ${key.name}`}
                onClick={() => setDeleteTarget(key)}
              >
                <Trash2 className="size-4" />
              </Button>
            </li>
          );
        })}
      </ul>

      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('apiKey.management.deleteTitle')}</DialogTitle>
            <DialogDescription>{t('apiKey.management.deleteConfirm', { name: deleteTarget?.name || '' })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t('token_index.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmDelete} disabled={deleting}>
              {t('token_index.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

ManagementKeyList.propTypes = { reloadKey: PropTypes.number };
